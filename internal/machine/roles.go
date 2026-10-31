package machine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/PoeAudits/overlord/internal/catalog"
	"github.com/PoeAudits/overlord/internal/harness"
)

func syncRoles(run syncRun) ([]Result, error) {
	if _, err := os.Stat(filepath.Join(run.agentsDir, "roles.yaml")); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	roles, err := catalog.LoadRoles(run.agentsDir)
	if err != nil {
		return nil, err
	}
	agentDirs := globalAgentDirs(run.sys.Home)
	var results []Result
	for _, role := range roles {
		for _, file := range harness.RenderRole(role, DisplayPath(run.sys.Home, filepath.Join(run.agentsDir, "roles"))) {
			result, err := run.writeGeneratedFile(filepath.Join(agentDirs[file.Harness], file.Name), file.Content, harness.IsGenerated, "hand-written agent file")
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}
	main := slices.IndexFunc(roles, func(role catalog.Role) bool { return role.Name == catalog.MainRole })
	if main < 0 {
		return results, nil
	}
	models, err := run.writeMainModels(roles[main].Models)
	if err != nil {
		return nil, err
	}
	return append(results, models...), nil
}

// globalAgentDirs is where each Harness reads global agent files.
func globalAgentDirs(home string) map[harness.Harness]string {
	return map[harness.Harness]string{
		harness.Claude:   filepath.Join(home, ".claude", "agents"),
		harness.Codex:    filepath.Join(home, ".codex", "agents"),
		harness.OpenCode: filepath.Join(home, ".config", "opencode", "agents"),
	}
}

// writeMainModels sets the main Model for Claude Code and Codex. OpenCode's is
// not written: T3 Code picks the Model per thread.
func (run syncRun) writeMainModels(models catalog.Models) ([]Result, error) {
	claude, err := run.editFile(filepath.Join(run.sys.Home, ".claude", "settings.json"), "main Model", func(settings []byte) ([]byte, error) {
		return harness.SetClaudeModel(settings, models.Claude)
	})
	if err != nil {
		return nil, err
	}
	codex, err := run.editFile(filepath.Join(run.sys.Home, ".codex", "config.toml"), "main Model", func(config []byte) ([]byte, error) {
		return harness.SetCodexModel(config, models.Codex), nil
	})
	if err != nil {
		return nil, err
	}
	return []Result{claude, codex}, nil
}

// writeGeneratedFile writes a file Overlord generates unless a file it did
// not generate is already there; generated says which is which and
// handWritten names such a file in the report.
func (run syncRun) writeGeneratedFile(path string, content []byte, generated func([]byte) bool, handWritten string) (Result, error) {
	current, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return Result{Path: path, Action: Create}, run.writeFile(path, content)
	case err != nil:
		return Result{}, err
	case bytes.Equal(current, content):
		return Result{Path: path, Action: OK}, nil
	case !generated(current):
		return Result{Path: path, Action: Conflict, Detail: handWritten + "; left in place"}, nil
	}
	return Result{Path: path, Action: Replace}, run.writeFile(path, content)
}

// writeFile writes content to path, creating its directory, unless this is a
// check.
func (run syncRun) writeFile(path string, content []byte) error {
	if run.check {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

// editFile applies edit to the content of path (empty when absent) and writes
// the result when it differs. detail names what was edited.
func (run syncRun) editFile(path, detail string, edit func([]byte) ([]byte, error)) (Result, error) {
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	action := Update
	if err != nil {
		action = Create
	}
	content, err := edit(current)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", path, err)
	}
	if bytes.Equal(content, current) {
		return Result{Path: path, Action: OK, Detail: detail}, nil
	}
	return Result{Path: path, Action: action, Detail: detail}, run.writeFile(path, content)
}

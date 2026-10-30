package t3

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/PoeAudits/overlord/internal/system"
)

// ProjectsWithoutSetupScript returns the ids, sorted, of the projects in the
// settings file under home whose own defaultProjectScripts lack the setup
// script. T3 uses such an override instead of the global list, so Setup does
// not run in those projects' worktrees.
func ProjectsWithoutSetupScript(home string) ([]string, error) {
	content, err := os.ReadFile(SettingsPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var settings struct {
		Overrides map[string]struct {
			// A nil Scripts means the project has no scripts override.
			Scripts []scriptID `json:"defaultProjectScripts"`
		} `json:"projectSettingsOverrides"`
	}
	if err := json.Unmarshal(content, &settings); err != nil {
		return nil, fmt.Errorf("%s: %w", SettingsPath(home), err)
	}
	var ids []string
	for id, override := range settings.Overrides {
		if override.Scripts == nil {
			continue
		}
		if !slices.Contains(override.Scripts, scriptID{ID: SetupScriptID}) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

type scriptID struct {
	ID string `json:"id"`
}

// Project is a project T3 Code knows.
type Project struct {
	ID    string `json:"project_id"`
	Title string `json:"title"`
	Root  string `json:"workspace_root"`
}

// Projects reads T3's projects from its state database with the sqlite3 CLI,
// read-only. T3 has no command that lists projects.
func Projects(ctx context.Context, runner system.Runner, home string) ([]Project, error) {
	database := filepath.Join(home, ".t3", "userdata", "statev2.sqlite")
	output, err := runner.Run(ctx, "", "sqlite3", "-readonly", "-json", database,
		"SELECT project_id, title, workspace_root FROM projection_projects WHERE deleted_at IS NULL")
	if err != nil {
		return nil, fmt.Errorf("read T3 projects: %w: %s", err, output)
	}
	// sqlite3 prints nothing at all for no rows.
	if len(bytes.TrimSpace(output)) == 0 {
		return nil, nil
	}
	var projects []Project
	return projects, json.Unmarshal(output, &projects)
}

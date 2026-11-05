package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/system"
)

// NewProject says which Project to create and where.
type NewProject struct {
	Name        string
	Owner       string
	Root        string
	Description string
	Public      bool
	// Language is go, typescript, python or rust. Create writes that
	// language's marker file into the clone so Setup detects it.
	Language string
	// Launcher is the t3 command that registers the clone with T3 Code.
	Launcher string
	// Sleep pauses between label lookups while GitHub finishes creating the
	// repository.
	Sleep func(time.Duration)
}

// Create creates the GitHub repository Owner/Name, clones it into Root/Name,
// writes the Language's marker file, runs Setup there, commits and pushes the
// result, and registers the clone with T3 Code. It stops at the first step
// that fails, and Setup fails when any item failed or the triage labels could
// not be set up, so nothing incomplete is committed. The plan is Setup's,
// empty when Setup did not run.
func Create(ctx context.Context, runner system.Runner, p NewProject) (Plan, error) {
	if p.Name != filepath.Base(p.Name) || p.Name == "." || p.Name == ".." {
		return Plan{}, fmt.Errorf("project name %q must be a single directory name", p.Name)
	}
	markers := markerFiles(p)
	if markers == nil {
		return Plan{}, fmt.Errorf("unknown language %q; use go, typescript, python or rust", p.Language)
	}
	path := filepath.Join(p.Root, p.Name)
	if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
		return Plan{}, fmt.Errorf("%s already exists; choose another name or remove it", path)
	}
	if err := os.MkdirAll(p.Root, 0o755); err != nil {
		return Plan{}, err
	}
	repo := p.Owner + "/" + p.Name
	create := []string{"repo", "create", repo, visibility(p.Public)}
	if p.Description != "" {
		create = append(create, "--description", p.Description)
	}
	if err := step(ctx, runner, p.Root, "gh", create...); err != nil {
		return Plan{}, err
	}
	if err := step(ctx, runner, p.Root, "gh", "repo", "clone", repo, path); err != nil {
		return Plan{}, err
	}
	for name, content := range markers {
		file := filepath.Join(path, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			return Plan{}, err
		}
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			return Plan{}, err
		}
	}
	plan, err := Apply(ctx, runner, p.Sleep, path, path, p.Owner)
	if err != nil {
		return plan, fmt.Errorf("setup in %s: %w", path, err)
	}
	if plan.HasFailures() {
		return plan, fmt.Errorf("setup in %s could not set up every item; see the failed lines above", path)
	}
	// The clone tracks nothing yet, so only the triage labels can warn, and
	// the origin was created above: a warning here means the new repository
	// has no triage labels.
	if i := slices.IndexFunc(plan.Items, func(item Item) bool { return item.Status == Warning }); i >= 0 {
		return plan, fmt.Errorf("setup in %s could not set up the triage labels: %s", path, plan.Items[i].Detail)
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", "Initial project setup"},
		{"push", "-u", "origin", "HEAD"},
	} {
		if err := step(ctx, runner, path, "git", args...); err != nil {
			return plan, err
		}
	}
	return plan, step(ctx, runner, path, p.Launcher, "project", "add", "--title", p.Name, path)
}

// markerFiles returns the smallest files that make the Language a valid
// project of its kind, keyed by path in the clone, or nil for an unknown
// Language. Their marker is what Setup's language detection looks for.
func markerFiles(p NewProject) map[string]string {
	switch p.Language {
	case "go":
		return map[string]string{"go.mod": fmt.Sprintf("module github.com/%s/%s\n\ngo %s\n", p.Owner, p.Name, strings.TrimPrefix(runtime.Version(), "go"))}
	case "typescript":
		// npm rejects package names with capitals.
		return map[string]string{"package.json": fmt.Sprintf("{\n  \"name\": %q,\n  \"private\": true\n}\n", strings.ToLower(p.Name))}
	case "python":
		return map[string]string{"pyproject.toml": fmt.Sprintf("[project]\nname = %q\nversion = \"0.1.0\"\n", p.Name)}
	case "rust":
		// Cargo refuses a manifest without a target, so an empty library
		// comes with it.
		return map[string]string{
			"Cargo.toml": fmt.Sprintf("[package]\nname = %q\nversion = \"0.1.0\"\nedition = \"2024\"\n", p.Name),
			"src/lib.rs": "",
		}
	}
	return nil
}

func visibility(public bool) string {
	if public {
		return "--public"
	}
	return "--private"
}

// step runs one external command and turns a failure into an error naming
// the command and why it failed.
func step(ctx context.Context, runner system.Runner, dir, name string, args ...string) error {
	output, err := runner.Run(ctx, dir, name, args...)
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s failed: %s", strings.Join(append([]string{filepath.Base(name)}, args...), " "), failure(output, err))
}

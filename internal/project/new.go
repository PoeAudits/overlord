package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/system"
)

// NewProject says which Project to create and where.
type NewProject struct {
	Name        string
	Owner       string
	Root        string
	Description string
	Public      bool
	// Launcher is the t3 command that registers the clone with T3 Code.
	Launcher string
}

// Create creates the GitHub repository Owner/Name, clones it into Root/Name,
// runs Setup there, commits and pushes the result, and registers the clone
// with T3 Code. It stops at the first step that fails, and Setup fails when
// any item failed, so nothing incomplete is committed. The plan is Setup's,
// empty when Setup did not run.
func Create(ctx context.Context, runner system.Runner, p NewProject) (Plan, error) {
	if p.Name != filepath.Base(p.Name) || p.Name == "." || p.Name == ".." {
		return Plan{}, fmt.Errorf("project name %q must be a single directory name", p.Name)
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
	plan, err := Apply(ctx, runner, path, path)
	if err != nil {
		return plan, fmt.Errorf("setup in %s: %w", path, err)
	}
	if plan.HasFailures() {
		return plan, fmt.Errorf("setup in %s could not set up every item; see the failed lines above", path)
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

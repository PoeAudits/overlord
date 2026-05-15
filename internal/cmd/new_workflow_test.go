package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
)

func TestRunNew_CreatesProjectAndRegistersIt(t *testing.T) {
	origOpts := newOpts
	origLoadRegistry := newLoadRegistry
	origSaveRegistry := newSaveRegistry
	t.Cleanup(func() {
		newOpts = origOpts
		newLoadRegistry = origLoadRegistry
		newSaveRegistry = origSaveRegistry
		*lifecycle.DefaultBus = *lifecycle.NewBus()
	})

	baseDir := t.TempDir()
	reg := testRegistry(map[string]registry.Project{})
	reg.Settings.BaseDir = baseDir

	newOpts = newFlags{
		category:    string(registry.CategoryServices),
		description: "API service",
		aliases:     []string{"api"},
		noGit:       true,
		noOpen:      true,
		langGo:      true,
	}
	newLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
	saved := false
	newSaveRegistry = func(got *registry.Registry) error {
		saved = true
		return nil
	}

	if err := runNew(&cobra.Command{}, []string{"my-api"}); err != nil {
		t.Fatalf("runNew() returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected registry save to be called")
	}

	project, ok := reg.Projects["my-api"]
	if !ok {
		t.Fatal("expected project to be added to registry")
	}
	if project.Path != filepath.Join(registry.CategoryServices.Path(), "my-api") {
		t.Fatalf("Path = %q, want %q", project.Path, filepath.Join(registry.CategoryServices.Path(), "my-api"))
	}
	if project.Category != registry.CategoryServices {
		t.Fatalf("Category = %q, want %q", project.Category, registry.CategoryServices)
	}
	if project.Lang != registry.LanguageGo {
		t.Fatalf("Lang = %q, want %q", project.Lang, registry.LanguageGo)
	}
	if project.Description != "API service" {
		t.Fatalf("Description = %q, want %q", project.Description, "API service")
	}
	if len(project.Aliases) != 1 || project.Aliases[0] != "api" {
		t.Fatalf("Aliases = %v, want [api]", project.Aliases)
	}
	if project.Status.State != registry.StateActive {
		t.Fatalf("State = %q, want %q", project.Status.State, registry.StateActive)
	}
	if project.Created != time.Now().Format("2006-01-02") {
		t.Fatalf("Created = %q, want today's date", project.Created)
	}

	projectPath := filepath.Join(baseDir, registry.CategoryServices.Path(), "my-api")
	for _, rel := range []string{"README.md", "Makefile", "AGENTS.md", "opencode.jsonc", ".tmux.local"} {
		if _, err := os.Stat(filepath.Join(projectPath, rel)); err != nil {
			t.Fatalf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestRunNew_CleansUpProjectDirectoryWhenSaveFails(t *testing.T) {
	origOpts := newOpts
	origLoadRegistry := newLoadRegistry
	origSaveRegistry := newSaveRegistry
	t.Cleanup(func() {
		newOpts = origOpts
		newLoadRegistry = origLoadRegistry
		newSaveRegistry = origSaveRegistry
		*lifecycle.DefaultBus = *lifecycle.NewBus()
	})

	baseDir := t.TempDir()
	reg := testRegistry(map[string]registry.Project{})
	reg.Settings.BaseDir = baseDir

	newOpts = newFlags{
		category: string(registry.CategoryCLI),
		noGit:    true,
		noOpen:   true,
	}
	newLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
	newSaveRegistry = func(*registry.Registry) error {
		return errors.New("save exploded")
	}

	err := runNew(&cobra.Command{}, []string{"cleanup-me"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "save exploded" {
		t.Fatalf("unexpected error: %v", err)
	}

	projectPath := filepath.Join(baseDir, registry.CategoryCLI.Path(), "cleanup-me")
	if _, statErr := os.Stat(projectPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected project directory cleanup, stat err = %v", statErr)
	}
}

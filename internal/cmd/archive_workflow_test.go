package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
)

func TestRunArchive_MovesDirectoryAndUpdatesRegistry(t *testing.T) {
	origOpts := archiveOpts
	origLoadRegistry := archiveLoadRegistry
	origSaveRegistry := archiveSaveRegistry
	t.Cleanup(func() {
		archiveOpts = origOpts
		archiveLoadRegistry = origLoadRegistry
		archiveSaveRegistry = origSaveRegistry
		*lifecycle.DefaultBus = *lifecycle.NewBus()
	})

	baseDir := t.TempDir()
	projectPath := filepath.Join(baseDir, registry.CategoryServices.Path(), "api")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("failed to create project directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to seed project file: %v", err)
	}

	reg := testRegistry(map[string]registry.Project{
		"api": testProject(filepath.Join(registry.CategoryServices.Path(), "api"), registry.CategoryServices, registry.LanguageGo, registry.StateActive),
	})
	reg.Settings.BaseDir = baseDir

	archiveOpts = archiveFlags{noConfirm: true}
	archiveLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
	saved := false
	archiveSaveRegistry = func(*registry.Registry) error {
		saved = true
		return nil
	}

	if err := runArchive(&cobra.Command{}, []string{"api"}); err != nil {
		t.Fatalf("runArchive() returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected registry save to be called")
	}

	project := reg.Projects["api"]
	if project.Status.State != registry.StateArchived {
		t.Fatalf("State = %q, want %q", project.Status.State, registry.StateArchived)
	}
	if project.Path != "archive/api" {
		t.Fatalf("Path = %q, want %q", project.Path, "archive/api")
	}
	if _, err := os.Stat(projectPath); !os.IsNotExist(err) {
		t.Fatalf("expected source directory to be moved, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "archive", "api", "README.md")); err != nil {
		t.Fatalf("expected archived project file to exist: %v", err)
	}
}

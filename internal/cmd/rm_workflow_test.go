package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
)

func TestRunRm_RemovesResolvedProjectsFromRegistry(t *testing.T) {
	origOpts := rmOpts
	origLoadRegistry := rmLoadRegistry
	origSaveRegistry := rmSaveRegistry
	t.Cleanup(func() {
		rmOpts = origOpts
		rmLoadRegistry = origLoadRegistry
		rmSaveRegistry = origSaveRegistry
		*lifecycle.DefaultBus = *lifecycle.NewBus()
	})

	reg := testRegistry(map[string]registry.Project{
		"alpha": testProject(filepath.Join(registry.CategoryCLI.Path(), "alpha"), registry.CategoryCLI, registry.LanguageGo, registry.StateActive),
		"beta":  testProject(filepath.Join(registry.CategoryServices.Path(), "beta"), registry.CategoryServices, registry.LanguageGo, registry.StateActive),
	})
	reg.Settings.BaseDir = t.TempDir()

	rmOpts = rmFlags{force: false}
	rmLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
	saved := false
	rmSaveRegistry = func(*registry.Registry) error {
		saved = true
		return nil
	}

	if err := runRm(&cobra.Command{}, []string{"alpha"}); err != nil {
		t.Fatalf("runRm() returned error: %v", err)
	}
	if !saved {
		t.Fatal("expected registry save to be called")
	}
	if _, ok := reg.Projects["alpha"]; ok {
		t.Fatal("expected alpha to be removed from registry")
	}
	if _, ok := reg.Projects["beta"]; !ok {
		t.Fatal("expected beta to remain in registry")
	}
}

func TestRunRm_ForceSkipDoesNotRemoveProject(t *testing.T) {
	origOpts := rmOpts
	origLoadRegistry := rmLoadRegistry
	origSaveRegistry := rmSaveRegistry
	origConfirmDelete := rmConfirmDelete
	origRemoveAll := rmRemoveAll
	t.Cleanup(func() {
		rmOpts = origOpts
		rmLoadRegistry = origLoadRegistry
		rmSaveRegistry = origSaveRegistry
		rmConfirmDelete = origConfirmDelete
		rmRemoveAll = origRemoveAll
		*lifecycle.DefaultBus = *lifecycle.NewBus()
	})

	baseDir := t.TempDir()
	projectPath := filepath.Join(baseDir, registry.CategoryCLI.Path(), "alpha")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("failed to create project directory: %v", err)
	}

	reg := testRegistry(map[string]registry.Project{
		"alpha": testProject(filepath.Join(registry.CategoryCLI.Path(), "alpha"), registry.CategoryCLI, registry.LanguageGo, registry.StateActive),
	})
	reg.Settings.BaseDir = baseDir

	rmOpts = rmFlags{force: true}
	rmLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
	saved := false
	rmSaveRegistry = func(*registry.Registry) error {
		saved = true
		return nil
	}
	confirmed := 0
	rmConfirmDelete = func(projectName, projectPath string) (bool, error) {
		confirmed++
		return false, nil
	}
	removed := false
	rmRemoveAll = func(path string) error {
		removed = true
		return nil
	}

	if err := runRm(&cobra.Command{}, []string{"alpha"}); err != nil {
		t.Fatalf("runRm() returned error: %v", err)
	}
	if confirmed != 1 {
		t.Fatalf("confirmations = %d, want 1", confirmed)
	}
	if saved {
		t.Fatal("expected registry save to be skipped when all removals are declined")
	}
	if removed {
		t.Fatal("expected filesystem deletion not to run when removal is declined")
	}
	if _, ok := reg.Projects["alpha"]; !ok {
		t.Fatal("expected alpha to remain in registry when force removal is declined")
	}
	if _, err := os.Stat(projectPath); err != nil {
		t.Fatalf("expected project directory to remain: %v", err)
	}
}

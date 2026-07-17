package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/tmux"
)

func withSwitchDependencies(t *testing.T, reg *registry.Registry, openWindow func(string, string) error) {
	t.Helper()
	originalLoad := switchLoadRegistry
	originalOpen := switchOpenWindow
	t.Cleanup(func() {
		switchLoadRegistry = originalLoad
		switchOpenWindow = originalOpen
	})
	switchLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
	switchOpenWindow = openWindow
}

func TestRunSwitch_OpensCanonicalProjectWindowForAlias(t *testing.T) {
	projectDir := t.TempDir()
	reg := testRegistry(map[string]registry.Project{
		"agent-bench": testProject(projectDir, registry.CategoryCoreEvals, registry.LanguagePython, registry.StateActive, "agentbench"),
	})
	var gotName, gotDir string
	withSwitchDependencies(t, reg, func(name, dir string) error {
		gotName, gotDir = name, dir
		return nil
	})

	if err := runSwitch(&cobra.Command{}, []string{"agentbench"}); err != nil {
		t.Fatalf("runSwitch() returned error: %v", err)
	}
	if gotName != "agent-bench" || gotDir != projectDir {
		t.Fatalf("opened window (%q, %q), want (%q, %q)", gotName, gotDir, "agent-bench", projectDir)
	}
}

func TestRunSwitch_RequiresTmuxSession(t *testing.T) {
	projectDir := t.TempDir()
	reg := testRegistry(map[string]registry.Project{
		"container": testProject(projectDir, registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive),
	})
	withSwitchDependencies(t, reg, func(string, string) error {
		return tmux.ErrNotInSession
	})

	err := runSwitch(&cobra.Command{}, []string{"container"})
	if err == nil || !strings.Contains(err.Error(), "[error] switch: not inside a tmux session") || !strings.Contains(err.Error(), "overlord open <name>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSwitch_RejectsMissingProjectDirectory(t *testing.T) {
	projectDir := filepath.Join(t.TempDir(), "missing")
	reg := testRegistry(map[string]registry.Project{
		"missing": testProject(projectDir, registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive),
	})
	withSwitchDependencies(t, reg, func(string, string) error {
		t.Fatal("switchOpenWindow called for missing directory")
		return nil
	})

	err := runSwitch(&cobra.Command{}, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "project directory not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSwitch_RejectsProjectPathThatIsNotDirectory(t *testing.T) {
	projectPath := filepath.Join(t.TempDir(), "project.txt")
	if err := os.WriteFile(projectPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write project file: %v", err)
	}
	reg := testRegistry(map[string]registry.Project{
		"file": testProject(projectPath, registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive),
	})
	withSwitchDependencies(t, reg, func(string, string) error {
		t.Fatal("switchOpenWindow called for non-directory path")
		return nil
	})

	err := runSwitch(&cobra.Command{}, []string{"file"})
	if err == nil || !strings.Contains(err.Error(), "project path is not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunSwitch_NoMatchReturnsSwitchError(t *testing.T) {
	withSwitchDependencies(t, testRegistry(map[string]registry.Project{}), func(string, string) error {
		return nil
	})

	err := runSwitch(&cobra.Command{}, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "[error] switch: no project found matching 'missing'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

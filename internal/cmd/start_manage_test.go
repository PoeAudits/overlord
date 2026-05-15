package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/stack"
)

func TestRunStartAdd_AddsResolvedProjectsAndSkipsInvalidTargets(t *testing.T) {
	origLoadStackConfig := startManageLoadStackConfig
	origSaveStackConfig := startManageSaveStackConfig
	origLoadRegistry := startManageLoadRegistry
	t.Cleanup(func() {
		startManageLoadStackConfig = origLoadStackConfig
		startManageSaveStackConfig = origSaveStackConfig
		startManageLoadRegistry = origLoadRegistry
	})

	var saved *stack.StackConfig
	startManageLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{
			Version:  1,
			Projects: []string{"alpha"},
			Services: map[string]stack.ServiceConfig{},
		}, nil
	}
	startManageSaveStackConfig = func(cfg *stack.StackConfig) error {
		saved = &stack.StackConfig{
			Version:  cfg.Version,
			Projects: append([]string(nil), cfg.Projects...),
			Services: cfg.Services,
		}
		return nil
	}
	startManageLoadRegistry = func() (*registry.Registry, error) {
		return testRegistry(map[string]registry.Project{
			"alpha": testProject("projects/cli/alpha", registry.CategoryCLI, registry.LanguageGo, registry.StateActive),
			"beta":  testProject("projects/services/beta", registry.CategoryServices, registry.LanguageGo, registry.StateActive, "bee"),
			"gamma": testProject("archive/gamma", registry.CategoryCLI, registry.LanguageGo, registry.StateArchived),
		}), nil
	}

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := runStartAdd(cmd, []string{"bee", "gamma", "missing", "alpha"}); err != nil {
		t.Fatalf("runStartAdd() returned error: %v", err)
	}

	if saved == nil {
		t.Fatal("expected stack config to be saved")
	}
	if got := strings.Join(saved.Projects, ","); got != "alpha,beta" {
		t.Fatalf("saved projects = %q, want %q", got, "alpha,beta")
	}

	out := stderr.String()
	for _, want := range []string{
		"Added:",
		"beta",
		"project is archived",
		"could not resolve project \"missing\"",
		"already in the startup list",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRunStartRemove_RemovesExistingProjectsAndSkipsMissing(t *testing.T) {
	origLoadStackConfig := startManageLoadStackConfig
	origSaveStackConfig := startManageSaveStackConfig
	t.Cleanup(func() {
		startManageLoadStackConfig = origLoadStackConfig
		startManageSaveStackConfig = origSaveStackConfig
	})

	var saved *stack.StackConfig
	startManageLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{
			Version:  1,
			Projects: []string{"alpha", "beta", "gamma"},
			Services: map[string]stack.ServiceConfig{},
		}, nil
	}
	startManageSaveStackConfig = func(cfg *stack.StackConfig) error {
		saved = &stack.StackConfig{
			Version:  cfg.Version,
			Projects: append([]string(nil), cfg.Projects...),
			Services: cfg.Services,
		}
		return nil
	}

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := runStartRemove(cmd, []string{"beta", "missing", "gamma"}); err != nil {
		t.Fatalf("runStartRemove() returned error: %v", err)
	}

	if saved == nil {
		t.Fatal("expected stack config to be saved")
	}
	if got := strings.Join(saved.Projects, ","); got != "alpha" {
		t.Fatalf("saved projects = %q, want %q", got, "alpha")
	}

	out := stderr.String()
	for _, want := range []string{"Removed:", "beta", "gamma", "not in the startup list"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

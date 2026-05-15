package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestOpenResolvedProject_ArchivedProjectReturnsRestoreHint(t *testing.T) {
	err := openResolvedProject("archive-me", testProject("archive/archive-me", registry.CategoryCLI, registry.LanguageGo, registry.StateArchived), testRegistry(nil))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "'archive-me' is archived") {
		t.Fatalf("error missing archived message: %v", err)
	}
	if !strings.Contains(err.Error(), "Restore it first: overlord unarchive archive-me --category=cli") {
		t.Fatalf("error missing restore hint: %v", err)
	}
}

func TestOpenMultipleProjects_EnsuresBackgroundSessionsAndOpensLastProject(t *testing.T) {
	origEnsureProject := openEnsureProject
	origOpenProject := openOpenProject
	origSaveRegistry := openSaveRegistry
	origNow := openNow
	t.Cleanup(func() {
		openEnsureProject = origEnsureProject
		openOpenProject = origOpenProject
		openSaveRegistry = origSaveRegistry
		openNow = origNow
	})

	reg := testRegistry(map[string]registry.Project{
		"alpha": testProject("projects/cli/alpha", registry.CategoryCLI, registry.LanguageGo, registry.StateActive),
		"beta":  testProject("projects/services/beta", registry.CategoryServices, registry.LanguageGo, registry.StateActive),
	})
	fixedTime := time.Date(2026, time.April, 25, 12, 0, 0, 0, time.UTC)
	openNow = func() time.Time { return fixedTime }

	var background []string
	foreground := ""
	saveCalls := 0
	openEnsureProject = func(name string, project registry.Project, reg *registry.Registry) error {
		background = append(background, name)
		return nil
	}
	openOpenProject = func(name string, project registry.Project, reg *registry.Registry) error {
		foreground = name
		return nil
	}
	openSaveRegistry = func(reg *registry.Registry) error {
		saveCalls++
		return nil
	}

	err := openMultipleProjects(&cobra.Command{}, reg, []string{"alpha", "beta"})
	if err != nil {
		t.Fatalf("openMultipleProjects() returned error: %v", err)
	}
	if len(background) != 1 || background[0] != "alpha" {
		t.Fatalf("background sessions = %v, want [alpha]", background)
	}
	if foreground != "beta" {
		t.Fatalf("foreground project = %q, want %q", foreground, "beta")
	}
	if saveCalls != 2 {
		t.Fatalf("save calls = %d, want 2", saveCalls)
	}
	for _, name := range []string{"alpha", "beta"} {
		if reg.Projects[name].LastOpened != fixedTime.Format(time.RFC3339) {
			t.Fatalf("%s last_opened = %q, want %q", name, reg.Projects[name].LastOpened, fixedTime.Format(time.RFC3339))
		}
	}
}

func TestOpenMultipleProjects_RejectsArchivedProject(t *testing.T) {
	origEnsureProject := openEnsureProject
	origOpenProject := openOpenProject
	t.Cleanup(func() {
		openEnsureProject = origEnsureProject
		openOpenProject = origOpenProject
	})

	ensured := false
	opened := false
	openEnsureProject = func(name string, project registry.Project, reg *registry.Registry) error {
		ensured = true
		return nil
	}
	openOpenProject = func(name string, project registry.Project, reg *registry.Registry) error {
		opened = true
		return nil
	}

	reg := testRegistry(map[string]registry.Project{
		"alpha": testProject("projects/cli/alpha", registry.CategoryCLI, registry.LanguageGo, registry.StateActive),
		"beta":  testProject("archive/beta", registry.CategoryCLI, registry.LanguageGo, registry.StateArchived),
	})

	err := openMultipleProjects(&cobra.Command{}, reg, []string{"alpha", "beta"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "archived projects cannot be opened: beta") {
		t.Fatalf("unexpected error: %v", err)
	}
	if ensured || opened {
		t.Fatalf("expected no session work when archived project is present; ensured=%v opened=%v", ensured, opened)
	}
}

func TestShowPicker_NoActiveProjectsReturnsHelpfulError(t *testing.T) {
	origIsTerminal := openIsTerminal
	t.Cleanup(func() {
		openIsTerminal = origIsTerminal
	})

	openIsTerminal = func(int) bool { return true }

	_, _, err := showPicker(testRegistry(map[string]registry.Project{}), "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no active projects found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShowPickerWithResults_NonTerminalReturnsAmbiguousError(t *testing.T) {
	origIsTerminal := openIsTerminal
	t.Cleanup(func() {
		openIsTerminal = origIsTerminal
	})

	openIsTerminal = func(int) bool { return false }

	results := []registry.ResolveResult{
		{Name: "alpha", Project: testProject("projects/cli/alpha", registry.CategoryCLI, registry.LanguageGo, registry.StateActive)},
		{Name: "beta", Project: testProject("projects/services/beta", registry.CategoryServices, registry.LanguageGo, registry.StateActive)},
	}

	_, _, err := showPickerWithResults(results)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "multiple projects match: alpha, beta") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "Provide a more specific name") {
		t.Fatalf("error missing suggestion: %v", err)
	}
}

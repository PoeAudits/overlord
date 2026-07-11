package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

func withPathRegistry(t *testing.T, reg *registry.Registry) {
	t.Helper()
	orig := pathLoadRegistry
	t.Cleanup(func() {
		pathLoadRegistry = orig
	})
	pathLoadRegistry = func() (*registry.Registry, error) {
		return reg, nil
	}
}

func withPathStdout() (*cobra.Command, *bytes.Buffer) {
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	return cmd, &stdout
}

func TestRunPath_PrintsExpandedPathForProjectName(t *testing.T) {
	reg := testRegistry(map[string]registry.Project{
		"overlord": testProject("core/tools/overlord", registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive),
	})
	reg.Settings.BaseDir = "/home/thomas/Overlord"
	withPathRegistry(t, reg)
	cmd, stdout := withPathStdout()

	if err := runPath(cmd, []string{"overlord"}); err != nil {
		t.Fatalf("runPath() returned error: %v", err)
	}
	if got := stdout.String(); got != "/home/thomas/Overlord/core/tools/overlord\n" {
		t.Fatalf("stdout = %q, want %q", got, "/home/thomas/Overlord/core/tools/overlord\n")
	}
}

func TestRunPath_PrintsExpandedPathForAlias(t *testing.T) {
	reg := testRegistry(map[string]registry.Project{
		"overlord": testProject("core/tools/overlord", registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive, "ol"),
	})
	reg.Settings.BaseDir = "/home/thomas/Overlord"
	withPathRegistry(t, reg)
	cmd, stdout := withPathStdout()

	if err := runPath(cmd, []string{"ol"}); err != nil {
		t.Fatalf("runPath() returned error: %v", err)
	}
	if got := stdout.String(); got != "/home/thomas/Overlord/core/tools/overlord\n" {
		t.Fatalf("stdout = %q, want %q", got, "/home/thomas/Overlord/core/tools/overlord\n")
	}
}

func TestRunPath_PrintsExpandedPathsForMultipleArgs(t *testing.T) {
	reg := testRegistry(map[string]registry.Project{
		"overlord": testProject("core/tools/overlord", registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive),
		"opencode": testProject("core/tools/opencode", registry.CategoryCoreTools, registry.LanguageTypeScript, registry.StateActive, "oc"),
	})
	reg.Settings.BaseDir = "/home/thomas/Overlord"
	withPathRegistry(t, reg)
	cmd, stdout := withPathStdout()

	if err := runPath(cmd, []string{"overlord", "oc"}); err != nil {
		t.Fatalf("runPath() returned error: %v", err)
	}
	want := "/home/thomas/Overlord/core/tools/overlord\n/home/thomas/Overlord/core/tools/opencode\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunPath_NoMatchReturnsPathError(t *testing.T) {
	withPathRegistry(t, testRegistry(map[string]registry.Project{}))

	err := runPath(&cobra.Command{}, []string{"missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "[error] path: no project found matching 'missing'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPath_CancelledPickerPrintsNothing(t *testing.T) {
	origShowPicker := pathShowPicker
	t.Cleanup(func() {
		pathShowPicker = origShowPicker
	})

	withPathRegistry(t, testRegistry(map[string]registry.Project{}))
	pathShowPicker = func(reg *registry.Registry, initialQuery string) (string, registry.Project, error) {
		return "", registry.Project{}, nil
	}

	cmd, stdout := withPathStdout()
	if err := runPath(cmd, nil); err != nil {
		t.Fatalf("runPath() returned error: %v", err)
	}
	if got := stdout.String(); got != "" {
		t.Fatalf("stdout = %q, want empty", got)
	}
}

func TestRunPath_ExpandFailureReturnsPathError(t *testing.T) {
	reg := testRegistry(map[string]registry.Project{
		"overlord": testProject("core/tools/overlord", registry.CategoryCoreTools, registry.LanguageGo, registry.StateActive),
	})
	reg.Settings.BaseDir = "~other/Overlord"
	withPathRegistry(t, reg)

	err := runPath(&cobra.Command{}, []string{"overlord"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "[error] path: failed to expand path for 'core/tools/overlord'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

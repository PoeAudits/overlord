package cmd_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupWritesOneCIJobPerDetectedLanguage(t *testing.T) {
	tests := []struct {
		name    string
		markers []string
		golden  string
	}{
		{name: "Go", markers: []string{"go.mod"}, golden: "go.yml"},
		{name: "TypeScript", markers: []string{"package.json"}, golden: "typescript.yml"},
		{name: "Python", markers: []string{"pyproject.toml"}, golden: "python.yml"},
		{name: "Rust", markers: []string{"Cargo.toml"}, golden: "rust.yml"},
		{name: "every language at once", markers: []string{"Cargo.toml", "pyproject.toml", "tsconfig.json", "go.mod"}, golden: "all.yml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			for _, marker := range tt.markers {
				mustWrite(t, filepath.Join(project, marker), "")
			}

			stdout, code := run(t, newSystem(t, project), "setup")

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if got, want := readFile(t, filepath.Join(project, ".github", "workflows", "ci.yml")), readFile(t, filepath.Join("testdata", "ci", tt.golden)); got != want {
				t.Fatalf(".github/workflows/ci.yml:\n%s\nwant testdata/ci/%s:\n%s", got, tt.golden, want)
			}
			if !strings.Contains(stdout, "  created    .github/workflows/ci.yml\n") {
				t.Fatalf("stdout:\n%s", stdout)
			}
		})
	}
}

func TestSetupNeverRewritesAnExistingCIWorkflow(t *testing.T) {
	project := t.TempDir()
	mustWrite(t, filepath.Join(project, "go.mod"), "module example\n")
	mustWrite(t, filepath.Join(project, ".github", "workflows", "ci.yml"), "name: Mine\n")

	stdout, code := run(t, newSystem(t, project), "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := readFile(t, filepath.Join(project, ".github", "workflows", "ci.yml")); got != "name: Mine\n" {
		t.Fatalf("ci.yml = %q, want it untouched", got)
	}
	if !strings.Contains(stdout, "  ok         .github/workflows/ci.yml\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

// A workflow with no jobs is invalid, and a create item is never rewritten,
// so the workflow waits until the Project has a language.
func TestSetupWritesNoCIWorkflowUntilALanguageIsDetected(t *testing.T) {
	project := t.TempDir()

	stdout, code := run(t, newSystem(t, project), "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, ok := tree(t, project)[".github/workflows/ci.yml"]; ok || strings.Contains(stdout, "ci.yml") {
		t.Fatalf("ci.yml written for a Project without languages:\n%s", stdout)
	}
}

package cmd_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetupWritesOneStandardsBlockPerDetectedLanguage(t *testing.T) {
	tests := []struct {
		name    string
		markers []string
		golden  string
	}{
		{name: "Go via go.mod", markers: []string{"go.mod"}, golden: "go.md"},
		{name: "TypeScript via package.json", markers: []string{"package.json"}, golden: "typescript.md"},
		{name: "TypeScript via tsconfig.json", markers: []string{"tsconfig.json"}, golden: "typescript.md"},
		{name: "Python via pyproject.toml", markers: []string{"pyproject.toml"}, golden: "python.md"},
		{name: "Python via requirements.txt", markers: []string{"requirements.txt"}, golden: "python.md"},
		{name: "Rust via Cargo.toml", markers: []string{"Cargo.toml"}, golden: "rust.md"},
		{name: "every language at once", markers: []string{"Cargo.toml", "requirements.txt", "tsconfig.json", "go.mod"}, golden: "all.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			for _, marker := range tt.markers {
				mustWrite(t, filepath.Join(project, marker), "")
			}

			_, code := run(t, newSystem(t, project), "setup")

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if got, want := readFile(t, filepath.Join(project, "docs", "coding-standards.md")), readFile(t, filepath.Join("testdata", "standards", tt.golden)); got != want {
				t.Fatalf("docs/coding-standards.md:\n%s\nwant testdata/standards/%s:\n%s", got, tt.golden, want)
			}
		})
	}
}

func TestSetupDetectsLanguagesInT3CodeProjectRootAndWritesTheWorktree(t *testing.T) {
	mainCheckout := t.TempDir()
	mustWrite(t, filepath.Join(mainCheckout, "go.mod"), "module example\n")
	worktree := t.TempDir()
	sys := newSystem(t, worktree)
	sys.Getenv = func(key string) string {
		if key == "T3CODE_PROJECT_ROOT" {
			return mainCheckout
		}
		return ""
	}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := readFile(t, filepath.Join(worktree, "docs", "coding-standards.md")), readFile(t, filepath.Join("testdata", "standards", "go.md")); got != want {
		t.Fatalf("worktree docs/coding-standards.md:\n%s\nwant testdata/standards/go.md", got)
	}
	if !strings.Contains(stdout, "  created    docs/coding-standards.md#lang-go\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(mainCheckout, "docs")); err == nil {
		t.Fatal("setup wrote into T3CODE_PROJECT_ROOT")
	}
}

func TestSetupPrependsTheAgentsBlockAndKeepsTheExistingBodyByteForByte(t *testing.T) {
	project := t.TempDir()
	body := "# My project\n\nHand-written rules.\r\nNo trailing newline"
	mustWrite(t, filepath.Join(project, "AGENTS.md"), body)

	stdout, code := run(t, newSystem(t, project), "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := readFile(t, filepath.Join("testdata", "empty-project", "AGENTS.md")) + "\n" + body
	if got := readFile(t, filepath.Join(project, "AGENTS.md")); got != want {
		t.Fatalf("AGENTS.md:\n%q\nwant:\n%q", got, want)
	}
	if !strings.Contains(stdout, "  created    AGENTS.md#agents\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestSetupReplacesAStaleBlockAndKeepsTheTextAroundIt(t *testing.T) {
	golden := readFile(t, filepath.Join("testdata", "empty-project", "docs", "coding-standards.md"))
	tests := []struct {
		name  string
		stale func(string) string
	}{
		{name: "older version", stale: func(s string) string {
			return strings.Replace(s, "<!-- overlord:begin general v1 -->", "<!-- overlord:begin general v0 -->", 1)
		}},
		{name: "edited content", stale: func(s string) string {
			return strings.Replace(s, "## General\n", "## General\n\n- A rule someone added inside the block.\n", 1)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			before := "# Standards\n\nIntro written by the Project.\n\n"
			after := "\n- Project rule kept after the block.\n"
			path := filepath.Join(project, "docs", "coding-standards.md")
			mustWrite(t, path, before+tt.stale(golden)+after)

			stdout, code := run(t, newSystem(t, project), "setup")

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if got, want := readFile(t, path), before+golden+after; got != want {
				t.Fatalf("docs/coding-standards.md:\n%s\nwant:\n%s", got, want)
			}
			if !strings.Contains(stdout, "  refreshed  docs/coding-standards.md#general\n") {
				t.Fatalf("stdout:\n%s", stdout)
			}
		})
	}
}

func TestSetupAddsANewlyDetectedLanguageWithoutDisturbingTheRest(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	if _, code := run(t, sys, "setup"); code != 0 {
		t.Fatalf("first run exit code = %d", code)
	}
	path := filepath.Join(project, "docs", "coding-standards.md")
	projectRules := "\n- Every handler logs its request id.\n"
	mustWrite(t, path, readFile(t, path)+projectRules)
	mustWrite(t, filepath.Join(project, "go.mod"), "module example\n")

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := readFile(t, path), readFile(t, filepath.Join("testdata", "standards", "go.md"))+projectRules; got != want {
		t.Fatalf("docs/coding-standards.md:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range []string{"  ok         docs/coding-standards.md#general\n", "  created    docs/coding-standards.md#lang-go\n"} {
		if !strings.Contains(stdout, line) {
			t.Fatalf("stdout lacks %q:\n%s", line, stdout)
		}
	}
}

func TestSetupCheckReportsBlocksAsMissingStaleOrOKWithoutWriting(t *testing.T) {
	project := completeProject(t)
	agents := filepath.Join(project, "AGENTS.md")
	staleAgents := strings.Replace(readFile(t, agents), "agents v1", "agents v0", 1)
	mustWrite(t, agents, staleAgents)
	mustWrite(t, filepath.Join(project, "Cargo.toml"), "")
	before := tree(t, project)

	stdout, code := run(t, newSystem(t, project), "setup", "--check")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	for _, line := range []string{
		"  stale      AGENTS.md#agents\n",
		"  ok         docs/coding-standards.md#general\n",
		"  missing    docs/coding-standards.md#lang-rust\n",
	} {
		if !strings.Contains(stdout, line) {
			t.Fatalf("stdout lacks %q:\n%s", line, stdout)
		}
	}
	if strings.Contains(stdout, "lang-go") {
		t.Fatalf("plan lists a language that was not detected:\n%s", stdout)
	}
	if after := tree(t, project); !reflect.DeepEqual(before, after) {
		t.Fatal("setup --check changed the project")
	}
}

func TestSetupLeavesABlockWithoutAnEndMarkerUntouched(t *testing.T) {
	project := t.TempDir()
	broken := "<!-- overlord:begin agents v1 -->\nhalf a block\n"
	mustWrite(t, filepath.Join(project, "AGENTS.md"), broken)

	stdout, code := run(t, newSystem(t, project), "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := readFile(t, filepath.Join(project, "AGENTS.md")); got != broken {
		t.Fatalf("AGENTS.md = %q, want it untouched", got)
	}
	if want := "  conflict   AGENTS.md#agents (begin marker without end marker; left untouched)\n"; !strings.Contains(stdout, want) {
		t.Fatalf("stdout lacks %q:\n%s", want, stdout)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

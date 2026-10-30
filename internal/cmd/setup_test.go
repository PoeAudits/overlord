package cmd_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/cmd"
	"github.com/PoeAudits/overlord/internal/system"
)

func TestSetupCheckOnEmptyProjectListsEveryItemMissingAndWritesNothing(t *testing.T) {
	project := t.TempDir()

	stdout, code := run(t, newSystem(t, project), "setup", "--check")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "Setup plan for " + project + "\n" +
		"  missing    AGENTS.md#agents\n" +
		"  missing    docs/coding-standards.md#general\n" +
		"  missing    GLOSSARY.md\n" +
		"  missing    docs/adr/0001-lint-and-tests-must-pass-before-merge.md\n" +
		"  missing    docs/agents/issue-tracker.md\n" +
		"  missing    docs/agents/triage-labels.md\n" +
		"  missing    docs/agents/domain.md\n" +
		"  missing    docs/adr\n" +
		"  missing    .agents/skills\n" +
		"  missing    .claude/skills\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	entries, err := os.ReadDir(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("setup --check wrote %d entries into the project", len(entries))
	}
}

func TestSetupCheckOnCompleteProjectReportsEveryItemOK(t *testing.T) {
	project := completeProject(t)

	stdout, code := run(t, newSystem(t, project), "setup", "--check")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "Setup plan for " + project + "\n" +
		"  ok         AGENTS.md#agents\n" +
		"  ok         docs/coding-standards.md#general\n" +
		"  ok         GLOSSARY.md\n" +
		"  ok         docs/adr/0001-lint-and-tests-must-pass-before-merge.md\n" +
		"  ok         docs/agents/issue-tracker.md\n" +
		"  ok         docs/agents/triage-labels.md\n" +
		"  ok         docs/agents/domain.md\n" +
		"  ok         docs/adr\n" +
		"  ok         .agents/skills\n" +
		"  ok         .claude/skills\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestSetupCheckJSONEmitsThePlan(t *testing.T) {
	project := t.TempDir()
	mustMkdir(t, filepath.Join(project, "docs", "adr"))
	mustWrite(t, filepath.Join(project, "docs", "adr", ".gitkeep"), "")

	stdout, code := run(t, newSystem(t, project), "setup", "--check", "--json")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var got any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	var want any
	if err := json.Unmarshal([]byte(`{
		"root": "`+project+`",
		"items": [
			{"path": "AGENTS.md", "block": "agents", "strategy": "block", "status": "missing"},
			{"path": "docs/coding-standards.md", "block": "general", "strategy": "block", "status": "missing"},
			{"path": "GLOSSARY.md", "strategy": "create", "status": "missing"},
			{"path": "docs/adr/0001-lint-and-tests-must-pass-before-merge.md", "strategy": "create", "status": "missing"},
			{"path": "docs/agents/issue-tracker.md", "strategy": "create", "status": "missing"},
			{"path": "docs/agents/triage-labels.md", "strategy": "create", "status": "missing"},
			{"path": "docs/agents/domain.md", "strategy": "create", "status": "missing"},
			{"path": "docs/adr", "strategy": "dir", "status": "ok"},
			{"path": ".agents/skills", "strategy": "dir", "status": "missing"},
			{"path": ".claude/skills", "strategy": "symlink", "status": "missing"}
		]
	}`), &want); err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(got, want) {
		t.Fatalf("plan JSON:\n%s", stdout)
	}
}

func TestSetupCheckProjectRoot(t *testing.T) {
	complete := completeProject(t)
	empty := t.TempDir()

	tests := []struct {
		name        string
		envRoot     string
		envWorktree string
		workdir     string
		args        []string
	}{
		{name: "path argument wins over T3 worktree variables", envRoot: empty, envWorktree: empty, workdir: empty, args: []string{complete}},
		{name: "T3CODE_WORKTREE_PATH wins over the working directory", envRoot: empty, envWorktree: complete, workdir: empty},
		{name: "T3CODE_PROJECT_ROOT never picks the target", envRoot: empty, workdir: complete},
		{name: "path argument wins over the working directory", workdir: empty, args: []string{complete}},
		{name: "relative path argument resolves against the working directory", workdir: filepath.Dir(complete), args: []string{filepath.Base(complete)}},
		{name: "working directory is the fallback", workdir: complete},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := newSystem(t, tt.workdir)
			sys.Getenv = func(key string) string {
				switch key {
				case "T3CODE_PROJECT_ROOT":
					return tt.envRoot
				case "T3CODE_WORKTREE_PATH":
					return tt.envWorktree
				}
				return ""
			}

			stdout, code := run(t, sys, append([]string{"setup", "--check"}, tt.args...)...)

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if !strings.HasPrefix(stdout, "Setup plan for "+complete+"\n") || strings.Contains(stdout, "missing") {
				t.Fatalf("checked the wrong root:\n%s", stdout)
			}
		})
	}
}

// completeProject returns a Project directory that already has every item:
// a copy of the golden files for Setup on an empty Project.
func completeProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	if err := os.CopyFS(project, os.DirFS(filepath.Join("testdata", "empty-project"))); err != nil {
		t.Fatal(err)
	}
	return project
}

func jsonEqual(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}

// newSystem returns a System with a temporary HOME, no environment, the given
// working directory and a runner that reports the PoeAudits/sample GitHub
// origin.
func newSystem(t *testing.T, workdir string) system.System {
	t.Helper()
	return system.System{
		Home:    t.TempDir(),
		Getenv:  func(string) string { return "" },
		Workdir: workdir,
		Runner:  &fakeRunner{outputs: map[string]string{"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n"}},
	}
}

func run(t *testing.T, sys system.System, args ...string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	sys.Stdout = &stdout
	sys.Stderr = &stderr
	code := cmd.Execute(sys, args)
	if stderr.Len() > 0 {
		t.Logf("stderr: %s", stderr.String())
	}
	return stdout.String(), code
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

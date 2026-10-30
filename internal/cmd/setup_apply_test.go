package cmd_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSetupOnEmptyProjectCreatesEveryItemFromTheGoldenFiles(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	runner := &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
	}}
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := tree(t, project), tree(t, filepath.Join("testdata", "empty-project")); !reflect.DeepEqual(got, want) {
		t.Fatalf("project files differ from testdata/empty-project:\ngot  %v\nwant %v", keys(got), keys(want))
	}
	want := "Setup applied to " + project + "\n" +
		"  created    AGENTS.md#agents\n" +
		"  created    docs/coding-standards.md#general\n" +
		"  created    GLOSSARY.md\n" +
		"  created    docs/adr/0001-lint-and-tests-must-pass-before-merge.md\n" +
		"  created    docs/agents/issue-tracker.md\n" +
		"  created    docs/agents/triage-labels.md\n" +
		"  created    docs/agents/domain.md\n" +
		"  created    docs/adr\n" +
		"  created    .agents/skills\n" +
		"  created    .claude/skills\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	if want := []string{project + ": git remote get-url origin"}; !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands = %q, want %q", runner.calls, want)
	}
}

// tree maps every regular file under root to its content, and every symlink
// to "-> target", keyed by slash-separated relative path.
func tree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			files[filepath.ToSlash(rel)] = "-> " + target
			return err
		}
		content, err := os.ReadFile(path)
		files[filepath.ToSlash(rel)] = string(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func keys(files map[string]string) []string {
	var names []string
	for name := range files {
		names = append(names, name)
	}
	return names
}

// fakeRunner records every command as "dir: name args" and answers from
// outputs keyed by "name args"; unknown commands fail like a missing remote.
type fakeRunner struct {
	outputs map[string]string
	calls   []string
}

func (r *fakeRunner) Run(_ context.Context, dir, name string, args ...string) ([]byte, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, dir+": "+command)
	output, ok := r.outputs[command]
	if !ok {
		return []byte("error: No such remote 'origin'\n"), errors.New("exit status 2")
	}
	return []byte(output), nil
}

func TestSetupSecondRunChangesNothingAndReportsEveryItemOK(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}
	if _, code := run(t, sys, "setup"); code != 0 {
		t.Fatalf("first run exit code = %d", code)
	}
	before := tree(t, project)

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if after := tree(t, project); !reflect.DeepEqual(after, before) {
		t.Fatalf("second run changed the project:\nbefore %v\nafter  %v", keys(before), keys(after))
	}
	want := "Setup applied to " + project + "\n" +
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

func TestSetupNeverRewritesAnExistingFile(t *testing.T) {
	project := t.TempDir()
	mustWrite(t, filepath.Join(project, "GLOSSARY.md"), "# Mine\n\nHand-written.\n")
	mustWrite(t, filepath.Join(project, "docs", "agents", "domain.md"), "custom\n")
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	files := tree(t, project)
	if files["GLOSSARY.md"] != "# Mine\n\nHand-written.\n" || files["docs/agents/domain.md"] != "custom\n" {
		t.Fatalf("existing files were rewritten: GLOSSARY.md=%q domain.md=%q", files["GLOSSARY.md"], files["docs/agents/domain.md"])
	}
	for _, line := range []string{"  ok         GLOSSARY.md\n", "  ok         docs/agents/domain.md\n", "  created    docs/agents/issue-tracker.md\n"} {
		if !strings.Contains(stdout, line) {
			t.Fatalf("stdout lacks %q:\n%s", line, stdout)
		}
	}
}

func TestSetupLeavesARealClaudeSkillsDirectoryWithContentUntouched(t *testing.T) {
	project := t.TempDir()
	mustWrite(t, filepath.Join(project, ".claude", "skills", "verify", "SKILL.md"), "# verify\n")
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := tree(t, project)[".claude/skills/verify/SKILL.md"]; got != "# verify\n" {
		t.Fatalf(".claude/skills/verify/SKILL.md = %q, want it untouched", got)
	}
	want := "  conflict   .claude/skills (real directory with content; left untouched)\n"
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout lacks %q:\n%s", want, stdout)
	}
	checkOut, _ := run(t, sys, "setup", "--check")
	if !strings.Contains(checkOut, want) {
		t.Fatalf("setup --check lacks %q:\n%s", want, checkOut)
	}
}

func TestSetupReplacesAnEmptyRealClaudeSkillsDirectoryWithTheLink(t *testing.T) {
	project := t.TempDir()
	mustMkdir(t, filepath.Join(project, ".claude", "skills"))
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := tree(t, project)[".claude/skills"]; got != "-> ../.agents/skills" {
		t.Fatalf(".claude/skills = %q, want the link", got)
	}
	if !strings.Contains(stdout, "  refreshed  .claude/skills\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestSetupSkipsTheStarterADRWhenTheProjectHasAnADR(t *testing.T) {
	project := t.TempDir()
	mustWrite(t, filepath.Join(project, "docs", "adr", "0001-use-postgres.md"), "# Use Postgres\n")
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var adrs []string
	for path := range tree(t, project) {
		if strings.HasPrefix(path, "docs/adr/") {
			adrs = append(adrs, path)
		}
	}
	if want := []string{"docs/adr/0001-use-postgres.md"}; !reflect.DeepEqual(adrs, want) {
		t.Fatalf("docs/adr holds %v, want %v", adrs, want)
	}
	if !strings.Contains(stdout, "  ok         docs/adr/0001-lint-and-tests-must-pass-before-merge.md\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestSetupRemovesTheGitkeepOnceADirectoryHasContent(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}
	if _, code := run(t, sys, "setup"); code != 0 {
		t.Fatalf("first run exit code = %d", code)
	}
	mustWrite(t, filepath.Join(project, ".agents", "skills", "verify", "SKILL.md"), "# verify\n")

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, ok := tree(t, project)[".agents/skills/.gitkeep"]; ok {
		t.Fatal(".agents/skills/.gitkeep still present beside a skill")
	}
	if !strings.Contains(stdout, "  refreshed  .agents/skills\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}

func TestSetupWithoutAGitHubRemoteNamesTheProjectAfterItsDirectory(t *testing.T) {
	project := filepath.Join(t.TempDir(), "my-tool")
	mustMkdir(t, project)
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	_, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := tree(t, project)["GLOSSARY.md"]; got != "# my-tool\n\n## Language\n" {
		t.Fatalf("GLOSSARY.md = %q", got)
	}
}

func TestSetupExitsNonZeroWhenAFileCannotBeWritten(t *testing.T) {
	project := t.TempDir()
	if err := os.Chmod(project, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(project, 0o755) })
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	_, code := run(t, sys, "setup")

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
}

func TestSetupLeavesAFileWhereADirectoryBelongsUntouched(t *testing.T) {
	project := t.TempDir()
	mustWrite(t, filepath.Join(project, ".agents", "skills"), "not a directory\n")
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := tree(t, project)[".agents/skills"]; got != "not a directory\n" {
		t.Fatalf(".agents/skills = %q, want it untouched", got)
	}
	if want := "  conflict   .agents/skills (not a directory; left untouched)\n"; !strings.Contains(stdout, want) {
		t.Fatalf("stdout lacks %q:\n%s", want, stdout)
	}
}

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

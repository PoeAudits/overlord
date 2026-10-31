package cmd_test

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/cmd"
	"github.com/PoeAudits/overlord/internal/system"
)

// cloneRunner answers like fakeRunner, and also acts like the real tools
// where the command's effect matters: a clone creates the directory, and a
// commit snapshots the files it would commit.
type cloneRunner struct {
	fakeRunner
	clonePath string
	committed map[string]string
	t         *testing.T
}

func (r *cloneRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	output, err := r.fakeRunner.Run(ctx, dir, name, args...)
	if err != nil {
		return output, err
	}
	command := strings.Join(append([]string{name}, args...), " ")
	if strings.HasPrefix(command, "gh repo clone ") {
		mustMkdir(r.t, r.clonePath)
	}
	if strings.HasPrefix(command, "git commit ") {
		r.committed = tree(r.t, r.clonePath)
	}
	return output, nil
}

func newProjectRunner(t *testing.T, home, name string) *cloneRunner {
	path := filepath.Join(home, "projects", name)
	t3 := filepath.Join(home, ".t3", "bin", "t3")
	return &cloneRunner{t: t, clonePath: path, fakeRunner: fakeRunner{outputs: map[string]string{
		"gh repo create PoeAudits/" + name + " --private":                           "",
		"gh repo create PoeAudits/" + name + " --public --description A small tool": "",
		"gh repo clone PoeAudits/" + name + " " + path:                              "",
		"git remote get-url origin":                                                 "git@github.com:PoeAudits/" + name + ".git\n",
		labelList:                                                                   allTriageLabels,
		"git add -A":                                                                "",
		"git commit -m Initial project setup":                                       "",
		"git push -u origin HEAD":                                                   "",
		t3 + " project add --title " + name + " " + path:                            "",
	}}}
}

func TestNewProjectCreatesClonesSetsUpCommitsPushesAndRegistersInOrder(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	sys.Runner = runner

	stdout, code := run(t, sys, "new-project", "sample")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	root := filepath.Join(sys.Home, "projects")
	path := filepath.Join(root, "sample")
	want := []string{
		root + ": gh repo create PoeAudits/sample --private",
		root + ": gh repo clone PoeAudits/sample " + path,
		path + ": git remote get-url origin",
		path + ": " + labelList,
		path + ": git add -A",
		path + ": git commit -m Initial project setup",
		path + ": git push -u origin HEAD",
		path + ": " + filepath.Join(sys.Home, ".t3", "bin", "t3") + " project add --title sample " + path,
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(runner.calls, "\n"), strings.Join(want, "\n"))
	}
	if got, want := runner.committed, tree(t, filepath.Join("testdata", "empty-project")); !reflect.DeepEqual(got, want) {
		t.Fatalf("files at commit differ from testdata/empty-project:\ngot  %v\nwant %v", keys(got), keys(want))
	}
}

func TestNewProjectRefusesAnExistingDirectoryBeforeAnyRemoteCall(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	sys.Runner = runner
	mustMkdir(t, runner.clonePath)
	var stderr strings.Builder

	code := cmdExecute(sys, &stderr, "new-project", "sample")

	if code == 0 {
		t.Fatal("exit code = 0, want non-zero")
	}
	if len(runner.calls) != 0 {
		t.Fatalf("commands ran: %q", runner.calls)
	}
	if want := runner.clonePath + " already exists"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
}

func TestNewProjectStopsAtTheFirstFailingRemoteStep(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	delete(runner.outputs, "git push -u origin HEAD")
	sys.Runner = runner
	var stderr strings.Builder

	code := cmdExecute(sys, &stderr, "new-project", "sample")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if last := runner.calls[len(runner.calls)-1]; !strings.HasSuffix(last, ": git push -u origin HEAD") {
		t.Fatalf("last command = %q, want the push; t3 registration must not run", last)
	}
	want := "overlord: git push -u origin HEAD failed: fake: command failed (exit status 1)\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestNewProjectPublicWithDescriptionPassesBothToGitHub(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	sys.Runner = runner

	_, code := run(t, sys, "new-project", "sample", "--public", "--description", "A small tool")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := filepath.Join(sys.Home, "projects") + ": gh repo create PoeAudits/sample --public --description A small tool"
	if runner.calls[0] != want {
		t.Fatalf("first command = %q, want %q", runner.calls[0], want)
	}
}

// cmdExecute runs overlord with stderr captured in stderr.
func cmdExecute(sys system.System, stderr *strings.Builder, args ...string) int {
	sys.Stdout = &strings.Builder{}
	sys.Stderr = stderr
	return cmd.Execute(sys, args)
}

func TestNewProjectClonesUnderTheProjectsRootFromOverlordsConfig(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	mustWrite(t, filepath.Join(sys.Home, ".config", "overlord", "config.yaml"), "projectsRoot: ~/code\n")
	root := filepath.Join(sys.Home, "code")
	path := filepath.Join(root, "sample")
	runner := newProjectRunner(t, sys.Home, "sample")
	runner.clonePath = path
	runner.outputs["gh repo clone PoeAudits/sample "+path] = ""
	runner.outputs[filepath.Join(sys.Home, ".t3", "bin", "t3")+" project add --title sample "+path] = ""
	sys.Runner = runner

	if stdout, code := run(t, sys, "new-project", "sample"); code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}

	if want := root + ": gh repo clone PoeAudits/sample " + path; runner.calls[1] != want {
		t.Fatalf("second command = %q, want %q", runner.calls[1], want)
	}
}

func TestNewProjectCreatesTheRepoUnderTheGitHubOwnerFromOverlordsConfig(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	mustWrite(t, filepath.Join(sys.Home, ".config", "overlord", "config.yaml"), "githubOwner: acme\n")
	path := filepath.Join(sys.Home, "projects", "sample")
	runner := newProjectRunner(t, sys.Home, "sample")
	runner.outputs["gh repo create acme/sample --private"] = ""
	runner.outputs["gh repo clone acme/sample "+path] = ""
	sys.Runner = runner

	stdout, code := run(t, sys, "new-project", "sample")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	root := filepath.Join(sys.Home, "projects")
	if got, want := runner.calls[:2], []string{
		root + ": gh repo create acme/sample --private",
		root + ": gh repo clone acme/sample " + path,
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first commands = %q, want %q", got, want)
	}
	if want := "Created acme/sample at " + path + " and registered it with T3 Code\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
}

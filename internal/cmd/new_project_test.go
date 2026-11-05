package cmd_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/PoeAudits/overlord/internal/cmd"
	"github.com/PoeAudits/overlord/internal/system"
)

// cloneRunner answers like fakeRunner, and also acts like the real tools
// where the command's effect matters: a clone creates the directory, and a
// commit snapshots the files it would commit.
type cloneRunner struct {
	fakeRunner
	clonePath string
	// afterClone, when set, runs once the clone directory exists.
	afterClone func()
	// labelListFailures is how many times gh label list fails, the way it
	// does for a repository GitHub is still creating, before it answers.
	labelListFailures int
	committed         map[string]string
	t                 *testing.T
}

func (r *cloneRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	output, err := r.fakeRunner.Run(ctx, dir, name, args...)
	command := strings.Join(append([]string{name}, args...), " ")
	if command == labelList && r.labelListFailures > 0 {
		r.labelListFailures--
		return nil, errors.New("exit status 1: HTTP 404: Not Found (https://api.github.com/repos/PoeAudits/sample/labels)")
	}
	if err != nil {
		return output, err
	}
	if strings.HasPrefix(command, "gh repo clone ") {
		mustMkdir(r.t, r.clonePath)
		if r.afterClone != nil {
			r.afterClone()
		}
	}
	if strings.HasPrefix(command, "git commit ") {
		r.committed = tree(r.t, r.clonePath)
	}
	return output, nil
}

func newProjectRunner(t *testing.T, home, name string) *cloneRunner {
	path := filepath.Join(home, "Overlord", "projects", name)
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

	stdout, code := run(t, sys, "new", "sample", "--lang", "go")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertFile(t, filepath.Join(sys.Home, ".config", "overlord", "config.yaml"), defaultConfig)
	root := filepath.Join(sys.Home, "Overlord", "projects")
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
	// --lang go makes the empty Project a Go one: go.mod, the Go standards
	// block and the Go CI job arrive in the first commit.
	files := tree(t, filepath.Join("testdata", "empty-project"))
	files["go.mod"] = "module github.com/PoeAudits/sample\n\ngo " + strings.TrimPrefix(runtime.Version(), "go") + "\n"
	files["docs/coding-standards.md"] = readFile(t, filepath.Join("testdata", "standards", "go.md"))
	files[".github/workflows/ci.yml"] = readFile(t, filepath.Join("testdata", "ci", "go.yml"))
	if got := runner.committed; !reflect.DeepEqual(got, files) {
		t.Fatalf("files at commit differ from testdata/empty-project with Go:\ngot  %v\nwant %v", got, files)
	}
}

func TestNewProjectCommitsTheLanguagesMarkerStandardsAndCI(t *testing.T) {
	tests := []struct {
		lang, name string
		markers    []string
	}{
		{lang: "typescript", name: "typescript", markers: []string{"package.json"}},
		{lang: "python", name: "python", markers: []string{"pyproject.toml"}},
		{lang: "rust", name: "rust", markers: []string{"Cargo.toml", "src/lib.rs"}},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			sys := newSystem(t, t.TempDir())
			runner := newProjectRunner(t, sys.Home, "sample")
			sys.Runner = runner

			if stdout, code := run(t, sys, "new", "sample", "--lang", tt.lang); code != 0 {
				t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
			}

			for _, marker := range tt.markers {
				if _, ok := runner.committed[marker]; !ok {
					t.Fatalf("%s not committed: %v", marker, keys(runner.committed))
				}
			}
			if got, want := runner.committed["docs/coding-standards.md"], readFile(t, filepath.Join("testdata", "standards", tt.name+".md")); got != want {
				t.Fatalf("docs/coding-standards.md:\n%s\nwant testdata/standards/%s.md", got, tt.name)
			}
			if got, want := runner.committed[".github/workflows/ci.yml"], readFile(t, filepath.Join("testdata", "ci", tt.name+".yml")); got != want {
				t.Fatalf(".github/workflows/ci.yml:\n%s\nwant testdata/ci/%s.yml", got, tt.name)
			}
		})
	}
}

func TestNewProjectRefusesAMissingOrUnknownLanguageBeforeAnyRemoteCall(t *testing.T) {
	tests := []struct {
		name, want string
		args       []string
	}{
		{name: "missing", args: []string{"new", "sample"}, want: `overlord: required flag(s) "lang" not set` + "\n"},
		{name: "unknown", args: []string{"new", "sample", "--lang", "cobol"}, want: `overlord: unknown language "cobol"; use go, typescript, python or rust` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := newSystem(t, t.TempDir())
			runner := newProjectRunner(t, sys.Home, "sample")
			sys.Runner = runner
			var stderr strings.Builder

			code := cmdExecute(sys, &stderr, tt.args...)

			if code == 0 || len(runner.calls) != 0 {
				t.Fatalf("exit code = %d, commands %q; want non-zero and none", code, runner.calls)
			}
			if stderr.String() != tt.want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestNewProjectRetriesTheLabelListWhileGitHubCreatesTheRepository(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	runner.labelListFailures = 2
	sys.Runner = runner
	var slept []time.Duration
	sys.Sleep = func(d time.Duration) { slept = append(slept, d) }

	stdout, code := run(t, sys, "new", "sample", "--lang", "go")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	if want := []time.Duration{2 * time.Second, 2 * time.Second}; !reflect.DeepEqual(slept, want) {
		t.Fatalf("slept %v, want %v", slept, want)
	}
	if got := strings.Count(strings.Join(runner.calls, "\n"), labelList); got != 3 {
		t.Fatalf("gh label list ran %d times, want 3", got)
	}
	if runner.committed == nil {
		t.Fatal("nothing committed after the label list answered")
	}
}

func TestNewProjectFailsWithoutCommittingWhenTheTriageLabelsCannotBeSetUp(t *testing.T) {
	tests := []struct {
		name string
		// failures is how many times gh label list fails; -1 leaves the
		// label list answering with no labels, so every create fails.
		failures   int
		wantSleeps int
		lastCall   string
		detail     string
	}{
		{
			name:       "label list keeps failing",
			failures:   4,
			wantSleeps: 3,
			lastCall:   labelList,
			detail:     "gh label list failed: exit status 1: HTTP 404: Not Found (https://api.github.com/repos/PoeAudits/sample/labels); triage labels not checked and may be missing, so the issue triage workflow may not work until overlord setup is re-run",
		},
		{
			name:       "label create fails",
			failures:   0,
			wantSleeps: 0,
			lastCall:   "gh label create needs-triage --repo PoeAudits/sample --description Maintainer needs to evaluate this issue --color e4e669",
			detail:     "gh label create needs-triage failed: fake: command failed; triage labels are missing, so the issue triage workflow will not work until overlord setup is re-run",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sys := newSystem(t, t.TempDir())
			runner := newProjectRunner(t, sys.Home, "sample")
			runner.labelListFailures = tt.failures
			if tt.failures == 0 {
				runner.outputs[labelList] = "[]"
			}
			sys.Runner = runner
			sleeps := 0
			sys.Sleep = func(time.Duration) { sleeps++ }

			stdout, stderr, code := runCapturing(sys, "new", "sample", "--lang", "go")

			if code != 1 {
				t.Fatalf("exit code = %d, want 1", code)
			}
			if sleeps != tt.wantSleeps {
				t.Fatalf("slept %d times, want %d", sleeps, tt.wantSleeps)
			}
			if last := runner.calls[len(runner.calls)-1]; !strings.HasSuffix(last, ": "+tt.lastCall) {
				t.Fatalf("last command = %q, want %q; nothing may be committed", last, tt.lastCall)
			}
			if want := "  warning    triage labels (" + tt.detail + ")\n"; !strings.HasSuffix(stdout, want) {
				t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
			}
			if want := "overlord: setup in " + runner.clonePath + " could not set up the triage labels: " + tt.detail + "\n"; stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		})
	}
}

func TestNewProjectRefusesAnExistingDirectoryBeforeAnyRemoteCall(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	sys.Runner = runner
	mustMkdir(t, runner.clonePath)
	var stderr strings.Builder

	code := cmdExecute(sys, &stderr, "new", "sample", "--lang", "go")

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

	code := cmdExecute(sys, &stderr, "new", "sample", "--lang", "go")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if last := runner.calls[len(runner.calls)-1]; !strings.HasSuffix(last, ": git push -u origin HEAD") {
		t.Fatalf("last command = %q, want the push; t3 registration must not run", last)
	}
	want := "overlord: git push -u origin HEAD failed: fake: command failed\n"
	if stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestNewProjectStopsBeforeCommittingWhenSetupFailsAnItem(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	agents := filepath.Join(runner.clonePath, ".agents")
	runner.afterClone = func() {
		mustMkdir(t, agents)
		if err := os.Chmod(agents, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(agents, 0o755) })
	}
	sys.Runner = runner

	stdout, stderr, code := runCapturing(sys, "new", "sample", "--lang", "go")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if last := runner.calls[len(runner.calls)-1]; !strings.HasSuffix(last, ": "+labelList) {
		t.Fatalf("last command = %q, want Setup's label list; nothing may be committed", last)
	}
	if want := "  failed     .agents/skills (mkdir " + filepath.Join(agents, "skills") + ": permission denied)\n"; !strings.Contains(stdout, want) {
		t.Fatalf("stdout lacks %q:\n%s", want, stdout)
	}
	if want := "overlord: setup in " + runner.clonePath + " could not set up every item; see the failed lines above\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestNewProjectPublicWithDescriptionPassesBothToGitHub(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	runner := newProjectRunner(t, sys.Home, "sample")
	sys.Runner = runner

	_, code := run(t, sys, "new", "sample", "--lang", "go", "--public", "--description", "A small tool")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := filepath.Join(sys.Home, "Overlord", "projects") + ": gh repo create PoeAudits/sample --public --description A small tool"
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

	if stdout, code := run(t, sys, "new", "sample", "--lang", "go"); code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}

	if want := root + ": gh repo clone PoeAudits/sample " + path; runner.calls[1] != want {
		t.Fatalf("second command = %q, want %q", runner.calls[1], want)
	}
}

func TestNewProjectCreatesTheRepoUnderTheGitHubOwnerFromOverlordsConfig(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	mustWrite(t, filepath.Join(sys.Home, ".config", "overlord", "config.yaml"), "githubOwner: acme\n")
	path := filepath.Join(sys.Home, "Overlord", "projects", "sample")
	runner := newProjectRunner(t, sys.Home, "sample")
	runner.outputs["gh repo create acme/sample --private"] = ""
	runner.outputs["gh repo clone acme/sample "+path] = ""
	sys.Runner = runner

	stdout, code := run(t, sys, "new", "sample", "--lang", "go")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	root := filepath.Join(sys.Home, "Overlord", "projects")
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

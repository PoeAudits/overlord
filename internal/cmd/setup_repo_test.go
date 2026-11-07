package cmd_test

import (
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const (
	notFound = "GraphQL: Could not resolve to a Repository with the name 'PoeAudits/sample'. (repository)"
	repoView = "gh repo view PoeAudits/sample --json isEmpty"
	gitLog   = "git -c core.quotePath=false log --branches --tags --name-only --format="
	branches = "git for-each-ref --format=%(refname) refs/heads"
	pushAll  = "git push -u origin refs/heads/*:refs/heads/* refs/tags/*:refs/tags/*"
)

// noOrigin is a runner for the git repository at project with no origin
// remote, whose history tracks the files in history and which has branches.
func noOrigin(project, history string) *fakeRunner {
	return &fakeRunner{
		outputs: map[string]string{
			"git rev-parse --show-toplevel": project + "\n",
			gitLog:                          history,
			branches:                        "refs/heads/main\nrefs/heads/feature\n",
			"gh repo create PoeAudits/sample --private --source " + project + " --remote origin": "",
			pushAll:   "",
			labelList: allTriageLabels,
		},
		stderr: map[string]string{repoView: notFound},
	}
}

// sampleProject is an empty directory named sample, since Setup names the
// GitHub repository after the directory.
func sampleProject(t *testing.T) string {
	t.Helper()
	project := filepath.Join(t.TempDir(), "sample")
	mustMkdir(t, project)
	return project
}

func TestSetupCheckReportsTheRepositoryItWouldCreateAndCreatesNothing(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "README.md\n.env.example\n")
	sys.Runner = runner

	stdout, code := run(t, sys, "setup", "--check")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	assertContains(t, stdout, "  missing    GitHub repository (would create private PoeAudits/sample and push every branch and tag)\n")
	for _, call := range runner.calls {
		if strings.Contains(call, "repo create") || strings.Contains(call, "git push") || strings.Contains(call, "git init") {
			t.Fatalf("--check ran %q", call)
		}
	}
}

func TestSetupCreatesAPrivateRepositoryPushesEveryBranchAndTagThenCreatesLabels(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "README.md\n.env.example\n")
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertContains(t, stdout, "  created    GitHub repository (private PoeAudits/sample, every branch and tag pushed)\n  ok         triage labels\n")
	want := []string{
		project + ": " + repoView,
		project + ": gh repo create PoeAudits/sample --private --source " + project + " --remote origin",
		project + ": " + branches,
		project + ": " + pushAll,
		project + ": " + labelList,
	}
	if got := afterCall(runner.calls, project+": "+gitLog); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after the history check:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSetupGitInitsADirectoryThatIsNotARepositoryAndPushesNothingWithoutBranches(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "")
	delete(runner.outputs, "git rev-parse --show-toplevel")
	runner.outputs["git init"] = ""
	runner.outputs[branches] = ""
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertContains(t, stdout, "  created    GitHub repository (private PoeAudits/sample created; nothing to push yet)\n")
	want := []string{
		project + ": " + repoView,
		project + ": git init",
		project + ": gh repo create PoeAudits/sample --private --source " + project + " --remote origin",
		project + ": " + branches,
		project + ": " + labelList,
	}
	if got := afterCall(runner.calls, project+": git rev-parse --show-toplevel"); !reflect.DeepEqual(got, want) {
		t.Fatalf("commands after the repository check:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSetupRefusesToPushAHistoryThatTracksASecret(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "README.md\n.env.example\nconfig/.env.pass\n\nREADME.md\ndeploy/server.pem\n")
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1: an unpushable Project is a real failure", code)
	}
	assertContains(t, stdout, "  failed     GitHub repository (history tracks secret files: config/.env.pass, deploy/server.pem; nothing is pushed until")
	assertNoCall(t, runner, "gh repo")
	assertNoCall(t, runner, "git push")
	assertContains(t, stdout, "  warning    triage labels (no GitHub origin remote)\n")
}

func TestSetupRefusesARepositoryNameThatExistsOnGitHubWithContent(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "README.md\n")
	delete(runner.stderr, repoView)
	runner.outputs[repoView] = `{"isEmpty":false}`
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	assertContains(t, stdout, "  failed     GitHub repository (PoeAudits/sample already exists on GitHub and is not empty; left untouched")
	assertNoCall(t, runner, "gh repo create")
	assertNoCall(t, runner, "git remote add")
	assertNoCall(t, runner, "git push")
}

func TestSetupPushesToAnExistingEmptyRepositoryOfTheOwner(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "README.md\n")
	delete(runner.stderr, repoView)
	runner.outputs[repoView] = `{"isEmpty":true}`
	runner.outputs["gh config get git_protocol"] = "ssh\n"
	runner.outputs["git remote add origin git@github.com:PoeAudits/sample.git"] = ""
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertContains(t, stdout, "  created    GitHub repository (the existing empty PoeAudits/sample, every branch and tag pushed)\n")
	assertNoCall(t, runner, "gh repo create")
	if !slices.Contains(runner.calls, project+": "+pushAll) {
		t.Fatalf("no push in %q", runner.calls)
	}
}

func TestSetupAddsAnExistingEmptyRepositoryAsOriginAndPushesNothingWithoutBranches(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "")
	delete(runner.stderr, repoView)
	runner.outputs[repoView] = `{"isEmpty":true}`
	runner.outputs[branches] = ""
	runner.outputs["git remote add origin https://github.com/PoeAudits/sample.git"] = ""
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertContains(t, stdout, "  created    GitHub repository (the existing empty PoeAudits/sample added as origin; nothing to push yet)\n")
	assertNoCall(t, runner, "gh repo create")
	assertNoCall(t, runner, "git push")
}

func TestSetupLeavesAProjectInsideAnotherRepositoryWithoutAnOrigin(t *testing.T) {
	project := sampleProject(t)
	sys := newSystem(t, project)
	runner := noOrigin(project, "")
	runner.outputs["git rev-parse --show-toplevel"] = filepath.Dir(project) + "\n"
	sys.Runner = runner

	stdout, code := run(t, sys, "setup", "--check")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	assertContains(t, stdout, "  conflict   GitHub repository (inside the git repository at "+filepath.Dir(project)+"; no GitHub repository created)\n")
	assertNoCall(t, runner, "gh repo")
}

// afterCall returns the calls after the first call equal to marker.
func afterCall(calls []string, marker string) []string {
	for i, call := range calls {
		if call == marker {
			return calls[i+1:]
		}
	}
	return nil
}

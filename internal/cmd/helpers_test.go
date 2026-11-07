package cmd_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PoeAudits/overlord/internal/cmd"
	"github.com/PoeAudits/overlord/internal/system"
)

// System and running

// newSystem returns a System with a temporary HOME, no environment, the given
// working directory and a runner that reports the PoeAudits/sample GitHub
// origin with every triage label present.
func newSystem(t *testing.T, workdir string) system.System {
	t.Helper()
	return system.System{
		Home:    t.TempDir(),
		Getenv:  func(string) string { return "" },
		Workdir: workdir,
		Runner: &fakeRunner{outputs: map[string]string{
			"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
			labelList:                   allTriageLabels,
		}},
	}
}

// startSystem is a System for start with a catalog in its HOME, a fake runner
// and a health checker that answers with healthy.
func startSystem(t *testing.T, healthy func(url string, calls int) bool) (system.System, *fakeRunner) {
	t.Helper()
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	runner := &fakeRunner{}
	sys.Runner = runner
	sys.Health = &fakeHealth{healthy: healthy, calls: map[string]int{}}
	sys.Sleep = func(time.Duration) {}
	return sys, runner
}

// run runs overlord and returns its stdout and exit code, logging any stderr.
func run(t *testing.T, sys system.System, args ...string) (string, int) {
	t.Helper()
	stdout, stderr, code := runCapturing(sys, args...)
	if stderr != "" {
		t.Logf("stderr: %s", stderr)
	}
	return stdout, code
}

// runCapturing runs overlord and returns its stdout, stderr and exit code.
func runCapturing(sys system.System, args ...string) (stdout, stderr string, code int) {
	var out, errOut bytes.Buffer
	sys.Stdout = &out
	sys.Stderr = &errOut
	code = cmd.Execute(sys, args)
	return out.String(), errOut.String(), code
}

// Fake runner and health

const (
	labelList       = "gh label list --repo PoeAudits/sample --json name --limit 1000"
	allTriageLabels = `[{"name":"needs-triage"},{"name":"needs-info"},{"name":"ready-for-agent"},{"name":"ready-for-human"},{"name":"wontfix"}]`
)

// fakeRunner records every command as "dir: name args" and answers from
// outputs keyed by "name args"; unknown commands fail like a missing remote.
type fakeRunner struct {
	outputs map[string]string
	// stderr makes a command fail with this stderr, carried in the error the
	// way system.ExecRunner carries it.
	stderr map[string]string
	calls  []string
}

func (r *fakeRunner) Run(_ context.Context, dir, name string, args ...string) ([]byte, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, dir+": "+command)
	if stderr, ok := r.stderr[command]; ok {
		return nil, errors.New("exit status 1: " + stderr)
	}
	output, ok := r.outputs[command]
	if !ok && command == "git remote get-url origin" {
		return []byte("error: No such remote 'origin'\n"), errors.New("exit status 2")
	}
	if !ok {
		// Any command without an output fails, the way gh does when it is
		// not logged in or offline.
		return []byte("fake: command failed\n"), errors.New("exit status 1")
	}
	return []byte(output), nil
}

func (r *fakeRunner) Launch(dir, logPath, name string, args ...string) error {
	command := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, dir+": launch "+logPath+": "+command)
	return nil
}

// lsIgnored is the command Setup runs to find tracked files that the general
// .gitignore block ignores. The fake fails it, as git does outside a
// repository, unless a test gives it an output.
const lsIgnored = "git ls-files --cached --ignored --exclude=.env --exclude=.env.* --exclude=!.env.example --exclude=!.env.sample --exclude=*.pass --exclude=*.pem --exclude=*.key --exclude=*.p12 --exclude=*.pfx --exclude=*.jks --exclude=*.keystore --exclude=id_rsa* --exclude=id_ed25519* --exclude=credentials*.json --exclude=service-account*.json --exclude=secrets.json --exclude=secrets.yaml --exclude=secrets.yml --exclude=secrets.toml --exclude=*.secret --exclude=.npmrc --exclude=.pypirc --exclude=node_modules/ --exclude=.venv/ --exclude=__pycache__/ --exclude=.DS_Store --exclude=*.log --exclude=.idea/ --exclude=.vscode/* --exclude=!.vscode/extensions.json --exclude=!.vscode/settings.json"

// fakeHealth answers each check with healthy(url, n), where n counts the
// checks of url so far, starting at 1.
type fakeHealth struct {
	healthy func(url string, calls int) bool
	calls   map[string]int
}

func (h *fakeHealth) Healthy(_ context.Context, url string) bool {
	h.calls[url]++
	return h.healthy(url, h.calls[url])
}

func allHealthy(string, int) bool { return true }

// Filesystem fixtures

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

// newAgentsDir creates home/.agents with one skill directory per name and the
// global AGENTS.md, and returns its path.
func newAgentsDir(t *testing.T, home string, skills ...string) string {
	t.Helper()
	agents := filepath.Join(home, ".agents")
	for _, name := range skills {
		mustWrite(t, filepath.Join(agents, "skills", name, "SKILL.md"), "# "+name+"\n")
	}
	mustWrite(t, filepath.Join(agents, "AGENTS.md"), "# Global rules\n")
	return agents
}

// addRoles copies the harness package's Role fixture (roles.yaml with main,
// explorer and restricted, and their prompts) into the agents directory.
func addRoles(t *testing.T, agents string) {
	t.Helper()
	fixture := filepath.Join("..", "harness", "testdata", "agents")
	for _, name := range []string{"roles.yaml", "roles/explorer.md", "roles/restricted.md"} {
		content, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(agents, name), string(content))
	}
}

// defaultConfig is the config file a first run writes.
const defaultConfig = `# Overlord's config. Written once with defaults; Overlord never rewrites it.
#
# agentsDir: the source of skills, Roles and global instructions for Sync.
# projectsRoot: where overlord new clones new Projects.
# githubOwner: the GitHub account or organisation overlord new creates
# repositories under.
# services: what start brings up, in order. A service is healthy when its
# health URL answers with any HTTP status below 500. When it is not, start
# runs command with sh -c, detached, output appended to
# ~/.local/state/overlord/<name>.log, and waits up to timeout for health.
# Set enabled: false to skip a service.
# {t3Origin} in a health URL is replaced, each time start runs, with the
# origin T3 Code's server last ran on (from
# ~/.t3/userdata/server-runtime.json; http://127.0.0.1:3773 when absent).
agentsDir: ~/.agents
projectsRoot: ~/Overlord/projects
githubOwner: PoeAudits
services:
  - name: t3
    command: ~/.t3/bin/t3 service install
    health: "{t3Origin}/.well-known/t3/environment"
    timeout: 30s
  - name: sleev
    command: sleev gateway start
    health: http://127.0.0.1:17321/__sleeve_local/health
    timeout: 30s
  - name: opencode
    command: opencode serve --hostname 127.0.0.1 --port 4096
    # /api/info answers 401 without the server password, which proves the
    # API is up; other paths return the web page even when the API is broken.
    health: http://127.0.0.1:4096/api/info
    timeout: 30s
`

// t3Settings is the path of T3 Code's settings file under home.
func t3Settings(home string) string {
	return filepath.Join(home, ".t3", "userdata", "settings.json")
}

// setupScriptJSON is the T3 Code project script Sync adds to run overlord setup.
const setupScriptJSON = `{"id": "overlord-setup", "name": "Overlord setup", "command": "overlord setup", "icon": "configure", "runOnWorktreeCreate": true, "async": false}`

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

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
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

// keys lists the paths of files, for failure messages.
func keys(files map[string]string) []string {
	var names []string
	for name := range files {
		names = append(names, name)
	}
	return names
}

// Assertions

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("%s:\n%s\nwant:\n%s", path, got, want)
	}
}

func assertContains(t *testing.T, stdout, want string) {
	t.Helper()
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout:\n%s\nwant it to contain:\n%s", stdout, want)
	}
}

func assertLink(t *testing.T, path, target string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("%s is not a symlink: %v", path, err)
	}
	if got != target {
		t.Fatalf("%s -> %s, want -> %s", path, got, target)
	}
}

func assertNoCall(t *testing.T, runner *fakeRunner, command string) {
	t.Helper()
	for _, call := range runner.calls {
		if strings.Contains(call, ": "+command) {
			t.Fatalf("ran %q", call)
		}
	}
}

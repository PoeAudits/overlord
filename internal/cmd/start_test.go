package cmd_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PoeAudits/overlord/internal/system"
)

func TestStartFirstRunWritesTheDefaultConfigAndNeverRewritesIt(t *testing.T) {
	sys, _ := startSystem(t, allHealthy)
	configPath := filepath.Join(sys.Home, ".config", "overlord", "config.yaml")

	if _, code := run(t, sys, "start"); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	assertFile(t, configPath, defaultConfig)
	mustWrite(t, configPath, "# edited by hand\nservices: []\n")
	if _, code := run(t, sys, "start"); code != 0 {
		t.Fatalf("second run exit code = %d, want 0", code)
	}
	assertFile(t, configPath, "# edited by hand\nservices: []\n")
}

// defaultConfig is the config file a first run writes.
const defaultConfig = `# Overlord's config. Written once with defaults; Overlord never rewrites it.
#
# agentsDir: the source of skills, Roles and global instructions for Sync.
# projectsRoot: where new-project clones new Projects.
# services: what start brings up, in order. A service is healthy when its
# health URL answers with any HTTP status below 500. When it is not, start
# runs command with sh -c, detached, output appended to
# ~/.local/state/overlord/<name>.log, and waits up to timeout for health.
# Set enabled: false to skip a service.
# {t3Origin} in a health URL is replaced, each time start runs, with the
# origin T3 Code's server last ran on (from
# ~/.t3/userdata/server-runtime.json; http://127.0.0.1:3773 when absent).
agentsDir: ~/.agents
projectsRoot: ~/projects
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

func TestStartWithEveryServiceHealthyAndTheAppRunningLaunchesNothing(t *testing.T) {
	sys, runner := startSystem(t, allHealthy)
	runner.outputs = map[string]string{appRunningCheck: "381421\n"}

	stdout, code := run(t, sys, "start")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertLink(t, filepath.Join(sys.Home, ".claude", "skills", "tdd"), filepath.Join(sys.Home, ".agents", "skills", "tdd"))
	want := "Start\n" +
		"  ok        t3: healthy\n" +
		"  ok        sleev: healthy\n" +
		"  ok        opencode: healthy\n" +
		"  ok        T3 Code app: running\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	assertCalls(t, runner, sys.Home+": "+appRunningCheck)
}

// appRunningCheck finds the T3 Code desktop app's main process: the binary
// with no arguments. Its helpers and the server all carry arguments.
const appRunningCheck = "pgrep -f -x [^ ]*/t3code"

func assertCalls(t *testing.T, runner *fakeRunner, want ...string) {
	t.Helper()
	if strings.Join(runner.calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(runner.calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestStartLaunchesAnUnhealthyServiceOnceAndWaitsForHealth(t *testing.T) {
	// sleev answers only on the third check: one before the launch and two
	// while start waits.
	sys, runner := startSystem(t, func(url string, calls int) bool {
		return url != sleevHealth || calls >= 3
	})

	stdout, code := run(t, sys, "start")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	want := "Start\n" +
		"  ok        t3: healthy\n" +
		"  started   sleev: healthy\n" +
		"  ok        opencode: healthy\n" +
		"  started   T3 Code app: launched\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	logs := filepath.Join(sys.Home, ".local", "state", "overlord")
	assertCalls(t, runner,
		sys.Home+": launch "+filepath.Join(logs, "sleev.log")+": sh -c sleev gateway start",
		sys.Home+": "+appRunningCheck,
		sys.Home+": launch "+filepath.Join(logs, "t3-app.log")+": gtk-launch t3code",
	)
	if info, err := os.Stat(logs); err != nil || !info.IsDir() {
		t.Fatalf("log directory %s was not created: %v", logs, err)
	}
}

const sleevHealth = "http://127.0.0.1:17321/__sleeve_local/health"

func TestStartReportsAServiceThatNeverBecomesHealthyAndStillStartsTheRest(t *testing.T) {
	sys, runner := startSystem(t, func(url string, calls int) bool {
		switch url {
		case t3Health:
			return false
		case sleevHealth:
			return calls >= 2
		}
		return true
	})
	runner.outputs = map[string]string{appRunningCheck: "381421\n"}

	stdout, code := run(t, sys, "start")

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero\n%s", stdout)
	}
	want := "Start\n" +
		"  failed    t3: not healthy after 30s; see ~/.local/state/overlord/t3.log\n" +
		"  started   sleev: healthy\n" +
		"  ok        opencode: healthy\n" +
		"  ok        T3 Code app: running\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	logs := filepath.Join(sys.Home, ".local", "state", "overlord")
	assertCalls(t, runner,
		sys.Home+": launch "+filepath.Join(logs, "t3.log")+": sh -c ~/.t3/bin/t3 service install",
		sys.Home+": launch "+filepath.Join(logs, "sleev.log")+": sh -c sleev gateway start",
		sys.Home+": "+appRunningCheck,
	)
}

const t3Health = "http://127.0.0.1:3773/.well-known/t3/environment"

func TestStartSkipsADisabledService(t *testing.T) {
	sys, runner := startSystem(t, func(string, int) bool { return false })
	runner.outputs = map[string]string{appRunningCheck: "381421\n"}
	mustWrite(t, filepath.Join(sys.Home, ".config", "overlord", "config.yaml"), `services:
  - name: web
    command: serve-web
    health: http://127.0.0.1:9000/
    enabled: false
`)

	stdout, code := run(t, sys, "start")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	want := "Start\n" +
		"  skipped   web: disabled\n" +
		"  ok        T3 Code app: running\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	assertCalls(t, runner, sys.Home+": "+appRunningCheck)
}

func TestStartChecksT3HealthOnThePortInT3ServerRuntimeFileAtStartTime(t *testing.T) {
	sys, _ := startSystem(t, allHealthy)
	if _, code := run(t, sys, "start"); code != 0 {
		t.Fatalf("first run exit code = %d, want 0", code)
	}
	mustWrite(t, filepath.Join(sys.Home, ".t3", "userdata", "server-runtime.json"),
		`{"version":1,"pid":381664,"host":"127.0.0.1","port":3801,"origin":"http://127.0.0.1:3801"}`)

	if _, code := run(t, sys, "start"); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	health := sys.Health.(*fakeHealth)
	if health.calls["http://127.0.0.1:3801/.well-known/t3/environment"] != 1 {
		t.Fatalf("health checks = %v, want one of T3 on port 3801", health.calls)
	}
}

func TestStartStillStartsServicesWhenSyncFails(t *testing.T) {
	sys, runner := startSystem(t, func(url string, calls int) bool {
		return url != sleevHealth || calls >= 2
	})
	runner.outputs = map[string]string{appRunningCheck: "381421\n"}
	mustWrite(t, t3Settings(sys.Home), "not json\n")

	stdout, stderr, code := runCapturing(sys, "start")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, stdout)
	}
	want := "Start\n" +
		"  ok        t3: healthy\n" +
		"  started   sleev: healthy\n" +
		"  ok        opencode: healthy\n" +
		"  ok        T3 Code app: running\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	if !strings.Contains(stderr, "sync t3: "+t3Settings(sys.Home)) {
		t.Fatalf("stderr does not report the Sync error:\n%s", stderr)
	}
}

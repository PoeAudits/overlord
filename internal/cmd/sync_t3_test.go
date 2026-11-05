package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const setupScriptJSON = `{"id": "overlord-setup", "name": "Overlord setup", "command": "overlord setup", "icon": "configure", "runOnWorktreeCreate": true, "async": false}`

func TestSyncRemovesTheRetiredSetupScriptAndKeepsEverythingElse(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	settings := `{
  "defaultProjectScripts": [
    {"id": "test", "name": "Test", "command": "go test ./...", "icon": "test", "runOnWorktreeCreate": false},
    ` + setupScriptJSON + `
  ],
  "favorites": [
    {
      "provider": "opencode",
      "model": "openai/gpt-6.1-sol"
    }
  ],
  "projectSettingsOverrides": {"p1": {"defaultProjectScripts": [` + setupScriptJSON + `]}},
  "storageCleanup": {"worktreeOnMerge": true}
}
`
	mustWrite(t, t3Settings(sys.Home), settings)

	stdout, code := run(t, sys, "sync", "--check")
	if code != 1 {
		t.Fatalf("check exit code = %d, want 1\n%s", code, stdout)
	}
	assertContains(t, stdout, "  remove    ~/.t3/userdata/settings.json: retired overlord-setup script\n")
	assertFile(t, t3Settings(sys.Home), settings)

	stdout, code = run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	assertContains(t, stdout, "  removed   ~/.t3/userdata/settings.json: retired overlord-setup script\n")
	// Only the global list is Sync's to clean; a project's override is the user's.
	want := `{
  "defaultProjectScripts": [
    {
      "id": "test",
      "name": "Test",
      "command": "go test ./...",
      "icon": "test",
      "runOnWorktreeCreate": false
    }
  ],
  "favorites": [
    {
      "provider": "opencode",
      "model": "openai/gpt-6.1-sol"
    }
  ],
  "projectSettingsOverrides": {"p1": {"defaultProjectScripts": [` + setupScriptJSON + `]}},
  "storageCleanup": {"worktreeOnMerge": true}
}
`
	assertFile(t, t3Settings(sys.Home), want)

	stdout, code = run(t, sys, "sync")
	if code != 0 || strings.Contains(stdout, "~/.t3") {
		t.Fatalf("second sync exit code = %d, want no T3 line:\n%s", code, stdout)
	}
	assertFile(t, t3Settings(sys.Home), want)
}

func TestSyncRemovesEveryCopyOfTheRetiredSetupScript(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	mustWrite(t, t3Settings(sys.Home), `{"defaultProjectScripts": [`+setupScriptJSON+`, `+setupScriptJSON+`], "defaultAutoPull": false}`)

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	assertFile(t, t3Settings(sys.Home), `{"defaultProjectScripts": [], "defaultAutoPull": false}`)
}

func TestSyncLeavesT3SettingsWithoutTheSetupScriptByteForByte(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	settings := `{"defaultProjectScripts": [{"id": "lint", "name": "Lint", "command": "make lint", "icon": "lint", "runOnWorktreeCreate": true}],
  "defaultAutoPull": true}
`
	mustWrite(t, t3Settings(sys.Home), settings)

	stdout, code := run(t, sys, "sync")

	if code != 0 || strings.Contains(stdout, "~/.t3") {
		t.Fatalf("exit code = %d, want no T3 line:\n%s", code, stdout)
	}
	assertFile(t, t3Settings(sys.Home), settings)
}

func TestSyncDoesNotCreateT3Settings(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")

	if stdout, code := run(t, sys, "sync"); code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	if _, err := os.Lstat(t3Settings(sys.Home)); err == nil {
		t.Fatal("sync created T3's settings file")
	}
}

func t3Settings(home string) string {
	return filepath.Join(home, ".t3", "userdata", "settings.json")
}

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

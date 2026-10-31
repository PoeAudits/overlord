package cmd_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const setupScriptJSON = `{
      "id": "overlord-setup",
      "name": "Overlord setup",
      "command": "overlord setup",
      "icon": "configure",
      "runOnWorktreeCreate": true,
      "async": false
    }`

func TestSyncCreatesT3SettingsWithTheSetupScriptWhenMissing(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	want := "{\n  \"defaultProjectScripts\": [\n    " + setupScriptJSON + "\n  ]\n}\n"
	assertFile(t, t3Settings(sys.Home), want)
	assertContains(t, stdout, "  created   ~/.t3/userdata/settings.json: overlord-setup script\n")
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

func TestSyncAddsTheSetupScriptAndKeepsEveryOtherSettingByteForByte(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	mustWrite(t, t3Settings(sys.Home), `{
  "favorites": [
    {
      "provider": "opencode",
      "model": "openai/gpt-6.1-sol"
    }
  ],
  "projectSettingsFolded": true,
  "storageCleanup": {"worktreeOnMerge": true}
}
`)

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	assertFile(t, t3Settings(sys.Home), `{
  "defaultProjectScripts": [
    `+setupScriptJSON+`
  ],
  "favorites": [
    {
      "provider": "opencode",
      "model": "openai/gpt-6.1-sol"
    }
  ],
  "projectSettingsFolded": true,
  "storageCleanup": {"worktreeOnMerge": true}
}
`)
	assertContains(t, stdout, "  created   ~/.t3/userdata/settings.json: overlord-setup script\n")
}

func TestSyncAddsTheSetupScriptOnceAfterExistingScripts(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	mustWrite(t, t3Settings(sys.Home), `{
  "defaultProjectScripts": [{"id": "test", "name": "Test", "command": "go test ./...", "icon": "test", "runOnWorktreeCreate": false}],
  "defaultAutoPull": false
}
`)
	want := `{
  "defaultProjectScripts": [
    {
      "id": "test",
      "name": "Test",
      "command": "go test ./...",
      "icon": "test",
      "runOnWorktreeCreate": false
    },
    ` + setupScriptJSON + `
  ],
  "defaultAutoPull": false
}
`

	for range 2 {
		if stdout, code := run(t, sys, "sync"); code != 0 {
			t.Fatalf("exit code = %d\n%s", code, stdout)
		}
		assertFile(t, t3Settings(sys.Home), want)
	}
}

func TestSyncCorrectsASetupScriptWithDifferentFields(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	mustWrite(t, t3Settings(sys.Home), `{"defaultProjectScripts": [
  {"id": "overlord-setup", "name": "Old", "command": "overlord setup", "icon": "play", "runOnWorktreeCreate": true, "async": true},
  {"id": "lint", "name": "Lint", "command": "make lint", "icon": "lint", "runOnWorktreeCreate": false}
]}`)

	stdout, code := run(t, sys, "sync", "--check")
	if code != 1 {
		t.Fatalf("check exit code = %d, want 1\n%s", code, stdout)
	}
	assertContains(t, stdout, "  replace   ~/.t3/userdata/settings.json: overlord-setup script\n")

	stdout, code = run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	assertFile(t, t3Settings(sys.Home), `{"defaultProjectScripts": [
    `+setupScriptJSON+`,
    {
      "id": "lint",
      "name": "Lint",
      "command": "make lint",
      "icon": "lint",
      "runOnWorktreeCreate": false
    }
  ]}`)
	assertContains(t, stdout, "  replaced  ~/.t3/userdata/settings.json: overlord-setup script\n")
}

func TestSyncCheckReportsAMissingSetupScriptAndLeavesTheSettingsAlone(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	settings := "{\"defaultAutoPull\": true}\n"
	mustWrite(t, t3Settings(sys.Home), settings)

	stdout, code := run(t, sys, "sync", "--check")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, stdout)
	}
	assertContains(t, stdout, "  create    ~/.t3/userdata/settings.json: overlord-setup script\n")
	assertFile(t, t3Settings(sys.Home), settings)
}

func TestSyncKeepsOnlyTheFirstSetupScript(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	entry := `{"id": "overlord-setup", "name": "Overlord setup", "command": "overlord setup", "icon": "configure", "runOnWorktreeCreate": true, "async": false}`
	mustWrite(t, t3Settings(sys.Home), `{"defaultProjectScripts": [`+entry+`, `+entry+`]}`)

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d\n%s", code, stdout)
	}
	assertFile(t, t3Settings(sys.Home), `{"defaultProjectScripts": [
    `+setupScriptJSON+`
  ]}`)
	assertContains(t, stdout, "  replaced  ~/.t3/userdata/settings.json: overlord-setup script\n")
}

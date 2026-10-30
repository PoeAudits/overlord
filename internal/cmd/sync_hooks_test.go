package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncOnFreshHomeInstallsTheSessionHooksAndTheOpenCodePlugin(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	for _, path := range []string{filepath.Join(sys.Home, ".claude", "settings.json"), filepath.Join(sys.Home, ".codex", "hooks.json")} {
		if commands := sessionStartCommands(t, path); len(commands) != 1 || commands[0] != "overlord sync --quiet" {
			t.Errorf("%s SessionStart commands = %q", path, commands)
		}
	}
	plugin := string(readFile(t, filepath.Join(sys.Home, ".config", "opencode", "plugins", "overlord-sync.js")))
	for _, want := range []string{`"session.created"`, `spawn("overlord", ["sync", "--quiet"]`} {
		if !strings.Contains(plugin, want) {
			t.Errorf("plugin lacks %s:\n%s", want, plugin)
		}
	}
	want := "  created   ~/.claude/settings.json: SessionStart hook\n" +
		"  created   ~/.codex/hooks.json: SessionStart hook\n" +
		"  created   ~/.config/opencode/plugins/overlord-sync.js\n"
	if !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout:\n%s\nwant suffix:\n%s", stdout, want)
	}
}

func TestSyncKeepsExistingHooksAndAddsOverlordsOnce(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	settings := filepath.Join(sys.Home, ".claude", "settings.json")
	codexHooks := filepath.Join(sys.Home, ".codex", "hooks.json")
	mustWrite(t, settings, `{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "guard"}]}],
    "SessionStart": [
      {
        "hooks": [
          {"type": "command", "command": "echo hi"}
        ]
      }
    ]
  }
}
`)
	mustWrite(t, codexHooks, `{"hooks": {"Stop": [{"hooks": [{"type": "command", "command": "notify"}]}]}}`)

	if stdout, code := run(t, sys, "sync"); code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}

	wantSettings := `{
  "hooks": {
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "guard"}]}],
    "SessionStart": [
      {
        "hooks": [
          {"type": "command", "command": "echo hi"}
        ]
      },
      {
        "hooks": [
          {
            "type": "command",
            "command": "overlord sync --quiet"
          }
        ]
      }
    ]
  }
}
`
	if got := string(readFile(t, settings)); got != wantSettings {
		t.Errorf("settings.json:\n%s\nwant:\n%s", got, wantSettings)
	}
	wantCodex := `{"hooks": {"SessionStart": [{"hooks":[{"type":"command","command":"overlord sync --quiet"}]}], "Stop": [{"hooks": [{"type": "command", "command": "notify"}]}]}}`
	if got := string(readFile(t, codexHooks)); got != wantCodex {
		t.Errorf("hooks.json:\n%s\nwant:\n%s", got, wantCodex)
	}

	stdout, code := run(t, sys, "sync")
	if code != 0 || !strings.Contains(stdout, "  ok        ~/.claude/settings.json: SessionStart hook\n") {
		t.Fatalf("second sync exit code = %d, stdout:\n%s", code, stdout)
	}
	if got := string(readFile(t, settings)); got != wantSettings {
		t.Errorf("second sync changed settings.json:\n%s", got)
	}
	if got := string(readFile(t, codexHooks)); got != wantCodex {
		t.Errorf("second sync changed hooks.json:\n%s", got)
	}
}

func TestSyncSecondRunOnAFullCatalogIsANoOpAndQuietPrintsNothing(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	addRoles(t, newAgentsDir(t, sys.Home, "tdd"))
	if stdout, code := run(t, sys, "sync"); code != 0 {
		t.Fatalf("first sync exit code = %d\n%s", code, stdout)
	}
	before := homeFiles(t, sys.Home)

	stdout, code := run(t, sys, "sync")
	if code != 0 {
		t.Fatalf("second sync exit code = %d\n%s", code, stdout)
	}
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")[1:] {
		if !strings.HasPrefix(line, "  ok  ") {
			t.Errorf("second sync did something: %q", line)
		}
	}
	if stdout, code := run(t, sys, "sync", "--quiet"); code != 0 || stdout != "" {
		t.Fatalf("sync --quiet exit code = %d, stdout:\n%s", code, stdout)
	}
	after := homeFiles(t, sys.Home)
	if len(after) != len(before) {
		t.Fatalf("file count changed from %d to %d", len(before), len(after))
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("%s changed", path)
		}
	}
}

func sessionStartCommands(t *testing.T, path string) []string {
	t.Helper()
	var file struct {
		Hooks struct {
			SessionStart []struct {
				Hooks []struct{ Type, Command string }
			}
		}
	}
	if err := json.Unmarshal(readFile(t, path), &file); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var commands []string
	for _, group := range file.Hooks.SessionStart {
		for _, hook := range group.Hooks {
			if hook.Type != "command" {
				t.Errorf("%s: hook type %q", path, hook.Type)
			}
			commands = append(commands, hook.Command)
		}
	}
	return commands
}

// homeFiles maps every regular file and link under home to its content or
// link target.
func homeFiles(t *testing.T, home string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			files[path] = "-> " + target
			return err
		}
		content, err := os.ReadFile(path)
		files[path] = string(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestSyncCheckWithAFullCatalogWritesNothing(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	addRoles(t, newAgentsDir(t, sys.Home, "tdd"))
	before := homeFiles(t, sys.Home)

	stdout, code := run(t, sys, "sync", "--check")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	for _, line := range []string{
		"  create    ~/.claude/agents/explorer.md\n",
		"  create    ~/.codex/config.toml: main Model\n",
		"  create    ~/.config/opencode/plugins/overlord-sync.js\n",
	} {
		if !strings.Contains(stdout, line) {
			t.Errorf("stdout lacks %q:\n%s", line, stdout)
		}
	}
	if after := homeFiles(t, sys.Home); len(after) != len(before) {
		t.Fatalf("sync --check wrote %d files", len(after)-len(before))
	}
}

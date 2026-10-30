package cmd_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/PoeAudits/overlord/internal/system"
)

// syncedHome returns a System whose HOME has a full catalog checked out with
// git and has been synced, so doctor should find nothing.
func syncedHome(t *testing.T) (sys systemWithRunner) {
	t.Helper()
	sys = newSystemWithRunner(t)
	agents := newAgentsDir(t, sys.Home, "tdd")
	addRoles(t, agents)
	mustMkdir(t, filepath.Join(agents, ".git"))
	if stdout, code := run(t, sys.System, "sync"); code != 0 {
		t.Fatalf("sync exit code = %d\n%s", code, stdout)
	}
	return sys
}

func TestDoctorOnASyncedHomeReportsCleanAndExitsZero(t *testing.T) {
	sys := syncedHome(t)

	stdout, code := run(t, sys.System, "doctor")

	if code != 0 || stdout != "Doctor: no drift found\n" {
		t.Fatalf("exit code = %d, stdout:\n%s", code, stdout)
	}
}

// systemWithRunner keeps the fake runner reachable so tests can script and
// inspect the external commands doctor runs.
type systemWithRunner struct {
	system.System
	runner *fakeRunner
}

func newSystemWithRunner(t *testing.T) systemWithRunner {
	t.Helper()
	runner := &fakeRunner{outputs: map[string]string{}}
	sys := newSystem(t, t.TempDir())
	sys.Runner = runner
	return systemWithRunner{System: sys, runner: runner}
}

func TestDoctorReportsEachDriftConditionAndWritesNothing(t *testing.T) {
	cases := []struct {
		name  string
		drift func(t *testing.T, home string)
		want  string
	}{
		{
			name:  "missing skill link",
			drift: func(t *testing.T, home string) { mustRemove(t, filepath.Join(home, ".claude", "skills", "tdd")) },
			want:  "  ~/.claude/skills/tdd: skill link missing\n",
		},
		{
			name:  "dangling skill link",
			drift: func(t *testing.T, home string) { mustRemove(t, filepath.Join(home, ".agents", "skills", "tdd")) },
			want:  "  ~/.claude/skills/tdd: dangling skill link\n",
		},
		{
			name: "stale OpenCode skills copy",
			drift: func(t *testing.T, home string) {
				mustWrite(t, filepath.Join(home, ".config", "opencode", "skills", "tdd", "SKILL.md"), "# tdd\n")
			},
			want: "  ~/.config/opencode/skills: stale skill copy; OpenCode reads ~/.claude/skills, so remove it\n",
		},
		{
			name:  "agents skills that are not the git checkout",
			drift: func(t *testing.T, home string) { mustRemove(t, filepath.Join(home, ".agents", ".git")) },
			want:  "  ~/.agents/skills: old skill copy; the agents directory is not a git checkout\n",
		},
		{
			name: "global instructions file that differs and is not a symlink",
			drift: func(t *testing.T, home string) {
				path := filepath.Join(home, ".claude", "CLAUDE.md")
				mustRemove(t, path)
				mustWrite(t, path, "# My own rules\n")
			},
			want: "  ~/.claude/CLAUDE.md: not a symlink to the catalog's AGENTS.md (content differs; merge it by hand)\n",
		},
		{
			name: "global instructions file that matches but is not a symlink",
			drift: func(t *testing.T, home string) {
				path := filepath.Join(home, ".codex", "AGENTS.md")
				mustRemove(t, path)
				mustWrite(t, path, "# Global rules\n")
			},
			want: "  ~/.codex/AGENTS.md: not a symlink to the catalog's AGENTS.md (same content; sync replaces it)\n",
		},
		{
			name: "missing Claude session hook",
			drift: func(t *testing.T, home string) {
				path := filepath.Join(home, ".claude", "settings.json")
				var settings map[string]any
				if err := json.Unmarshal([]byte(readFile(t, path)), &settings); err != nil {
					t.Fatal(err)
				}
				delete(settings, "hooks")
				content, err := json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, path, string(content))
			},
			want: "  ~/.claude/settings.json: SessionStart hook missing\n",
		},
		{
			name:  "missing Codex session hook",
			drift: func(t *testing.T, home string) { mustWrite(t, filepath.Join(home, ".codex", "hooks.json"), "{}\n") },
			want:  "  ~/.codex/hooks.json: SessionStart hook missing\n",
		},
		{
			name: "missing OpenCode sync plugin",
			drift: func(t *testing.T, home string) {
				mustRemove(t, filepath.Join(home, ".config", "opencode", "plugins", "overlord-sync.js"))
			},
			want: "  ~/.config/opencode/plugins/overlord-sync.js: session sync plugin missing\n",
		},
		{
			name: "hand-written agent file that Sync would render",
			drift: func(t *testing.T, home string) {
				mustWrite(t, filepath.Join(home, ".codex", "agents", "explorer.toml"), "name = \"explorer\"\n")
			},
			want: "  ~/.codex/agents/explorer.toml: agent file without the generated header\n",
		},
		{
			name: "hand-written agent file with no Role",
			drift: func(t *testing.T, home string) {
				mustWrite(t, filepath.Join(home, ".claude", "agents", "reviewer.md"), "---\nname: reviewer\n---\n")
			},
			want: "  ~/.claude/agents/reviewer.md: agent file without the generated header\n",
		},
		{
			name:  "missing T3 default script",
			drift: func(t *testing.T, home string) { mustWrite(t, t3Settings(home), "{}\n") },
			want:  "  ~/.t3/userdata/settings.json: overlord-setup script missing\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sys := syncedHome(t)
			c.drift(t, sys.Home)
			before := homeFiles(t, sys.Home)

			stdout, code := run(t, sys.System, "doctor")

			if code != 1 {
				t.Errorf("exit code = %d, want 1", code)
			}
			if stdout != "Doctor found drift\n"+c.want {
				t.Errorf("stdout:\n%s\nwant:\nDoctor found drift\n%s", stdout, c.want)
			}
			assertUnchanged(t, before, homeFiles(t, sys.Home))
		})
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
}

func assertUnchanged(t *testing.T, before, after map[string]string) {
	t.Helper()
	if !maps.Equal(before, after) {
		t.Errorf("doctor changed HOME:\nbefore %v\nafter  %v", keys(before), keys(after))
	}
}

// addProjectOverrides adds per-project settings to T3's synced settings: one
// project whose scripts override lacks the setup script, one whose override
// has it, and one with no scripts override at all.
func addProjectOverrides(t *testing.T, home string) {
	t.Helper()
	var settings map[string]any
	if err := json.Unmarshal([]byte(readFile(t, t3Settings(home))), &settings); err != nil {
		t.Fatal(err)
	}
	lint := map[string]any{"id": "lint", "name": "Lint", "command": "make lint", "icon": "lint", "runOnWorktreeCreate": false}
	settings["projectSettingsOverrides"] = map[string]any{
		"p-dropped": map[string]any{"defaultProjectScripts": []any{lint}},
		"p-kept":    map[string]any{"defaultProjectScripts": append([]any{lint}, settings["defaultProjectScripts"].([]any)...)},
		"p-other":   map[string]any{"defaultAutoPull": true},
	}
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, t3Settings(home), string(content))
}

func t3ProjectsQuery(home string) string {
	return "sqlite3 -readonly -json " + filepath.Join(home, ".t3", "userdata", "statev2.sqlite") +
		" SELECT project_id, title, workspace_root FROM projection_projects WHERE deleted_at IS NULL"
}

func TestDoctorNamesT3ProjectsWhoseScriptsOverrideOmitsTheSetupScript(t *testing.T) {
	sys := syncedHome(t)
	addProjectOverrides(t, sys.Home)
	sys.runner.outputs[t3ProjectsQuery(sys.Home)] = `[{"project_id":"p-dropped","title":"pixie","workspace_root":"/src/pixie"},` +
		`{"project_id":"p-kept","title":"overlord","workspace_root":"/src/overlord"}]` + "\n"
	before := homeFiles(t, sys.Home)

	stdout, code := run(t, sys.System, "doctor")

	want := "Doctor found drift\n" +
		"  ~/.t3/userdata/settings.json: T3 project pixie (/src/pixie) has a scripts override without overlord-setup, so Setup does not run there\n"
	if code != 1 || stdout != want {
		t.Fatalf("exit code = %d, stdout:\n%s\nwant:\n%s", code, stdout, want)
	}
	assertUnchanged(t, before, homeFiles(t, sys.Home))
}

func TestDoctorNamesAnUnknownT3ProjectByItsID(t *testing.T) {
	sys := syncedHome(t)
	addProjectOverrides(t, sys.Home)

	stdout, code := run(t, sys.System, "doctor")

	want := "Doctor found drift\n" +
		"  ~/.t3/userdata/settings.json: T3 project p-dropped has a scripts override without overlord-setup, so Setup does not run there\n"
	if code != 1 || stdout != want {
		t.Fatalf("exit code = %d, stdout:\n%s\nwant:\n%s", code, stdout, want)
	}
}

const upstreamFile = `# Upstream skills

Prose and headings are ignored; only entries of the form below are read.

- tdd https://github.com/mattpocock/skills 1111111111111111111111111111111111111111
- ` + "`principle-laziness-protocol`" + ` https://github.com/cursor/plugins ccb5507
`

func TestDoctorReportsUpstreamSourcesWhoseRemoteHeadMoved(t *testing.T) {
	sys := syncedHome(t)
	mustWrite(t, filepath.Join(sys.Home, ".agents", "UPSTREAM.md"), upstreamFile)
	sys.runner.outputs["git ls-remote https://github.com/mattpocock/skills HEAD"] = "1111111111111111111111111111111111111111\tHEAD\n"
	sys.runner.outputs["git ls-remote https://github.com/cursor/plugins HEAD"] = "9f3e2d1c0b9a8f7e6d5c4b3a2f1e0d9c8b7a6f5e\tHEAD\n"

	stdout, code := run(t, sys.System, "doctor")

	want := "Doctor found drift\n" +
		"  ~/.agents/UPSTREAM.md: principle-laziness-protocol: https://github.com/cursor/plugins HEAD is 9f3e2d1c0b9a, recorded ccb5507\n"
	if code != 1 || stdout != want {
		t.Fatalf("exit code = %d, stdout:\n%s\nwant:\n%s", code, stdout, want)
	}
}

func TestDoctorSkipsUpstreamChecksWithANoteWhenOffline(t *testing.T) {
	sys := syncedHome(t)
	mustWrite(t, filepath.Join(sys.Home, ".agents", "UPSTREAM.md"), upstreamFile)

	stdout, code := run(t, sys.System, "doctor")

	want := "Doctor: no drift found\n" +
		"Not checked\n" +
		"  ~/.agents/UPSTREAM.md: tdd: could not reach https://github.com/mattpocock/skills\n" +
		"  ~/.agents/UPSTREAM.md: principle-laziness-protocol: could not reach https://github.com/cursor/plugins\n"
	if code != 0 || stdout != want {
		t.Fatalf("exit code = %d, stdout:\n%s\nwant:\n%s", code, stdout, want)
	}
	if !slices.Contains(sys.runner.calls, ": git ls-remote https://github.com/mattpocock/skills HEAD") {
		t.Errorf("recorded commands lack the remote lookup: %v", sys.runner.calls)
	}
}

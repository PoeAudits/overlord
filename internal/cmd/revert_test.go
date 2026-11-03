package cmd_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Temporary, with internal/revert.

// existingSetup lays out a machine the way it is before the move to v2: a
// catalog that is out of step with real skill directories, instruction files
// that Sync would link over, and settings Sync edits.
func existingSetup(t *testing.T, home string) {
	t.Helper()
	newAgentsDir(t, home, "grill", "tdd")
	mustWrite(t, filepath.Join(home, ".claude", "skills", "tdd", "SKILL.md"), "# my newer tdd\n")
	mustWrite(t, filepath.Join(home, ".claude", "skills", "synced", "keep.md"), "claude owns this\n")
	mustWrite(t, filepath.Join(home, ".claude", "CLAUDE.md"), "# Global rules\n")
	mustWrite(t, filepath.Join(home, ".claude", "settings.json"), "{\"theme\": \"dark\"}\n")
	mustWrite(t, filepath.Join(home, ".claude", "agents", "explorer.md"), "old explorer\n")
	mustWrite(t, filepath.Join(home, ".config", "opencode", "skills", "stale", "SKILL.md"), "stale\n")
	mustWrite(t, t3Settings(home), "{\"keep\": true}\n")
	if err := os.Symlink(filepath.Join(home, ".agents", "skills", "gone"), filepath.Join(home, ".claude", "skills", "gone")); err != nil {
		t.Fatal(err)
	}
}

// machineState describes every file, link and non-empty directory under home
// except Overlord's own state directory. Revert may leave a directory that
// Sync created, such as ~/.codex, empty; that is harmless and ignored.
func machineState(t *testing.T, home string) map[string]string {
	t.Helper()
	state := map[string]string{}
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(home, path)
		switch {
		case rel == ".local/state/overlord" || strings.HasPrefix(rel, ".local/state/overlord/"):
			return filepath.SkipDir
		case entry.Type()&fs.ModeSymlink != 0:
			target, _ := os.Readlink(path)
			state[rel] = "link to " + target
		case entry.IsDir():
			children, _ := os.ReadDir(path)
			if len(children) > 0 && rel != ".local/state" && rel != ".local" {
				state[rel] = "directory"
			}
		default:
			data, _ := os.ReadFile(path)
			state[rel] = "file: " + string(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func assertSameState(t *testing.T, got, want map[string]string) {
	t.Helper()
	for path, wantState := range want {
		if got[path] != wantState {
			t.Errorf("%s: got %q, want %q", path, got[path], wantState)
		}
	}
	for path, gotState := range got {
		if _, ok := want[path]; !ok {
			t.Errorf("%s should not exist but is %q", path, gotState)
		}
	}
}

func TestRevertAfterSyncRestoresTheMachineToItsPriorState(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	existingSetup(t, sys.Home)
	before := machineState(t, sys.Home)

	if _, code := run(t, sys, "sync"); code != 0 {
		t.Fatalf("sync exit code = %d, want 0", code)
	}
	if machineState(t, sys.Home)[".claude/skills/grill"] == "" {
		t.Fatal("sync changed nothing, so this test proves nothing")
	}
	stdout, code := run(t, sys, "revert")

	if code != 0 {
		t.Fatalf("revert exit code = %d, want 0\n%s", code, stdout)
	}
	assertSameState(t, machineState(t, sys.Home), before)
	assertContains(t, stdout, "Reverted to the saved setup\n")
}

func TestRevertAfterStartAlsoRemovesTheConfigFileItWrote(t *testing.T) {
	sys, _ := startSystem(t, allHealthy)
	existingSetup(t, sys.Home)
	before := machineState(t, sys.Home)

	if _, code := run(t, sys, "start"); code != 0 {
		t.Fatalf("start exit code = %d, want 0", code)
	}
	if _, code := run(t, sys, "revert"); code != 0 {
		t.Fatalf("revert exit code = %d, want 0", code)
	}

	assertSameState(t, machineState(t, sys.Home), before)
}

func TestRevertGoesBackToBeforeTheFirstSyncEvenAfterLaterSyncs(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	existingSetup(t, sys.Home)
	before := machineState(t, sys.Home)

	run(t, sys, "sync")
	mustWrite(t, filepath.Join(sys.Home, ".agents", "skills", "new", "SKILL.md"), "# new\n")
	run(t, sys, "sync")
	if _, code := run(t, sys, "revert"); code != 0 {
		t.Fatalf("revert exit code = %d, want 0", code)
	}

	got := machineState(t, sys.Home)
	delete(got, ".agents/skills/new")
	delete(got, ".agents/skills/new/SKILL.md")
	assertSameState(t, got, before)
}

func TestRevertKeepsWhatItReplaces(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	existingSetup(t, sys.Home)
	run(t, sys, "sync")
	mustWrite(t, filepath.Join(sys.Home, ".claude", "settings.json"), "{\"edited\": \"after sync\"}\n")

	stdout, code := run(t, sys, "revert")

	if code != 0 {
		t.Fatalf("revert exit code = %d, want 0", code)
	}
	kept, err := filepath.Glob(filepath.Join(sys.Home, ".local", "state", "overlord", "revert-replaced-*", sys.Home, ".claude", "settings.json"))
	if err != nil || len(kept) != 1 {
		t.Fatalf("replaced settings.json kept at %v (%v)\n%s", kept, err, stdout)
	}
	assertFile(t, kept[0], "{\"edited\": \"after sync\"}\n")
	assertContains(t, stdout, "what it replaced is kept in ~/.local/state/overlord/revert-replaced-")
}

func TestRevertWithoutASavedCopyFailsAndChangesNothing(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	existingSetup(t, sys.Home)
	before := machineState(t, sys.Home)

	stdout, stderr, code := runCapturing(sys, "revert")

	if code != 1 {
		t.Fatalf("exit code = %d, want 1\n%s", code, stdout)
	}
	assertContains(t, stderr, "nothing to revert")
	assertSameState(t, machineState(t, sys.Home), before)
}

func TestSyncCheckDoesNotSaveAndSnapshotSavesOnce(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	existingSetup(t, sys.Home)

	run(t, sys, "sync", "--check")
	if _, _, code := runCapturing(sys, "revert"); code != 1 {
		t.Fatalf("revert after a check: exit code = %d, want 1 (a check must not save)", code)
	}

	stdout, _ := run(t, sys, "snapshot")
	assertContains(t, stdout, "Saved to ~/.local/state/overlord/revert\n")
	stdout, _ = run(t, sys, "snapshot")
	assertContains(t, stdout, "A saved copy already exists")
}

package cmd_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncOnFreshHomeLinksEverySkillAndInstructionFile(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	agents := newAgentsDir(t, sys.Home, "grill", "tdd")

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	assertLink(t, filepath.Join(sys.Home, ".claude", "skills", "grill"), filepath.Join(agents, "skills", "grill"))
	assertLink(t, filepath.Join(sys.Home, ".claude", "skills", "tdd"), filepath.Join(agents, "skills", "tdd"))
	for _, path := range instructionPaths(sys.Home) {
		assertLink(t, path, filepath.Join(agents, "AGENTS.md"))
	}
	want := "Sync\n" +
		"  created   ~/.claude/skills/grill\n" +
		"  created   ~/.claude/skills/tdd\n" +
		"  created   ~/.claude/CLAUDE.md\n" +
		"  created   ~/.codex/AGENTS.md\n" +
		"  created   ~/.config/opencode/AGENTS.md\n" +
		hookLines("created") +
		"  created   ~/.t3/userdata/settings.json: overlord-setup script\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestSyncSecondRunChangesNothingAndCheckReportsEverythingOK(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	if _, code := run(t, sys, "sync"); code != 0 {
		t.Fatalf("first sync exit code = %d", code)
	}

	allOK := "  ok        ~/.claude/skills/tdd\n" +
		"  ok        ~/.claude/CLAUDE.md\n" +
		"  ok        ~/.codex/AGENTS.md\n" +
		"  ok        ~/.config/opencode/AGENTS.md\n" +
		hookLines("ok") +
		"  ok        ~/.t3/userdata/settings.json: overlord-setup script\n"
	stdout, code := run(t, sys, "sync")
	if code != 0 || stdout != "Sync\n"+allOK {
		t.Fatalf("second sync exit code = %d, stdout:\n%s", code, stdout)
	}
	stdout, code = run(t, sys, "sync", "--check")
	if code != 0 || stdout != "Sync check (nothing changed)\n"+allOK {
		t.Fatalf("sync --check exit code = %d, stdout:\n%s", code, stdout)
	}
}

func TestSyncCheckOnFreshHomeReportsWhatItWouldCreateAndWritesNothing(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")

	stdout, code := run(t, sys, "sync", "--check")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "Sync check (nothing changed)\n" +
		"  create    ~/.claude/skills/tdd\n" +
		"  create    ~/.claude/CLAUDE.md\n" +
		"  create    ~/.codex/AGENTS.md\n" +
		"  create    ~/.config/opencode/AGENTS.md\n" +
		hookLines("create") +
		"  create    ~/.t3/userdata/settings.json: overlord-setup script\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	for _, dir := range []string{".claude", ".codex", ".config", ".t3"} {
		if _, err := os.Lstat(filepath.Join(sys.Home, dir)); err == nil {
			t.Fatalf("sync --check created ~/%s", dir)
		}
	}
}

func TestSyncLeavesSyncedTrashAndRealSkillDirectoriesAloneAndReportsTheShadow(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "grill", "tdd")
	skills := filepath.Join(sys.Home, ".claude", "skills")
	untouched := map[string]string{
		filepath.Join(skills, "synced", "claude-ai", "SKILL.md"): "# from claude.ai\n",
		filepath.Join(skills, ".trash", "old", "SKILL.md"):       "# deleted\n",
		filepath.Join(skills, "tdd", "SKILL.md"):                 "# my own tdd\n",
	}
	for path, content := range untouched {
		mustWrite(t, path, content)
	}
	// A broken link inside synced belongs to claude.ai sync, not to Overlord.
	if err := os.Symlink(filepath.Join(sys.Home, ".agents", "skills", "gone"), filepath.Join(skills, "synced", "gone")); err != nil {
		t.Fatal(err)
	}

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	for path, content := range untouched {
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatalf("%s changed: %q, %v", path, got, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(skills, "synced", "gone")); err != nil {
		t.Fatalf("link inside synced was removed: %v", err)
	}
	assertLink(t, filepath.Join(skills, "grill"), filepath.Join(sys.Home, ".agents", "skills", "grill"))
	want := "Sync\n" +
		"  created   ~/.claude/skills/grill\n" +
		"  shadowed  ~/.claude/skills/tdd: a real directory hides the catalog skill\n"
	if !strings.HasPrefix(stdout, want) {
		t.Fatalf("stdout:\n%s\nwant prefix:\n%s", stdout, want)
	}
}

func TestSyncRemovesDanglingLinksIntoTheCatalogAndLeavesOtherLinksAlone(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	agents := newAgentsDir(t, sys.Home, "tdd")
	skills := filepath.Join(sys.Home, ".claude", "skills")
	mustMkdir(t, skills)
	elsewhere := filepath.Join(t.TempDir(), "vanished")
	links := map[string]string{
		"deleted":          filepath.Join(agents, "skills", "deleted"),
		"deleted-relative": "../../.agents/skills/deleted-relative",
		"elsewhere":        elsewhere,
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(skills, name)); err != nil {
			t.Fatal(err)
		}
	}

	stdout, code := run(t, sys, "sync")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}
	for _, name := range []string{"deleted", "deleted-relative"} {
		if _, err := os.Lstat(filepath.Join(skills, name)); !os.IsNotExist(err) {
			t.Fatalf("dangling link %s into the catalog was not removed: %v", name, err)
		}
		if !strings.Contains(stdout, "  removed   ~/.claude/skills/"+name+"\n") {
			t.Fatalf("removal of %s not reported:\n%s", name, stdout)
		}
	}
	assertLink(t, filepath.Join(skills, "elsewhere"), elsewhere)
	if strings.Contains(stdout, "elsewhere") {
		t.Fatalf("a link Overlord does not own was reported:\n%s", stdout)
	}
}

func TestSyncCheckReportsDanglingLinksWithoutRemovingThem(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	agents := newAgentsDir(t, sys.Home, "tdd")
	link := filepath.Join(sys.Home, ".claude", "skills", "deleted")
	mustMkdir(t, filepath.Dir(link))
	if err := os.Symlink(filepath.Join(agents, "skills", "deleted"), link); err != nil {
		t.Fatal(err)
	}

	stdout, code := run(t, sys, "sync", "--check")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "  remove    ~/.claude/skills/deleted\n") {
		t.Fatalf("stdout:\n%s", stdout)
	}
	if _, err := os.Lstat(link); err != nil {
		t.Fatalf("sync --check removed the link: %v", err)
	}
}

func TestSyncPreservesAndReportsADifferingInstructionFileAndCheckFails(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "tdd")
	claudeMD := filepath.Join(sys.Home, ".claude", "CLAUDE.md")
	mustWrite(t, claudeMD, "# My own rules\n")
	conflict := "  conflict  ~/.claude/CLAUDE.md: differs from the catalog's AGENTS.md; left in place, merge it by hand\n"

	stdout, code := run(t, sys, "sync", "--check")
	if code == 0 {
		t.Fatalf("sync --check exit code = 0, want non-zero\n%s", stdout)
	}
	if !strings.Contains(stdout, conflict) {
		t.Fatalf("sync --check stdout:\n%s", stdout)
	}

	stdout, code = run(t, sys, "sync")
	if code != 0 {
		t.Fatalf("sync exit code = %d, want 0\n%s", code, stdout)
	}
	if !strings.Contains(stdout, conflict) {
		t.Fatalf("sync stdout:\n%s", stdout)
	}
	got, err := os.ReadFile(claudeMD)
	if err != nil || string(got) != "# My own rules\n" {
		t.Fatalf("CLAUDE.md was changed: %q, %v", got, err)
	}
	if info, err := os.Lstat(claudeMD); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("CLAUDE.md is no longer a regular file: %v", err)
	}
}

func TestSyncReplacesAnIdenticalInstructionFileWithTheLink(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	agents := newAgentsDir(t, sys.Home, "tdd")
	claudeMD := filepath.Join(sys.Home, ".claude", "CLAUDE.md")
	mustWrite(t, claudeMD, "# Global rules\n")

	stdout, code := run(t, sys, "sync", "--check")
	if code != 0 || !strings.Contains(stdout, "  replace   ~/.claude/CLAUDE.md\n") {
		t.Fatalf("sync --check exit code = %d, stdout:\n%s", code, stdout)
	}
	if info, err := os.Lstat(claudeMD); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("sync --check replaced CLAUDE.md: %v", err)
	}

	stdout, code = run(t, sys, "sync")
	if code != 0 || !strings.Contains(stdout, "  replaced  ~/.claude/CLAUDE.md\n") {
		t.Fatalf("sync exit code = %d, stdout:\n%s", code, stdout)
	}
	assertLink(t, claudeMD, filepath.Join(agents, "AGENTS.md"))
}

func TestSyncQuietPrintsOnlyWhatNeedsTheUser(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	newAgentsDir(t, sys.Home, "grill", "tdd")
	mustWrite(t, filepath.Join(sys.Home, ".claude", "skills", "tdd", "SKILL.md"), "# my own tdd\n")
	mustWrite(t, filepath.Join(sys.Home, ".codex", "AGENTS.md"), "# Codex rules\n")

	stdout, code := run(t, sys, "sync", "--quiet")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "  shadowed  ~/.claude/skills/tdd: a real directory hides the catalog skill\n" +
		"  conflict  ~/.codex/AGENTS.md: differs from the catalog's AGENTS.md; left in place, merge it by hand\n"
	if stdout != want {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, want)
	}
	assertLink(t, filepath.Join(sys.Home, ".claude", "skills", "grill"), filepath.Join(sys.Home, ".agents", "skills", "grill"))

	if stdout, _ := run(t, sys, "sync", "--quiet"); stdout != want {
		t.Fatalf("second quiet run stdout:\n%s", stdout)
	}
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

// hookLines is the report of the hooks step when every hook has the given
// action label.
func hookLines(label string) string {
	return fmt.Sprintf("  %-8s  ~/.claude/settings.json: SessionStart hook\n", label) +
		fmt.Sprintf("  %-8s  ~/.codex/hooks.json: SessionStart hook\n", label) +
		fmt.Sprintf("  %-8s  ~/.config/opencode/plugins/overlord-sync.js\n", label)
}

func instructionPaths(home string) []string {
	return []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".codex", "AGENTS.md"),
		filepath.Join(home, ".config", "opencode", "AGENTS.md"),
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

func TestSyncUsesTheAgentsDirFromOverlordsConfig(t *testing.T) {
	sys := newSystem(t, t.TempDir())
	catalog := filepath.Join(sys.Home, "src", "agents")
	mustWrite(t, filepath.Join(catalog, "skills", "grill", "SKILL.md"), "# grill\n")
	mustWrite(t, filepath.Join(catalog, "AGENTS.md"), "# Global rules\n")
	mustWrite(t, filepath.Join(sys.Home, ".config", "overlord", "config.yaml"), "agentsDir: ~/src/agents\n")

	if stdout, code := run(t, sys, "sync"); code != 0 {
		t.Fatalf("exit code = %d, want 0\n%s", code, stdout)
	}

	assertLink(t, filepath.Join(sys.Home, ".claude", "skills", "grill"), filepath.Join(catalog, "skills", "grill"))
	assertLink(t, filepath.Join(sys.Home, ".codex", "AGENTS.md"), filepath.Join(catalog, "AGENTS.md"))
}

package cmd

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestAddRemoveBackupTag(t *testing.T) {
	project := registry.Project{Tags: []string{"alpha"}}

	if !addBackupTag(&project) {
		t.Fatal("expected backup tag to be added")
	}
	if !reflect.DeepEqual(project.Tags, []string{"alpha", "backup"}) {
		t.Fatalf("tags = %v, want [alpha backup]", project.Tags)
	}
	if addBackupTag(&project) {
		t.Fatal("expected duplicate backup tag to be ignored")
	}

	if !removeBackupTag(&project) {
		t.Fatal("expected backup tag to be removed")
	}
	if !reflect.DeepEqual(project.Tags, []string{"alpha"}) {
		t.Fatalf("tags = %v, want [alpha]", project.Tags)
	}
	if removeBackupTag(&project) {
		t.Fatal("expected missing backup tag removal to be ignored")
	}
}

func TestGitBackupFilesIncludesUntrackedNonIgnored(t *testing.T) {
	requireGit(t)

	dir := t.TempDir()
	git(t, dir, "init")
	writeFile(t, filepath.Join(dir, ".gitignore"), "ignored.txt\nnode_modules/\n")
	writeFile(t, filepath.Join(dir, "tracked.txt"), "tracked")
	writeFile(t, filepath.Join(dir, "untracked.txt"), "untracked")
	writeFile(t, filepath.Join(dir, "ignored.txt"), "ignored")
	writeFile(t, filepath.Join(dir, ".env"), "secret")
	writeFile(t, filepath.Join(dir, ".env.local"), "secret")
	writeFile(t, filepath.Join(dir, "node_modules/pkg/index.js"), "dependency")
	git(t, dir, "add", ".gitignore", "tracked.txt")

	files, usedGit, err := gitBackupFiles(dir)
	if err != nil {
		t.Fatalf("gitBackupFiles returned error: %v", err)
	}
	if !usedGit {
		t.Fatal("expected gitBackupFiles to use git")
	}

	want := []string{".gitignore", "tracked.txt", "untracked.txt"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
}

func TestCleanBackupFileListExcludesTopLevelGeneratedDirs(t *testing.T) {
	entries := [][]byte{
		[]byte("src/App.sol"),
		[]byte("lib/openzeppelin/src/ERC20.sol"),
		[]byte("broadcast/deploy.json"),
		[]byte("MyEnv/bin/python"),
		[]byte("artifacts/Contract.json"),
		[]byte("cache/solidity-files-cache.json"),
		[]byte("tmp/scratch.txt"),
		[]byte(".worktrees/feature/file.txt"),
		[]byte("web/tsconfig.tsbuildinfo"),
		[]byte("tests/fixtures/node_modules/pkg/index.js"),
		[]byte("bot.session"),
		[]byte("offchain/lib/cache.py"),
		[]byte("shared/artifacts/store.py"),
	}

	files := cleanBackupFileList(entries)
	want := []string{"offchain/lib/cache.py", "shared/artifacts/store.py", "src/App.sol"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
}

func TestRunBackupCreatesTimestampedZip(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, "keep.txt"), "keep")
	writeFile(t, filepath.Join(projectDir, ".env"), "secret")
	writeFile(t, filepath.Join(projectDir, "node_modules/pkg/index.js"), "dependency")

	origLoad := backupLoadRegistry
	origSave := backupSaveRegistry
	origNow := backupNow
	defer func() {
		backupLoadRegistry = origLoad
		backupSaveRegistry = origSave
		backupNow = origNow
	}()

	backupLoadRegistry = func() (*registry.Registry, error) {
		return &registry.Registry{
			Version:  2,
			Settings: registry.Settings{BaseDir: homeDir},
			Projects: map[string]registry.Project{
				"alpha": {
					Path:        projectDir,
					Category:    registry.CategoryCLI,
					Lang:        registry.LanguageBase,
					Created:     "2026-01-01",
					Description: "alpha project",
					Tags:        []string{"backup"},
					Status:      registry.Status{State: registry.StateActive},
				},
			},
		}, nil
	}
	backupSaveRegistry = func(*registry.Registry) error { return nil }
	backupNow = func() time.Time { return time.Date(2026, 4, 29, 12, 34, 56, 0, time.UTC) }

	if err := runBackup(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runBackup returned error: %v", err)
	}

	zipPath := filepath.Join(homeDir, "overlord-backup-20260429-123456.zip")
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to open backup zip: %v", err)
	}
	defer reader.Close()

	var names []string
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	want := []string{"overlord-backup-20260429-123456/alpha/keep.txt"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("zip entries = %v, want %v", names, want)
	}
}

func TestBackupListProjectsSplitsBackupMembership(t *testing.T) {
	reg := testRegistry(map[string]registry.Project{
		"alpha": testProject("projects/alpha", registry.CategoryCLI, registry.LanguageGo, registry.StateActive),
		"beta":  testProject("projects/beta", registry.CategoryWeb, registry.LanguageTypeScript, registry.StateActive),
	})
	alpha := reg.Projects["alpha"]
	alpha.Tags = []string{"backup"}
	reg.Projects["alpha"] = alpha

	backedUp, notBackedUp := splitBackupListProjects(backupListProjects(reg))

	if len(backedUp) != 1 || backedUp[0].name != "alpha" {
		t.Fatalf("backedUp = %+v, want alpha", backedUp)
	}
	if len(notBackedUp) != 1 || notBackedUp[0].name != "beta" {
		t.Fatalf("notBackedUp = %+v, want beta", notBackedUp)
	}
}

func TestSortBackupListProjectsActiveRecentCategoryThenArchived(t *testing.T) {
	projects := []backupListProject{
		{name: "archived-recent", status: "archived", lastOpened: "2026-01-10T00:00:00Z", category: "cli", lang: "go"},
		{name: "active-old", status: "active", lastOpened: "2026-01-01T00:00:00Z", category: "web", lang: "typescript"},
		{name: "active-new", status: "active", lastOpened: "2026-01-09T00:00:00Z", category: "contracts", lang: "solidity"},
		{name: "active-no-recent-a", status: "active", category: "cli", lang: "go"},
		{name: "active-no-recent-b", status: "active", category: "web", lang: "typescript"},
	}

	sortBackupListProjects(projects)

	got := make([]string, 0, len(projects))
	for _, project := range projects {
		got = append(got, project.name)
	}
	want := []string{"active-new", "active-old", "active-no-recent-a", "active-no-recent-b", "archived-recent"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted projects = %v, want %v", got, want)
	}
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

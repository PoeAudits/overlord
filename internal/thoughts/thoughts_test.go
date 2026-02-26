package thoughts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

var subdirNames = []string{"plans", "logs", "docs", "research", "sessions", "handoffs", "briefs", "audits", "reviews"}

func TestPathVariants(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := "delta"

	tests := []struct {
		name string
		fn   func(string, string) Paths
		base string
	}{
		{
			name: "active",
			fn:   ProjectPaths,
			base: filepath.Join(root, "projects", project),
		},
		{
			name: "archive",
			fn:   ArchivePaths,
			base: filepath.Join(root, "archive", project),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := tt.fn(root, project)
			expected := expectedPathMap(tt.base)
			assertPathsMatch(t, paths, expected)
		})
	}
}

func TestPathsToSlice(t *testing.T) {
	t.Parallel()
	paths := Paths{
		Plans:    "plans",
		Logs:     "logs",
		Docs:     "docs",
		Research: "research",
		Sessions: "sessions",
		Handoffs: "handoffs",
		Briefs:   "briefs",
		Audits:   "audits",
		Reviews:  "reviews",
	}

	wantSlice := []string{paths.Plans, paths.Logs, paths.Docs, paths.Research, paths.Sessions, paths.Handoffs, paths.Briefs, paths.Audits, paths.Reviews}
	gotSlice := paths.ToSlice()
	if len(gotSlice) != len(wantSlice) {
		t.Fatalf("expected %d entries, got %d", len(wantSlice), len(gotSlice))
	}
	for i, want := range wantSlice {
		if gotSlice[i] != want {
			t.Fatalf("slice entry %d mismatch: want %q got %q", i, want, gotSlice[i])
		}
	}
}

func TestExist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := "ghost"
	if Exist(root, project) {
		t.Fatalf("expected thoughts to be absent")
	}

	paths := expectedPathMap(filepath.Join(root, "projects", project))
	if err := os.MkdirAll(paths["plans"], 0o755); err != nil {
		t.Fatalf("failed to create plans directory: %v", err)
	}

	if !Exist(root, project) {
		t.Fatalf("expected thoughts to exist after directory creation")
	}
}

func TestCreateStructure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	project := "nova"
	if err := CreateStructure(root, project); err != nil {
		t.Fatalf("CreateStructure failed: %v", err)
	}

	paths := expectedPathMap(filepath.Join(root, "projects", project))
	for name, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
		if !info.IsDir() {
			t.Fatalf("expected %s to be directory", name)
		}
	}
}

func TestEnsureProjectSymlinkCreatesLinks(t *testing.T) {
	t.Parallel()
	thoughtsDir := t.TempDir()
	project := "atlas"
	paths := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	ensureDirectories(t, paths)

	projectPath := t.TempDir()
	writeFile(t, filepath.Join(projectPath, "README.md"), "readme")
	writeFile(t, filepath.Join(projectPath, "AGENTS.md"), "agents")

	if err := EnsureProjectSymlink(thoughtsDir, project, projectPath); err != nil {
		t.Fatalf("EnsureProjectSymlink failed: %v", err)
	}

	docsDir := paths["docs"]
	assertSymlinkTarget(t, filepath.Join(docsDir, "README.md"), filepath.Join(projectPath, "README.md"))
	assertSymlinkTarget(t, filepath.Join(docsDir, "AGENTS.md"), filepath.Join(projectPath, "AGENTS.md"))
}

func TestUpdateSymlinksRefreshesTargets(t *testing.T) {
	t.Parallel()
	thoughtsDir := t.TempDir()
	project := "relay"
	paths := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	ensureDirectories(t, paths)

	projectPath := t.TempDir()
	writeFile(t, filepath.Join(projectPath, "README.md"), "alpha")
	writeFile(t, filepath.Join(projectPath, "AGENTS.md"), "alpha")

	if err := EnsureProjectSymlink(thoughtsDir, project, projectPath); err != nil {
		t.Fatalf("EnsureProjectSymlink failed: %v", err)
	}

	newProjectPath := t.TempDir()
	writeFile(t, filepath.Join(newProjectPath, "README.md"), "beta")
	writeFile(t, filepath.Join(newProjectPath, "AGENTS.md"), "beta")

	if err := UpdateSymlinks(thoughtsDir, project, newProjectPath); err != nil {
		t.Fatalf("UpdateSymlinks failed: %v", err)
	}

	docsDir := paths["docs"]
	assertSymlinkTarget(t, filepath.Join(docsDir, "README.md"), filepath.Join(newProjectPath, "README.md"))
	assertSymlinkTarget(t, filepath.Join(docsDir, "AGENTS.md"), filepath.Join(newProjectPath, "AGENTS.md"))
}

func TestMoveToArchive(t *testing.T) {
	t.Parallel()
	thoughtsDir := t.TempDir()
	project := "voyager"
	activeBase := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	ensureDirectories(t, activeBase)
	writeFile(t, filepath.Join(activeBase["plans"], "plan.md"), "plan")

	if err := MoveToArchive(thoughtsDir, project); err != nil {
		t.Fatalf("MoveToArchive failed: %v", err)
	}

	for name, path := range activeBase {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected %s to be removed from active location", name)
		}
	}

	archiveBase := expectedPathMap(filepath.Join(thoughtsDir, "archive", project))
	for name, path := range archiveBase {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist in archive: %v", name, err)
		}
	}
	assertFileContents(t, filepath.Join(archiveBase["plans"], "plan.md"), "plan")
}

func TestMoveFromArchive(t *testing.T) {
	t.Parallel()
	thoughtsDir := t.TempDir()
	project := "return"
	archiveBase := expectedPathMap(filepath.Join(thoughtsDir, "archive", project))
	ensureDirectories(t, archiveBase)
	writeFile(t, filepath.Join(archiveBase["logs"], "entry.md"), "entry")

	if err := MoveFromArchive(thoughtsDir, project); err != nil {
		t.Fatalf("MoveFromArchive failed: %v", err)
	}

	activeBase := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	for name, path := range activeBase {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist after restore: %v", name, err)
		}
	}
	assertFileContents(t, filepath.Join(activeBase["logs"], "entry.md"), "entry")

	if _, err := os.Stat(filepath.Join(thoughtsDir, "archive", project)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected archive base to be removed after restore")
	}
}

func assertPathsMatch(t *testing.T, paths Paths, expected map[string]string) {
	t.Helper()
	checks := map[string]string{
		"plans":    paths.Plans,
		"logs":     paths.Logs,
		"docs":     paths.Docs,
		"research": paths.Research,
		"sessions": paths.Sessions,
		"handoffs": paths.Handoffs,
		"briefs":   paths.Briefs,
		"audits":   paths.Audits,
		"reviews":  paths.Reviews,
	}
	for name, got := range checks {
		if want := expected[name]; got != want {
			t.Fatalf("%s mismatch: want %q got %q", name, want, got)
		}
	}
}

func expectedPathMap(base string) map[string]string {
	return map[string]string{
		"plans":    filepath.Join(base, "plans"),
		"logs":     filepath.Join(base, "logs"),
		"docs":     filepath.Join(base, "docs"),
		"research": filepath.Join(base, "research"),
		"sessions": filepath.Join(base, "sessions"),
		"handoffs": filepath.Join(base, "handoffs"),
		"briefs":   filepath.Join(base, "briefs"),
		"audits":   filepath.Join(base, "audits"),
		"reviews":  filepath.Join(base, "reviews"),
	}
}

func ensureDirectories(t *testing.T, dirs map[string]string) {
	t.Helper()
	for name, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent directories: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func assertSymlinkTarget(t *testing.T, linkPath, wantTarget string) {
	t.Helper()
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("expected symlink %s: %v", linkPath, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink", linkPath)
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("failed to read symlink %s: %v", linkPath, err)
	}
	resolved := target
	if !filepath.IsAbs(target) {
		resolved = filepath.Clean(filepath.Join(filepath.Dir(linkPath), target))
	}
	if filepath.Clean(resolved) != filepath.Clean(wantTarget) {
		t.Fatalf("symlink %s target mismatch: want %q got %q", linkPath, wantTarget, resolved)
	}
}

func assertFileContents(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	if string(b) != want {
		t.Fatalf("file %s contents mismatch: want %q got %q", path, want, string(b))
	}
}

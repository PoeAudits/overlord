package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetThoughtsPaths(t *testing.T) {
	paths := GetThoughtsPaths("/home/user/thoughts", "my-project")

	if paths.Briefs != "/home/user/thoughts/projects/my-project/briefs" {
		t.Fatalf("Briefs = %q", paths.Briefs)
	}
	if paths.Research != "/home/user/thoughts/projects/my-project/research" {
		t.Fatalf("Research = %q", paths.Research)
	}
	if paths.Audits != "/home/user/thoughts/projects/my-project/audits" {
		t.Fatalf("Audits = %q", paths.Audits)
	}
	if paths.Plans != "/home/user/thoughts/projects/my-project/plans" {
		t.Fatalf("Plans = %q", paths.Plans)
	}
	if paths.Logs != "/home/user/thoughts/projects/my-project/logs" {
		t.Fatalf("Logs = %q", paths.Logs)
	}
	if paths.Reviews != "/home/user/thoughts/projects/my-project/reviews" {
		t.Fatalf("Reviews = %q", paths.Reviews)
	}
	if paths.Docs != "/home/user/thoughts/projects/my-project/docs" {
		t.Fatalf("Docs = %q", paths.Docs)
	}
}

func TestGetArchiveThoughtsPaths(t *testing.T) {
	paths := GetArchiveThoughtsPaths("/home/user/thoughts", "my-project")

	if paths.Briefs != "/home/user/thoughts/archive/my-project/briefs" {
		t.Fatalf("Briefs = %q", paths.Briefs)
	}
	if paths.Research != "/home/user/thoughts/archive/my-project/research" {
		t.Fatalf("Research = %q", paths.Research)
	}
	if paths.Audits != "/home/user/thoughts/archive/my-project/audits" {
		t.Fatalf("Audits = %q", paths.Audits)
	}
	if paths.Plans != "/home/user/thoughts/archive/my-project/plans" {
		t.Fatalf("Plans = %q", paths.Plans)
	}
	if paths.Logs != "/home/user/thoughts/archive/my-project/logs" {
		t.Fatalf("Logs = %q", paths.Logs)
	}
	if paths.Reviews != "/home/user/thoughts/archive/my-project/reviews" {
		t.Fatalf("Reviews = %q", paths.Reviews)
	}
	if paths.Docs != "/home/user/thoughts/archive/my-project/docs" {
		t.Fatalf("Docs = %q", paths.Docs)
	}
}

func TestThoughtsPathsCollections(t *testing.T) {
	paths := ThoughtsPaths{
		Briefs:   "/a/briefs",
		Research: "/a/research",
		Audits:   "/a/audits",
		Plans:    "/a/plans",
		Logs:     "/a/logs",
		Reviews:  "/a/reviews",
		Docs:     "/a/docs",
	}

	slice := paths.toSlice()
	if len(slice) != 7 {
		t.Fatalf("expected 7 paths, got %d", len(slice))
	}

	expected := []string{"/a/briefs", "/a/research", "/a/audits", "/a/plans", "/a/logs", "/a/reviews", "/a/docs"}
	for i, want := range expected {
		if slice[i] != want {
			t.Fatalf("slice[%d] = %q, want %q", i, slice[i], want)
		}
	}

	m := paths.toMap()
	if len(m) != 7 {
		t.Fatalf("expected 7 map entries, got %d", len(m))
	}
}

func TestThoughtsExist(t *testing.T) {
	tmpDir := t.TempDir()
	paths := ThoughtsPaths{
		Briefs:   filepath.Join(tmpDir, "briefs"),
		Research: filepath.Join(tmpDir, "research"),
		Audits:   filepath.Join(tmpDir, "audits"),
		Plans:    filepath.Join(tmpDir, "plans"),
		Logs:     filepath.Join(tmpDir, "logs"),
		Reviews:  filepath.Join(tmpDir, "reviews"),
		Docs:     filepath.Join(tmpDir, "docs"),
	}

	exists, missing := ThoughtsExist(paths)
	if exists {
		t.Fatal("expected exists=false")
	}
	if len(missing) != 7 {
		t.Fatalf("expected 7 missing, got %d", len(missing))
	}

	if err := os.MkdirAll(paths.Plans, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Logs, 0o755); err != nil {
		t.Fatal(err)
	}

	exists, missing = ThoughtsExist(paths)
	if !exists {
		t.Fatal("expected exists=true")
	}
	if len(missing) != 5 {
		t.Fatalf("expected 5 missing, got %d", len(missing))
	}
}

func TestMoveThoughtsToArchive(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "test-project"

	srcPaths := GetThoughtsPaths(tmpDir, projectName)
	for _, p := range srcPaths.toSlice() {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "test.txt"), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	warnings, err := MoveThoughtsToArchive(tmpDir, projectName)
	if err != nil {
		t.Fatalf("MoveThoughtsToArchive failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	dstPaths := GetArchiveThoughtsPaths(tmpDir, projectName)
	for _, p := range dstPaths.toSlice() {
		if _, err := os.Stat(filepath.Join(p, "test.txt")); err != nil {
			t.Fatalf("expected moved file in %s: %v", p, err)
		}
	}
}

func TestMoveThoughtsFromArchive(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "test-project"

	archivePaths := GetArchiveThoughtsPaths(tmpDir, projectName)
	for _, p := range archivePaths.toSlice() {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "test.txt"), []byte("test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	warnings, err := MoveThoughtsFromArchive(tmpDir, projectName)
	if err != nil {
		t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	mainPaths := GetThoughtsPaths(tmpDir, projectName)
	for _, p := range mainPaths.toSlice() {
		if _, err := os.Stat(filepath.Join(p, "test.txt")); err != nil {
			t.Fatalf("expected restored file in %s: %v", p, err)
		}
	}
}

func TestUpdateProjectSymlinks(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "project")
	docsDir := filepath.Join(tmpDir, "thoughts", "projects", "test", "docs")

	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# README"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateProjectSymlinks(docsDir, projectDir); err != nil {
		t.Fatalf("UpdateProjectSymlinks failed: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(docsDir, "README.md")); err != nil {
		t.Fatalf("README symlink missing: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(docsDir, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatal("AGENTS.md symlink should not exist when target missing")
	}
}

func TestEnsureProjectThoughtsSymlink(t *testing.T) {
	t.Run("creates active symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}

		warning, err := EnsureProjectThoughtsSymlink(filepath.Join(tmpDir, "thoughts"), "my-project", projectDir, false)
		if err != nil {
			t.Fatalf("EnsureProjectThoughtsSymlink failed: %v", err)
		}
		if warning != "" {
			t.Fatalf("unexpected warning: %s", warning)
		}

		link := filepath.Join(projectDir, "thoughts")
		target, err := os.Readlink(link)
		if err != nil {
			t.Fatalf("failed reading link: %v", err)
		}
		if target != filepath.Join("..", "thoughts", "projects", "my-project") {
			t.Fatalf("unexpected target: %s", target)
		}
	})

	t.Run("updates wrong symlink target", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink("/wrong/target", filepath.Join(projectDir, "thoughts")); err != nil {
			t.Fatal(err)
		}

		warning, err := EnsureProjectThoughtsSymlink(filepath.Join(tmpDir, "thoughts"), "my-project", projectDir, true)
		if err != nil {
			t.Fatalf("EnsureProjectThoughtsSymlink failed: %v", err)
		}
		if warning != "" {
			t.Fatalf("unexpected warning: %s", warning)
		}

		target, err := os.Readlink(filepath.Join(projectDir, "thoughts"))
		if err != nil {
			t.Fatal(err)
		}
		if target != filepath.Join("..", "thoughts", "archive", "my-project") {
			t.Fatalf("unexpected target: %s", target)
		}
	})

	t.Run("warns when thoughts exists as non-symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		if err := os.MkdirAll(filepath.Join(projectDir, "thoughts"), 0o755); err != nil {
			t.Fatal(err)
		}

		warning, err := EnsureProjectThoughtsSymlink(filepath.Join(tmpDir, "thoughts"), "my-project", projectDir, false)
		if err != nil {
			t.Fatalf("EnsureProjectThoughtsSymlink failed: %v", err)
		}
		if warning == "" {
			t.Fatal("expected warning for non-symlink thoughts path")
		}
	})
}

func TestMoveDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "src")
	dstDir := filepath.Join(tmpDir, "dst")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := moveDirectory(srcDir, dstDir); err != nil {
		t.Fatalf("moveDirectory failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "test.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "empty")
	if err := os.MkdirAll(emptyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeEmptyDir(emptyDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
		t.Fatal("expected empty directory removed")
	}
}

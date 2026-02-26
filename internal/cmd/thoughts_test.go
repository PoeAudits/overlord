package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureProjectThoughtsSymlink(t *testing.T) {
	t.Run("creates active symlink and doc links", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		docsDir := filepath.Join(thoughtsDir, "projects", "my-project", "docs")

		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# README"), 0o644); err != nil {
			t.Fatal(err)
		}

		warning, err := EnsureProjectThoughtsSymlink(thoughtsDir, "my-project", projectDir, false)
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

		readmePath := filepath.Join(docsDir, "README.md")
		content, err := os.ReadFile(readmePath)
		if err != nil {
			t.Fatalf("reading README symlink failed: %v", err)
		}
		if string(content) != "# README" {
			t.Fatalf("unexpected README content: %s", content)
		}
	})

	t.Run("updates wrong symlink target for archive", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		docsDir := filepath.Join(thoughtsDir, "archive", "my-project", "docs")

		if err := os.MkdirAll(projectDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), []byte("# Agents"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := os.Symlink("/wrong/target", filepath.Join(projectDir, "thoughts")); err != nil {
			t.Fatal(err)
		}

		warning, err := EnsureProjectThoughtsSymlink(thoughtsDir, "my-project", projectDir, true)
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

		agentsPath := filepath.Join(docsDir, "AGENTS.md")
		content, err := os.ReadFile(agentsPath)
		if err != nil {
			t.Fatalf("reading AGENTS symlink failed: %v", err)
		}
		if string(content) != "# Agents" {
			t.Fatalf("unexpected AGENTS content: %s", content)
		}
	})

	t.Run("warns when thoughts exists as directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectDir := filepath.Join(tmpDir, "project")
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		docsDir := filepath.Join(thoughtsDir, "projects", "my-project", "docs")

		if err := os.MkdirAll(filepath.Join(projectDir, "thoughts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(docsDir, 0o755); err != nil {
			t.Fatal(err)
		}

		warning, err := EnsureProjectThoughtsSymlink(thoughtsDir, "my-project", projectDir, false)
		if err != nil {
			t.Fatalf("EnsureProjectThoughtsSymlink failed: %v", err)
		}
		if warning == "" {
			t.Fatal("expected warning for non-symlink thoughts path")
		}
	})
}

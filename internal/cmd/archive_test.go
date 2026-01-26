package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

// TestArchiveMovesThoughts tests that the archive command moves thoughts directories
func TestArchiveMovesThoughts(t *testing.T) {
	t.Run("archive moves thoughts to archive location", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "test-project"

		// Create project directory
		projectDir := filepath.Join(tmpDir, "Overlord", "projects", "web", projectName)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# Test"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), []byte("# Agents"), 0644); err != nil {
			t.Fatalf("failed to create AGENTS.md: %v", err)
		}

		// Create thoughts directories with content
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		srcPaths := GetThoughtsPaths(thoughtsDir, projectName)
		for _, p := range srcPaths.toSlice() {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatalf("failed to create thoughts dir: %v", err)
			}
			testFile := filepath.Join(p, "test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		// Create registry
		reg := &registry.Registry{
			Version: 2,
			Settings: registry.Settings{
				BaseDir:     filepath.Join(tmpDir, "Overlord"),
				ThoughtsDir: thoughtsDir,
			},
			Projects: map[string]registry.Project{
				projectName: {
					Path:        "projects/web/" + projectName,
					Category:    registry.CategoryWeb,
					Lang:        registry.LanguageGo,
					Created:     "2024-01-01",
					Description: "Test project",
					Status:      registry.Status{State: registry.StateActive},
				},
			},
		}

		// Save registry
		registryPath := filepath.Join(tmpDir, "registry.yaml")
		if err := registry.Save(registryPath, reg); err != nil {
			t.Fatalf("failed to save registry: %v", err)
		}

		// Simulate archive operation - move project
		archiveDir := filepath.Join(tmpDir, "Overlord", "archive", projectName)
		if err := os.MkdirAll(filepath.Dir(archiveDir), 0755); err != nil {
			t.Fatalf("failed to create archive parent: %v", err)
		}
		if err := os.Rename(projectDir, archiveDir); err != nil {
			t.Fatalf("failed to move project: %v", err)
		}

		// Call MoveThoughtsToArchive (the function we're testing)
		warnings, err := MoveThoughtsToArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsToArchive failed: %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}

		// Verify source thoughts directories no longer exist
		for _, p := range srcPaths.toSlice() {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("source thoughts directory should not exist: %s", p)
			}
		}

		// Verify archive thoughts directories exist with content
		dstPaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		for _, p := range dstPaths.toSlice() {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				t.Errorf("archive thoughts directory should exist: %s", p)
			}
			testFile := filepath.Join(p, "test.txt")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				t.Errorf("test file should exist in archive: %s", testFile)
			}
		}

		// Update symlinks
		if err := UpdateProjectSymlinks(dstPaths.Docs, archiveDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlinks point to archive location
		readmeLink := filepath.Join(dstPaths.Docs, "README.md")
		if info, err := os.Lstat(readmeLink); err != nil {
			t.Errorf("README.md symlink should exist: %v", err)
		} else if info.Mode()&os.ModeSymlink == 0 {
			t.Error("README.md should be a symlink")
		}

		// Verify symlink resolves correctly
		if content, err := os.ReadFile(readmeLink); err != nil {
			t.Errorf("failed to read through README.md symlink: %v", err)
		} else if string(content) != "# Test" {
			t.Errorf("README.md content mismatch: got %q", string(content))
		}
	})

	t.Run("archive warns for missing thoughts directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "partial-thoughts-project"

		// Create only plans and logs (not sessions or projects)
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		srcPaths := GetThoughtsPaths(thoughtsDir, projectName)
		if err := os.MkdirAll(srcPaths.Plans, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}
		if err := os.MkdirAll(srcPaths.Logs, 0755); err != nil {
			t.Fatalf("failed to create logs dir: %v", err)
		}

		// Call MoveThoughtsToArchive
		warnings, err := MoveThoughtsToArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsToArchive failed: %v", err)
		}

		// Should have 6 warnings for missing subdirectories (docs, research, sessions, handoffs, reviews, briefs)
		if len(warnings) != 6 {
			t.Errorf("expected 6 warnings, got %d: %v", len(warnings), warnings)
		}

		// Verify plans and logs were moved
		dstPaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		if _, err := os.Stat(dstPaths.Plans); os.IsNotExist(err) {
			t.Error("plans should be moved to archive")
		}
		if _, err := os.Stat(dstPaths.Logs); os.IsNotExist(err) {
			t.Error("logs should be moved to archive")
		}
	})

	t.Run("archive continues when no thoughts exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "no-thoughts-project"
		thoughtsDir := filepath.Join(tmpDir, "thoughts")

		// Don't create any thoughts directories

		// Call MoveThoughtsToArchive
		warnings, err := MoveThoughtsToArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsToArchive failed: %v", err)
		}

		// Should have 8 warnings for missing directories, plus 1 for failed cleanup
		if len(warnings) < 8 {
			t.Errorf("expected at least 8 warnings, got %d: %v", len(warnings), warnings)
		}
	})
}

// TestArchiveSymlinkUpdate tests that symlinks are updated after archive
func TestArchiveSymlinkUpdate(t *testing.T) {
	t.Run("symlinks updated to point to archive location", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "symlink-test"

		// Create archive project directory with README.md and AGENTS.md
		archiveProjectDir := filepath.Join(tmpDir, "Overlord", "archive", projectName)
		if err := os.MkdirAll(archiveProjectDir, 0755); err != nil {
			t.Fatalf("failed to create archive project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(archiveProjectDir, "README.md"), []byte("# Archived"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(archiveProjectDir, "AGENTS.md"), []byte("# Archived Agents"), 0644); err != nil {
			t.Fatalf("failed to create AGENTS.md: %v", err)
		}

		// Create archived thoughts docs directory (symlinks go in docs/)
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		archiveThoughtsPaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		if err := os.MkdirAll(archiveThoughtsPaths.Docs, 0755); err != nil {
			t.Fatalf("failed to create archive thoughts docs dir: %v", err)
		}

		// Update symlinks
		if err := UpdateProjectSymlinks(archiveThoughtsPaths.Docs, archiveProjectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlinks exist and resolve correctly
		readmeLink := filepath.Join(archiveThoughtsPaths.Docs, "README.md")
		if content, err := os.ReadFile(readmeLink); err != nil {
			t.Errorf("failed to read through README.md symlink: %v", err)
		} else if string(content) != "# Archived" {
			t.Errorf("README.md content mismatch: got %q", string(content))
		}

		agentsLink := filepath.Join(archiveThoughtsPaths.Docs, "AGENTS.md")
		if content, err := os.ReadFile(agentsLink); err != nil {
			t.Errorf("failed to read through AGENTS.md symlink: %v", err)
		} else if string(content) != "# Archived Agents" {
			t.Errorf("AGENTS.md content mismatch: got %q", string(content))
		}
	})

	t.Run("symlinks skip missing target files", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "partial-symlink-test"

		// Create archive project directory with only README.md (no AGENTS.md)
		archiveProjectDir := filepath.Join(tmpDir, "Overlord", "archive", projectName)
		if err := os.MkdirAll(archiveProjectDir, 0755); err != nil {
			t.Fatalf("failed to create archive project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(archiveProjectDir, "README.md"), []byte("# Archived"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}

		// Create archived thoughts docs directory (symlinks go in docs/)
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		archiveThoughtsPaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		if err := os.MkdirAll(archiveThoughtsPaths.Docs, 0755); err != nil {
			t.Fatalf("failed to create archive thoughts docs dir: %v", err)
		}

		// Update symlinks
		if err := UpdateProjectSymlinks(archiveThoughtsPaths.Docs, archiveProjectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// README.md symlink should exist
		readmeLink := filepath.Join(archiveThoughtsPaths.Docs, "README.md")
		if _, err := os.Lstat(readmeLink); err != nil {
			t.Errorf("README.md symlink should exist: %v", err)
		}

		// AGENTS.md symlink should NOT exist (target doesn't exist)
		agentsLink := filepath.Join(archiveThoughtsPaths.Docs, "AGENTS.md")
		if _, err := os.Lstat(agentsLink); !os.IsNotExist(err) {
			t.Error("AGENTS.md symlink should not exist when target is missing")
		}
	})
}

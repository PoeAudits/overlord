package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

// TestUnarchiveRestoresThoughts tests that the unarchive command restores thoughts directories
func TestUnarchiveRestoresThoughts(t *testing.T) {
	t.Run("unarchive restores thoughts from archive location", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "test-project"

		// Create archived project directory
		archiveProjectDir := filepath.Join(tmpDir, "Overlord", "archive", projectName)
		if err := os.MkdirAll(archiveProjectDir, 0755); err != nil {
			t.Fatalf("failed to create archive project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(archiveProjectDir, "README.md"), []byte("# Test"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(archiveProjectDir, "AGENTS.md"), []byte("# Agents"), 0644); err != nil {
			t.Fatalf("failed to create AGENTS.md: %v", err)
		}

		// Create archived thoughts directories with content
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		archivePaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		for _, p := range archivePaths.toSlice() {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatalf("failed to create archive thoughts dir: %v", err)
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
					Path:        "archive/" + projectName,
					Category:    registry.CategoryWeb,
					Lang:        registry.LanguageGo,
					Created:     "2024-01-01",
					Description: "Test project",
					Status:      registry.Status{State: registry.StateArchived},
				},
			},
		}

		// Save registry
		registryPath := filepath.Join(tmpDir, "registry.yaml")
		if err := registry.Save(registryPath, reg); err != nil {
			t.Fatalf("failed to save registry: %v", err)
		}

		// Simulate unarchive operation - move project to target category
		targetDir := filepath.Join(tmpDir, "Overlord", "projects", "web", projectName)
		if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
			t.Fatalf("failed to create target parent: %v", err)
		}
		if err := os.Rename(archiveProjectDir, targetDir); err != nil {
			t.Fatalf("failed to move project: %v", err)
		}

		// Call MoveThoughtsFromArchive (the function we're testing)
		warnings, err := MoveThoughtsFromArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}

		// Verify archive thoughts directories no longer exist
		for _, p := range archivePaths.toSlice() {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("archive thoughts directory should not exist: %s", p)
			}
		}

		// Verify main thoughts directories exist with content
		mainPaths := GetThoughtsPaths(thoughtsDir, projectName)
		for _, p := range mainPaths.toSlice() {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				t.Errorf("main thoughts directory should exist: %s", p)
			}
			testFile := filepath.Join(p, "test.txt")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				t.Errorf("test file should exist in main location: %s", testFile)
			}
		}

		// Update symlinks (symlinks go in docs/ subdirectory)
		if err := UpdateProjectSymlinks(mainPaths.Docs, targetDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlinks point to restored location
		readmeLink := filepath.Join(mainPaths.Docs, "README.md")
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

	t.Run("unarchive handles missing archive gracefully", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "legacy-project"
		thoughtsDir := filepath.Join(tmpDir, "thoughts")

		// Don't create any archived thoughts (simulates project archived before this feature)

		// Call MoveThoughtsFromArchive
		warnings, err := MoveThoughtsFromArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}

		// Should have 1 warning about no archived thoughts found
		if len(warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
		}

		// Verify warning message
		if len(warnings) > 0 && warnings[0] != "no archived thoughts found for legacy-project (project may have been archived before this feature)" {
			t.Errorf("unexpected warning message: %s", warnings[0])
		}
	})

	t.Run("unarchive warns for partial archive", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "partial-archive-project"

		// Create only plans and logs in archive (not all 8 subdirectories)
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		archivePaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		if err := os.MkdirAll(archivePaths.Plans, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}
		if err := os.MkdirAll(archivePaths.Logs, 0755); err != nil {
			t.Fatalf("failed to create logs dir: %v", err)
		}

		// Call MoveThoughtsFromArchive
		warnings, err := MoveThoughtsFromArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}

		// Should have 6 warnings (8 total - 2 created = 6 missing)
		if len(warnings) != 6 {
			t.Errorf("expected 6 warnings, got %d: %v", len(warnings), warnings)
		}

		// Verify plans and logs were restored
		mainPaths := GetThoughtsPaths(thoughtsDir, projectName)
		if _, err := os.Stat(mainPaths.Plans); os.IsNotExist(err) {
			t.Error("plans should be restored from archive")
		}
		if _, err := os.Stat(mainPaths.Logs); os.IsNotExist(err) {
			t.Error("logs should be restored from archive")
		}
	})

	t.Run("unarchive cleans up empty archive directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "cleanup-test"

		// Create all archived thoughts directories
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		archivePaths := GetArchiveThoughtsPaths(thoughtsDir, projectName)
		for _, p := range archivePaths.toSlice() {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatalf("failed to create archive thoughts dir: %v", err)
			}
		}

		// Call MoveThoughtsFromArchive
		warnings, err := MoveThoughtsFromArchive(thoughtsDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}

		// Verify archive base directory was cleaned up
		archiveBase := filepath.Join(thoughtsDir, "archive", projectName)
		if _, err := os.Stat(archiveBase); !os.IsNotExist(err) {
			t.Errorf("archive base directory should be removed: %s", archiveBase)
		}
	})
}

// TestUnarchiveSymlinkUpdate tests that symlinks are updated after unarchive
func TestUnarchiveSymlinkUpdate(t *testing.T) {
	t.Run("symlinks updated to point to restored location", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "symlink-restore-test"

		// Create restored project directory with README.md and AGENTS.md
		restoredProjectDir := filepath.Join(tmpDir, "Overlord", "projects", "web", projectName)
		if err := os.MkdirAll(restoredProjectDir, 0755); err != nil {
			t.Fatalf("failed to create restored project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(restoredProjectDir, "README.md"), []byte("# Restored"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(restoredProjectDir, "AGENTS.md"), []byte("# Restored Agents"), 0644); err != nil {
			t.Fatalf("failed to create AGENTS.md: %v", err)
		}

		// Create main thoughts docs directory (symlinks go in docs/)
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		mainThoughtsPaths := GetThoughtsPaths(thoughtsDir, projectName)
		if err := os.MkdirAll(mainThoughtsPaths.Docs, 0755); err != nil {
			t.Fatalf("failed to create main thoughts docs dir: %v", err)
		}

		// Update symlinks
		if err := UpdateProjectSymlinks(mainThoughtsPaths.Docs, restoredProjectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlinks exist and resolve correctly
		readmeLink := filepath.Join(mainThoughtsPaths.Docs, "README.md")
		if content, err := os.ReadFile(readmeLink); err != nil {
			t.Errorf("failed to read through README.md symlink: %v", err)
		} else if string(content) != "# Restored" {
			t.Errorf("README.md content mismatch: got %q", string(content))
		}

		agentsLink := filepath.Join(mainThoughtsPaths.Docs, "AGENTS.md")
		if content, err := os.ReadFile(agentsLink); err != nil {
			t.Errorf("failed to read through AGENTS.md symlink: %v", err)
		} else if string(content) != "# Restored Agents" {
			t.Errorf("AGENTS.md content mismatch: got %q", string(content))
		}
	})

	t.Run("symlinks updated when thoughts exist but not in archive", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "pre-existing-thoughts"

		// Create restored project directory
		restoredProjectDir := filepath.Join(tmpDir, "Overlord", "projects", "web", projectName)
		if err := os.MkdirAll(restoredProjectDir, 0755); err != nil {
			t.Fatalf("failed to create restored project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(restoredProjectDir, "README.md"), []byte("# Restored"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}

		// Create main thoughts docs directory (pre-existing, not from archive)
		thoughtsDir := filepath.Join(tmpDir, "thoughts")
		mainThoughtsPaths := GetThoughtsPaths(thoughtsDir, projectName)
		if err := os.MkdirAll(mainThoughtsPaths.Docs, 0755); err != nil {
			t.Fatalf("failed to create main thoughts docs dir: %v", err)
		}

		// Create an old symlink pointing to wrong location
		oldSymlink := filepath.Join(mainThoughtsPaths.Docs, "README.md")
		if err := os.Symlink("/nonexistent/path/README.md", oldSymlink); err != nil {
			t.Fatalf("failed to create old symlink: %v", err)
		}

		// Update symlinks (should replace old symlink)
		if err := UpdateProjectSymlinks(mainThoughtsPaths.Docs, restoredProjectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlink now points to correct location
		if content, err := os.ReadFile(oldSymlink); err != nil {
			t.Errorf("failed to read through README.md symlink: %v", err)
		} else if string(content) != "# Restored" {
			t.Errorf("README.md content mismatch: got %q", string(content))
		}
	})
}

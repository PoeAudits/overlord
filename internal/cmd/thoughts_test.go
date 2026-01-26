package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetThoughtsPaths(t *testing.T) {
	tests := []struct {
		name         string
		thoughtsDir  string
		projectName  string
		wantPlans    string
		wantLogs     string
		wantDocs     string
		wantResearch string
		wantSess     string
		wantHandoffs string
		wantReviews  string
		wantBriefs   string
	}{
		{
			name:         "basic paths",
			thoughtsDir:  "/home/user/thoughts",
			projectName:  "my-project",
			wantPlans:    "/home/user/thoughts/projects/my-project/plans",
			wantLogs:     "/home/user/thoughts/projects/my-project/logs",
			wantDocs:     "/home/user/thoughts/projects/my-project/docs",
			wantResearch: "/home/user/thoughts/projects/my-project/research",
			wantSess:     "/home/user/thoughts/projects/my-project/sessions",
			wantHandoffs: "/home/user/thoughts/projects/my-project/handoffs",
			wantReviews:  "/home/user/thoughts/projects/my-project/reviews",
			wantBriefs:   "/home/user/thoughts/projects/my-project/briefs",
		},
		{
			name:         "project with dashes",
			thoughtsDir:  "/thoughts",
			projectName:  "my-cool-project",
			wantPlans:    "/thoughts/projects/my-cool-project/plans",
			wantLogs:     "/thoughts/projects/my-cool-project/logs",
			wantDocs:     "/thoughts/projects/my-cool-project/docs",
			wantResearch: "/thoughts/projects/my-cool-project/research",
			wantSess:     "/thoughts/projects/my-cool-project/sessions",
			wantHandoffs: "/thoughts/projects/my-cool-project/handoffs",
			wantReviews:  "/thoughts/projects/my-cool-project/reviews",
			wantBriefs:   "/thoughts/projects/my-cool-project/briefs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := GetThoughtsPaths(tt.thoughtsDir, tt.projectName)

			if paths.Plans != tt.wantPlans {
				t.Errorf("Plans = %q, want %q", paths.Plans, tt.wantPlans)
			}
			if paths.Logs != tt.wantLogs {
				t.Errorf("Logs = %q, want %q", paths.Logs, tt.wantLogs)
			}
			if paths.Docs != tt.wantDocs {
				t.Errorf("Docs = %q, want %q", paths.Docs, tt.wantDocs)
			}
			if paths.Research != tt.wantResearch {
				t.Errorf("Research = %q, want %q", paths.Research, tt.wantResearch)
			}
			if paths.Sessions != tt.wantSess {
				t.Errorf("Sessions = %q, want %q", paths.Sessions, tt.wantSess)
			}
			if paths.Handoffs != tt.wantHandoffs {
				t.Errorf("Handoffs = %q, want %q", paths.Handoffs, tt.wantHandoffs)
			}
			if paths.Reviews != tt.wantReviews {
				t.Errorf("Reviews = %q, want %q", paths.Reviews, tt.wantReviews)
			}
			if paths.Briefs != tt.wantBriefs {
				t.Errorf("Briefs = %q, want %q", paths.Briefs, tt.wantBriefs)
			}
		})
	}
}

func TestGetArchiveThoughtsPaths(t *testing.T) {
	tests := []struct {
		name         string
		thoughtsDir  string
		projectName  string
		wantPlans    string
		wantLogs     string
		wantDocs     string
		wantResearch string
		wantSess     string
		wantHandoffs string
		wantReviews  string
		wantBriefs   string
	}{
		{
			name:         "basic archive paths",
			thoughtsDir:  "/home/user/thoughts",
			projectName:  "my-project",
			wantPlans:    "/home/user/thoughts/archive/my-project/plans",
			wantLogs:     "/home/user/thoughts/archive/my-project/logs",
			wantDocs:     "/home/user/thoughts/archive/my-project/docs",
			wantResearch: "/home/user/thoughts/archive/my-project/research",
			wantSess:     "/home/user/thoughts/archive/my-project/sessions",
			wantHandoffs: "/home/user/thoughts/archive/my-project/handoffs",
			wantReviews:  "/home/user/thoughts/archive/my-project/reviews",
			wantBriefs:   "/home/user/thoughts/archive/my-project/briefs",
		},
		{
			name:         "project with dashes",
			thoughtsDir:  "/thoughts",
			projectName:  "my-cool-project",
			wantPlans:    "/thoughts/archive/my-cool-project/plans",
			wantLogs:     "/thoughts/archive/my-cool-project/logs",
			wantDocs:     "/thoughts/archive/my-cool-project/docs",
			wantResearch: "/thoughts/archive/my-cool-project/research",
			wantSess:     "/thoughts/archive/my-cool-project/sessions",
			wantHandoffs: "/thoughts/archive/my-cool-project/handoffs",
			wantReviews:  "/thoughts/archive/my-cool-project/reviews",
			wantBriefs:   "/thoughts/archive/my-cool-project/briefs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := GetArchiveThoughtsPaths(tt.thoughtsDir, tt.projectName)

			if paths.Plans != tt.wantPlans {
				t.Errorf("Plans = %q, want %q", paths.Plans, tt.wantPlans)
			}
			if paths.Logs != tt.wantLogs {
				t.Errorf("Logs = %q, want %q", paths.Logs, tt.wantLogs)
			}
			if paths.Docs != tt.wantDocs {
				t.Errorf("Docs = %q, want %q", paths.Docs, tt.wantDocs)
			}
			if paths.Research != tt.wantResearch {
				t.Errorf("Research = %q, want %q", paths.Research, tt.wantResearch)
			}
			if paths.Sessions != tt.wantSess {
				t.Errorf("Sessions = %q, want %q", paths.Sessions, tt.wantSess)
			}
			if paths.Handoffs != tt.wantHandoffs {
				t.Errorf("Handoffs = %q, want %q", paths.Handoffs, tt.wantHandoffs)
			}
			if paths.Reviews != tt.wantReviews {
				t.Errorf("Reviews = %q, want %q", paths.Reviews, tt.wantReviews)
			}
			if paths.Briefs != tt.wantBriefs {
				t.Errorf("Briefs = %q, want %q", paths.Briefs, tt.wantBriefs)
			}
		})
	}
}

func TestThoughtsPaths_toSlice(t *testing.T) {
	paths := ThoughtsPaths{
		Plans:    "/a/plans",
		Logs:     "/a/logs",
		Docs:     "/a/docs",
		Research: "/a/research",
		Sessions: "/a/sessions",
		Handoffs: "/a/handoffs",
		Reviews:  "/a/reviews",
		Briefs:   "/a/briefs",
	}

	slice := paths.toSlice()

	if len(slice) != 8 {
		t.Fatalf("expected 8 paths, got %d", len(slice))
	}

	// Order should be: plans, logs, docs, research, sessions, handoffs, reviews, briefs
	expected := []string{"/a/plans", "/a/logs", "/a/docs", "/a/research", "/a/sessions", "/a/handoffs", "/a/reviews", "/a/briefs"}
	for i, want := range expected {
		if slice[i] != want {
			t.Errorf("slice[%d] = %q, want %q", i, slice[i], want)
		}
	}
}

func TestThoughtsPaths_toMap(t *testing.T) {
	paths := ThoughtsPaths{
		Plans:    "/a/plans",
		Logs:     "/a/logs",
		Docs:     "/a/docs",
		Research: "/a/research",
		Sessions: "/a/sessions",
		Handoffs: "/a/handoffs",
		Reviews:  "/a/reviews",
		Briefs:   "/a/briefs",
	}

	m := paths.toMap()

	if len(m) != 8 {
		t.Fatalf("expected 8 entries, got %d", len(m))
	}

	tests := map[string]string{
		"plans":    "/a/plans",
		"logs":     "/a/logs",
		"docs":     "/a/docs",
		"research": "/a/research",
		"sessions": "/a/sessions",
		"handoffs": "/a/handoffs",
		"reviews":  "/a/reviews",
		"briefs":   "/a/briefs",
	}

	for key, want := range tests {
		if got := m[key]; got != want {
			t.Errorf("m[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestThoughtsExist(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	t.Run("no directories exist", func(t *testing.T) {
		paths := ThoughtsPaths{
			Plans:    filepath.Join(tmpDir, "nonexistent", "plans"),
			Logs:     filepath.Join(tmpDir, "nonexistent", "logs"),
			Docs:     filepath.Join(tmpDir, "nonexistent", "docs"),
			Research: filepath.Join(tmpDir, "nonexistent", "research"),
			Sessions: filepath.Join(tmpDir, "nonexistent", "sessions"),
			Handoffs: filepath.Join(tmpDir, "nonexistent", "handoffs"),
			Reviews:  filepath.Join(tmpDir, "nonexistent", "reviews"),
			Briefs:   filepath.Join(tmpDir, "nonexistent", "briefs"),
		}

		exists, missing := ThoughtsExist(paths)

		if exists {
			t.Error("expected exists=false when no directories exist")
		}
		if len(missing) != 8 {
			t.Errorf("expected 8 missing, got %d", len(missing))
		}
	})

	t.Run("all directories exist", func(t *testing.T) {
		// Create all directories
		paths := ThoughtsPaths{
			Plans:    filepath.Join(tmpDir, "all", "plans"),
			Logs:     filepath.Join(tmpDir, "all", "logs"),
			Docs:     filepath.Join(tmpDir, "all", "docs"),
			Research: filepath.Join(tmpDir, "all", "research"),
			Sessions: filepath.Join(tmpDir, "all", "sessions"),
			Handoffs: filepath.Join(tmpDir, "all", "handoffs"),
			Reviews:  filepath.Join(tmpDir, "all", "reviews"),
			Briefs:   filepath.Join(tmpDir, "all", "briefs"),
		}

		for _, p := range paths.toSlice() {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatalf("failed to create test dir: %v", err)
			}
		}

		exists, missing := ThoughtsExist(paths)

		if !exists {
			t.Error("expected exists=true when all directories exist")
		}
		if len(missing) != 0 {
			t.Errorf("expected 0 missing, got %d: %v", len(missing), missing)
		}
	})

	t.Run("some directories exist", func(t *testing.T) {
		// Create only plans and logs
		paths := ThoughtsPaths{
			Plans:    filepath.Join(tmpDir, "partial", "plans"),
			Logs:     filepath.Join(tmpDir, "partial", "logs"),
			Docs:     filepath.Join(tmpDir, "partial", "docs"),
			Research: filepath.Join(tmpDir, "partial", "research"),
			Sessions: filepath.Join(tmpDir, "partial", "sessions"),
			Handoffs: filepath.Join(tmpDir, "partial", "handoffs"),
			Reviews:  filepath.Join(tmpDir, "partial", "reviews"),
			Briefs:   filepath.Join(tmpDir, "partial", "briefs"),
		}

		if err := os.MkdirAll(paths.Plans, 0755); err != nil {
			t.Fatalf("failed to create test dir: %v", err)
		}
		if err := os.MkdirAll(paths.Logs, 0755); err != nil {
			t.Fatalf("failed to create test dir: %v", err)
		}

		exists, missing := ThoughtsExist(paths)

		if !exists {
			t.Error("expected exists=true when some directories exist")
		}
		if len(missing) != 6 {
			t.Errorf("expected 6 missing, got %d: %v", len(missing), missing)
		}

		// Check that docs, research, sessions, handoffs, reviews, briefs are in missing
		missingMap := make(map[string]bool)
		for _, m := range missing {
			missingMap[m] = true
		}
		expectedMissing := []string{"docs", "research", "sessions", "handoffs", "reviews", "briefs"}
		for _, expected := range expectedMissing {
			if !missingMap[expected] {
				t.Errorf("expected '%s' in missing list", expected)
			}
		}
	})
}

func TestMoveThoughtsToArchive(t *testing.T) {
	t.Run("move all directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "test-project"

		// Create source directories with content
		srcPaths := GetThoughtsPaths(tmpDir, projectName)
		for _, p := range srcPaths.toSlice() {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatalf("failed to create source dir: %v", err)
			}
			// Add a file to verify content is moved
			testFile := filepath.Join(p, "test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		// Move to archive
		warnings, err := MoveThoughtsToArchive(tmpDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsToArchive failed: %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}

		// Verify source directories no longer exist
		for _, p := range srcPaths.toSlice() {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("source directory should not exist: %s", p)
			}
		}

		// Verify archive directories exist with content
		dstPaths := GetArchiveThoughtsPaths(tmpDir, projectName)
		for _, p := range dstPaths.toSlice() {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				t.Errorf("archive directory should exist: %s", p)
			}
			testFile := filepath.Join(p, "test.txt")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				t.Errorf("test file should exist in archive: %s", testFile)
			}
		}
	})

	t.Run("warn for missing directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "partial-project"

		// Create only plans and logs
		srcPaths := GetThoughtsPaths(tmpDir, projectName)
		if err := os.MkdirAll(srcPaths.Plans, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}
		if err := os.MkdirAll(srcPaths.Logs, 0755); err != nil {
			t.Fatalf("failed to create logs dir: %v", err)
		}

		// Move to archive
		warnings, err := MoveThoughtsToArchive(tmpDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsToArchive failed: %v", err)
		}

		// Should have 6 warnings for docs, research, sessions, handoffs, reviews, briefs
		if len(warnings) != 6 {
			t.Errorf("expected 6 warnings, got %d: %v", len(warnings), warnings)
		}

		// Verify plans and logs were moved
		dstPaths := GetArchiveThoughtsPaths(tmpDir, projectName)
		if _, err := os.Stat(dstPaths.Plans); os.IsNotExist(err) {
			t.Error("plans should be moved to archive")
		}
		if _, err := os.Stat(dstPaths.Logs); os.IsNotExist(err) {
			t.Error("logs should be moved to archive")
		}
	})

	t.Run("all directories missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "nonexistent-project"

		// Don't create any directories

		warnings, err := MoveThoughtsToArchive(tmpDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsToArchive failed: %v", err)
		}

		// Should have 8 warnings (one for each missing directory)
		// Plus potentially one more for empty project directory cleanup
		if len(warnings) < 8 {
			t.Errorf("expected at least 8 warnings, got %d: %v", len(warnings), warnings)
		}
	})
}

func TestMoveThoughtsFromArchive(t *testing.T) {
	t.Run("restore all directories", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "test-project"

		// Create archive directories with content
		dstPaths := GetArchiveThoughtsPaths(tmpDir, projectName)
		for _, p := range dstPaths.toSlice() {
			if err := os.MkdirAll(p, 0755); err != nil {
				t.Fatalf("failed to create archive dir: %v", err)
			}
			// Add a file to verify content is moved
			testFile := filepath.Join(p, "test.txt")
			if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}
		}

		// Restore from archive
		warnings, err := MoveThoughtsFromArchive(tmpDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}
		if len(warnings) != 0 {
			t.Errorf("expected no warnings, got: %v", warnings)
		}

		// Verify archive directories no longer exist
		for _, p := range dstPaths.toSlice() {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("archive directory should not exist: %s", p)
			}
		}

		// Verify main directories exist with content
		srcPaths := GetThoughtsPaths(tmpDir, projectName)
		for _, p := range srcPaths.toSlice() {
			if _, err := os.Stat(p); os.IsNotExist(err) {
				t.Errorf("main directory should exist: %s", p)
			}
			testFile := filepath.Join(p, "test.txt")
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				t.Errorf("test file should exist in main: %s", testFile)
			}
		}

		// Verify archive base directory is cleaned up
		archiveBase := filepath.Join(tmpDir, "archive", projectName)
		if _, err := os.Stat(archiveBase); !os.IsNotExist(err) {
			t.Error("archive base directory should be removed when empty")
		}
	})

	t.Run("no archive exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "no-archive-project"

		// Don't create any archive directories

		warnings, err := MoveThoughtsFromArchive(tmpDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}

		// Should have 1 warning about no archive
		if len(warnings) != 1 {
			t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
		}
	})

	t.Run("partial archive", func(t *testing.T) {
		tmpDir := t.TempDir()
		projectName := "partial-archive"

		// Create only plans and logs in archive
		archivePaths := GetArchiveThoughtsPaths(tmpDir, projectName)
		if err := os.MkdirAll(archivePaths.Plans, 0755); err != nil {
			t.Fatalf("failed to create plans dir: %v", err)
		}
		if err := os.MkdirAll(archivePaths.Logs, 0755); err != nil {
			t.Fatalf("failed to create logs dir: %v", err)
		}

		// Restore from archive
		warnings, err := MoveThoughtsFromArchive(tmpDir, projectName)
		if err != nil {
			t.Fatalf("MoveThoughtsFromArchive failed: %v", err)
		}

		// Should have 6 warnings for docs, research, sessions, handoffs, reviews, briefs
		if len(warnings) != 6 {
			t.Errorf("expected 6 warnings, got %d: %v", len(warnings), warnings)
		}

		// Verify plans and logs were restored
		mainPaths := GetThoughtsPaths(tmpDir, projectName)
		if _, err := os.Stat(mainPaths.Plans); os.IsNotExist(err) {
			t.Error("plans should be restored")
		}
		if _, err := os.Stat(mainPaths.Logs); os.IsNotExist(err) {
			t.Error("logs should be restored")
		}
	})
}

func TestUpdateProjectSymlinks(t *testing.T) {
	t.Run("create symlinks for existing files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create project directory with README.md and AGENTS.md
		projectDir := filepath.Join(tmpDir, "project")
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# README"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "AGENTS.md"), []byte("# AGENTS"), 0644); err != nil {
			t.Fatalf("failed to create AGENTS.md: %v", err)
		}

		// Create thoughts project docs directory (new structure)
		thoughtsDir := filepath.Join(tmpDir, "thoughts", "projects", "test", "docs")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts dir: %v", err)
		}

		// Update symlinks
		if err := UpdateProjectSymlinks(thoughtsDir, projectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlinks exist and point to correct targets
		readmeLink := filepath.Join(thoughtsDir, "README.md")
		agentsLink := filepath.Join(thoughtsDir, "AGENTS.md")

		// Check README.md symlink
		if info, err := os.Lstat(readmeLink); err != nil {
			t.Errorf("README.md symlink should exist: %v", err)
		} else if info.Mode()&os.ModeSymlink == 0 {
			t.Error("README.md should be a symlink")
		}

		// Check AGENTS.md symlink
		if info, err := os.Lstat(agentsLink); err != nil {
			t.Errorf("AGENTS.md symlink should exist: %v", err)
		} else if info.Mode()&os.ModeSymlink == 0 {
			t.Error("AGENTS.md should be a symlink")
		}

		// Verify symlinks resolve correctly
		if content, err := os.ReadFile(readmeLink); err != nil {
			t.Errorf("failed to read through README.md symlink: %v", err)
		} else if string(content) != "# README" {
			t.Errorf("README.md content mismatch: got %q", string(content))
		}
	})

	t.Run("skip symlinks for missing files", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create project directory with only README.md (no AGENTS.md)
		projectDir := filepath.Join(tmpDir, "project")
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# README"), 0644); err != nil {
			t.Fatalf("failed to create README.md: %v", err)
		}

		// Create thoughts project docs directory (new structure)
		thoughtsDir := filepath.Join(tmpDir, "thoughts", "projects", "test", "docs")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts dir: %v", err)
		}

		// Update symlinks
		if err := UpdateProjectSymlinks(thoughtsDir, projectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// README.md symlink should exist
		readmeLink := filepath.Join(thoughtsDir, "README.md")
		if _, err := os.Lstat(readmeLink); err != nil {
			t.Errorf("README.md symlink should exist: %v", err)
		}

		// AGENTS.md symlink should NOT exist (target doesn't exist)
		agentsLink := filepath.Join(thoughtsDir, "AGENTS.md")
		if _, err := os.Lstat(agentsLink); !os.IsNotExist(err) {
			t.Error("AGENTS.md symlink should not exist when target is missing")
		}
	})

	t.Run("replace existing symlinks", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create old project directory
		oldProjectDir := filepath.Join(tmpDir, "old-project")
		if err := os.MkdirAll(oldProjectDir, 0755); err != nil {
			t.Fatalf("failed to create old project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(oldProjectDir, "README.md"), []byte("# OLD"), 0644); err != nil {
			t.Fatalf("failed to create old README.md: %v", err)
		}

		// Create new project directory
		newProjectDir := filepath.Join(tmpDir, "new-project")
		if err := os.MkdirAll(newProjectDir, 0755); err != nil {
			t.Fatalf("failed to create new project dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(newProjectDir, "README.md"), []byte("# NEW"), 0644); err != nil {
			t.Fatalf("failed to create new README.md: %v", err)
		}

		// Create thoughts project docs directory with old symlink (new structure)
		thoughtsDir := filepath.Join(tmpDir, "thoughts", "projects", "test", "docs")
		if err := os.MkdirAll(thoughtsDir, 0755); err != nil {
			t.Fatalf("failed to create thoughts dir: %v", err)
		}

		// Create old symlink
		oldRelPath, _ := filepath.Rel(thoughtsDir, oldProjectDir)
		oldTarget := filepath.Join(oldRelPath, "README.md")
		if err := os.Symlink(oldTarget, filepath.Join(thoughtsDir, "README.md")); err != nil {
			t.Fatalf("failed to create old symlink: %v", err)
		}

		// Update symlinks to point to new project
		if err := UpdateProjectSymlinks(thoughtsDir, newProjectDir); err != nil {
			t.Fatalf("UpdateProjectSymlinks failed: %v", err)
		}

		// Verify symlink now points to new content
		readmeLink := filepath.Join(thoughtsDir, "README.md")
		if content, err := os.ReadFile(readmeLink); err != nil {
			t.Errorf("failed to read through README.md symlink: %v", err)
		} else if string(content) != "# NEW" {
			t.Errorf("README.md should point to new content, got %q", string(content))
		}
	})

	t.Run("error on missing thoughts directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		projectDir := filepath.Join(tmpDir, "project")
		thoughtsDir := filepath.Join(tmpDir, "nonexistent")

		err := UpdateProjectSymlinks(thoughtsDir, projectDir)
		if err == nil {
			t.Error("expected error for missing thoughts directory")
		}
	})
}

func TestMoveDirectory(t *testing.T) {
	t.Run("move directory on same filesystem", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create source directory with content
		srcDir := filepath.Join(tmpDir, "src")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatalf("failed to create src dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Move to destination
		dstDir := filepath.Join(tmpDir, "dst")
		if err := moveDirectory(srcDir, dstDir); err != nil {
			t.Fatalf("moveDirectory failed: %v", err)
		}

		// Verify source doesn't exist
		if _, err := os.Stat(srcDir); !os.IsNotExist(err) {
			t.Error("source directory should not exist after move")
		}

		// Verify destination exists with content
		if _, err := os.Stat(dstDir); os.IsNotExist(err) {
			t.Error("destination directory should exist")
		}
		if content, err := os.ReadFile(filepath.Join(dstDir, "test.txt")); err != nil {
			t.Errorf("failed to read test file: %v", err)
		} else if string(content) != "content" {
			t.Errorf("content mismatch: got %q", string(content))
		}
	})
}

func TestRemoveEmptyDir(t *testing.T) {
	t.Run("remove empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyDir := filepath.Join(tmpDir, "empty")
		if err := os.MkdirAll(emptyDir, 0755); err != nil {
			t.Fatalf("failed to create empty dir: %v", err)
		}

		if err := removeEmptyDir(emptyDir); err != nil {
			t.Errorf("removeEmptyDir failed: %v", err)
		}

		if _, err := os.Stat(emptyDir); !os.IsNotExist(err) {
			t.Error("empty directory should be removed")
		}
	})

	t.Run("don't remove non-empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonEmptyDir := filepath.Join(tmpDir, "nonempty")
		if err := os.MkdirAll(nonEmptyDir, 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(nonEmptyDir, "file.txt"), []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if err := removeEmptyDir(nonEmptyDir); err != nil {
			t.Errorf("removeEmptyDir should not error for non-empty dir: %v", err)
		}

		if _, err := os.Stat(nonEmptyDir); os.IsNotExist(err) {
			t.Error("non-empty directory should not be removed")
		}
	})
}

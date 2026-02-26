package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init test repo: %v", err)
	}

	// Configure git user (required for commits)
	configName := exec.Command("git", "config", "user.name", "Test User")
	configName.Dir = dir
	if err := configName.Run(); err != nil {
		t.Fatalf("failed to config git user.name: %v", err)
	}

	configEmail := exec.Command("git", "config", "user.email", "test@example.com")
	configEmail.Dir = dir
	if err := configEmail.Run(); err != nil {
		t.Fatalf("failed to config git user.email: %v", err)
	}

	return dir
}

// createTestFile creates a file in the given directory.
func createTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		setup func(t *testing.T, dir string)
	}{
		{
			name:  "add all files",
			files: nil, // Should default to "."
			setup: func(t *testing.T, dir string) {
				createTestFile(t, dir, "file1.txt", "content1")
				createTestFile(t, dir, "file2.txt", "content2")
			},
		},
		{
			name:  "add specific file",
			files: []string{"file1.txt"},
			setup: func(t *testing.T, dir string) {
				createTestFile(t, dir, "file1.txt", "content1")
				createTestFile(t, dir, "file2.txt", "content2")
			},
		},
		{
			name:  "add multiple files",
			files: []string{"file1.txt", "file2.txt"},
			setup: func(t *testing.T, dir string) {
				createTestFile(t, dir, "file1.txt", "content1")
				createTestFile(t, dir, "file2.txt", "content2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupTestRepo(t)
			tt.setup(t, dir)

			gitOps := New(dir)
			err := gitOps.Add(context.Background(), tt.files...)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify files were staged using git status --porcelain
			cmd := exec.Command("git", "-C", dir, "status", "--porcelain")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("failed to check changes: %v", err)
			}

			if len(strings.TrimSpace(string(output))) == 0 {
				t.Error("expected staged changes after add")
			}
		})
	}
}

func TestAdd_InvalidFile(t *testing.T) {
	dir := setupTestRepo(t)
	gitOps := New(dir)

	// Try to add a file that doesn't exist with pathspec
	err := gitOps.Add(context.Background(), "nonexistent.txt")

	// Git add with pathspec for nonexistent file should fail
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestCommit(t *testing.T) {
	dir := setupTestRepo(t)
	createTestFile(t, dir, "test.txt", "content")

	gitOps := New(dir)

	// Stage the file
	if err := gitOps.Add(context.Background()); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	// Commit
	message := "test commit"
	err := gitOps.Commit(context.Background(), message)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify commit was created with correct message
	cmd := exec.Command("git", "log", "-1", "--pretty=%s")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get commit message: %v", err)
	}

	commitMsg := strings.TrimSpace(string(output))
	expectedMsg := "overlord: " + message

	if commitMsg != expectedMsg {
		t.Errorf("expected commit message %q, got %q", expectedMsg, commitMsg)
	}
}

func TestCommit_NothingToCommit(t *testing.T) {
	dir := setupTestRepo(t)
	gitOps := New(dir)

	// Try to commit with no staged changes
	err := gitOps.Commit(context.Background(), "test")

	if err == nil {
		t.Fatal("expected error for commit with no changes, got nil")
	}

	if !strings.Contains(err.Error(), "failed to git commit") {
		t.Errorf("expected error message about git commit, got: %v", err)
	}
}

func TestPush_NoRemote(t *testing.T) {
	dir := setupTestRepo(t)
	gitOps := New(dir)

	// Push should succeed (no-op) when there's no remote
	err := gitOps.Push(context.Background())

	if err != nil {
		t.Errorf("expected no error for push without remote, got: %v", err)
	}
}

func TestPull_NoRemote(t *testing.T) {
	dir := setupTestRepo(t)
	gitOps := New(dir)

	// Pull should succeed (no-op) when there's no remote
	err := gitOps.Pull(context.Background())

	if err != nil {
		t.Errorf("expected no error for pull without remote, got: %v", err)
	}
}

func TestPush_WithRemote(t *testing.T) {
	// Create a bare repo to use as remote
	remoteDir := t.TempDir()
	initBare := exec.Command("git", "init", "--bare")
	initBare.Dir = remoteDir
	if err := initBare.Run(); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	// Create local repo
	dir := setupTestRepo(t)
	createTestFile(t, dir, "test.txt", "content")

	// Add remote
	addRemote := exec.Command("git", "remote", "add", "origin", remoteDir)
	addRemote.Dir = dir
	if err := addRemote.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	gitOps := New(dir)

	// Create a commit
	if err := gitOps.Add(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gitOps.Commit(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}

	// Set upstream and push
	setUpstream := exec.Command("git", "push", "-u", "origin", "master")
	setUpstream.Dir = dir
	if err := setUpstream.Run(); err != nil {
		// Try "main" branch if "master" fails
		setUpstream = exec.Command("git", "branch", "-M", "main")
		setUpstream.Dir = dir
		if err := setUpstream.Run(); err != nil {
			t.Fatalf("failed to rename branch: %v", err)
		}
		setUpstream = exec.Command("git", "push", "-u", "origin", "main")
		setUpstream.Dir = dir
		if err := setUpstream.Run(); err != nil {
			t.Fatalf("failed to set upstream: %v", err)
		}
	}

	// Now test our Push method
	createTestFile(t, dir, "test2.txt", "content2")
	if err := gitOps.Add(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gitOps.Commit(context.Background(), "second commit"); err != nil {
		t.Fatal(err)
	}

	err := gitOps.Push(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPull_WithRemote(t *testing.T) {
	// Create a bare repo to use as remote
	remoteDir := t.TempDir()
	initBare := exec.Command("git", "init", "--bare")
	initBare.Dir = remoteDir
	if err := initBare.Run(); err != nil {
		t.Fatalf("failed to create bare repo: %v", err)
	}

	// Create first local repo and push
	dir1 := setupTestRepo(t)
	createTestFile(t, dir1, "test.txt", "content")

	addRemote := exec.Command("git", "remote", "add", "origin", remoteDir)
	addRemote.Dir = dir1
	if err := addRemote.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	gitOps1 := New(dir1)
	if err := gitOps1.Add(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gitOps1.Commit(context.Background(), "initial"); err != nil {
		t.Fatal(err)
	}

	// Set upstream and push
	setUpstream := exec.Command("git", "push", "-u", "origin", "master")
	setUpstream.Dir = dir1
	if err := setUpstream.Run(); err != nil {
		// Try "main" branch if "master" fails
		setUpstream = exec.Command("git", "branch", "-M", "main")
		setUpstream.Dir = dir1
		if err := setUpstream.Run(); err != nil {
			t.Fatalf("failed to rename branch: %v", err)
		}
		setUpstream = exec.Command("git", "push", "-u", "origin", "main")
		setUpstream.Dir = dir1
		if err := setUpstream.Run(); err != nil {
			t.Fatalf("failed to set upstream: %v", err)
		}
	}

	// Clone to second repo
	dir2 := t.TempDir()
	clone := exec.Command("git", "clone", remoteDir, dir2)
	if err := clone.Run(); err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	// Configure git user in cloned repo
	configName := exec.Command("git", "config", "user.name", "Test User")
	configName.Dir = dir2
	if err := configName.Run(); err != nil {
		t.Fatalf("failed to config git user.name: %v", err)
	}

	configEmail := exec.Command("git", "config", "user.email", "test@example.com")
	configEmail.Dir = dir2
	if err := configEmail.Run(); err != nil {
		t.Fatalf("failed to config git user.email: %v", err)
	}

	// Make a change in first repo and push
	createTestFile(t, dir1, "test2.txt", "content2")
	if err := gitOps1.Add(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := gitOps1.Commit(context.Background(), "second"); err != nil {
		t.Fatal(err)
	}
	if err := gitOps1.Push(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Pull in second repo
	gitOps2 := New(dir2)
	err := gitOps2.Pull(context.Background())

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the file exists in second repo
	if _, err := os.Stat(filepath.Join(dir2, "test2.txt")); err != nil {
		t.Errorf("expected test2.txt to exist after pull: %v", err)
	}
}

func TestPush_InvalidRepo(t *testing.T) {
	gitOps := New("/nonexistent/path")
	err := gitOps.Push(context.Background())

	if err == nil {
		t.Fatal("expected error for invalid repo, got nil")
	}

	if !strings.Contains(err.Error(), "failed to check git remote") {
		t.Errorf("expected error message about git remote, got: %v", err)
	}
}

func TestPull_InvalidRepo(t *testing.T) {
	gitOps := New("/nonexistent/path")
	err := gitOps.Pull(context.Background())

	if err == nil {
		t.Fatal("expected error for invalid repo, got nil")
	}

	if !strings.Contains(err.Error(), "failed to check git remote") {
		t.Errorf("expected error message about git remote, got: %v", err)
	}
}

func TestPush_WithEnv_NoRemote(t *testing.T) {
	// Push with Env set should still work when there's no remote (no-op)
	dir := setupTestRepo(t)
	gitOps := New(dir)
	gitOps.Env = []string{"GIT_SSH_COMMAND=ssh -o ControlMaster=auto"}

	err := gitOps.Push(context.Background())
	if err != nil {
		t.Errorf("expected no error for push without remote with Env, got: %v", err)
	}
}

func TestPull_WithEnv_NoRemote(t *testing.T) {
	// Pull with Env set should still work when there's no remote (no-op)
	dir := setupTestRepo(t)
	gitOps := New(dir)
	gitOps.Env = []string{"GIT_SSH_COMMAND=ssh -o ControlMaster=auto"}

	err := gitOps.Pull(context.Background())
	if err != nil {
		t.Errorf("expected no error for pull without remote with Env, got: %v", err)
	}
}

func TestHasUsableRemote(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T) string
		expected bool
	}{
		{
			name: "not a git repo",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			expected: false,
		},
		{
			name: "nonexistent path",
			setup: func(t *testing.T) string {
				return "/nonexistent/path/that/does/not/exist"
			},
			expected: false,
		},
		{
			name: "git repo without remote",
			setup: func(t *testing.T) string {
				return setupTestRepo(t)
			},
			expected: false,
		},
		{
			name: "git repo with remote URL",
			setup: func(t *testing.T) string {
				dir := setupTestRepo(t)
				addRemote := exec.Command("git", "remote", "add", "origin", "git@github.com:user/repo.git")
				addRemote.Dir = dir
				if err := addRemote.Run(); err != nil {
					t.Fatalf("failed to add remote: %v", err)
				}
				return dir
			},
			expected: true,
		},
		{
			name: "git repo with local bare remote",
			setup: func(t *testing.T) string {
				// Create a bare repo as remote
				remoteDir := t.TempDir()
				initBare := exec.Command("git", "init", "--bare")
				initBare.Dir = remoteDir
				if err := initBare.Run(); err != nil {
					t.Fatalf("failed to create bare repo: %v", err)
				}

				dir := setupTestRepo(t)
				addRemote := exec.Command("git", "remote", "add", "origin", remoteDir)
				addRemote.Dir = dir
				if err := addRemote.Run(); err != nil {
					t.Fatalf("failed to add remote: %v", err)
				}
				return dir
			},
			expected: true,
		},
		{
			name: "git repo with multiple remotes",
			setup: func(t *testing.T) string {
				dir := setupTestRepo(t)
				addOrigin := exec.Command("git", "remote", "add", "origin", "git@github.com:user/repo.git")
				addOrigin.Dir = dir
				if err := addOrigin.Run(); err != nil {
					t.Fatalf("failed to add origin: %v", err)
				}
				addUpstream := exec.Command("git", "remote", "add", "upstream", "git@github.com:upstream/repo.git")
				addUpstream.Dir = dir
				if err := addUpstream.Run(); err != nil {
					t.Fatalf("failed to add upstream: %v", err)
				}
				return dir
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			got := HasUsableRemote(path)
			if got != tt.expected {
				t.Errorf("HasUsableRemote(%q) = %v, want %v", path, got, tt.expected)
			}
		})
	}
}

package gitops

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitOps provides git operations for a specific repository.
type GitOps struct {
	RepoPath string

	// Env holds additional environment variables for git commands.
	// These are appended to the current process environment.
	// Typically used to set GIT_SSH_COMMAND for SSH multiplexing.
	Env []string
}

// New creates a new GitOps instance for the given repository path.
func New(repoPath string) *GitOps {
	return &GitOps{
		RepoPath: repoPath,
	}
}

// Add stages the specified files for commit.
// If no files are specified, stages all changes.
func (g *GitOps) Add(ctx context.Context, files ...string) error {
	if len(files) == 0 {
		files = []string{"."}
	}

	args := append([]string{"-C", g.RepoPath, "add"}, files...)
	cmd := exec.CommandContext(ctx, "git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git add: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Commit creates a commit with the given message.
// The message is automatically prefixed with "overlord: ".
func (g *GitOps) Commit(ctx context.Context, message string) error {
	fullMessage := "overlord: " + message
	cmd := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "commit", "-m", fullMessage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git commit: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Push pushes commits to the remote repository.
// If the repository has no remote configured, returns nil with no error.
func (g *GitOps) Push(ctx context.Context) error {
	// First check if a remote exists
	checkRemote := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "remote")
	remoteOutput, err := checkRemote.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check git remote: %w (output: %s)", err, strings.TrimSpace(string(remoteOutput)))
	}

	// If no remote configured, skip push (not an error)
	if len(strings.TrimSpace(string(remoteOutput))) == 0 {
		return nil
	}

	// Push to remote
	cmd := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "push")
	if len(g.Env) > 0 {
		cmd.Env = append(os.Environ(), g.Env...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git push: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// HasUsableRemote checks whether the repository has at least one git remote
// with a configured URL. This is used by the sync system to decide whether to
// use git-based sync (pull/push) instead of rsync for a project.
//
// "Usable remote" means:
//   - The directory is a git repository
//   - At least one remote is configured with a URL (i.e., `git remote -v` returns output)
//
// This check is purely local — it does NOT verify that the remote is reachable
// (that would be too slow for sync loops). If the remote is unreachable, the
// subsequent git pull/push will fail and the error will be reported.
func HasUsableRemote(path string) bool {
	// Check if the directory is a git repo by running `git -C <path> rev-parse --git-dir`.
	// This is faster than checking for .git directory and handles worktrees correctly.
	checkRepo := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	if err := checkRepo.Run(); err != nil {
		return false
	}

	// Check if any remote has a URL configured via `git remote -v`.
	// This outputs lines like: "origin\tgit@github.com:user/repo.git (fetch)"
	// If no remotes are configured, the output is empty.
	checkRemote := exec.Command("git", "-C", path, "remote", "-v")
	output, err := checkRemote.CombinedOutput()
	if err != nil {
		return false
	}

	return len(strings.TrimSpace(string(output))) > 0
}

// Pull pulls changes from the remote repository with rebase.
// If the repository has no remote configured, returns nil with no error.
func (g *GitOps) Pull(ctx context.Context) error {
	// First check if a remote exists
	checkRemote := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "remote")
	remoteOutput, err := checkRemote.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check git remote: %w (output: %s)", err, strings.TrimSpace(string(remoteOutput)))
	}

	// If no remote configured, skip pull (not an error)
	if len(strings.TrimSpace(string(remoteOutput))) == 0 {
		return nil
	}

	// Pull with rebase
	cmd := exec.CommandContext(ctx, "git", "-C", g.RepoPath, "pull", "--rebase")
	if len(g.Env) > 0 {
		cmd.Env = append(os.Environ(), g.Env...)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git pull: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

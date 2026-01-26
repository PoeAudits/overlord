package gitops

import (
	"fmt"
	"os/exec"
	"strings"
)

// GitOps provides git operations for a specific repository.
type GitOps struct {
	RepoPath string
}

// New creates a new GitOps instance for the given repository path.
func New(repoPath string) *GitOps {
	return &GitOps{
		RepoPath: repoPath,
	}
}

// HasChanges checks if there are uncommitted changes in the repository.
// Returns true if there are changes (staged or unstaged), false otherwise.
func (g *GitOps) HasChanges() (bool, error) {
	cmd := exec.Command("git", "-C", g.RepoPath, "status", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	// If output is empty, there are no changes
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// Add stages the specified files for commit.
// If no files are specified, stages all changes.
func (g *GitOps) Add(files ...string) error {
	if len(files) == 0 {
		files = []string{"."}
	}

	args := append([]string{"-C", g.RepoPath, "add"}, files...)
	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git add: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Commit creates a commit with the given message.
// The message is automatically prefixed with "overlord: ".
func (g *GitOps) Commit(message string) error {
	fullMessage := "overlord: " + message
	cmd := exec.Command("git", "-C", g.RepoPath, "commit", "-m", fullMessage)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git commit: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Push pushes commits to the remote repository.
// If the repository has no remote configured, returns nil with no error.
func (g *GitOps) Push() error {
	// First check if a remote exists
	checkRemote := exec.Command("git", "-C", g.RepoPath, "remote")
	remoteOutput, err := checkRemote.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check git remote: %w (output: %s)", err, strings.TrimSpace(string(remoteOutput)))
	}

	// If no remote configured, skip push (not an error)
	if len(strings.TrimSpace(string(remoteOutput))) == 0 {
		return nil
	}

	// Push to remote
	cmd := exec.Command("git", "-C", g.RepoPath, "push")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git push: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

// Pull pulls changes from the remote repository with rebase.
// If the repository has no remote configured, returns nil with no error.
func (g *GitOps) Pull() error {
	// First check if a remote exists
	checkRemote := exec.Command("git", "-C", g.RepoPath, "remote")
	remoteOutput, err := checkRemote.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check git remote: %w (output: %s)", err, strings.TrimSpace(string(remoteOutput)))
	}

	// If no remote configured, skip pull (not an error)
	if len(strings.TrimSpace(string(remoteOutput))) == 0 {
		return nil
	}

	// Pull with rebase
	cmd := exec.Command("git", "-C", g.RepoPath, "pull", "--rebase")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to git pull: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	return nil
}

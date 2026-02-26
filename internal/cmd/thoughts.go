// Package cmd contains Cobra command definitions and command-layer helpers.
//
// This file owns the project-root thoughts symlink: the ./thoughts entry inside
// the project repository that points to the thoughts directory tree managed by
// internal/thoughts. EnsureProjectThoughtsSymlink lives here (rather than in
// internal/thoughts) because it requires knowledge of the project path and
// archived state, which are command concerns. After creating or validating the
// project-root symlink it delegates to thoughts.UpdateSymlinksForState to
// refresh the doc-level symlinks (README.md, AGENTS.md) inside thoughts/docs/.
package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/thoughts"
)

// EnsureProjectThoughtsSymlink ensures ./thoughts points to the correct thoughts path.
// If a non-symlink exists at ./thoughts, it warns and skips.
func EnsureProjectThoughtsSymlink(thoughtsDir, projectName, projectPath string, archived bool) (string, error) {
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute project path: %w", err)
	}

	thoughtsBase := "projects"
	if archived {
		thoughtsBase = "archive"
	}

	targetPath := filepath.Join(thoughtsDir, thoughtsBase, projectName)
	linkPath := filepath.Join(absProjectPath, "thoughts")

	info, err := os.Lstat(linkPath)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return "warning: 'thoughts' exists and is not a symlink, skipping", nil
		}

		currentTarget, readErr := os.Readlink(linkPath)
		if readErr != nil {
			return "", fmt.Errorf("failed to read existing thoughts symlink: %w", readErr)
		}

		resolvedCurrentTarget := currentTarget
		if !filepath.IsAbs(currentTarget) {
			resolvedCurrentTarget = filepath.Join(absProjectPath, currentTarget)
		}

		if filepath.Clean(resolvedCurrentTarget) == filepath.Clean(targetPath) {
			return updateDocsSymlinks(thoughtsDir, projectName, absProjectPath, archived)
		}

		if err := os.Remove(linkPath); err != nil {
			return "", fmt.Errorf("failed to remove existing thoughts symlink: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("failed to check thoughts symlink: %w", err)
	}

	relTarget, err := filepath.Rel(absProjectPath, targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate thoughts symlink target: %w", err)
	}

	if err := os.Symlink(relTarget, linkPath); err != nil {
		return "", fmt.Errorf("failed to create thoughts symlink: %w", err)
	}

	return updateDocsSymlinks(thoughtsDir, projectName, absProjectPath, archived)
}

func updateDocsSymlinks(thoughtsDir, projectName, projectPath string, archived bool) (string, error) {
	if err := thoughts.UpdateSymlinksForState(thoughtsDir, projectName, projectPath, archived); err != nil {
		return "", fmt.Errorf("failed to update thoughts doc symlinks: %w", err)
	}
	return "", nil
}

package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
)

// resolveProjectPath resolves a project path to an absolute path.
// If the project path is already absolute, it returns it directly.
// Otherwise, it joins it with the base directory.
func resolveProjectPath(projectPath, baseDir string) string {
	if filepath.IsAbs(projectPath) {
		return projectPath
	}
	return filepath.Join(baseDir, projectPath)
}

// copyDirectory recursively copies a directory from src to dst
func copyDirectory(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDirectory(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFileWithMode(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFileWithMode copies a single file from src to dst, preserving file mode
func copyFileWithMode(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Get source file info
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	// Copy contents; on failure, close and remove the partial destination file
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(dst)
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return dstFile.Close()
}

// formatPathWithTilde converts absolute paths to use ~ notation for display
func formatPathWithTilde(path string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if strings.HasPrefix(path, homeDir) {
		return "~" + strings.TrimPrefix(path, homeDir)
	}

	return path
}

// moveDirectory moves a directory from src to dst.
// Uses rename first, fallback to copy+delete for cross-filesystem moves.
func moveDirectory(src, dst string) error {
	// Try rename first (fast, same filesystem)
	if err := os.Rename(src, dst); err != nil {
		// Check if it's a cross-device link error
		if strings.Contains(err.Error(), "cross-device") || strings.Contains(err.Error(), "invalid cross-device link") {
			// Cross-filesystem move - copy then delete
			if err := copyDirectory(src, dst); err != nil {
				return fmt.Errorf("failed to copy directory: %w", err)
			}

			// Remove source after successful copy
			if err := os.RemoveAll(src); err != nil {
				return fmt.Errorf("failed to remove source directory after copy: %w", err)
			}
		} else {
			return fmt.Errorf("failed to move directory: %w", err)
		}
	}

	return nil
}

// expandProjectPath expands a project path to absolute path
func expandProjectPath(projectPath, baseDir string) (string, error) {
	expandedBaseDir, err := registry.ExpandPath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base dir: %w", err)
	}

	// Resolve project path (handles both relative and absolute paths)
	fullPath := resolveProjectPath(projectPath, expandedBaseDir)

	// Clean the path
	return filepath.Clean(fullPath), nil
}

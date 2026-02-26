package fileutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Filesystem operation hooks — overridden in tests for dependency injection.
var (
	renameFunc    = os.Rename
	removeAllFunc = os.RemoveAll
	copyFunc      = io.Copy
)

// FormatPathWithTilde replaces the user's home directory prefix with ~ when possible.
func FormatPathWithTilde(path string) string {
	if path == "" {
		return ""
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return path
	}

	if strings.HasPrefix(path, homeDir) {
		return "~" + strings.TrimPrefix(path, homeDir)
	}

	return path
}

// MoveDirectory moves a directory from src to dst, handling cross-device moves.
func MoveDirectory(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("failed to stat source directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create destination parent: %w", err)
	}

	if err := renameFunc(src, dst); err != nil {
		if isCrossDeviceErr(err) {
			if err := CopyDirectory(src, dst); err != nil {
				return fmt.Errorf("failed to copy directory: %w", err)
			}
			if err := removeAllFunc(src); err != nil {
				return fmt.Errorf("failed to remove source directory after copy: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to move directory: %w", err)
	}

	return nil
}

// CopyDirectory recursively copies a directory tree.
func CopyDirectory(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := CopyDirectory(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := CopyFileWithMode(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

// CopyFileWithMode copies a file from src to dst, preserving its file mode.
func CopyFileWithMode(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("failed to create destination parent: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}

	if _, err := copyFunc(dstFile, srcFile); err != nil {
		dstFile.Close()
		_ = os.Remove(dst) // Best-effort cleanup of partial destination file
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}

	return nil
}

// AtomicWriteFile writes data to path via a temp file, then atomically renames it.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, perm); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath) // Best-effort cleanup of temp file on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// ExpandProjectPath resolves the project path relative to the expanded base directory.
func ExpandProjectPath(projectPath, baseDir string) (string, error) {
	expandedBase, err := expandPath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base dir: %w", err)
	}

	fullPath := ResolveProjectPath(projectPath, expandedBase)
	return filepath.Clean(fullPath), nil
}

// ResolveProjectPath resolves a potentially relative project path against the provided base directory.
func ResolveProjectPath(projectPath, baseDir string) string {
	if filepath.IsAbs(projectPath) {
		return filepath.Clean(projectPath)
	}
	return filepath.Join(baseDir, projectPath)
}

func expandPath(path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("path cannot be empty")
	}

	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}

		if len(path) == 1 {
			path = homeDir
		} else if path[1] == '/' || path[1] == filepath.Separator {
			path = filepath.Join(homeDir, path[2:])
		} else {
			return "", fmt.Errorf("~user expansion not supported, use absolute path")
		}
	}

	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		path = absPath
	}

	return path, nil
}

func isCrossDeviceErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return errors.Is(linkErr.Err, syscall.EXDEV)
	}
	return false
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// thoughtsSubdirs defines the 8 thoughts subdirectories per project
var thoughtsSubdirs = []string{"plans", "logs", "docs", "research", "sessions", "handoffs", "reviews", "briefs"}

// ThoughtsPaths holds paths for the 8 thoughts subdirectories per project
type ThoughtsPaths struct {
	Plans    string
	Logs     string
	Docs     string
	Research string
	Sessions string
	Handoffs string
	Reviews  string
	Briefs   string
}

// GetThoughtsPaths returns paths for the 8 thoughts subdirectories in the main location.
// Structure: ~/thoughts/projects/{name}/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/
func GetThoughtsPaths(thoughtsDir, projectName string) ThoughtsPaths {
	projectBase := filepath.Join(thoughtsDir, "projects", projectName)
	return ThoughtsPaths{
		Plans:    filepath.Join(projectBase, "plans"),
		Logs:     filepath.Join(projectBase, "logs"),
		Docs:     filepath.Join(projectBase, "docs"),
		Research: filepath.Join(projectBase, "research"),
		Sessions: filepath.Join(projectBase, "sessions"),
		Handoffs: filepath.Join(projectBase, "handoffs"),
		Reviews:  filepath.Join(projectBase, "reviews"),
		Briefs:   filepath.Join(projectBase, "briefs"),
	}
}

// GetArchiveThoughtsPaths returns paths for the archived thoughts structure.
// Structure: ~/thoughts/archive/{name}/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/
func GetArchiveThoughtsPaths(thoughtsDir, projectName string) ThoughtsPaths {
	archiveBase := filepath.Join(thoughtsDir, "archive", projectName)
	return ThoughtsPaths{
		Plans:    filepath.Join(archiveBase, "plans"),
		Logs:     filepath.Join(archiveBase, "logs"),
		Docs:     filepath.Join(archiveBase, "docs"),
		Research: filepath.Join(archiveBase, "research"),
		Sessions: filepath.Join(archiveBase, "sessions"),
		Handoffs: filepath.Join(archiveBase, "handoffs"),
		Reviews:  filepath.Join(archiveBase, "reviews"),
		Briefs:   filepath.Join(archiveBase, "briefs"),
	}
}

// toSlice returns the paths as a slice in consistent order: plans, logs, docs, research, sessions, handoffs, reviews, briefs
func (p ThoughtsPaths) toSlice() []string {
	return []string{p.Plans, p.Logs, p.Docs, p.Research, p.Sessions, p.Handoffs, p.Reviews, p.Briefs}
}

// toMap returns the paths as a map with subdirectory names as keys
func (p ThoughtsPaths) toMap() map[string]string {
	return map[string]string{
		"plans":    p.Plans,
		"logs":     p.Logs,
		"docs":     p.Docs,
		"research": p.Research,
		"sessions": p.Sessions,
		"handoffs": p.Handoffs,
		"reviews":  p.Reviews,
		"briefs":   p.Briefs,
	}
}

// ThoughtsExist checks which directories exist.
// Returns true if any exist, and a list of missing directory names.
func ThoughtsExist(paths ThoughtsPaths) (exists bool, missing []string) {
	pathMap := paths.toMap()

	for _, subdir := range thoughtsSubdirs {
		path := pathMap[subdir]
		if _, err := os.Stat(path); os.IsNotExist(err) {
			missing = append(missing, subdir)
		} else if err == nil {
			exists = true
		}
	}

	return exists, missing
}

// MoveThoughtsToArchive moves all 8 thoughts directories to archive location.
// Returns warnings for missing directories (not errors).
// Uses rename first, fallback to copy+delete for cross-filesystem moves.
func MoveThoughtsToArchive(thoughtsDir, projectName string) (warnings []string, err error) {
	// Expand thoughts directory
	expandedThoughtsDir, err := expandPath(thoughtsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	srcPaths := GetThoughtsPaths(expandedThoughtsDir, projectName)
	dstPaths := GetArchiveThoughtsPaths(expandedThoughtsDir, projectName)

	// Create archive base directory
	archiveBase := filepath.Join(expandedThoughtsDir, "archive", projectName)
	if err := os.MkdirAll(archiveBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Move each subdirectory
	srcMap := srcPaths.toMap()
	dstMap := dstPaths.toMap()

	for _, subdir := range thoughtsSubdirs {
		srcPath := srcMap[subdir]
		dstPath := dstMap[subdir]

		// Check if source exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("thoughts/projects/%s/%s does not exist, skipping", projectName, subdir))
			continue
		}

		// Move directory
		if err := moveDirectory(srcPath, dstPath); err != nil {
			return warnings, fmt.Errorf("failed to move %s to archive: %w", subdir, err)
		}
	}

	// Clean up empty project directory in active location
	projectBase := filepath.Join(expandedThoughtsDir, "projects", projectName)
	if err := removeEmptyDir(projectBase); err != nil {
		// Non-fatal - just warn
		warnings = append(warnings, fmt.Sprintf("could not remove empty project directory: %v", err))
	}

	return warnings, nil
}

// MoveThoughtsFromArchive restores thoughts from archive back to main location.
// Returns warnings for missing directories (not errors).
// Handles case where archive doesn't exist (project archived before this feature).
func MoveThoughtsFromArchive(thoughtsDir, projectName string) (warnings []string, err error) {
	// Expand thoughts directory
	expandedThoughtsDir, err := expandPath(thoughtsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	srcPaths := GetArchiveThoughtsPaths(expandedThoughtsDir, projectName)
	dstPaths := GetThoughtsPaths(expandedThoughtsDir, projectName)

	// Check if archive exists at all
	archiveBase := filepath.Join(expandedThoughtsDir, "archive", projectName)
	if _, err := os.Stat(archiveBase); os.IsNotExist(err) {
		warnings = append(warnings, fmt.Sprintf("no archived thoughts found for %s (project may have been archived before this feature)", projectName))
		return warnings, nil
	}

	// Create project base directory for destination
	projectBase := filepath.Join(expandedThoughtsDir, "projects", projectName)
	if err := os.MkdirAll(projectBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create project directory: %w", err)
	}

	// Move each subdirectory
	srcMap := srcPaths.toMap()
	dstMap := dstPaths.toMap()

	for _, subdir := range thoughtsSubdirs {
		srcPath := srcMap[subdir]
		dstPath := dstMap[subdir]

		// Check if source exists
		if _, err := os.Stat(srcPath); os.IsNotExist(err) {
			warnings = append(warnings, fmt.Sprintf("archive/%s/%s does not exist, skipping", projectName, subdir))
			continue
		}

		// Move directory
		if err := moveDirectory(srcPath, dstPath); err != nil {
			return warnings, fmt.Errorf("failed to restore %s from archive: %w", subdir, err)
		}
	}

	// Clean up empty archive directory
	if err := removeEmptyDir(archiveBase); err != nil {
		// Non-fatal - just warn
		warnings = append(warnings, fmt.Sprintf("could not remove empty archive directory: %v", err))
	}

	return warnings, nil
}

// UpdateProjectSymlinks updates symlinks in the thoughts project directory to point to the actual project location.
// thoughtsProjectDir should be the docs subdirectory: ~/thoughts/projects/{name}/docs/
// Removes existing symlinks and creates new relative symlinks for README.md and AGENTS.md.
// Skips creating symlinks if target files don't exist in the project.
func UpdateProjectSymlinks(thoughtsProjectDir, actualProjectPath string) error {
	// Ensure thoughts project directory exists
	if _, err := os.Stat(thoughtsProjectDir); os.IsNotExist(err) {
		return fmt.Errorf("thoughts project directory does not exist: %s", thoughtsProjectDir)
	}

	// Calculate relative path from thoughts project dir to actual project
	relPath, err := filepath.Rel(thoughtsProjectDir, actualProjectPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Files to symlink
	symlinkTargets := []string{"README.md", "AGENTS.md"}

	for _, target := range symlinkTargets {
		linkPath := filepath.Join(thoughtsProjectDir, target)
		targetPath := filepath.Join(actualProjectPath, target)
		linkTarget := filepath.Join(relPath, target)

		// Remove existing symlink if it exists
		if info, err := os.Lstat(linkPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(linkPath); err != nil {
					return fmt.Errorf("failed to remove existing symlink %s: %w", target, err)
				}
			}
		}

		// Check if target file exists in the project
		if _, err := os.Stat(targetPath); os.IsNotExist(err) {
			// Target doesn't exist - skip creating symlink (don't create broken symlinks)
			continue
		}

		// Create new symlink
		if err := os.Symlink(linkTarget, linkPath); err != nil {
			return fmt.Errorf("failed to create symlink for %s: %w", target, err)
		}
	}

	return nil
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

// removeEmptyDir removes a directory only if it's empty
func removeEmptyDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return os.Remove(path)
	}

	return nil
}

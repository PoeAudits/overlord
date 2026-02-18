package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/registry"
)

// thoughtsSubdirs defines the thoughts subdirectories per project.
var thoughtsSubdirs = []string{"briefs", "research", "audits", "plans", "logs", "reviews", "docs"}

// ThoughtsPaths holds paths for the thoughts subdirectories per project.
type ThoughtsPaths struct {
	Briefs   string
	Research string
	Audits   string
	Plans    string
	Logs     string
	Reviews  string
	Docs     string
}

// GetThoughtsPaths returns paths for the thoughts subdirectories in the main location.
// Structure: ~/thoughts/projects/{name}/{briefs,research,audits,plans,logs,reviews,docs}/
func GetThoughtsPaths(thoughtsDir, projectName string) ThoughtsPaths {
	projectBase := filepath.Join(thoughtsDir, "projects", projectName)
	return ThoughtsPaths{
		Briefs:   filepath.Join(projectBase, "briefs"),
		Research: filepath.Join(projectBase, "research"),
		Audits:   filepath.Join(projectBase, "audits"),
		Plans:    filepath.Join(projectBase, "plans"),
		Logs:     filepath.Join(projectBase, "logs"),
		Reviews:  filepath.Join(projectBase, "reviews"),
		Docs:     filepath.Join(projectBase, "docs"),
	}
}

// GetArchiveThoughtsPaths returns paths for the archived thoughts structure.
// Structure: ~/thoughts/archive/{name}/{briefs,research,audits,plans,logs,reviews,docs}/
func GetArchiveThoughtsPaths(thoughtsDir, projectName string) ThoughtsPaths {
	archiveBase := filepath.Join(thoughtsDir, "archive", projectName)
	return ThoughtsPaths{
		Briefs:   filepath.Join(archiveBase, "briefs"),
		Research: filepath.Join(archiveBase, "research"),
		Audits:   filepath.Join(archiveBase, "audits"),
		Plans:    filepath.Join(archiveBase, "plans"),
		Logs:     filepath.Join(archiveBase, "logs"),
		Reviews:  filepath.Join(archiveBase, "reviews"),
		Docs:     filepath.Join(archiveBase, "docs"),
	}
}

// toSlice returns the paths as a slice in consistent order.
func (p ThoughtsPaths) toSlice() []string {
	return []string{p.Briefs, p.Research, p.Audits, p.Plans, p.Logs, p.Reviews, p.Docs}
}

// toMap returns the paths as a map with subdirectory names as keys
func (p ThoughtsPaths) toMap() map[string]string {
	return map[string]string{
		"briefs":   p.Briefs,
		"research": p.Research,
		"audits":   p.Audits,
		"plans":    p.Plans,
		"logs":     p.Logs,
		"reviews":  p.Reviews,
		"docs":     p.Docs,
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

// MoveThoughtsToArchive moves thoughts directories to archive location.
// Returns warnings for missing directories (not errors).
// Uses rename first, fallback to copy+delete for cross-filesystem moves.
func MoveThoughtsToArchive(thoughtsDir, projectName string) (warnings []string, err error) {
	// Expand thoughts directory
	expandedThoughtsDir, err := registry.ExpandPath(thoughtsDir)
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
	expandedThoughtsDir, err := registry.ExpandPath(thoughtsDir)
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

// CreateThoughtsStructure creates thoughts directories and symlinks for a project.
// Creates all subdirectories under ~/thoughts/projects/{name}/ and symlinks in docs/.
// The projectPath should be an absolute path to the project directory.
func CreateThoughtsStructure(thoughtsDir, projectName, projectPath string) error {
	// Ensure project path is absolute
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get all thoughts subdirectory paths
	paths := GetThoughtsPaths(thoughtsDir, projectName)

	// Create all subdirectories
	for _, dir := range paths.toSlice() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create thoughts directory %s: %w", dir, err)
		}
	}

	// Create symlinks in docs/ subdirectory
	if err := UpdateProjectSymlinks(paths.Docs, absProjectPath); err != nil {
		return fmt.Errorf("failed to create symlinks: %w", err)
	}

	return nil
}

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
			return "", nil
		}

		if err := os.Remove(linkPath); err != nil {
			return "", fmt.Errorf("failed to remove existing thoughts symlink: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check thoughts symlink: %w", err)
	}

	relTarget, err := filepath.Rel(absProjectPath, targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate thoughts symlink target: %w", err)
	}

	if err := os.Symlink(relTarget, linkPath); err != nil {
		return "", fmt.Errorf("failed to create thoughts symlink: %w", err)
	}

	return "", nil
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

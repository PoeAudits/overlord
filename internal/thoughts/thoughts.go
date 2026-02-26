// Package thoughts manages the structured thoughts directory for each project.
//
// Responsibility split:
//   - This package (internal/thoughts) owns the thoughts directory tree itself:
//     creating subdirectories (plans, logs, docs, research, sessions, handoffs,
//     briefs, audits, reviews), moving them on archive/unarchive, and maintaining
//     doc-level symlinks inside thoughts/docs/ that point back at project files
//     such as README.md and AGENTS.md. These functions (EnsureProjectSymlink,
//     UpdateSymlinks, UpdateSymlinksForState) are called by the lifecycle addon
//     and by internal/cmd/thoughts.go.
//   - internal/cmd/thoughts.go owns the project-root symlink: the ./thoughts
//     entry that lives inside the project repository and points to the thoughts
//     directory tree. EnsureProjectThoughtsSymlink lives in the cmd package so
//     it can be called by commands (new, open, archive, unarchive) that know
//     about the project path and archived state.
package thoughts

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/fileutil"
)

// Paths represents the structured layout for a project's thoughts directories.
type Paths struct {
	Plans    string
	Logs     string
	Docs     string
	Research string
	Sessions string
	Handoffs string
	Briefs   string
	Audits   string
	Reviews  string
}

var docSymlinkFiles = []string{"README.md", "AGENTS.md"}

// ProjectPaths returns paths for the primary thoughts directories.
func ProjectPaths(thoughtsDir, projectName string) Paths {
	projectBase := filepath.Join(thoughtsDir, "projects", projectName)
	return buildPaths(projectBase)
}

// ArchivePaths returns paths for the archived thoughts directories.
func ArchivePaths(thoughtsDir, projectName string) Paths {
	archiveBase := filepath.Join(thoughtsDir, "archive", projectName)
	return buildPaths(archiveBase)
}

// ToSlice converts the paths into a slice.
func (p Paths) ToSlice() []string {
	return []string{
		p.Plans,
		p.Logs,
		p.Docs,
		p.Research,
		p.Sessions,
		p.Handoffs,
		p.Briefs,
		p.Audits,
		p.Reviews,
	}
}

// Exist checks whether any thoughts subdirectory exists for the given project.
// Returns true if at least one subdirectory is present, which supports detection
// of partially created or recovered thoughts structures.
func Exist(thoughtsDir, projectName string) bool {
	paths := ProjectPaths(thoughtsDir, projectName)
	for _, path := range paths.ToSlice() {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}

// MoveToArchive moves a project's thoughts directory into the archive.
func MoveToArchive(thoughtsDir, projectName string) error {
	activeBase := filepath.Join(thoughtsDir, "projects", projectName)
	archiveBase := filepath.Join(thoughtsDir, "archive", projectName)

	if _, err := os.Stat(activeBase); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to stat thoughts project directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(thoughtsDir, "archive"), 0o755); err != nil {
		return fmt.Errorf("failed to create archive base directory: %w", err)
	}

	if err := fileutil.MoveDirectory(activeBase, archiveBase); err != nil {
		return fmt.Errorf("failed to move thoughts to archive: %w", err)
	}

	return nil
}

// MoveFromArchive restores a project's thoughts directory from the archive.
func MoveFromArchive(thoughtsDir, projectName string) error {
	archiveBase := filepath.Join(thoughtsDir, "archive", projectName)
	activeBase := filepath.Join(thoughtsDir, "projects", projectName)

	if _, err := os.Stat(archiveBase); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to stat archived thoughts directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(thoughtsDir, "projects"), 0o755); err != nil {
		return fmt.Errorf("failed to create projects base directory: %w", err)
	}

	if err := fileutil.MoveDirectory(archiveBase, activeBase); err != nil {
		return fmt.Errorf("failed to restore thoughts from archive: %w", err)
	}

	return nil
}

// CreateStructure creates the base thoughts directory structure for a project.
func CreateStructure(thoughtsDir, projectName string) error {
	paths := ProjectPaths(thoughtsDir, projectName)
	for _, path := range paths.ToSlice() {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("failed to create thoughts directory %s: %w", path, err)
		}
	}

	return nil
}

// EnsureProjectSymlink ensures thoughts docs reference the project path.
func EnsureProjectSymlink(thoughtsDir, projectName, projectPath string) error {
	return ensureProjectSymlinkForPaths(ProjectPaths(thoughtsDir, projectName), projectPath)
}

// UpdateSymlinks refreshes symlinks when the project path changes.
func UpdateSymlinks(thoughtsDir, projectName, newProjectPath string) error {
	return UpdateSymlinksForState(thoughtsDir, projectName, newProjectPath, false)
}

// UpdateSymlinksForState refreshes symlinks for active or archived thoughts paths.
func UpdateSymlinksForState(thoughtsDir, projectName, newProjectPath string, archived bool) error {
	paths := ProjectPaths(thoughtsDir, projectName)
	if archived {
		paths = ArchivePaths(thoughtsDir, projectName)
	}

	if err := ensureProjectSymlinkForPaths(paths, newProjectPath); err != nil {
		return fmt.Errorf("failed to update project symlinks: %w", err)
	}

	return nil
}

func buildPaths(base string) Paths {
	return Paths{
		Plans:    filepath.Join(base, "plans"),
		Logs:     filepath.Join(base, "logs"),
		Docs:     filepath.Join(base, "docs"),
		Research: filepath.Join(base, "research"),
		Sessions: filepath.Join(base, "sessions"),
		Handoffs: filepath.Join(base, "handoffs"),
		Briefs:   filepath.Join(base, "briefs"),
		Audits:   filepath.Join(base, "audits"),
		Reviews:  filepath.Join(base, "reviews"),
	}
}

func ensureProjectSymlinkForPaths(paths Paths, projectPath string) error {
	docsDir := paths.Docs

	info, err := os.Stat(docsDir)
	if err != nil {
		return fmt.Errorf("failed to stat docs directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("docs path is not a directory: %s", docsDir)
	}

	for _, name := range docSymlinkFiles {
		linkPath := filepath.Join(docsDir, name)
		targetPath := filepath.Join(projectPath, name)

		if linkInfo, lerr := os.Lstat(linkPath); lerr == nil {
			if linkInfo.Mode()&os.ModeSymlink == 0 {
				return fmt.Errorf("path exists and is not a symlink: %s", linkPath)
			}
			if rerr := os.Remove(linkPath); rerr != nil {
				return fmt.Errorf("failed to remove existing symlink %s: %w", linkPath, rerr)
			}
		} else if !errors.Is(lerr, fs.ErrNotExist) {
			return fmt.Errorf("failed to inspect symlink path %s: %w", linkPath, lerr)
		}

		if _, serr := os.Stat(targetPath); errors.Is(serr, fs.ErrNotExist) {
			continue
		} else if serr != nil {
			return fmt.Errorf("failed to stat symlink target %s: %w", targetPath, serr)
		}

		targetDir := filepath.Dir(linkPath)
		relTarget, rerr := filepath.Rel(targetDir, targetPath)
		if rerr != nil {
			return fmt.Errorf("failed to calculate relative symlink target: %w", rerr)
		}

		if serr := os.Symlink(relTarget, linkPath); serr != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", linkPath, relTarget, serr)
		}
	}

	return nil
}

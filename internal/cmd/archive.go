package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
)

var (
	archiveNoConfirm bool
)

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use:   "archive <name> [name...]",
	Short: "Archive one or more projects by moving them to the archive directory",
	Long: `Archive one or more projects by moving their directories to ~/Overlord/archive/<name>/ and
updating their status to 'archived' in the registry.

Name arguments can be project names or aliases.

Thoughts directories are also moved to archive:
  From: ~/thoughts/projects/<project>/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/
  To:   ~/thoughts/archive/<project>/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/

Symlinks in the docs directory are updated to point to the new archive location.

Examples:
  overlord archive myproject              # Archive a single project
  overlord archive foo bar baz            # Archive multiple projects
  overlord archive myproject --yes        # Archive without confirmation`,
	Args: cobra.MinimumNArgs(1),
	RunE: runArchive,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.Flags().BoolVarP(&archiveNoConfirm, "yes", "y", false, "Skip confirmation prompt")
}

// resolvedProject holds pre-resolved info for a project to be archived.
type resolvedProject struct {
	name       string
	project    registry.Project
	sourcePath string
	destPath   string
}

func runArchive(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve all projects upfront
	var resolved []resolvedProject
	var resolveErrors []string

	for _, name := range args {
		result, err := reg.ResolveOne(name)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': not found", name))
			continue
		}

		project := result.Project

		if project.Status.State == registry.StateArchived {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': already archived (location: %s)", result.Name, project.Path))
			continue
		}

		sourcePath, err := expandProjectPath(project.Path, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand source path: %v", result.Name, err))
			continue
		}

		archivePath := fmt.Sprintf("archive/%s", result.Name)
		destPath, err := expandProjectPath(archivePath, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand destination path: %v", result.Name, err))
			continue
		}

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': directory does not exist: %s", result.Name, sourcePath))
			continue
		} else if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to check directory: %v", result.Name, err))
			continue
		}

		resolved = append(resolved, resolvedProject{
			name:       result.Name,
			project:    project,
			sourcePath: sourcePath,
			destPath:   destPath,
		})
	}

	// Print resolution errors
	if len(resolveErrors) > 0 {
		strongWarnStyle := NewStyles().StrongWarn
		for _, e := range resolveErrors {
			fmt.Printf("%s %s\n", strongWarnStyle.Render("Skipping"), e)
		}
		fmt.Println()
	}

	if len(resolved) == 0 {
		return fmt.Errorf("no valid projects to archive")
	}

	// Confirm before archiving
	if !archiveNoConfirm {
		confirmed, err := confirmArchiveMultiple(resolved)
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Archive each project
	var archiveErrors []string

	for _, rp := range resolved {
		err := archiveOneProject(reg, rp)
		if err != nil {
			archiveErrors = append(archiveErrors, fmt.Sprintf("'%s': %v", rp.name, err))
		}
	}

	// Save registry (once, after all archives)
	if err := saveRegistry(reg); err != nil {
		return err
	}

	// Print archive errors
	if len(archiveErrors) > 0 {
		errStyle := NewStyles().Error
		fmt.Println()
		for _, e := range archiveErrors {
			fmt.Printf("%s %s\n", errStyle.Render("Failed"), e)
		}
	}

	totalErrors := len(resolveErrors) + len(archiveErrors)
	if totalErrors > 0 && len(resolved) > len(archiveErrors) {
		// Some succeeded, some failed
		return nil
	} else if len(archiveErrors) == len(resolved) {
		return fmt.Errorf("all archive operations failed")
	}

	return nil
}

// archiveOneProject archives a single pre-resolved project.
func archiveOneProject(reg *registry.Registry, rp resolvedProject) error {
	// Define styles for output
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Success
	pathStyle := s.Path

	// Create archive directory parent
	archiveParent := filepath.Dir(rp.destPath)
	if err := os.MkdirAll(archiveParent, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Move directory
	if err := moveDirectory(rp.sourcePath, rp.destPath); err != nil {
		return fmt.Errorf("failed to move directory: %w", err)
	}

	// Move thoughts directories to archive (secondary operation - warn on failure, don't fail archive)
	thoughtsDir := reg.Settings.ThoughtsDir
	var thoughtsWarnings []string
	var thoughtsMoved bool

	thoughtsWarnings, err := MoveThoughtsToArchive(thoughtsDir, rp.name)
	if err != nil {
		// Thoughts move failed - warn but continue (project archive succeeded)
		fmt.Printf("%s Failed to move thoughts directories for '%s': %v\n", s.StrongWarn.Render("Warning:"), rp.name, err)
	} else {
		thoughtsMoved = true

		// Update symlinks in the archived thoughts docs directory
		expandedThoughtsDir, expandErr := registry.ExpandPath(thoughtsDir)
		if expandErr == nil {
			archiveThoughtsPaths := GetArchiveThoughtsPaths(expandedThoughtsDir, rp.name)
			if symlinkErr := UpdateProjectSymlinks(archiveThoughtsPaths.Docs, rp.destPath); symlinkErr != nil {
				// Symlink update failed - warn but continue
				thoughtsWarnings = append(thoughtsWarnings, fmt.Sprintf("failed to update symlinks: %v", symlinkErr))
			}
		} else {
			thoughtsWarnings = append(thoughtsWarnings, fmt.Sprintf("failed to expand thoughts directory: %v", expandErr))
		}
	}

	// Update registry
	archivePath := fmt.Sprintf("archive/%s", rp.name)
	project := rp.project
	project.Path = archivePath
	project.Status.State = registry.StateArchived
	reg.Projects[rp.name] = project

	// Print success message
	fmt.Printf("%s '%s'\n", successStyle.Render("Archived"), rp.name)
	fmt.Printf("  %s %s\n", labelStyle.Render("From:"), pathStyle.Render(formatPathWithTilde(rp.sourcePath)))
	fmt.Printf("  %s %s\n", labelStyle.Render("To:"), pathStyle.Render(formatPathWithTilde(rp.destPath)))

	// Print thoughts status
	if thoughtsMoved {
		fmt.Printf("  %s %s/archive/%s/\n", labelStyle.Render("Thoughts:"), thoughtsDir, rp.name)
	}
	fmt.Println()

	// Print any warnings from thoughts move
	if len(thoughtsWarnings) > 0 {
		for _, warning := range thoughtsWarnings {
			fmt.Printf("%s %s\n", s.StrongWarn.Render("Warning:"), warning)
		}
		fmt.Println()
	}

	return nil
}

// confirmArchiveMultiple prompts the user for confirmation before archiving multiple projects.
func confirmArchiveMultiple(projects []resolvedProject) (bool, error) {
	if len(projects) == 1 {
		p := projects[0]
		fmt.Printf("Archive project '%s'?\n", p.name)
		fmt.Printf("  From: %s\n", formatPathWithTilde(p.sourcePath))
		fmt.Printf("  To:   %s\n", formatPathWithTilde(p.destPath))
	} else {
		fmt.Printf("Archive %d projects?\n", len(projects))
		for _, p := range projects {
			fmt.Printf("  %s\n", p.name)
			fmt.Printf("    From: %s\n", formatPathWithTilde(p.sourcePath))
			fmt.Printf("    To:   %s\n", formatPathWithTilde(p.destPath))
		}
	}
	return confirmAction("Continue?")
}

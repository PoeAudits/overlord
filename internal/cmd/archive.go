package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	archiveNoConfirm bool
)

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use:   "archive <name>",
	Short: "Archive a project by moving it to the archive directory",
	Long: `Archive a project by moving its directory to ~/Overlord/archive/<name>/ and
updating its status to 'archived' in the registry.

The name argument can be either a project name or alias.

Thoughts directories are also moved to archive:
  From: ~/thoughts/projects/<project>/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/
  To:   ~/thoughts/archive/<project>/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/

Symlinks in the docs directory are updated to point to the new archive location.

Examples:
  overlord archive myproject              # Archive with confirmation
  overlord archive myproject --yes        # Archive without confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: runArchive,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.Flags().BoolVarP(&archiveNoConfirm, "yes", "y", false, "Skip confirmation prompt")
}

func runArchive(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Resolve project using fuzzy matching
	result, err := reg.ResolveOne(name)
	if err != nil {
		return fmt.Errorf("Error: project '%s' not found\nRun 'overlord list' to see all projects", name)
	}
	projectName := result.Name
	project := result.Project

	// Check if already archived
	if project.Status.State == registry.StateArchived {
		return fmt.Errorf("Error: project '%s' is already archived\nLocation: %s", projectName, project.Path)
	}

	// Calculate paths
	sourcePath, err := expandProjectPath(project.Path, reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}

	archivePath := fmt.Sprintf("archive/%s", projectName)
	destPath, err := expandProjectPath(archivePath, reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand destination path: %w", err)
	}

	// Check if source directory exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", sourcePath)
	} else if err != nil {
		return fmt.Errorf("failed to check project directory: %w", err)
	}

	// Confirm before archiving
	if !archiveNoConfirm {
		confirmed, err := confirmArchive(projectName, sourcePath, destPath)
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Create archive directory parent
	archiveParent := filepath.Dir(destPath)
	if err := os.MkdirAll(archiveParent, 0755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	// Move directory (try rename first, fallback to copy+delete for cross-filesystem)
	if err := os.Rename(sourcePath, destPath); err != nil {
		// Check if it's a cross-device link error
		if strings.Contains(err.Error(), "cross-device") || strings.Contains(err.Error(), "invalid cross-device link") {
			fmt.Println("Cross-filesystem move detected, copying files...")
			if err := copyDirectory(sourcePath, destPath); err != nil {
				return fmt.Errorf("failed to copy directory: %w", err)
			}

			// Remove source after successful copy
			if err := os.RemoveAll(sourcePath); err != nil {
				return fmt.Errorf("failed to remove source directory after copy: %w", err)
			}
		} else {
			return fmt.Errorf("failed to move directory: %w", err)
		}
	}

	// Move thoughts directories to archive (secondary operation - warn on failure, don't fail archive)
	thoughtsDir := reg.Settings.ThoughtsDir
	var thoughtsWarnings []string
	var thoughtsMoved bool

	thoughtsWarnings, err = MoveThoughtsToArchive(thoughtsDir, projectName)
	if err != nil {
		// Thoughts move failed - warn but continue (project archive succeeded)
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
		fmt.Printf("%s Failed to move thoughts directories: %v\n", warnStyle.Render("Warning:"), err)
	} else {
		thoughtsMoved = true

		// Update symlinks in the archived thoughts docs directory
		expandedThoughtsDir, expandErr := expandPath(thoughtsDir)
		if expandErr == nil {
			archiveThoughtsPaths := GetArchiveThoughtsPaths(expandedThoughtsDir, projectName)
			if symlinkErr := UpdateProjectSymlinks(archiveThoughtsPaths.Docs, destPath); symlinkErr != nil {
				// Symlink update failed - warn but continue
				thoughtsWarnings = append(thoughtsWarnings, fmt.Sprintf("failed to update symlinks: %v", symlinkErr))
			}
		} else {
			thoughtsWarnings = append(thoughtsWarnings, fmt.Sprintf("failed to expand thoughts directory: %v", expandErr))
		}
	}

	// Update registry
	project.Path = archivePath
	project.Status.State = registry.StateArchived
	reg.Projects[projectName] = project

	// Save registry
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Define styles for output
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(gray)

	// Print success message
	fmt.Printf("%s '%s'\n", successStyle.Render("Archived"), projectName)
	fmt.Printf("  %s %s\n", labelStyle.Render("From:"), pathStyle.Render(formatPathWithTilde(sourcePath)))
	fmt.Printf("  %s %s\n", labelStyle.Render("To:"), pathStyle.Render(formatPathWithTilde(destPath)))

	// Print thoughts status
	if thoughtsMoved {
		fmt.Printf("  %s %s/archive/%s/\n", labelStyle.Render("Thoughts:"), thoughtsDir, projectName)
	}
	fmt.Println()

	// Print any warnings from thoughts move
	if len(thoughtsWarnings) > 0 {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		for _, warning := range thoughtsWarnings {
			fmt.Printf("%s %s\n", warnStyle.Render("Warning:"), warning)
		}
		fmt.Println()
	}

	return nil
}

// confirmArchive prompts the user for confirmation before archiving
func confirmArchive(projectName, sourcePath, destPath string) (bool, error) {
	fmt.Printf("Archive project '%s'?\n", projectName)
	fmt.Printf("  From: %s\n", formatPathWithTilde(sourcePath))
	fmt.Printf("  To:   %s\n", formatPathWithTilde(destPath))
	fmt.Print("Continue? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
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

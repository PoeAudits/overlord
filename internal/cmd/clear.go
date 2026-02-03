package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear <name> [name...]",
	Short: "Permanently remove projects, their files, and thoughts directories",
	Long: `Permanently remove one or more projects from the registry, delete their project
directories, and delete their thoughts directories.

This is a destructive operation that cannot be undone. A confirmation prompt will
list everything that will be removed before proceeding.

Works on both active and archived projects.

Name arguments can be project names or aliases.

What gets removed:
  - Registry entry
  - Project directory (e.g. ~/Overlord/web/myproject/)
  - Thoughts directories (~/thoughts/projects/myproject/)
  - Archived thoughts (~/thoughts/archive/myproject/)

Examples:
  overlord clear myproject              # Clear a single project
  overlord clear proj1 proj2 proj3      # Clear multiple projects`,
	Args: cobra.MinimumNArgs(1),
	RunE: runClear,
}

func init() {
	rootCmd.AddCommand(clearCmd)
}

// clearTarget holds pre-resolved info for a project to be cleared.
type clearTarget struct {
	name        string
	projectPath string // expanded absolute path to project directory
	thoughtsDir string // expanded absolute path to ~/thoughts/projects/<name>
	archiveDir  string // expanded absolute path to ~/thoughts/archive/<name>

	// existence flags (checked during resolution)
	projectExists  bool
	thoughtsExists bool
	archiveExists  bool
}

func runClear(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve all projects upfront
	var targets []clearTarget
	var resolveErrors []string

	// Expand thoughts directory once
	expandedThoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	for _, name := range args {
		result, err := reg.ResolveOne(name)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': not found", name))
			continue
		}

		projectPath, err := expandProjectPath(result.Project.Path, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand path: %v", result.Name, err))
			continue
		}

		thoughtsDir := filepath.Join(expandedThoughtsDir, "projects", result.Name)
		archiveDir := filepath.Join(expandedThoughtsDir, "archive", result.Name)

		target := clearTarget{
			name:        result.Name,
			projectPath: projectPath,
			thoughtsDir: thoughtsDir,
			archiveDir:  archiveDir,
		}

		// Check what exists on disk
		if _, err := os.Stat(projectPath); err == nil {
			target.projectExists = true
		}
		if _, err := os.Stat(thoughtsDir); err == nil {
			target.thoughtsExists = true
		}
		if _, err := os.Stat(archiveDir); err == nil {
			target.archiveExists = true
		}

		targets = append(targets, target)
	}

	// If any names failed to resolve, report and abort
	if len(resolveErrors) > 0 {
		for _, e := range resolveErrors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", e)
		}
		return fmt.Errorf("failed to resolve %d project(s). Run 'overlord list --all' to see all projects", len(resolveErrors))
	}

	if len(targets) == 0 {
		return fmt.Errorf("no valid projects to clear")
	}

	// Display confirmation prompt listing everything to be removed
	confirmed, err := confirmClear(targets)
	if err != nil {
		return fmt.Errorf("failed to read confirmation: %w", err)
	}
	if !confirmed {
		fmt.Println("Cancelled")
		return nil
	}

	// Execute deletions
	var clearErrors []string

	for _, t := range targets {
		if err := clearOneProject(reg, t); err != nil {
			clearErrors = append(clearErrors, fmt.Sprintf("'%s': %v", t.name, err))
		}
	}

	// Save registry (once, after all clears)
	if err := saveRegistry(reg); err != nil {
		return err
	}

	// Print any errors
	if len(clearErrors) > 0 {
		errStyle := NewStyles().Error
		fmt.Println()
		for _, e := range clearErrors {
			fmt.Printf("%s %s\n", errStyle.Render("Failed"), e)
		}
	}

	if len(clearErrors) == len(targets) {
		return fmt.Errorf("all clear operations failed")
	}

	return nil
}

// clearOneProject removes a single project's files, thoughts, and registry entry.
func clearOneProject(reg *registry.Registry, t clearTarget) error {
	// Define styles (red success style intentional - destructive operation)
	s := NewStyles()
	labelStyle := s.Label
	successStyle := lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
	pathStyle := s.Path

	// Delete project directory
	if t.projectExists {
		if err := os.RemoveAll(t.projectPath); err != nil {
			return fmt.Errorf("failed to delete project directory: %w", err)
		}
	}

	// Delete thoughts directory (active location)
	if t.thoughtsExists {
		if err := os.RemoveAll(t.thoughtsDir); err != nil {
			// Warn but continue — thoughts are secondary
			fmt.Printf("%s Failed to delete thoughts directory for '%s': %v\n", s.StrongWarn.Render("Warning:"), t.name, err)
		}
	}

	// Delete thoughts directory (archive location)
	if t.archiveExists {
		if err := os.RemoveAll(t.archiveDir); err != nil {
			// Warn but continue — thoughts are secondary
			fmt.Printf("%s Failed to delete archived thoughts for '%s': %v\n", s.StrongWarn.Render("Warning:"), t.name, err)
		}
	}

	// Remove from registry
	delete(reg.Projects, t.name)

	// Print success
	fmt.Printf("%s '%s'\n", successStyle.Render("Cleared"), t.name)
	if t.projectExists {
		fmt.Printf("  %s %s\n", labelStyle.Render("Project:"), pathStyle.Render(formatPathWithTilde(t.projectPath)))
	}
	if t.thoughtsExists {
		fmt.Printf("  %s %s\n", labelStyle.Render("Thoughts:"), pathStyle.Render(formatPathWithTilde(t.thoughtsDir)))
	}
	if t.archiveExists {
		fmt.Printf("  %s %s\n", labelStyle.Render("Archived:"), pathStyle.Render(formatPathWithTilde(t.archiveDir)))
	}
	fmt.Println()

	return nil
}

// confirmClear displays everything that will be removed and prompts for confirmation.
func confirmClear(targets []clearTarget) (bool, error) {
	// Styles for the warning
	s := NewStyles()
	warnStyle := s.Error
	nameStyle := s.Label
	pathStyle := s.Path
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	fmt.Println()
	fmt.Printf("%s This will permanently delete the following:\n\n", warnStyle.Render("WARNING:"))

	for _, t := range targets {
		fmt.Printf("  %s\n", nameStyle.Render(t.name))
		fmt.Printf("    %s\n", dimStyle.Render("Registry entry"))

		if t.projectExists {
			fmt.Printf("    %s\n", pathStyle.Render(formatPathWithTilde(t.projectPath)))
		} else {
			fmt.Printf("    %s %s\n", pathStyle.Render(formatPathWithTilde(t.projectPath)), dimStyle.Render("(not found)"))
		}

		if t.thoughtsExists {
			fmt.Printf("    %s\n", pathStyle.Render(formatPathWithTilde(t.thoughtsDir)))
		}

		if t.archiveExists {
			fmt.Printf("    %s\n", pathStyle.Render(formatPathWithTilde(t.archiveDir)))
		}

		if !t.thoughtsExists && !t.archiveExists {
			fmt.Printf("    %s\n", dimStyle.Render("No thoughts directories found"))
		}

		fmt.Println()
	}

	fmt.Printf("%s This cannot be undone.\n", warnStyle.Render("CAUTION:"))
	return confirmStrict("")
}

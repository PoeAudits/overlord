package cmd

import (
	"fmt"
	"os"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
)

// clearCmd represents the clear command
var clearCmd = &cobra.Command{
	Use:   "clear <name> [name...]",
	Short: "Permanently remove projects and their files",
	Long: `Permanently remove one or more projects from the registry and delete their
project directories.

This is a destructive operation that cannot be undone. A confirmation prompt will
list everything that will be removed before proceeding.

Works on both active and archived projects.

Name arguments can be project names or aliases.

What gets removed:
  - Registry entry
  - Project directory (e.g. ~/Overlord/web/myproject/)

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

	// existence flags (checked during resolution)
	projectExists bool
}

func runClear(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve all projects upfront
	s := NewStyles()
	targets, resolveErrors := resolveClearTargets(reg, args)

	// Print resolution errors and continue with valid targets
	printSkippedProjects(resolveErrors, s)

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
		errStyle := s.Error
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

// clearOneProject removes a single project's files and registry entry.
func clearOneProject(reg *registry.Registry, t clearTarget) error {
	// Define styles (red success style intentional - destructive operation)
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Destructive
	pathStyle := s.Path

	// Delete project directory
	if t.projectExists {
		if err := os.RemoveAll(t.projectPath); err != nil {
			return fmt.Errorf("failed to delete project directory: %w", err)
		}
	}

	// Remove from registry
	delete(reg.Projects, t.name)

	// Print success
	fmt.Printf("%s '%s'\n", successStyle.Render("Cleared"), t.name)
	if t.projectExists {
		fmt.Printf("  %s %s\n", labelStyle.Render("Project:"), pathStyle.Render(fileutil.FormatPathWithTilde(t.projectPath)))
	}
	fmt.Println()

	return nil
}

func resolveClearTargets(reg *registry.Registry, args []string) ([]clearTarget, []string) {
	var targets []clearTarget
	var resolveErrors []string

	for _, name := range args {
		result, err := reg.ResolveOne(name)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': not found", name))
			continue
		}

		projectPath, err := fileutil.ExpandProjectPath(result.Project.Path, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand path: %v", result.Name, err))
			continue
		}

		target := clearTarget{
			name:        result.Name,
			projectPath: projectPath,
		}

		// Check what exists on disk
		if _, err := os.Stat(projectPath); err == nil {
			target.projectExists = true
		}

		targets = append(targets, target)
	}

	return targets, resolveErrors
}

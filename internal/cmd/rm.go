package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rmForce bool
)

// rmCmd represents the rm command
var rmCmd = &cobra.Command{
	Use:   "rm <name> [name...]",
	Short: "Remove one or more projects from the registry",
	Long: `Remove one or more projects from the registry. By default, only removes the
registry entries and leaves project directories intact.

Use --force to also delete project directories from disk (requires confirmation).

Name arguments can be project names or aliases.

Note: Thoughts directories are never deleted and remain at:
  ~/thoughts/projects/<project>/
  ~/thoughts/archive/<project>/

Examples:
  overlord rm myproject              # Remove one project from registry
  overlord rm proj1 proj2 proj3      # Remove multiple projects
  overlord rm myproject --force      # Remove from registry and delete files`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVar(&rmForce, "force", false, "Delete project directory from disk (requires confirmation)")
}

func runRm(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve all projects first before making any changes
	type resolvedProject struct {
		name string
		path string
	}
	var resolved []resolvedProject
	var resolveErrors []string

	for _, name := range args {
		result, err := reg.ResolveOne(name)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s' not found", name))
			continue
		}
		projectPath, err := expandProjectPath(result.Project.Path, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand path: %v", name, err))
			continue
		}
		resolved = append(resolved, resolvedProject{name: result.Name, path: projectPath})
	}

	// If any names failed to resolve, report and abort
	if len(resolveErrors) > 0 {
		for _, e := range resolveErrors {
			fmt.Fprintf(os.Stderr, "Error: %s\n", e)
		}
		return fmt.Errorf("failed to resolve %d project(s). Run 'overlord list' to see all projects", len(resolveErrors))
	}

	// Handle force mode (delete files)
	if rmForce {
		for _, p := range resolved {
			if _, err := os.Stat(p.path); os.IsNotExist(err) {
				fmt.Printf("Warning: Project directory does not exist: %s\n", p.path)
				fmt.Println("Removing from registry only...")
			} else if err != nil {
				return fmt.Errorf("failed to check project directory '%s': %w", p.name, err)
			} else {
				confirmed, err := confirmDelete(p.name, p.path)
				if err != nil {
					return fmt.Errorf("failed to read confirmation: %w", err)
				}
				if !confirmed {
					fmt.Printf("Skipping '%s'\n", p.name)
					continue
				}
				if err := os.RemoveAll(p.path); err != nil {
					return fmt.Errorf("failed to delete project directory '%s': %w", p.name, err)
				}
			}
		}
	}

	// Remove all from registry
	for _, p := range resolved {
		delete(reg.Projects, p.name)
	}

	// Save registry
	if err := saveRegistry(reg); err != nil {
		return err
	}

	// Define styles for output
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Success
	pathStyle := s.Path

	// Print success messages
	for _, p := range resolved {
		if rmForce {
			fmt.Printf("%s '%s'\n", successStyle.Render("Deleted project"), p.name)
			fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(p.path))
		} else {
			fmt.Printf("%s '%s'\n", successStyle.Render("Removed from registry"), p.name)
			fmt.Printf("%s %s\n", labelStyle.Render("Directory:"), pathStyle.Render(p.path))
		}
	}

	// Print thoughts note
	thoughtsDir := reg.Settings.ThoughtsDir
	if len(resolved) == 1 {
		fmt.Printf("\n%s Thoughts at %s/{projects,archive}/%s/ remain intact\n",
			labelStyle.Render("Note:"), thoughtsDir, resolved[0].name)
	} else {
		fmt.Printf("\n%s Thoughts directories remain intact at %s/\n",
			labelStyle.Render("Note:"), thoughtsDir)
	}

	return nil
}

// confirmDelete prompts the user for confirmation before deleting
func confirmDelete(projectName, projectPath string) (bool, error) {
	return confirmAction(fmt.Sprintf("Delete project '%s' and all files at %s?", projectName, projectPath))
}

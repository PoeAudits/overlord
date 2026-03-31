package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/lifecycle"
)

type rmFlags struct {
	force bool
}

var rmOpts rmFlags

// rmCmd represents the rm command
var rmCmd = &cobra.Command{
	Use:   "rm <name> [name...]",
	Short: "Remove one or more projects from the registry",
	Long: `Remove one or more projects from the registry. By default, only removes the
registry entries and leaves project directories intact.

Use --force to also delete project directories from disk (requires confirmation).

Name arguments can be project names or aliases.

Examples:
  overlord rm myproject              # Remove one project from registry
  overlord rm proj1 proj2 proj3      # Remove multiple projects
  overlord rm myproject --force      # Remove from registry and delete files`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVar(&rmOpts.force, "force", false, "Delete project directory from disk (requires confirmation)")
}

func runRm(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve all projects first before making any changes
	s := NewStyles()
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
		projectPath, err := fileutil.ExpandProjectPath(result.Project.Path, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand path: %v", name, err))
			continue
		}
		resolved = append(resolved, resolvedProject{name: result.Name, path: projectPath})
	}

	// Print resolution errors and continue with valid projects
	if len(resolveErrors) > 0 {
		for _, e := range resolveErrors {
			fmt.Printf("%s %s\n", s.StrongWarn.Render("Skipping"), e)
		}
		fmt.Println()
	}

	if len(resolved) == 0 {
		return cmdError("rm", "no valid projects to remove", "Run 'overlord list' to see available projects")
	}

	// Handle force mode (delete files)
	if rmOpts.force {
		for _, p := range resolved {
			_, err := os.Stat(p.path)
			if errors.Is(err, fs.ErrNotExist) {
				fmt.Printf("Warning: Project directory does not exist: %s\n", p.path)
				fmt.Println("Removing from registry only...")
				continue
			}

			if err != nil {
				return fmt.Errorf("failed to check project directory '%s': %w", p.name, err)
			}

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

	// Remove all from registry
	for _, p := range resolved {
		preRemoveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
			Type:        lifecycle.ProjectRemoving,
			ProjectName: p.name,
			ProjectPath: p.path,
			Settings:    nil,
		})
		if preRemoveResult.Err != nil {
			return fmt.Errorf("pre-remove hook failed for '%s': %w", p.name, preRemoveResult.Err)
		}

		delete(reg.Projects, p.name)
	}

	// Save registry
	if err := saveRegistry(reg); err != nil {
		return err
	}

	for _, p := range resolved {
		postRemoveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
			Type:        lifecycle.ProjectRemoved,
			ProjectName: p.name,
			ProjectPath: p.path,
			Settings:    nil,
		})
		for _, warning := range postRemoveResult.Warnings {
			fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
		}
	}

	// Define styles for output
	labelStyle := s.Label
	successStyle := s.Success
	pathStyle := s.Path

	// Print success messages
	for _, p := range resolved {
		if rmOpts.force {
			fmt.Printf("%s '%s'\n", successStyle.Render("Deleted project"), p.name)
			fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(p.path))
		} else {
			fmt.Printf("%s '%s'\n", successStyle.Render("Removed from registry"), p.name)
			fmt.Printf("%s %s\n", labelStyle.Render("Directory:"), pathStyle.Render(p.path))
		}
	}

	return nil
}

package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
)

type rmFlags struct {
	force bool
}

var rmOpts rmFlags

var (
	rmLoadRegistry  = loadRegistry
	rmSaveRegistry  = saveRegistry
	rmConfirmDelete = confirmDelete
	rmRemoveAll     = os.RemoveAll
)

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
	reg, err := rmLoadRegistry()
	if err != nil {
		return err
	}

	// Resolve all projects first before making any changes
	s := NewStyles()
	resolved, resolveErrors := resolveRemovalProjects(reg, args)

	// Print resolution errors and continue with valid projects
	printSkippedProjects(resolveErrors, s)

	if len(resolved) == 0 {
		return cmdError("rm", "no valid projects to remove", "Run 'overlord list' to see available projects")
	}

	// Handle force mode (delete files)
	if rmOpts.force {
		resolved, err = deleteRemovalDirectories(resolved)
		if err != nil {
			return err
		}
		if len(resolved) == 0 {
			return nil
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
	if err := rmSaveRegistry(reg); err != nil {
		return err
	}

	for _, p := range resolved {
		postRemoveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
			Type:        lifecycle.ProjectRemoved,
			ProjectName: p.name,
			ProjectPath: p.path,
			Settings:    nil,
		})
		printLifecycleWarnings(postRemoveResult.Warnings)
	}

	message, label := "Removed from registry", "Directory:"
	if rmOpts.force {
		message, label = "Deleted project", "Path:"
	}
	for _, p := range resolved {
		fmt.Printf("%s '%s'\n", s.Success.Render(message), p.name)
		fmt.Printf("%s %s\n", s.Label.Render(label), s.Path.Render(p.path))
	}

	return nil
}

type removalProject struct{ name, path string }

func resolveRemovalProjects(reg *registry.Registry, args []string) ([]removalProject, []string) {
	var resolved []removalProject
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
		resolved = append(resolved, removalProject{name: result.Name, path: projectPath})
	}

	return resolved, resolveErrors
}

func deleteRemovalDirectories(resolved []removalProject) ([]removalProject, error) {
	filtered := make([]removalProject, 0, len(resolved))
	for _, p := range resolved {
		_, err := os.Stat(p.path)
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Printf("Warning: Project directory does not exist: %s\n", p.path)
			fmt.Println("Removing from registry only...")
			filtered = append(filtered, p)
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("failed to check project directory '%s': %w", p.name, err)
		}

		confirmed, err := rmConfirmDelete(p.name, p.path)
		if err != nil {
			return nil, fmt.Errorf("failed to read confirmation: %w", err)
		}
		if !confirmed {
			fmt.Printf("Skipping '%s'\n", p.name)
			continue
		}
		if err := rmRemoveAll(p.path); err != nil {
			return nil, fmt.Errorf("failed to delete project directory '%s': %w", p.name, err)
		}
		filtered = append(filtered, p)
	}
	return filtered, nil
}

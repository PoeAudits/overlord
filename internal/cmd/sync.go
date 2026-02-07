package cmd

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"
	"github.com/spf13/cobra"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync [project]",
	Short: "Show Syncthing sync status",
	Long: `Show Syncthing sync status for active projects.

If no project is specified, shows status for all projects with sync status 'active'.
If a project name is provided, shows status only for that project.

Examples:
  overlord sync                    # Status for all active projects
  overlord sync myproject          # Status for a specific project`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

// syncResult tracks the result of checking a single project.
type syncResult struct {
	name   string
	status *syncthing.FolderStatus
	err    error
}

// runSync handles the sync command logic
func runSync(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// 1. Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	client, err := localSyncthingClient(machineConfig)
	if err != nil {
		return err
	}

	// 2. Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// 3. Determine which projects to sync
	projectsToSync, err := resolveSyncProjects(args, reg, s)
	if err != nil {
		return err
	}
	if projectsToSync == nil {
		return nil // No projects to sync (message already printed)
	}

	results := make([]syncResult, 0, len(projectsToSync))
	ctx := context.Background()
	for _, name := range sortedProjectNames(projectsToSync) {
		result := syncResult{name: name}
		status, err := client.GetFolderStatus(ctx, name)
		if err != nil {
			result.err = err
		} else {
			result.status = status
		}
		results = append(results, result)
	}

	fmt.Printf("\n%s\n", s.Label.Render("Syncthing status:"))
	for _, result := range results {
		fmt.Printf("  %s %s\n", s.Label.Render("Project:"), result.name)
		if result.err != nil {
			if syncthing.IsNotFound(result.err) {
				fmt.Printf("    %s folder not found in Syncthing\n", s.Warn.Render("Warning:"))
			} else {
				fmt.Printf("    %s %v\n", s.Error.Render("Error:"), result.err)
			}
			continue
		}
		fmt.Printf("    %s %s\n", s.Label.Render("State:"), result.status.State)
		fmt.Printf("    %s %s\n", s.Label.Render("Need:"), formatBytes(result.status.NeedBytes))
		fmt.Printf("    %s %s\n", s.Label.Render("Local:"), formatBytes(result.status.LocalBytes))
		fmt.Printf("    %s %s\n", s.Label.Render("Global:"), formatBytes(result.status.GlobalBytes))
		if result.status.Errors > 0 || result.status.PullErrors > 0 {
			fmt.Printf("    %s errors=%d pull=%d\n", s.Warn.Render("Warning:"), result.status.Errors, result.status.PullErrors)
		}
	}

	return nil
}

// getSyncActiveProjects returns all projects with sync status 'active'
func getSyncActiveProjects(reg *registry.Registry) map[string]registry.Project {
	result := make(map[string]registry.Project)
	for name, project := range reg.Projects {
		if project.Sync.Status == registry.SyncActive {
			result[name] = project
		}
	}
	return result
}

// resolveSyncProjects determines which projects to sync based on args.
// If args is empty, returns all sync-active projects.
// If args has a project name, resolves it and verifies it's sync-active.
// Returns nil map (not error) if no projects to sync, with a message printed.
func resolveSyncProjects(args []string, reg *registry.Registry, s Styles) (map[string]registry.Project, error) {
	var projectsToSync map[string]registry.Project

	if len(args) > 0 {
		// Specific project requested
		resolvedName, project, err := resolveProject(reg, args[0])
		if err != nil {
			return nil, err
		}

		// Check if project has sync enabled
		if project.Sync.Status != registry.SyncActive {
			return nil, fmt.Errorf("project '%s' is not active for sync\nRun 'overlord activate %s' to enable sync first", resolvedName, resolvedName)
		}

		projectsToSync = map[string]registry.Project{
			resolvedName: project,
		}
	} else {
		// Get all sync-active projects
		projectsToSync = getSyncActiveProjects(reg)
	}

	if len(projectsToSync) == 0 {
		fmt.Printf("%s No projects are active for sync\n", s.Info.Render("Info:"))
		fmt.Printf("  Run 'overlord activate <project>' to enable sync for a project\n")
		return nil, nil
	}

	return projectsToSync, nil
}

// sortedProjectNames returns project names sorted alphabetically.
func sortedProjectNames(projects map[string]registry.Project) []string {
	names := make([]string, 0, len(projects))
	for name := range projects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	base := float64(value)
	exp := int(math.Min(float64(len(units)-1), math.Floor(math.Log(base)/math.Log(1024))))
	formatted := base / math.Pow(1024, float64(exp))
	return fmt.Sprintf("%.1f %s", formatted, units[exp])
}

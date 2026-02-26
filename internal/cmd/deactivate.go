package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	syncpkg "github.com/PoeAudits/overlord/internal/sync"
	"github.com/PoeAudits/overlord/internal/syncthing"
	"github.com/spf13/cobra"
)

// deactivateFlags holds the flags for the deactivate command
type deactivateFlags struct {
	peer         string
	removeRemote bool
	all          bool
}

var deactivateOpts deactivateFlags

// deactivateCmd represents the deactivate command
var deactivateCmd = &cobra.Command{
	Use:   "deactivate [name ...]",
	Short: "Deactivate Syncthing sync for a project",
	Long: `Deactivate a project by removing its Syncthing folder and updating
the registry sync status to 'inactive'. Optionally remove the folder on the
remote peer manually as well.

Arguments can be project names or aliases. Use --all to deactivate every project.

Flags:
  --peer          Syncthing peer name from peers.yaml
  --remove-remote Print reminder to remove the folder on the peer machine
  --all           Deactivate sync for all projects

Examples:
  overlord deactivate myproject
  overlord deactivate app api
  overlord deactivate --all
  overlord deactivate myproject --peer vps --remove-remote`,
	Args: cobra.ArbitraryArgs,
	RunE: runDeactivate,
}

func init() {
	rootCmd.AddCommand(deactivateCmd)
	deactivateCmd.Flags().StringVar(&deactivateOpts.peer, "peer", "", "Syncthing peer name from peers.yaml")
	deactivateCmd.Flags().BoolVar(&deactivateOpts.removeRemote, "remove-remote", false, "Print reminder to remove folder on peer machine")
	deactivateCmd.Flags().BoolVar(&deactivateOpts.all, "all", false, "Deactivate sync for all projects")
}

// runDeactivate handles the deactivate command logic
func runDeactivate(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	if deactivateOpts.all && len(args) > 0 {
		return fmt.Errorf("--all cannot be combined with project names")
	}
	if !deactivateOpts.all && len(args) == 0 {
		return fmt.Errorf("requires at least one project name or --all")
	}

	// 1. Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	localClient, err := syncpkg.LocalClient(machineConfig)
	if err != nil {
		return err
	}

	if deactivateOpts.removeRemote {
		peersConfig, loadErr := machine.LoadPeers(machine.DefaultPeersPath)
		if loadErr != nil {
			return fmt.Errorf("failed to load peers config: %w", loadErr)
		}
		_, err = syncpkg.ResolvePeer(peersConfig, deactivateOpts.peer)
		if err != nil {
			return err
		}
	}

	// 2. Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	targets, err := resolveDeactivateTargets(reg, args, deactivateOpts.all)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Printf("%s No projects selected for deactivation\n", s.Info.Render("Info:"))
		return nil
	}

	if len(targets) > 1 {
		if err := enforceRegistryLockOwnership(); err != nil {
			return err
		}
	}

	results := make([]string, 0, len(targets))
	failures := 0
	updated := false

	for _, projectName := range targets {
		project := reg.Projects[projectName]
		changed, deactivateErr := deactivateProjectSync(context.Background(), localClient, projectName, project)
		if deactivateErr != nil {
			failures++
			results = append(results, fmt.Sprintf("%s %s - %v", s.Error.Render("✗"), projectName, deactivateErr))
			continue
		}

		if changed {
			project.Sync.Status = registry.SyncInactive
			project.Sync.Method = registry.SyncMethodSyncthing
			reg.Projects[projectName] = project
			updated = true
			results = append(results, fmt.Sprintf("%s %s", s.Success.Render("✓"), projectName))
		} else {
			results = append(results, fmt.Sprintf("%s %s (already inactive)", s.Info.Render("i"), projectName))
		}
	}

	for _, line := range results {
		fmt.Println(line)
	}

	if updated {
		if err := saveRegistry(reg); err != nil {
			return err
		}
	}

	if deactivateOpts.removeRemote {
		fmt.Printf("%s Remote folder removal is no longer automatic. Run this command on the peer machine for the same project names to remove remote folders.\n",
			s.Warn.Render("Warning:"))
	}

	if failures > 0 {
		return fmt.Errorf("deactivate completed with %d failure(s)", failures)
	}

	if len(targets) == 1 {
		fmt.Printf("%s inactive\n", s.Label.Render("Sync status:"))
	}

	return nil
}

func resolveDeactivateTargets(reg *registry.Registry, args []string, all bool) ([]string, error) {
	if all {
		names := make([]string, 0, len(reg.Projects))
		for name := range reg.Projects {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, nil
	}

	seen := make(map[string]struct{}, len(args))
	targets := make([]string, 0, len(args))
	for _, arg := range args {
		resolved, _, err := resolveProject(reg, arg)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		targets = append(targets, resolved)
	}

	return targets, nil
}

func deactivateProjectSync(ctx context.Context, localClient *syncthing.Client, projectName string, project registry.Project) (bool, error) {
	if project.Status.State == registry.StateArchived {
		return false, fmt.Errorf("archived projects cannot be deactivated from sync")
	}

	if project.Sync.Status != registry.SyncActive {
		return false, nil
	}

	if err := localClient.DeleteFolder(ctx, projectName); err != nil && !syncthing.IsNotFound(err) {
		return false, fmt.Errorf("failed to remove local syncthing folder: %w", err)
	}

	return true, nil
}

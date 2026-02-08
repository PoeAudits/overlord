package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"
	"github.com/spf13/cobra"
)

// activateCmd represents the activate command
var activateCmd = &cobra.Command{
	Use:   "activate [name ...]",
	Short: "Activate Syncthing sync for a project",
	Long: `Activate Syncthing sync by registering the project as a shared folder
on the local Syncthing instance and sharing it with a configured peer device.
The project is also marked as sync-active in the registry.

Arguments can be project names or aliases. Use --all to activate every project.

Examples:
	 overlord activate myproject         # Activate sync for myproject
	 overlord activate app api web       # Activate multiple projects
	 overlord activate --all             # Activate all projects
	 overlord activate myproject --peer vps
	 overlord activate mp                # Activate using alias`,
	Args: cobra.ArbitraryArgs,
	RunE: runActivate,
}

type activateFlags struct {
	peer string
	all  bool
}

var activateOpts activateFlags

func init() {
	rootCmd.AddCommand(activateCmd)
	activateCmd.Flags().StringVar(&activateOpts.peer, "peer", "", "Syncthing peer name from peers.yaml")
	activateCmd.Flags().BoolVar(&activateOpts.all, "all", false, "Activate sync for all projects")
}

// runActivate handles the activate command logic
func runActivate(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	if activateOpts.all && len(args) > 0 {
		return fmt.Errorf("--all cannot be combined with project names")
	}
	if !activateOpts.all && len(args) == 0 {
		return fmt.Errorf("requires at least one project name or --all")
	}

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	targets, err := resolveActivateTargets(reg, args, activateOpts.all)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		fmt.Printf("%s No projects selected for activation\n", s.Info.Render("Info:"))
		return nil
	}

	if len(targets) > 1 {
		if err := enforceRegistryLockOwnership(); err != nil {
			return err
		}
	}

	// Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	localClient, err := localSyncthingClient(machineConfig)
	if err != nil {
		return err
	}

	peersConfig, err := machine.LoadPeers(machine.DefaultPeersPath)
	if err != nil {
		return fmt.Errorf("failed to load peers config: %w", err)
	}

	peerSelection, err := resolveSyncthingPeer(peersConfig, activateOpts.peer)
	if err != nil {
		return err
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}
	ctx := context.Background()

	results := make([]string, 0, len(targets))
	failures := make([]string, 0)
	updated := false

	for _, projectName := range targets {
		project := reg.Projects[projectName]
		peerProjectPath, activateErr := activateProjectSync(ctx, reg, baseDir, machineConfig, localClient, peerSelection, projectName, project)
		if activateErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", projectName, activateErr))
			results = append(results, fmt.Sprintf("%s %s - %v", s.Error.Render("✗"), projectName, activateErr))
			continue
		}

		project.Sync.Status = registry.SyncActive
		project.Sync.Method = registry.SyncMethodSyncthing
		reg.Projects[projectName] = project
		updated = true
		results = append(results, fmt.Sprintf("%s %s (peer: %s, path: %s)", s.Success.Render("✓"), projectName, peerSelection.name, peerProjectPath))
	}

	for _, line := range results {
		fmt.Println(line)
	}

	if updated {
		if err := saveRegistry(reg); err != nil {
			return err
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("activate completed with %d failure(s)", len(failures))
	}

	if len(targets) == 1 {
		fmt.Printf("%s sync active\n", s.Label.Render("Status:"))
	}

	return nil
}

func resolveActivateTargets(reg *registry.Registry, args []string, all bool) ([]string, error) {
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

func activateProjectSync(
	ctx context.Context,
	reg *registry.Registry,
	baseDir string,
	machineConfig *machine.MachineConfig,
	localClient *syncthing.Client,
	peerSelection syncthingPeerSelection,
	projectName string,
	project registry.Project,
) (string, error) {
	if project.Status.State == registry.StateArchived {
		return "", fmt.Errorf("archived (run 'overlord unarchive %s' first)", projectName)
	}

	projectPath := resolveProjectPath(project.Path, baseDir)
	peerProjectPath, err := projectPathForPeer(project, reg.Settings.BaseDir, peerSelection.peer.BaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve peer path: %w", err)
	}

	if err := ensureIgnoreFile(projectPath); err != nil {
		return "", err
	}

	if err := ensureSyncthingDevice(ctx, localClient, peerSelection.peer.Syncthing.DeviceID, peerSelection.name, peerSelection.peer.Syncthing.Address); err != nil {
		return "", fmt.Errorf("failed to ensure peer device on local syncthing: %w", err)
	}

	localFolder := buildSyncthingFolder(projectName, projectPath, machineConfig.Syncthing.DeviceID, peerSelection.peer.Syncthing.DeviceID)
	if _, err := localClient.EnsureFolder(ctx, localFolder); err != nil {
		return "", fmt.Errorf("failed to configure local syncthing folder: %w", err)
	}

	return peerProjectPath, nil
}

package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	syncpkg "github.com/PoeAudits/overlord/internal/sync"
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

type activateContext struct {
	reg           *registry.Registry
	baseDir       string
	machineConfig *machine.MachineConfig
	localClient   *syncthing.Client
	peerSelection syncpkg.PeerSelection
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

	if err := enforceRegistryLockOwnership(); err != nil {
		return err
	}

	// Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	localClient, err := syncpkg.LocalClient(machineConfig)
	if err != nil {
		return err
	}

	peersConfig, err := machine.LoadPeers(machine.DefaultPeersPath)
	if err != nil {
		return fmt.Errorf("failed to load peers config: %w", err)
	}

	peerSelection, err := syncpkg.ResolvePeer(peersConfig, activateOpts.peer)
	if err != nil {
		return err
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}
	ctx := context.Background()
	activateCtx := activateContext{
		reg:           reg,
		baseDir:       baseDir,
		machineConfig: machineConfig,
		localClient:   localClient,
		peerSelection: peerSelection,
	}

	results := make([]string, 0, len(targets))
	failures := make([]string, 0)
	updated := false

	for _, projectName := range targets {
		project := reg.Projects[projectName]
		peerProjectPath, activateErr := activateProjectSync(ctx, activateCtx, projectName, project)
		if activateErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", projectName, activateErr))
			results = append(results, fmt.Sprintf("%s %s - %v", s.Error.Render("✗"), projectName, activateErr))
			continue
		}

		project.Sync.Status = registry.SyncActive
		project.Sync.Method = registry.SyncMethodSyncthing
		reg.Projects[projectName] = project
		updated = true
		results = append(results, fmt.Sprintf("%s %s (peer: %s, path: %s)", s.Success.Render("✓"), projectName, peerSelection.Name, peerProjectPath))
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
	activateCtx activateContext,
	projectName string,
	project registry.Project,
) (string, error) {
	if project.Status.State == registry.StateArchived {
		return "", fmt.Errorf("archived (run 'overlord unarchive %s' first)", projectName)
	}

	projectPath := fileutil.ResolveProjectPath(project.Path, activateCtx.baseDir)
	peerProjectPath, err := syncpkg.ProjectPathForPeer(project, activateCtx.reg.Settings.BaseDir, activateCtx.peerSelection.Peer.BaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve peer path: %w", err)
	}

	if err := syncpkg.EnsureIgnoreFile(projectPath); err != nil {
		return "", err
	}

	if err := syncpkg.EnsureDevice(ctx, activateCtx.localClient, activateCtx.peerSelection); err != nil {
		return "", fmt.Errorf("failed to ensure peer device on local syncthing: %w", err)
	}

	localFolder := syncpkg.BuildFolder(projectName, projectPath, activateCtx.machineConfig.Syncthing.DeviceID, activateCtx.peerSelection.Peer.Syncthing.DeviceID)
	if _, err := activateCtx.localClient.EnsureFolder(ctx, localFolder); err != nil {
		return "", fmt.Errorf("failed to configure local syncthing folder: %w", err)
	}

	return peerProjectPath, nil
}

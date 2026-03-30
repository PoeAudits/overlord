package cmd

import (
	"context"
	"fmt"

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
		return cmdError("activate", "--all cannot be combined with project names", "")
	}
	if !activateOpts.all && len(args) == 0 {
		return cmdError("activate", "requires at least one project name or --all", "Usage: overlord activate <name> [name...] or --all")
	}

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	targets, err := resolveProjectTargets(reg, args, activateOpts.all)
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
	failures := 0
	updated := false

	for _, projectName := range targets {
		project := reg.Projects[projectName]
		peerProjectPath, activateErr := activateProjectSync(ctx, activateCtx, projectName, project)
		if activateErr != nil {
			failures++
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

	if failures > 0 {
		return fmt.Errorf("activate completed with %d failure(s)", failures)
	}

	if len(targets) == 1 {
		fmt.Printf("%s sync active\n", s.Label.Render("Sync status:"))
	}

	return nil
}
func activateProjectSync(
	ctx context.Context,
	activateCtx activateContext,
	projectName string,
	project registry.Project,
) (string, error) {
	if project.Status.State == registry.StateArchived {
		return "", cmdError("activate", fmt.Sprintf("'%s' is archived", projectName), fmt.Sprintf("Run 'overlord unarchive %s' first", projectName))
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

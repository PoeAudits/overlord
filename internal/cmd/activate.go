package cmd

import (
	"context"
	"fmt"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
)

// activateCmd represents the activate command
var activateCmd = &cobra.Command{
	Use:   "activate <name>",
	Short: "Activate Syncthing sync for a project",
	Long: `Activate Syncthing sync by registering the project as a shared folder
on both the local and remote Syncthing instances. The project is also marked
as sync-active in the registry, which is committed and pushed to git.

The name argument can be either a project name or alias.

Examples:
  overlord activate myproject         # Activate sync for myproject
  overlord activate myproject --peer vps
  overlord activate mp                # Activate using alias`,
	Args: cobra.ExactArgs(1),
	RunE: runActivate,
}

type activateFlags struct {
	peer string
}

var activateOpts activateFlags

func init() {
	rootCmd.AddCommand(activateCmd)
	activateCmd.Flags().StringVar(&activateOpts.peer, "peer", "", "Syncthing peer name from machine.yaml")
}

// runActivate handles the activate command logic
func runActivate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := NewStyles()

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve project using fuzzy matching
	projectName, project, err := resolveProject(reg, name)
	if err != nil {
		return err
	}

	// Check if project is archived
	if project.Status.State == registry.StateArchived {
		return fmt.Errorf("project '%s' is archived\nRun 'overlord unarchive %s' first to restore it", projectName, projectName)
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

	peerClient, peerSelection, err := peerSyncthingClient(machineConfig, activateOpts.peer)
	if err != nil {
		return err
	}

	remoteBaseDir := peerSelection.peer.BaseDir
	if remoteBaseDir == "" {
		remoteBaseDir = reg.Settings.BaseDir
	}

	// Resolve project paths
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}
	projectPath := resolveProjectPath(project.Path, baseDir)
	peerProjectPath, err := projectPathForPeer(project, reg.Settings.BaseDir, peerSelection.peer.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve peer project path: %w", err)
	}

	// Ensure ignore file exists locally
	if err := ensureIgnoreFile(projectPath); err != nil {
		return err
	}

	// Ensure devices exist on both sides
	ctx := context.Background()
	if err := ensureSyncthingDevice(ctx, localClient, peerSelection.peer.DeviceID, peerSelection.name, peerSelection.peer.Address); err != nil {
		return fmt.Errorf("failed to ensure peer device on local syncthing: %w", err)
	}
	if err := ensureSyncthingDevice(ctx, peerClient, machineConfig.Syncthing.DeviceID, machineConfig.Name, machineConfig.Syncthing.Address); err != nil {
		return fmt.Errorf("failed to ensure local device on peer syncthing: %w", err)
	}

	// Ensure folders on both sides
	localFolder := buildSyncthingFolder(projectName, projectPath, machineConfig.Syncthing.DeviceID, peerSelection.peer.DeviceID)
	peerFolder := buildSyncthingFolder(projectName, peerProjectPath, machineConfig.Syncthing.DeviceID, peerSelection.peer.DeviceID)

	if _, err := localClient.EnsureFolder(ctx, localFolder); err != nil {
		return fmt.Errorf("failed to configure local syncthing folder: %w", err)
	}
	if _, err := peerClient.EnsureFolder(ctx, peerFolder); err != nil {
		return fmt.Errorf("failed to configure peer syncthing folder: %w", err)
	}

	// Update sync status
	project.Sync.Status = registry.SyncActive
	project.Sync.Method = registry.SyncMethodSyncthing
	reg.Projects[projectName] = project

	// Save registry
	if err := saveRegistry(reg); err != nil {
		return err
	}

	// Commit and push registry changes
	if err := commitAndPushRegistry(fmt.Sprintf("activate %s", projectName)); err != nil {
		return err
	}

	// Print success message
	fmt.Printf("%s '%s'\n", s.Success.Render("✓ Activated"), projectName)
	fmt.Printf("  %s active\n", s.Label.Render("Sync status:"))
	fmt.Printf("  %s syncthing\n", s.Label.Render("Sync method:"))
	fmt.Printf("  %s %s\n", s.Label.Render("Peer:"), peerSelection.name)
	return nil
}

package cmd

import (
	"context"
	"fmt"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"
	"github.com/spf13/cobra"
)

// deactivateFlags holds the flags for the deactivate command
type deactivateFlags struct {
	peer         string
	removeRemote bool
}

var deactivateOpts deactivateFlags

// deactivateCmd represents the deactivate command
var deactivateCmd = &cobra.Command{
	Use:   "deactivate <name>",
	Short: "Deactivate Syncthing sync for a project",
	Long: `Deactivate a project by removing its Syncthing folder and updating
the registry sync status to 'inactive'. Optionally remove the folder on the
remote peer as well.

The registry change is automatically committed and pushed to git.

The name argument can be either a project name or alias.

Flags:
  --peer          Syncthing peer name from machine.yaml
  --remove-remote Also remove the folder from the peer's Syncthing instance

Examples:
  overlord deactivate myproject
  overlord deactivate myproject --peer vps --remove-remote`,
	Args: cobra.ExactArgs(1),
	RunE: runDeactivate,
}

func init() {
	rootCmd.AddCommand(deactivateCmd)
	deactivateCmd.Flags().StringVar(&deactivateOpts.peer, "peer", "", "Syncthing peer name from machine.yaml")
	deactivateCmd.Flags().BoolVar(&deactivateOpts.removeRemote, "remove-remote", false, "Remove folder from the peer Syncthing instance")
}

// runDeactivate handles the deactivate command logic
func runDeactivate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Define styles for output
	s := NewStyles()

	// 1. Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	localClient, err := localSyncthingClient(machineConfig)
	if err != nil {
		return err
	}

	var peerClient *syncthing.Client
	if deactivateOpts.removeRemote {
		peerClient, _, err = peerSyncthingClient(machineConfig, deactivateOpts.peer)
		if err != nil {
			return err
		}
	}

	// 2. Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// 3. Resolve project using fuzzy matching
	projectName, project, err := resolveProject(reg, name)
	if err != nil {
		return err
	}

	// 4. Check if project is archived
	if project.Status.State == registry.StateArchived {
		return fmt.Errorf("project '%s' is archived\nArchived projects cannot be deactivated from sync", projectName)
	}

	// 5. Check if already inactive
	if project.Sync.Status != registry.SyncActive {
		fmt.Printf("%s Project '%s' is not active for sync (nothing to deactivate)\n",
			s.Info.Render("Info:"), projectName)
		return nil
	}

	// 6. Remove local folder from Syncthing
	if err := localClient.DeleteFolder(context.Background(), projectName); err != nil && !syncthing.IsNotFound(err) {
		return fmt.Errorf("failed to remove local syncthing folder: %w", err)
	}

	// 7. Remove remote folder if requested
	if deactivateOpts.removeRemote {
		if err := peerClient.DeleteFolder(context.Background(), projectName); err != nil && !syncthing.IsNotFound(err) {
			return fmt.Errorf("failed to remove peer syncthing folder: %w", err)
		}
	}

	// 8. Update sync status
	project.Sync.Status = registry.SyncInactive
	project.Sync.Method = registry.SyncMethodSyncthing
	reg.Projects[projectName] = project

	// 9. Save registry
	if err := saveRegistry(reg); err != nil {
		return err
	}

	// 10. Commit and push registry changes
	if err := commitAndPushRegistry(fmt.Sprintf("deactivate %s", projectName)); err != nil {
		return err
	}

	// 11. Print success message
	fmt.Printf("%s '%s'\n", s.Success.Render("✓ Deactivated"), projectName)
	fmt.Printf("  %s inactive\n", s.Label.Render("Sync status:"))
	if deactivateOpts.removeRemote {
		fmt.Printf("  %s removed from peer\n", s.Label.Render("Remote:"))
	}

	return nil
}

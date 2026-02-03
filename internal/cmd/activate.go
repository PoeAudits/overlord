package cmd

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
)

// activateCmd represents the activate command
var activateCmd = &cobra.Command{
	Use:   "activate <name>",
	Short: "Mark a project for sync",
	Long: `Mark a project for sync by setting its sync status to 'active'.

This command prepares a project for multi-machine synchronization by updating
its sync status in the registry. The registry change is automatically committed
and pushed to git so other machines can see the updated status.

After activating, use 'overlord sync' to perform the actual file synchronization.

The name argument can be either a project name or alias.

Examples:
  overlord activate myproject      # Activate sync for myproject
  overlord activate mp             # Activate using alias
  
Typical workflow:
  1. overlord activate myproject   # Mark for sync
  2. overlord sync                 # Sync files between machines`,
	Args: cobra.ExactArgs(1),
	RunE: runActivate,
}

func init() {
	rootCmd.AddCommand(activateCmd)
}

// runActivate handles the activate command logic
func runActivate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve project using fuzzy matching
	result, err := reg.ResolveOne(name)
	if err != nil {
		return fmt.Errorf("project '%s' not found\nRun 'overlord list' to see all projects", name)
	}
	projectName := result.Name
	project := result.Project

	// Check if project is archived
	if project.Status.State == registry.StateArchived {
		return fmt.Errorf("project '%s' is archived\nRun 'overlord unarchive %s' first to restore it", projectName, projectName)
	}

	// Check if already active for sync
	if project.Sync.Status == registry.SyncActive {
		fmt.Printf("%s Project '%s' is already active for sync\n", NewStyles().Warn.Render("Info:"), projectName)
		return nil
	}

	// Update sync status
	project.Sync.Status = registry.SyncActive
	reg.Projects[projectName] = project

	// Save registry
	if err := saveRegistry(reg); err != nil {
		return err
	}

	// Commit and push registry changes
	if err := commitAndPushRegistry(fmt.Sprintf("activate %s", projectName)); err != nil {
		return err
	}

	s := NewStyles()
	successStyle := s.Success
	labelStyle := s.Label

	// Print success message
	fmt.Printf("%s '%s'\n", successStyle.Render("✓ Activated"), projectName)
	fmt.Printf("  %s active\n", labelStyle.Render("Sync status:"))
	fmt.Printf("\n%s Run 'overlord sync' to synchronize files between machines\n", labelStyle.Render("Next step:"))

	return nil
}

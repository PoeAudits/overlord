package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/PoeAudits/overlord/internal/gitops"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/sync"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// deactivateFlags holds the flags for the deactivate command
type deactivateFlags struct {
	force       bool
	keepLocal   bool
	forceRemove bool
}

var deactivateOpts deactivateFlags

// deactivateCmd represents the deactivate command
var deactivateCmd = &cobra.Command{
	Use:   "deactivate <name>",
	Short: "Deactivate a project from sync",
	Long: `Deactivate a project from sync by setting its sync status to 'inactive'.

This command safely deactivates a project from multi-machine synchronization.
On working-set machines, it syncs final changes to storage before deactivating
to prevent data loss.

Behavior by machine role:

  Working-set machine:
    1. Syncs final changes to storage (safety measure)
    2. Updates sync status to 'inactive'
    3. Optionally removes local directory (with confirmation)

  Storage machine:
    - Only updates sync status (directory never removed)

The registry change is automatically committed and pushed to git.

The name argument can be either a project name or alias.

Flags:
  --force        Skip confirmation prompts for directory removal
  --keep-local   Keep the local directory (only update sync status)
  --force-remove Remove directory even if sync to storage failed

Examples:
  overlord deactivate myproject              # Deactivate and prompt for removal
  overlord deactivate myproject --keep-local # Deactivate but keep local files
  overlord deactivate myproject --force      # Deactivate and remove without prompt`,
	Args: cobra.ExactArgs(1),
	RunE: runDeactivate,
}

func init() {
	rootCmd.AddCommand(deactivateCmd)

	deactivateCmd.Flags().BoolVar(&deactivateOpts.force, "force", false, "Skip confirmation prompts")
	deactivateCmd.Flags().BoolVar(&deactivateOpts.keepLocal, "keep-local", false, "Keep local directory (only update sync status)")
	deactivateCmd.Flags().BoolVar(&deactivateOpts.forceRemove, "force-remove", false, "Remove directory even if sync failed")
}

// runDeactivate handles the deactivate command logic
func runDeactivate(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Define styles for output
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	yellow := lipgloss.Color("220")
	red := lipgloss.Color("196")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(yellow)
	errorStyle := lipgloss.NewStyle().Foreground(red).Bold(true)
	infoStyle := lipgloss.NewStyle().Foreground(gray)

	// 1. Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	isWorkingSet := machineConfig.Role == machine.RoleWorkingSet

	// 2. Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// 3. Resolve project using fuzzy matching
	result, err := reg.ResolveOne(name)
	if err != nil {
		return fmt.Errorf("project '%s' not found\nRun 'overlord list' to see all projects", name)
	}
	projectName := result.Name
	project := result.Project

	// 4. Check if project is archived
	if project.Status.State == registry.StateArchived {
		return fmt.Errorf("project '%s' is archived\nArchived projects cannot be deactivated from sync", projectName)
	}

	// 5. Check if already inactive
	if project.Sync.Status != registry.SyncActive {
		fmt.Printf("%s Project '%s' is not active for sync (nothing to deactivate)\n",
			infoStyle.Render("Info:"), projectName)
		return nil
	}

	// 6. Get project path
	baseDir, err := expandPath(reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base directory: %w", err)
	}
	projectPath := resolveProjectPath(project.Path, baseDir)

	// 7. Sync to storage if on working-set machine
	syncSucceeded := true
	if isWorkingSet {
		fmt.Printf("%s Syncing final changes to storage...\n", labelStyle.Render("Step 1:"))

		// Check if local directory exists
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			fmt.Printf("  %s Local directory not found, skipping sync\n", warnStyle.Render("Warning:"))
		} else {
			// Build remote path
			remotePath := sync.FormatRemotePath(machineConfig.StorageHost, resolveProjectPath(project.Path, baseDir))

			// Merge exclusion patterns
			excludes := make([]string, 0, len(reg.Settings.Sync.DefaultExclude)+len(project.Sync.Exclude))
			excludes = append(excludes, reg.Settings.Sync.DefaultExclude...)
			excludes = append(excludes, project.Sync.Exclude...)

			// Push to storage
			rsync := sync.NewRsync()
			pushOpts := sync.RsyncOptions{
				Source:   projectPath,
				Dest:     remotePath,
				Excludes: excludes,
			}

			_, err := rsync.Push(pushOpts)
			if err != nil {
				syncSucceeded = false
				fmt.Printf("  %s Sync failed: %v\n", errorStyle.Render("Error:"), err)

				if !deactivateOpts.forceRemove {
					fmt.Printf("  %s Use --force-remove to remove directory anyway\n", warnStyle.Render("Warning:"))
					fmt.Printf("  %s Aborting deactivation to prevent data loss\n", errorStyle.Render("Aborted:"))
					return fmt.Errorf("sync to storage failed, aborting deactivation")
				}
				fmt.Printf("  %s Continuing with --force-remove flag\n", warnStyle.Render("Warning:"))
			} else {
				fmt.Printf("  %s\n", successStyle.Render("Done"))
			}
		}
	}

	// 8. Update sync status
	stepNum := "Step 1:"
	if isWorkingSet {
		stepNum = "Step 2:"
	}
	fmt.Printf("%s Updating registry...\n", labelStyle.Render(stepNum))

	project.Sync.Status = registry.SyncInactive
	reg.Projects[projectName] = project

	// 9. Save registry
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	fmt.Printf("  %s\n", successStyle.Render("Done"))

	// 10. Commit and push registry changes
	stepNum = "Step 2:"
	if isWorkingSet {
		stepNum = "Step 3:"
	}
	fmt.Printf("%s Committing registry changes...\n", labelStyle.Render(stepNum))

	gitRepoPath, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand git repo path: %w", err)
	}

	git := gitops.New(gitRepoPath)

	if err := git.Add("registry.yaml"); err != nil {
		return fmt.Errorf("failed to stage registry: %w", err)
	}

	if err := git.Commit(fmt.Sprintf("deactivate %s", projectName)); err != nil {
		return fmt.Errorf("failed to commit registry: %w", err)
	}

	// Push - warn but don't fail if push fails
	if err := git.Push(); err != nil {
		fmt.Printf("  %s Failed to push registry: %v\n", warnStyle.Render("Warning:"), err)
	} else {
		fmt.Printf("  %s\n", successStyle.Render("Done"))
	}

	// 11. Remove local directory if on working-set and not --keep-local
	if isWorkingSet && !deactivateOpts.keepLocal {
		// Check if sync succeeded or force-remove is set
		if !syncSucceeded && !deactivateOpts.forceRemove {
			fmt.Printf("\n%s Local directory kept due to sync failure\n", infoStyle.Render("Note:"))
			fmt.Printf("  %s %s\n", labelStyle.Render("Path:"), projectPath)
		} else {
			// Check if directory exists
			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				fmt.Printf("\n%s Local directory does not exist\n", infoStyle.Render("Note:"))
			} else {
				// Prompt for confirmation unless --force
				shouldRemove := deactivateOpts.force
				if !shouldRemove {
					confirmed, err := confirmDirectoryRemoval(projectName, projectPath)
					if err != nil {
						fmt.Printf("  %s Failed to read confirmation: %v\n", warnStyle.Render("Warning:"), err)
					} else {
						shouldRemove = confirmed
					}
				}

				if shouldRemove {
					stepNum = "Step 3:"
					if isWorkingSet {
						stepNum = "Step 4:"
					}
					fmt.Printf("%s Removing local directory...\n", labelStyle.Render(stepNum))

					if err := os.RemoveAll(projectPath); err != nil {
						return fmt.Errorf("failed to remove project directory: %w", err)
					}
					fmt.Printf("  %s\n", successStyle.Render("Done"))
					fmt.Printf("  %s %s\n", infoStyle.Render("Removed:"), projectPath)
				} else {
					fmt.Printf("\n%s Local directory kept\n", infoStyle.Render("Note:"))
					fmt.Printf("  %s %s\n", labelStyle.Render("Path:"), projectPath)
				}
			}
		}
	}

	// 12. Print success message
	fmt.Printf("\n%s '%s'\n", successStyle.Render("✓ Deactivated"), projectName)
	fmt.Printf("  %s inactive\n", labelStyle.Render("Sync status:"))

	if !isWorkingSet {
		fmt.Printf("\n%s This is a storage machine - directory not removed\n", infoStyle.Render("Note:"))
	} else if deactivateOpts.keepLocal {
		fmt.Printf("\n%s Local files preserved at: %s\n", infoStyle.Render("Note:"), projectPath)
	}

	return nil
}

// confirmDirectoryRemoval prompts the user for confirmation before removing directory
func confirmDirectoryRemoval(projectName, projectPath string) (bool, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Not interactive - don't proceed
		return false, nil
	}

	fmt.Printf("\nRemove local directory for '%s'?\n", projectName)
	fmt.Printf("  Path: %s\n", projectPath)
	fmt.Printf("  [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

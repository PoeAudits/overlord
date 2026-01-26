package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	rmForce bool
)

// rmCmd represents the rm command
var rmCmd = &cobra.Command{
	Use:   "rm <name>",
	Short: "Remove a project from the registry",
	Long: `Remove a project from the registry. By default, only removes the registry
entry and leaves the project directory intact.

Use --force to also delete the project directory from disk (requires confirmation).

The name argument can be either a project name or alias.

Note: Thoughts directories are never deleted and remain at:
  ~/thoughts/plans/<project>/
  ~/thoughts/logs/<project>/
  ~/thoughts/sessions/<project>/

Examples:
  overlord rm myproject              # Remove from registry only
  overlord rm myproject --force      # Remove from registry and delete files`,
	Args: cobra.ExactArgs(1),
	RunE: runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolVar(&rmForce, "force", false, "Delete project directory from disk (requires confirmation)")
}

func runRm(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Resolve project using fuzzy matching
	result, err := reg.ResolveOne(name)
	if err != nil {
		return fmt.Errorf("Error: project '%s' not found\nRun 'overlord list' to see all projects", name)
	}
	projectName := result.Name
	project := result.Project

	// Get absolute project path
	projectPath, err := expandProjectPath(project.Path, reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand project path: %w", err)
	}

	// Handle force mode (delete files)
	if rmForce {
		// Check if directory exists
		if _, err := os.Stat(projectPath); os.IsNotExist(err) {
			fmt.Printf("Warning: Project directory does not exist: %s\n", projectPath)
			fmt.Println("Removing from registry only...")
		} else if err != nil {
			return fmt.Errorf("failed to check project directory: %w", err)
		} else {
			// Directory exists, prompt for confirmation
			confirmed, err := confirmDelete(projectName, projectPath)
			if err != nil {
				return fmt.Errorf("failed to read confirmation: %w", err)
			}

			if !confirmed {
				fmt.Println("Cancelled")
				return nil
			}

			// Delete directory
			if err := os.RemoveAll(projectPath); err != nil {
				return fmt.Errorf("failed to delete project directory: %w", err)
			}
		}
	}

	// Remove from registry
	delete(reg.Projects, projectName)

	// Save registry
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Define styles for output
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(gray)

	// Print success message
	if rmForce {
		fmt.Printf("%s '%s'\n", successStyle.Render("Deleted project"), projectName)
		fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(projectPath))
	} else {
		fmt.Printf("%s '%s'\n", successStyle.Render("Removed from registry"), projectName)
		fmt.Printf("%s %s\n", labelStyle.Render("Directory:"), pathStyle.Render(projectPath))
	}

	// Print thoughts note
	thoughtsDir := reg.Settings.ThoughtsDir
	fmt.Printf("\n%s Thoughts at %s/{plans,logs,sessions}/%s/ remain intact\n",
		labelStyle.Render("Note:"), thoughtsDir, projectName)

	return nil
}

// confirmDelete prompts the user for confirmation before deleting
func confirmDelete(projectName, projectPath string) (bool, error) {
	fmt.Printf("Delete project '%s' and all files at %s? [y/N]: ", projectName, projectPath)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// expandProjectPath expands a project path to absolute path
func expandProjectPath(projectPath, baseDir string) (string, error) {
	// Expand base dir if it starts with ~
	expandedBaseDir, err := expandPath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base dir: %w", err)
	}

	// Resolve project path (handles both relative and absolute paths)
	fullPath := resolveProjectPath(projectPath, expandedBaseDir)

	// Clean the path
	return filepath.Clean(fullPath), nil
}

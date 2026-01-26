package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up overlord directories and configuration",
	Long: `Set up all required directories and configuration files for overlord.

Creates:
  - Config directory (~/.config/overlord/)
  - Git repository for registry sync (optional)
  - Registry file (~/.config/overlord/registry.yaml)
  - Machine config (~/.config/overlord/machine.yaml)
  - Base directory (~/Overlord by default)
  - Category directories (projects/*, core/*, sandbox/)
  - Thoughts directory (~/thoughts by default)

If directories or files already exist, they are left unchanged.
Git initialization prompts for a remote URL to clone from (for multi-machine sync).
Machine config creation prompts for machine role (storage or working-set).`,
	RunE: runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	fmt.Println("Setting up overlord...")

	// Expand config path first
	configDir, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand config path: %w", err)
	}

	// Step 1: Set up git repo (may clone, creating directory and registry)
	if err := setupGitRepo(configDir); err != nil {
		return err
	}

	// Step 2: Create config directory (no-op if clone already created it)
	if err := createDir(configDir, "config"); err != nil {
		return err
	}

	// Step 3: Ensure .gitignore has machine-specific exclusions
	if err := setupGitIgnore(configDir); err != nil {
		return err
	}

	// Step 4: Load or create registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Expand paths from settings
	baseDir, err := registry.ExpandPath(reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base_dir: %w", err)
	}

	thoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts_dir: %w", err)
	}

	// Create base directory
	if err := createDir(baseDir, "base"); err != nil {
		return err
	}

	// Create category directories
	for _, category := range registry.ValidCategories() {
		categoryPath := filepath.Join(baseDir, category.Path())
		if err := createDir(categoryPath, string(category)); err != nil {
			return err
		}
	}

	// Create thoughts directory
	if err := createDir(thoughtsDir, "thoughts"); err != nil {
		return err
	}

	// Save registry (creates it if it doesn't exist)
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	fmt.Printf("  Registry: %s\n", registry.DefaultRegistryPath)

	// Set up machine config
	if err := setupMachineConfig(configDir); err != nil {
		return err
	}

	fmt.Println("\nSetup complete!")
	return nil
}

// setupGitRepo initializes or clones a git repository for registry sync.
// If the directory already has a .git, it reports existing and returns.
// Otherwise, prompts the user to set up git with an optional remote.
func setupGitRepo(configDir string) error {
	gitDir := filepath.Join(configDir, ".git")

	// Already a git repo
	if _, err := os.Stat(gitDir); err == nil {
		fmt.Printf("  Git:     already initialized\n")
		return nil
	}

	// Non-interactive: skip
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("  Git:     skipped (non-interactive)\n")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n  Initialize git for registry sync? [y/N]: ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		fmt.Printf("  Git:     skipped\n")
		return nil
	}

	// Ask for remote URL
	fmt.Printf("  Git remote URL (leave empty for local-only): ")
	remoteURL, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	remoteURL = strings.TrimSpace(remoteURL)

	// Check if config directory exists and is empty (can clone directly)
	dirExists := false
	dirEmpty := true
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		dirExists = true
		entries, err := os.ReadDir(configDir)
		if err == nil {
			dirEmpty = len(entries) == 0
		}
	}

	// Clone if remote provided and directory doesn't exist or is empty
	if remoteURL != "" && (!dirExists || dirEmpty) {
		fmt.Printf("  Cloning from %s...\n", remoteURL)
		cmd := exec.Command("git", "clone", remoteURL, configDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone: %w", err)
		}
		fmt.Printf("  Git:     cloned from remote\n")
		return nil
	}

	// Directory exists with files: init in place and optionally add remote
	if !dirExists {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	cmd := exec.Command("git", "init", configDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to init git: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	fmt.Printf("  Git:     initialized\n")

	if remoteURL != "" {
		// Add remote
		cmd = exec.Command("git", "-C", configDir, "remote", "add", "origin", remoteURL)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to add remote: %w (output: %s)", err, strings.TrimSpace(string(output)))
		}

		// Fetch from remote
		fmt.Printf("  Fetching from remote...\n")
		fetchCmd := exec.Command("git", "-C", configDir, "fetch", "origin")
		fetchCmd.Stdout = os.Stdout
		fetchCmd.Stderr = os.Stderr
		fetchCmd.Stdin = os.Stdin
		if err := fetchCmd.Run(); err != nil {
			fmt.Printf("  Warning: Failed to fetch from remote: %v\n", err)
			fmt.Printf("  You can fetch manually later with: git -C %s fetch origin\n", configDir)
		} else {
			// Try to reset to remote default branch
			for _, branch := range []string{"main", "master", "dev"} {
				resetCmd := exec.Command("git", "-C", configDir, "reset", "--hard", "origin/"+branch)
				if resetOutput, resetErr := resetCmd.CombinedOutput(); resetErr == nil {
					fmt.Printf("  Git:     synced with origin/%s\n", branch)
					break
				} else {
					_ = resetOutput // try next branch
				}
			}
		}
	}

	return nil
}

// setupGitIgnore ensures .gitignore contains machine-specific exclusions.
func setupGitIgnore(configDir string) error {
	gitDir := filepath.Join(configDir, ".git")
	// Only manage .gitignore if this is a git repo
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	gitignorePath := filepath.Join(configDir, ".gitignore")

	// Required entries for machine-specific files
	requiredEntries := []string{
		"machine.yaml",
		"*.bak",
	}

	// Read existing .gitignore
	existingContent := ""
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existingContent = string(data)
	}

	// Check which entries are missing
	var missing []string
	for _, entry := range requiredEntries {
		if !strings.Contains(existingContent, entry) {
			missing = append(missing, entry)
		}
	}

	if len(missing) == 0 {
		return nil
	}

	// Append missing entries
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open .gitignore: %w", err)
	}
	defer f.Close()

	// Add separator if file has content
	if existingContent != "" && !strings.HasSuffix(existingContent, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}

	if _, err := f.WriteString("\n# Machine-specific (added by setup)\n"); err != nil {
		return err
	}
	for _, entry := range missing {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	fmt.Printf("  Updated: %s\n", gitignorePath)
	return nil
}

// setupMachineConfig creates machine.yaml if it doesn't exist.
// It prompts the user interactively for the machine role.
func setupMachineConfig(configDir string) error {
	machineConfigPath := filepath.Join(configDir, "machine.yaml")

	// Check if machine.yaml already exists
	if _, err := os.Stat(machineConfigPath); err == nil {
		fmt.Printf("  Exists:  %s\n", machineConfigPath)
		return nil
	}

	fmt.Println("\nSetting up machine configuration...")

	// Get hostname for machine name
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	config := &machine.MachineConfig{
		Name: hostname,
		Role: machine.RoleStorage,
	}

	// If interactive terminal, prompt for role
	if term.IsTerminal(int(os.Stdin.Fd())) {
		reader := bufio.NewReader(os.Stdin)

		fmt.Printf("  Machine role? [s]torage / [w]orking-set (default: storage): ")
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response == "w" || response == "working-set" {
			config.Role = machine.RoleWorkingSet

			// Ask for storage host
			for {
				fmt.Printf("  Storage host (hostname or IP of storage machine): ")
				host, err := reader.ReadString('\n')
				if err != nil {
					return fmt.Errorf("failed to read input: %w", err)
				}
				host = strings.TrimSpace(host)
				if host != "" {
					config.StorageHost = host
					break
				}
				fmt.Println("  Storage host cannot be empty.")
			}
		}
	} else {
		fmt.Println("  Non-interactive terminal detected, defaulting to storage role")
	}

	// Save machine config
	if err := machine.Save(machineConfigPath, config); err != nil {
		return fmt.Errorf("failed to save machine config: %w", err)
	}

	fmt.Printf("  Created: %s\n", machineConfigPath)
	fmt.Printf("  Name:    %s\n", config.Name)
	fmt.Printf("  Role:    %s\n", config.Role)
	if config.StorageHost != "" {
		fmt.Printf("  Storage: %s\n", config.StorageHost)
	}

	return nil
}

// createDir creates a directory if it doesn't exist
func createDir(path, name string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", name, err)
		}
		fmt.Printf("  Created: %s\n", path)
	} else if err != nil {
		return fmt.Errorf("failed to check %s directory: %w", name, err)
	} else {
		fmt.Printf("  Exists:  %s\n", path)
	}
	return nil
}

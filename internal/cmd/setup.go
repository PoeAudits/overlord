package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
)

// DefaultConfigRepoURL is the default git remote for the registry config repository.
const DefaultConfigRepoURL = "git@github.com:PoeAudits/overlord-config.git"

var defaultRemoteBranchFallbacks = []string{"main", "master", "dev"}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up overlord configuration",
	Long: `Set up overlord configuration directories and registry.

Creates:
  - Config directory (~/.config/overlord/)
  - Managed oc shim (~/.local/bin/oc)
  - Git repository for registry sync (optional)
  - Registry file (~/.config/overlord/registry.yaml)
  - Base directory (~/Overlord by default)
  - Category directories (projects/*, core/*, sandbox/)

If directories or files already exist, they are left unchanged.
Git initialization prompts for a remote URL to clone from (for multi-machine sync).`,
	Args: cobra.NoArgs,
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
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Expand paths from settings
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
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

	// Save registry (creates it if it doesn't exist)
	if err := saveRegistry(reg); err != nil {
		return err
	}
	fmt.Printf("  Registry: %s\n", registry.DefaultRegistryPath)

	shimResult, err := ensureOCShimInstalled()
	if err != nil {
		return err
	}
	switch shimResult.Action {
	case ocShimInstalled:
		fmt.Printf("  oc shim:  installed at %s\n", fileutil.FormatPathWithTilde(shimResult.Path))
	case ocShimUpdated:
		fmt.Printf("  oc shim:  updated at %s\n", fileutil.FormatPathWithTilde(shimResult.Path))
	case ocShimSkipped:
		fmt.Printf("  oc shim:  skipped (%s is not Overlord-managed)\n", fileutil.FormatPathWithTilde(shimResult.Path))
	default:
		fmt.Printf("  oc shim:  already current at %s\n", fileutil.FormatPathWithTilde(shimResult.Path))
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
	remoteURL, setupRequested, err := promptGitSetup(reader)
	if err != nil {
		return err
	}
	if !setupRequested {
		fmt.Printf("  Git:     skipped\n")
		return nil
	}

	cloned, err := cloneOrInitRepo(configDir, remoteURL)
	if err != nil {
		return err
	}
	if cloned {
		return nil
	}

	return fetchAndSyncRemote(configDir, remoteURL)
}

func promptGitSetup(reader *bufio.Reader) (string, bool, error) {
	fmt.Printf("\n  Initialize git for registry sync? [y/N]: ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return "", false, fmt.Errorf("failed to read input: %w", err)
	}
	response = strings.TrimSpace(strings.ToLower(response))

	if response != "y" && response != "yes" {
		return "", false, nil
	}

	fmt.Printf("  Git remote URL (enter 'none' for local-only)\n")
	fmt.Printf("  [%s]: ", DefaultConfigRepoURL)
	remoteURL, err := reader.ReadString('\n')
	if err != nil {
		return "", false, fmt.Errorf("failed to read input: %w", err)
	}
	remoteURL = strings.TrimSpace(remoteURL)

	if remoteURL == "" {
		remoteURL = DefaultConfigRepoURL
	} else if strings.ToLower(remoteURL) == "none" {
		remoteURL = ""
	}

	return remoteURL, true, nil
}

func cloneOrInitRepo(configDir, remoteURL string) (bool, error) {
	dirExists := false
	dirEmpty := true
	if info, err := os.Stat(configDir); err == nil && info.IsDir() {
		dirExists = true
		entries, err := os.ReadDir(configDir)
		if err == nil {
			dirEmpty = len(entries) == 0
		}
	}

	if remoteURL != "" && (!dirExists || dirEmpty) {
		fmt.Printf("  Cloning from %s...\n", remoteURL)
		cmd := exec.Command("git", "clone", remoteURL, configDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err := cmd.Run(); err != nil {
			return false, fmt.Errorf("failed to clone: %w", err)
		}
		fmt.Printf("  Git:     cloned from remote\n")
		return true, nil
	}

	if !dirExists {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			return false, fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	cmd := exec.Command("git", "init", configDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to init git: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	fmt.Printf("  Git:     initialized\n")
	return false, nil
}

func fetchAndSyncRemote(configDir, remoteURL string) error {
	if remoteURL == "" {
		return nil
	}

	cmd := exec.Command("git", "-C", configDir, "remote", "add", "origin", remoteURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add remote: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}

	fmt.Printf("  Fetching from remote...\n")
	fetchCmd := exec.Command("git", "-C", configDir, "fetch", "origin")
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	fetchCmd.Stdin = os.Stdin
	if err := fetchCmd.Run(); err != nil {
		fmt.Printf("  Warning: Failed to fetch from remote: %v\n", err)
		fmt.Printf("  You can fetch manually later with: git -C %s fetch origin\n", configDir)
		return nil
	}

	for _, branch := range defaultRemoteBranchFallbacks {
		resetCmd := exec.Command("git", "-C", configDir, "reset", "--hard", "origin/"+branch)
		if resetOutput, resetErr := resetCmd.CombinedOutput(); resetErr == nil {
			fmt.Printf("  Git:     synced with origin/%s\n", branch)
			return nil
		} else {
			_ = resetOutput
		}
	}

	return nil
}

// setupGitIgnore ensures .gitignore contains machine-specific exclusions.
func setupGitIgnore(configDir string) error {
	gitDir := filepath.Join(configDir, ".git")
	// Only manage .gitignore if this is a git repo
	if _, err := os.Stat(gitDir); errors.Is(err, fs.ErrNotExist) {
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
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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

// createDir creates a directory if it doesn't exist
func createDir(path, name string) error {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(path, 0o755); err != nil {
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

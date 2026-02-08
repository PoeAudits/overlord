package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

const setupInitSeedTimeout = 60 * time.Second

// DefaultConfigRepoURL is the default git remote for the registry config repository.
const DefaultConfigRepoURL = "git@github.com:PoeAudits/overlord-config.git"

var setupCmd = &cobra.Command{
	Use:   "setup [project]",
	Short: "Set up overlord configuration or bootstrap a project",
	Long: `Set up overlord configuration (default) or bootstrap a synced project.

When called without arguments, creates:
  - Config directory (~/.config/overlord/)
  - Git repository for registry sync (optional)
  - Registry file (~/.config/overlord/registry.yaml)
  - Machine config (~/.config/overlord/machine.yaml)
  - Base directory (~/Overlord by default)
  - Category directories (projects/*, core/*, sandbox/)
  - Thoughts directory (~/thoughts by default)

If directories or files already exist, they are left unchanged.
 Git initialization prompts for a remote URL to clone from (for multi-machine sync).
 Machine config creation stores the current hostname and optional Syncthing config.

When called with a project name, it:
  - Verifies the Syncthing folder exists
  - Pulls .env from pass (overlord env pull <name>)
  - Installs dependencies based on language
  - Runs .overlord-setup.sh if present`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSetup,
}

type setupFlags struct {
	init bool
}

var setupOpts setupFlags

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVar(&setupOpts.init, "init", false, "Initialize Syncthing-based overlord config sync")
}

func runSetup(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return runProjectSetup(args[0])
	}

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
	if err := saveRegistry(reg); err != nil {
		return err
	}
	fmt.Printf("  Registry: %s\n", registry.DefaultRegistryPath)

	// Set up machine config
	if err := setupMachineConfig(configDir, setupOpts.init); err != nil {
		return err
	}

	if setupOpts.init {
		if err := runSetupInitFlow(); err != nil {
			return err
		}
	}

	fmt.Println("\nSetup complete!")
	return nil
}

func runProjectSetup(projectArg string) error {
	s := NewStyles()
	labelStyle := s.Label

	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	projectName, project, err := resolveProject(reg, projectArg)
	if err != nil {
		return err
	}

	if project.Sync.Status != registry.SyncActive {
		return fmt.Errorf("project '%s' is not active for sync\nRun 'overlord activate %s' to enable sync first", projectName, projectName)
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}
	projectRoot := resolveProjectPath(project.Path, baseDir)
	if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", projectRoot)
	}

	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	client, err := localSyncthingClient(machineConfig)
	if err != nil {
		return err
	}

	status, err := client.GetFolderStatus(context.Background(), projectName)
	if err != nil {
		if syncthing.IsNotFound(err) {
			return fmt.Errorf("syncthing folder not found for '%s'", projectName)
		}
		return fmt.Errorf("failed to query syncthing status: %w", err)
	}

	if status.NeedBytes > 0 || status.State != "idle" {
		fmt.Printf("%s Project is still syncing (%s, need %s)\n", s.Warn.Render("Warning:"), status.State, formatBytes(status.NeedBytes))
	}

	fmt.Printf("%s Pulling .env from pass...\n", labelStyle.Render("Step 1:"))
	if err := runEnvPull(envPullCmd, []string{projectName}); err != nil {
		fmt.Printf("  %s %v\n", s.Warn.Render("Warning:"), err)
	} else {
		fmt.Printf("  %s\n", s.Success.Render("Done"))
	}

	fmt.Printf("%s Installing dependencies...\n", labelStyle.Render("Step 2:"))
	if err := installDependencies(project, projectRoot); err != nil {
		return err
	}

	if err := runProjectSetupScript(projectRoot, s); err != nil {
		return err
	}

	fmt.Printf("%s '%s'\n", s.Success.Render("✓ Setup complete"), projectName)
	return nil
}

func installDependencies(project registry.Project, projectRoot string) error {
	s := NewStyles()
	info := s.Info.Render("Info:")

	switch project.Lang {
	case registry.LanguageGo:
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); os.IsNotExist(err) {
			fmt.Printf("  %s go.mod not found, skipping\n", info)
			return nil
		}
		return runSetupCommand(projectRoot, "go", "mod", "download")
	case registry.LanguagePython:
		if _, err := os.Stat(filepath.Join(projectRoot, "pyproject.toml")); os.IsNotExist(err) {
			if _, err := os.Stat(filepath.Join(projectRoot, "requirements.txt")); os.IsNotExist(err) {
				fmt.Printf("  %s no Python dependency file found, skipping\n", info)
				return nil
			}
		}
		if _, err := exec.LookPath("uv"); err != nil {
			return fmt.Errorf("uv not found in PATH")
		}
		return runSetupCommand(projectRoot, "uv", "sync")
	case registry.LanguageTypeScript:
		if _, err := os.Stat(filepath.Join(projectRoot, "package.json")); os.IsNotExist(err) {
			fmt.Printf("  %s package.json not found, skipping\n", info)
			return nil
		}
		if _, err := exec.LookPath("bun"); err == nil {
			return runSetupCommand(projectRoot, "bun", "install")
		}
		if _, err := exec.LookPath("pnpm"); err == nil {
			return runSetupCommand(projectRoot, "pnpm", "install")
		}
		return fmt.Errorf("neither bun nor pnpm found in PATH")
	case registry.LanguageSolidity:
		if _, err := os.Stat(filepath.Join(projectRoot, "foundry.toml")); os.IsNotExist(err) {
			fmt.Printf("  %s foundry.toml not found, skipping\n", info)
			return nil
		}
		return runSetupCommand(projectRoot, "forge", "install")
	default:
		fmt.Printf("  %s no dependency installer configured for %s\n", info, project.Lang)
		return nil
	}
}

func runProjectSetupScript(projectRoot string, s Styles) error {
	setupPath := filepath.Join(projectRoot, ".overlord-setup.sh")
	if _, err := os.Stat(setupPath); os.IsNotExist(err) {
		return nil
	}

	fmt.Printf("%s Running .overlord-setup.sh...\n", s.Label.Render("Step 3:"))
	return runSetupCommand(projectRoot, "bash", setupPath)
}

func runSetupCommand(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s %v: %w", name, args, err)
	}
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

	// Ask for remote URL (default to config repo)
	fmt.Printf("  Git remote URL (enter 'none' for local-only)\n")
	fmt.Printf("  [%s]: ", DefaultConfigRepoURL)
	remoteURL, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	remoteURL = strings.TrimSpace(remoteURL)

	// Use default if empty, skip if "none"
	if remoteURL == "" {
		remoteURL = DefaultConfigRepoURL
	} else if strings.ToLower(remoteURL) == "none" {
		remoteURL = ""
	}

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

// setupMachineConfig creates or updates machine.yaml.
// In init mode, it ensures required bootstrap fields are present.
func setupMachineConfig(configDir string, initMode bool) error {
	machineConfigPath := filepath.Join(configDir, "machine.yaml")
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	reader := bufio.NewReader(os.Stdin)

	promptRole := func(defaultRole machine.Role) (machine.Role, error) {
		if !interactive {
			if defaultRole != "" {
				return defaultRole, nil
			}
			return "", fmt.Errorf("machine role is required for --init in non-interactive mode")
		}

		validRoles := machine.ValidRoles()
		roleNames := make([]string, 0, len(validRoles))
		for _, role := range validRoles {
			roleNames = append(roleNames, string(role))
		}
		fmt.Printf("  Machine role [%s] (%s): ", string(defaultRole), strings.Join(roleNames, "/"))
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read machine role: %w", err)
		}
		input = strings.TrimSpace(strings.ToLower(input))
		if input == "" {
			return defaultRole, nil
		}
		role := machine.Role(input)
		if !role.IsValid() {
			return "", fmt.Errorf("invalid machine role %q", input)
		}
		return role, nil
	}

	promptRequiredSyncthing := func(defaults machine.SyncthingConfig) (machine.SyncthingConfig, error) {
		if !interactive {
			if defaults.Validate() != nil {
				return machine.SyncthingConfig{}, fmt.Errorf("syncthing configuration is required for --init in non-interactive mode")
			}
			return defaults, nil
		}

		fmt.Printf("  Syncthing API URL [%s]: ", defaults.APIURL)
		apiURL, err := reader.ReadString('\n')
		if err != nil {
			return machine.SyncthingConfig{}, fmt.Errorf("failed to read syncthing api url: %w", err)
		}
		apiURL = strings.TrimSpace(apiURL)
		if apiURL == "" {
			apiURL = defaults.APIURL
		}

		fmt.Printf("  Syncthing API key [%s]: ", nonEmptyMask(defaults.APIKey))
		apiKey, err := reader.ReadString('\n')
		if err != nil {
			return machine.SyncthingConfig{}, fmt.Errorf("failed to read syncthing api key: %w", err)
		}
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			apiKey = defaults.APIKey
		}

		fmt.Printf("  Syncthing device ID [%s]: ", defaults.DeviceID)
		deviceID, err := reader.ReadString('\n')
		if err != nil {
			return machine.SyncthingConfig{}, fmt.Errorf("failed to read syncthing device id: %w", err)
		}
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			deviceID = defaults.DeviceID
		}

		fmt.Printf("  Syncthing address [%s]: ", defaults.Address)
		address, err := reader.ReadString('\n')
		if err != nil {
			return machine.SyncthingConfig{}, fmt.Errorf("failed to read syncthing address: %w", err)
		}
		address = strings.TrimSpace(address)
		if address == "" {
			address = defaults.Address
		}

		syncCfg := machine.SyncthingConfig{
			APIURL:   apiURL,
			APIKey:   apiKey,
			DeviceID: deviceID,
			Address:  address,
		}
		if err := syncCfg.Validate(); err != nil {
			return machine.SyncthingConfig{}, err
		}
		return syncCfg, nil
	}

	// Check if machine.yaml already exists
	if _, statErr := os.Stat(machineConfigPath); statErr == nil {
		if !initMode {
			fmt.Printf("  Exists:  %s\n", machineConfigPath)
			return nil
		}

		current, loadErr := machine.Load(machineConfigPath)
		if loadErr != nil {
			return fmt.Errorf("failed to load machine config: %w", loadErr)
		}

		changed := false
		if current.Role == "" {
			role, roleErr := promptRole(machine.RoleMain)
			if roleErr != nil {
				return roleErr
			}
			current.Role = role
			changed = true
		}

		if current.Syncthing.Validate() != nil {
			defaults := current.Syncthing
			if defaults.APIURL == "" {
				defaults.APIURL = "http://127.0.0.1:8384"
			}
			syncCfg, syncErr := promptRequiredSyncthing(defaults)
			if syncErr != nil {
				return syncErr
			}
			current.Syncthing = syncCfg
			changed = true
		}

		if changed {
			if saveErr := machine.Save(machineConfigPath, current); saveErr != nil {
				return fmt.Errorf("failed to update machine config: %w", saveErr)
			}
			fmt.Printf("  Updated: %s\n", machineConfigPath)
		} else {
			fmt.Printf("  Exists:  %s\n", machineConfigPath)
		}

		return nil
	}

	if _, statErr := os.Stat(machineConfigPath); statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to check machine config: %w", statErr)
	}

	fmt.Println("\nSetting up machine configuration...")

	// Get hostname for machine name
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	name := hostname
	if interactive {
		fmt.Printf("  Machine name [%s]: ", hostname)
		input, readErr := reader.ReadString('\n')
		if readErr != nil {
			return fmt.Errorf("failed to read machine name: %w", readErr)
		}
		input = strings.TrimSpace(input)
		if input != "" {
			name = input
		}
	}

	config := &machine.MachineConfig{Name: name}

	if initMode {
		role, roleErr := promptRole(machine.RoleMain)
		if roleErr != nil {
			return roleErr
		}
		config.Role = role

		syncCfg, syncErr := promptRequiredSyncthing(machine.SyncthingConfig{APIURL: "http://127.0.0.1:8384"})
		if syncErr != nil {
			return syncErr
		}
		config.Syncthing = syncCfg
	}

	// Save machine config
	if err := machine.Save(machineConfigPath, config); err != nil {
		return fmt.Errorf("failed to save machine config: %w", err)
	}

	fmt.Printf("  Created: %s\n", machineConfigPath)
	fmt.Printf("  Name:    %s\n", config.Name)
	if config.Role != "" {
		fmt.Printf("  Role:    %s\n", config.Role)
	}
	if config.Syncthing.DeviceID != "" {
		fmt.Printf("  Syncthing: configured\n")
	} else {
		fmt.Printf("  Syncthing: not configured\n")
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

func runSetupInitFlow() error {
	machineView, err := machine.LoadWithPeers(machine.DefaultConfigPath, machine.DefaultPeersPath)
	if err != nil {
		return fmt.Errorf("failed to load machine/peer config: %w", err)
	}

	if machineView.Peers == nil {
		machineView.Peers = &machine.PeersConfig{Peers: map[string]machine.Peer{}}
	}
	if machineView.Peers.Peers == nil {
		machineView.Peers.Peers = map[string]machine.Peer{}
	}

	if err := machine.SavePeers(machine.DefaultPeersPath, machineView.Peers); err != nil {
		return fmt.Errorf("failed to initialize peers config: %w", err)
	}

	client, err := localSyncthingClient(machineView.Machine)
	if err != nil {
		return err
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("  Optional peer device ID (blank to skip): ")
		peerDeviceID, readErr := reader.ReadString('\n')
		if readErr != nil {
			return fmt.Errorf("failed to read peer device id: %w", readErr)
		}
		peerDeviceID = strings.TrimSpace(peerDeviceID)
		if peerDeviceID != "" {
			if ensureErr := ensureSyncthingDevice(context.Background(), client, peerDeviceID, "bootstrap-peer", "dynamic"); ensureErr != nil {
				return fmt.Errorf("failed to ensure bootstrap peer device: %w", ensureErr)
			}
			fmt.Printf("  Syncthing: pre-authorized peer device %s\n", peerDeviceID)
		}
	}

	if err := ensureOverlordConfigSyncthingFolder(context.Background(), machineView.Machine, machineView.Peers); err != nil {
		return err
	}
	fmt.Printf("  Syncthing: configured overlord config folder '%s'\n", overlordConfigFolderID)

	lock, err := machine.LoadLock(machine.DefaultRegistryLockPath)
	if err != nil {
		return fmt.Errorf("failed to load registry lock: %w", err)
	}
	if lock == nil {
		seed := machine.NewRegistryLock(machineView.Machine.Name, "initial lock owner")
		if err := machine.SaveLock(machine.DefaultRegistryLockPath, seed); err != nil {
			return fmt.Errorf("failed to initialize registry lock: %w", err)
		}
		fmt.Printf("  Lock:     initialized with owner '%s'\n", machineView.Machine.Name)
	}

	if err := waitForSyncedArtifacts(setupInitSeedTimeout); err != nil {
		return err
	}
	fmt.Printf("  Sync:     seed artifacts available\n")

	latestRegistry, err := loadRegistry()
	if err != nil {
		return err
	}

	latestPeers, err := machine.LoadPeers(machine.DefaultPeersPath)
	if err != nil {
		return fmt.Errorf("failed to load peers config after sync: %w", err)
	}

	if err := reconcileActiveProjects(context.Background(), latestRegistry, machineView.Machine, latestPeers); err != nil {
		return err
	}

	activeProjects := len(getSyncActiveProjects(latestRegistry))
	fmt.Printf("  Reconcile: completed for %d sync-active project(s)\n", activeProjects)

	return nil
}

func nonEmptyMask(value string) string {
	if value == "" {
		return ""
	}
	return "*****"
}

func waitForSyncedArtifacts(timeout time.Duration) error {
	registryPath, err := registry.ExpandPath(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to resolve registry path: %w", err)
	}
	peersPath, err := registry.ExpandPath(machine.DefaultPeersPath)
	if err != nil {
		return fmt.Errorf("failed to resolve peers path: %w", err)
	}
	lockPath, err := registry.ExpandPath(machine.DefaultRegistryLockPath)
	if err != nil {
		return fmt.Errorf("failed to resolve lock path: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		if fileExists(registryPath) && fileExists(peersPath) && fileExists(lockPath) {
			if _, regErr := registry.Load(registry.DefaultRegistryPath); regErr == nil {
				if _, peersErr := machine.LoadPeers(machine.DefaultPeersPath); peersErr == nil {
					if lock, lockErr := machine.LoadLock(machine.DefaultRegistryLockPath); lockErr == nil && lock != nil {
						return nil
					}
				}
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for synced artifacts (registry.yaml, peers.yaml, registry.lock.yaml)")
		}

		time.Sleep(2 * time.Second)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func reconcileActiveProjects(ctx context.Context, reg *registry.Registry, machineConfig *machine.MachineConfig, peersConfig *machine.PeersConfig) error {
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	client, err := localSyncthingClient(machineConfig)
	if err != nil {
		return err
	}

	peerDevices := make([]syncthing.FolderDevice, 0, len(peersConfig.Peers))
	for name, peer := range peersConfig.Peers {
		if peer.Syncthing.DeviceID == "" {
			continue
		}
		if err := ensureSyncthingDevice(ctx, client, peer.Syncthing.DeviceID, name, peer.Syncthing.Address); err != nil {
			return fmt.Errorf("failed to ensure syncthing peer device %q: %w", name, err)
		}
		peerDevices = append(peerDevices, syncthing.FolderDevice{DeviceID: peer.Syncthing.DeviceID})
	}

	active := getSyncActiveProjects(reg)
	for _, projectName := range sortedProjectNames(active) {
		project := reg.Projects[projectName]
		if project.Status.State == registry.StateArchived {
			continue
		}

		projectPath := resolveProjectPath(project.Path, baseDir)
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			return fmt.Errorf("failed to create local project directory for %q: %w", projectName, err)
		}
		if err := ensureIgnoreFile(projectPath); err != nil {
			return fmt.Errorf("failed to ensure .stignore for %q: %w", projectName, err)
		}

		devices := []syncthing.FolderDevice{{DeviceID: machineConfig.Syncthing.DeviceID}}
		devices = append(devices, peerDevices...)
		folder := syncthing.Folder{
			ID:               projectName,
			Label:            projectName,
			Path:             projectPath,
			Type:             "sendreceive",
			Devices:          devices,
			FSWatcherEnabled: true,
			Copiers:          1,
			Hashers:          1,
			CaseSensitiveFS:  true,
		}
		if _, err := client.EnsureFolder(ctx, folder); err != nil {
			return fmt.Errorf("failed to reconcile syncthing folder for %q: %w", projectName, err)
		}
	}

	return nil
}

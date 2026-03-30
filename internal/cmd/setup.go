package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	syncpkg "github.com/PoeAudits/overlord/internal/sync"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

const setupInitSeedTimeout = 60 * time.Second

// DefaultConfigRepoURL is the default git remote for the registry config repository.
const DefaultConfigRepoURL = "git@github.com:PoeAudits/overlord-config.git"

var defaultRemoteBranchFallbacks = []string{"main", "master", "dev"}

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

	result, err := reg.ResolveOne(projectArg)
	if err != nil {
		return withHintOnNoProjectMatch(err, "run 'overlord list' to see available projects")
	}
	projectName := result.Name
	project := result.Project

	if project.Sync.Status != registry.SyncActive {
		return fmt.Errorf("project '%s' is not active for sync\nRun 'overlord activate %s' to enable sync first", projectName, projectName)
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}
	projectRoot := fileutil.ResolveProjectPath(project.Path, baseDir)
	if _, err := os.Stat(projectRoot); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("project directory does not exist: %s", projectRoot)
	}

	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	client, err := syncpkg.LocalClient(machineConfig)
	if err != nil {
		return err
	}

	status, err := client.FolderStatus(context.Background(), projectName)
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

	type langInstaller struct {
		manifests      []string
		tools          []string
		args           []string
		skipMessage    string
		missingToolErr string
	}

	installers := map[registry.Language]langInstaller{
		registry.LanguageGo: {
			manifests:      []string{"go.mod"},
			tools:          []string{"go"},
			args:           []string{"mod", "download"},
			skipMessage:    "go.mod not found, skipping",
			missingToolErr: "go not found in PATH",
		},
		registry.LanguagePython: {
			manifests:      []string{"pyproject.toml", "requirements.txt"},
			tools:          []string{"uv"},
			args:           []string{"sync"},
			skipMessage:    "no Python dependency file found, skipping",
			missingToolErr: "uv not found in PATH",
		},
		registry.LanguageTypeScript: {
			manifests:      []string{"package.json"},
			tools:          []string{"bun", "pnpm"},
			args:           []string{"install"},
			skipMessage:    "package.json not found, skipping",
			missingToolErr: "neither bun nor pnpm found in PATH",
		},
		registry.LanguageSolidity: {
			manifests:      []string{"foundry.toml"},
			tools:          []string{"forge"},
			args:           []string{"install"},
			skipMessage:    "foundry.toml not found, skipping",
			missingToolErr: "forge not found in PATH",
		},
	}

	installer, ok := installers[project.Lang]
	if !ok {
		fmt.Printf("  %s no dependency installer configured for %s\n", info, project.Lang)
		return nil
	}

	manifestExists, err := hasAnyManifest(projectRoot, installer.manifests)
	if err != nil {
		return err
	}
	if !manifestExists {
		fmt.Printf("  %s %s\n", info, installer.skipMessage)
		return nil
	}

	tool, found := firstAvailableTool(installer.tools)
	if !found {
		return fmt.Errorf("%s", installer.missingToolErr)
	}

	return runSetupCommand(projectRoot, tool, installer.args...)
}

func hasAnyManifest(projectRoot string, manifests []string) (bool, error) {
	for _, manifest := range manifests {
		manifestPath := filepath.Join(projectRoot, manifest)
		_, err := os.Stat(manifestPath)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return false, fmt.Errorf("failed to check dependency manifest %s: %w", manifestPath, err)
		}
	}
	return false, nil
}

func firstAvailableTool(tools []string) (string, bool) {
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			return tool, true
		}
	}
	return "", false
}

func runProjectSetupScript(projectRoot string, s styles) error {
	setupPath := filepath.Join(projectRoot, ".overlord-setup.sh")
	if _, err := os.Stat(setupPath); errors.Is(err, fs.ErrNotExist) {
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

// setupMachineConfig creates or updates machine.yaml.
// In init mode, it ensures required bootstrap fields are present.
func setupMachineConfig(configDir string, initMode bool) error {
	machineConfigPath := filepath.Join(configDir, "machine.yaml")
	interactive := term.IsTerminal(int(os.Stdin.Fd()))
	reader := bufio.NewReader(os.Stdin)

	handled, err := handleExistingMachineConfig(machineConfigPath, initMode, reader, interactive)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	return createMachineConfig(machineConfigPath, initMode, reader, interactive)
}

func promptMachineRole(reader *bufio.Reader, interactive bool, defaultRole machine.Role) (machine.Role, error) {
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

func promptSyncthingConfig(reader *bufio.Reader, interactive bool, defaults machine.SyncthingConfig) (machine.SyncthingConfig, error) {
	if !interactive {
		if defaults.Validate() != nil {
			return machine.SyncthingConfig{}, fmt.Errorf("syncthing configuration is required for --init in non-interactive mode")
		}
		return defaults, nil
	}

	apiURL := promptField(reader, fmt.Sprintf("Syncthing API URL [%s]", defaults.APIURL), defaults.APIURL)
	apiKey := promptField(reader, fmt.Sprintf("Syncthing API key [%s]", nonEmptyMask(defaults.APIKey)), defaults.APIKey)
	deviceID := promptField(reader, fmt.Sprintf("Syncthing device ID [%s]", defaults.DeviceID), defaults.DeviceID)
	address := promptField(reader, fmt.Sprintf("Syncthing address [%s]", defaults.Address), defaults.Address)

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

func promptField(reader *bufio.Reader, label, current string) string {
	fmt.Printf("  %s: ", label)
	value, err := reader.ReadString('\n')
	if err != nil {
		return current
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return current
	}

	return value
}

func handleExistingMachineConfig(machineConfigPath string, initMode bool, reader *bufio.Reader, interactive bool) (bool, error) {
	if _, err := os.Stat(machineConfigPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check machine config: %w", err)
	}

	if !initMode {
		fmt.Printf("  Exists:  %s\n", machineConfigPath)
		return true, nil
	}

	current, err := machine.Load(machineConfigPath)
	if err != nil {
		return true, fmt.Errorf("failed to load machine config: %w", err)
	}

	changed, err := ensureInitMachineFields(current, reader, interactive)
	if err != nil {
		return true, err
	}

	if !changed {
		fmt.Printf("  Exists:  %s\n", machineConfigPath)
		return true, nil
	}

	if err := machine.Save(machineConfigPath, current); err != nil {
		return true, fmt.Errorf("failed to update machine config: %w", err)
	}

	fmt.Printf("  Updated: %s\n", machineConfigPath)
	return true, nil
}

func ensureInitMachineFields(config *machine.MachineConfig, reader *bufio.Reader, interactive bool) (bool, error) {
	changed := false

	if config.Role == "" {
		role, err := promptMachineRole(reader, interactive, machine.RoleMain)
		if err != nil {
			return false, err
		}
		config.Role = role
		changed = true
	}

	if config.Syncthing.Validate() == nil {
		return changed, nil
	}

	defaults := config.Syncthing
	if defaults.APIURL == "" {
		defaults.APIURL = "http://127.0.0.1:8384"
	}

	syncCfg, err := promptSyncthingConfig(reader, interactive, defaults)
	if err != nil {
		return false, err
	}

	config.Syncthing = syncCfg
	return true, nil
}

func createMachineConfig(machineConfigPath string, initMode bool, reader *bufio.Reader, interactive bool) error {
	fmt.Println("\nSetting up machine configuration...")

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	name, err := promptMachineName(reader, interactive, hostname)
	if err != nil {
		return err
	}

	config := &machine.MachineConfig{Name: name}
	if initMode {
		role, err := promptMachineRole(reader, interactive, machine.RoleMain)
		if err != nil {
			return err
		}
		config.Role = role

		syncCfg, err := promptSyncthingConfig(reader, interactive, machine.SyncthingConfig{APIURL: "http://127.0.0.1:8384"})
		if err != nil {
			return err
		}
		config.Syncthing = syncCfg
	}

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

func promptMachineName(reader *bufio.Reader, interactive bool, defaultName string) (string, error) {
	if !interactive {
		return defaultName, nil
	}

	fmt.Printf("  Machine name [%s]: ", defaultName)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read machine name: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultName, nil
	}

	return input, nil
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

	client, err := syncpkg.LocalClient(machineView.Machine)
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
			selection := syncpkg.PeerSelection{
				Name: "bootstrap-peer",
				Peer: machine.Peer{Syncthing: machine.PeerSyncthing{DeviceID: peerDeviceID, Address: "dynamic"}},
			}
			if ensureErr := syncpkg.EnsureDevice(context.Background(), client, selection); ensureErr != nil {
				return fmt.Errorf("failed to ensure bootstrap peer device: %w", ensureErr)
			}
			fmt.Printf("  Syncthing: pre-authorized peer device %s\n", peerDeviceID)
		}
	}

	if err := syncpkg.EnsureConfigFolder(context.Background(), client, machineView.Machine, machineView.Peers); err != nil {
		return err
	}
	fmt.Printf("  Syncthing: configured overlord config folder '%s'\n", syncpkg.ConfigFolderID)

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

	activeProjects := len(syncActiveProjects(latestRegistry))
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
	// Bounded polling loop: terminates when all artifacts arrive or deadline expires.
	for {
		if allSyncedArtifactsValid(registryPath, peersPath, lockPath) == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for synced artifacts (registry.yaml, peers.yaml, registry.lock.yaml)")
		}

		time.Sleep(2 * time.Second)
	}
}

func allSyncedArtifactsValid(registryPath, peersPath, lockPath string) error {
	if err := validateRegistryArtifact(registryPath); err != nil {
		return err
	}

	if err := validatePeersArtifact(peersPath); err != nil {
		return err
	}

	if err := validateLockArtifact(lockPath); err != nil {
		return err
	}

	return nil
}

func validateRegistryArtifact(path string) error {
	if !fileExists(path) {
		return fmt.Errorf("registry artifact is not available")
	}

	if _, err := registry.Load(registry.DefaultRegistryPath); err != nil {
		return fmt.Errorf("failed to load registry artifact: %w", err)
	}

	return nil
}

func validatePeersArtifact(path string) error {
	if !fileExists(path) {
		return fmt.Errorf("peers artifact is not available")
	}

	if _, err := machine.LoadPeers(machine.DefaultPeersPath); err != nil {
		return fmt.Errorf("failed to load peers artifact: %w", err)
	}

	return nil
}

func validateLockArtifact(path string) error {
	if !fileExists(path) {
		return fmt.Errorf("registry lock artifact is not available")
	}

	lock, err := machine.LoadLock(machine.DefaultRegistryLockPath)
	if err != nil {
		return fmt.Errorf("failed to load lock artifact: %w", err)
	}
	if lock == nil {
		return fmt.Errorf("registry lock artifact is empty")
	}

	return nil
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

	client, err := syncpkg.LocalClient(machineConfig)
	if err != nil {
		return err
	}

	peerDevices := make([]syncthing.FolderDevice, 0, len(peersConfig.Peers))
	for name, peer := range peersConfig.Peers {
		if peer.Syncthing.DeviceID == "" {
			continue
		}
		selection := syncpkg.PeerSelection{Name: name, Peer: peer}
		if err := syncpkg.EnsureDevice(ctx, client, selection); err != nil {
			return fmt.Errorf("failed to ensure syncthing peer device %q: %w", name, err)
		}
		peerDevices = append(peerDevices, syncthing.FolderDevice{DeviceID: peer.Syncthing.DeviceID})
	}

	active := syncActiveProjects(reg)
	for _, projectName := range sortedProjectNames(active) {
		project := reg.Projects[projectName]
		if project.Status.State == registry.StateArchived {
			continue
		}

		projectPath := fileutil.ResolveProjectPath(project.Path, baseDir)
		if err := os.MkdirAll(projectPath, 0o755); err != nil {
			return fmt.Errorf("failed to create local project directory for %q: %w", projectName, err)
		}
		if err := syncpkg.EnsureIgnoreFile(projectPath); err != nil {
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

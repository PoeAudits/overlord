package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/PoeAudits/overlord/internal/gitops"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/ssh"
	"github.com/PoeAudits/overlord/internal/sync"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// syncFlags holds the flags for the sync command
type syncFlags struct {
	dryRun bool
	force  bool
	host   string
}

var syncOpts syncFlags

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync [project]",
	Short: "Synchronize files between machines",
	Long: `Synchronize project files between working-set and storage machines.

On a working-set machine:
  - Pulls the latest registry from git
  - Syncs active projects bidirectionally with the storage machine
  - First pulls changes from storage, then pushes local changes

On a storage machine:
  - Pushes active projects to a specified working-set machine
  - Use --host to specify target, or pick interactively
  - Creates remote directories automatically on first sync

If no project is specified, syncs all projects with sync status 'active'.
If a project name is provided, syncs only that specific project.

Exclusion patterns from both global settings (settings.sync.default_exclude)
and per-project settings (project.sync.exclude) are respected.

Examples:
  overlord sync                    # Sync all active projects
  overlord sync myproject          # Sync specific project
  overlord sync --dry-run          # Preview what would be synced
  overlord sync --force            # Sync even if conflicts detected
  overlord sync --host poe-vps     # Push to specific working-set host (storage only)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().BoolVar(&syncOpts.dryRun, "dry-run", false, "Preview sync without making changes")
	syncCmd.Flags().BoolVar(&syncOpts.force, "force", false, "Force sync even if conflicts are detected")
	syncCmd.Flags().StringVar(&syncOpts.host, "host", "", "Target working-set host name (storage machines only)")
}

// syncMethod indicates which sync strategy was used for a project.
type syncMethod string

const (
	// syncMethodRsync indicates the project was synced using rsync.
	syncMethodRsync syncMethod = "rsync"
	// syncMethodGit indicates the project was synced using git pull/push.
	syncMethodGit syncMethod = "git"
)

// syncResult tracks the result of syncing a single project
type syncResult struct {
	name      string
	method    syncMethod
	pulled    bool
	pushed    bool
	conflicts []sync.Conflict
	err       error
}

// runSync handles the sync command logic
func runSync(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// 1. Load machine config
	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config: %w", err)
	}

	// 2. Handle storage role - push projects to working-set machines
	if machineConfig.Role == machine.RoleStorage {
		return runSyncFromStorage(cmd, args, machineConfig, s)
	}

	// 3. Set up SSH multiplexing for connection reuse
	mux, err := ssh.NewMux()
	if err != nil {
		return fmt.Errorf("failed to create SSH multiplexer: %w", err)
	}
	defer mux.Close(context.Background())

	fmt.Printf("%s Establishing SSH connection to %s...\n", s.Label.Render("Setup:"), machineConfig.StorageHost)
	if err := mux.Connect(context.Background(), machineConfig.StorageHost); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", machineConfig.StorageHost, err)
	}

	// 4. Pull git registry first
	fmt.Printf("\n%s Pulling registry from git...\n", s.Label.Render("Step 1:"))

	gitRepoPath, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand git repo path: %w", err)
	}

	git := gitops.New(gitRepoPath)
	git.Env = []string{mux.GitSSHEnv()}
	if err := git.Pull(context.Background()); err != nil {
		fmt.Printf("  %s Failed to pull registry: %v\n", s.Warn.Render("Warning:"), err)
		fmt.Printf("  Continuing with local registry...\n")
	} else {
		fmt.Printf("  %s\n", s.Success.Render("Done"))
	}

	// 5. Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// 6. Determine which projects to sync
	var projectsToSync map[string]registry.Project

	if len(args) > 0 {
		// Specific project requested
		projectName := args[0]
		result, err := reg.ResolveOne(projectName)
		if err != nil {
			return fmt.Errorf("project '%s' not found\nRun 'overlord list' to see all projects", projectName)
		}

		// Check if project has sync enabled
		if result.Project.Sync.Status != registry.SyncActive {
			return fmt.Errorf("project '%s' is not active for sync\nRun 'overlord activate %s' to enable sync first", result.Name, result.Name)
		}

		projectsToSync = map[string]registry.Project{
			result.Name: result.Project,
		}
	} else {
		// Get all sync-active projects
		projectsToSync = getSyncActiveProjects(reg)
	}

	if len(projectsToSync) == 0 {
		fmt.Printf("%s No projects are active for sync\n", s.Info.Render("Info:"))
		fmt.Printf("  Run 'overlord activate <project>' to enable sync for a project\n")
		return nil
	}

	// 7. Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Remote base directory: use unexpanded path (e.g. ~/Overlord) so that
	// rsync/SSH expands ~ on the remote machine's side. This ensures correct
	// paths when local and remote home directories differ.
	remoteBaseDir := reg.Settings.BaseDir

	// 8. Get global exclusion patterns
	globalExcludes := reg.Settings.Sync.DefaultExclude

	// 9. Create rsync instance
	rsync := sync.NewRsync()

	// SSH command for rsync multiplexing
	sshCommand := mux.SSHCommand()

	// SSH env for git operations (GIT_SSH_COMMAND)
	sshEnv := mux.GitSSHEnv()

	// 10. Sync each project
	fmt.Printf("\n%s Syncing %d project(s)...\n", s.Label.Render("Step 2:"), len(projectsToSync))

	if syncOpts.dryRun {
		fmt.Printf("  %s\n", s.Warn.Render("(dry-run mode - no changes will be made)"))
	}

	results := make([]syncResult, 0, len(projectsToSync))

	// Sort project names for consistent output
	projectNames := make([]string, 0, len(projectsToSync))
	for name := range projectsToSync {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	for _, name := range projectNames {
		project := projectsToSync[name]
		ctx := &syncContext{
			name:           name,
			project:        project,
			baseDir:        baseDir,
			remoteBaseDir:  remoteBaseDir,
			storageHost:    machineConfig.StorageHost,
			globalExcludes: globalExcludes,
			rsyncClient:    rsync,
			sshCommand:     sshCommand,
			sshEnv:         sshEnv,
			dryRun:         syncOpts.dryRun,
			force:          syncOpts.force,
			styles:         s,
		}
		results = append(results, syncProject(ctx))
	}

	// 11. Print summary
	fmt.Printf("\n%s\n", s.Label.Render("Summary:"))
	printSyncSummary(results, s.Success, s.Warn, s.Error, s.Info)

	// Print next steps if there were failures
	hasFailures := false
	for _, r := range results {
		if r.err != nil && !strings.Contains(r.err.Error(), "cancelled") {
			hasFailures = true
			break
		}
	}
	if hasFailures {
		fmt.Printf("\n%s Check error messages above for details\n", s.Label.Render("Tip:"))
		fmt.Printf("  Run 'overlord sync --dry-run' to preview changes without syncing\n")
	}

	return nil
}

// getSyncActiveProjects returns all projects with sync status 'active'
func getSyncActiveProjects(reg *registry.Registry) map[string]registry.Project {
	result := make(map[string]registry.Project)
	for name, project := range reg.Projects {
		if project.Sync.Status == registry.SyncActive {
			result[name] = project
		}
	}
	return result
}

// syncContext bundles all parameters needed for syncing a project from a
// working-set machine. This replaces the 16-parameter syncProject signature.
type syncContext struct {
	// Project info
	name    string
	project registry.Project

	// Path info
	baseDir       string
	remoteBaseDir string
	storageHost   string

	// Sync config
	globalExcludes []string
	rsyncClient    *sync.Rsync
	sshCommand     string
	sshEnv         string

	// Options
	dryRun bool
	force  bool

	// Styles
	styles Styles
}

// buildExcludes merges global and per-project exclusion patterns.
func (ctx *syncContext) buildExcludes() []string {
	excludes := make([]string, 0, len(ctx.globalExcludes)+len(ctx.project.Sync.Exclude))
	excludes = append(excludes, ctx.globalExcludes...)
	excludes = append(excludes, ctx.project.Sync.Exclude...)
	return excludes
}

// ensureLocalDir creates the local project directory if it doesn't exist.
// Returns true if this is a first sync (directory was just created).
func (ctx *syncContext) ensureLocalDir(localPath string) (firstSync bool, err error) {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		fmt.Printf("    %s First sync - creating %s\n", ctx.styles.Info.Render("Info:"), localPath)
		if err := os.MkdirAll(localPath, 0755); err != nil {
			return false, fmt.Errorf("failed to create local directory: %s: %w", localPath, err)
		}
		return true, nil
	}
	return false, nil
}

// detectAndHandleConflicts checks for conflicts and prompts the user if needed.
// Returns the conflicts found and whether sync should proceed.
func (ctx *syncContext) detectAndHandleConflicts(localPath string, excludes []string) ([]sync.Conflict, bool) {
	fmt.Printf("    Checking for conflicts...")
	conflicts, err := sync.DetectConflictsWithRsyncSSH(
		context.Background(), ctx.rsyncClient, localPath, ctx.storageHost,
		resolveProjectPath(ctx.project.Path, ctx.remoteBaseDir),
		excludes, ctx.sshCommand,
	)
	if err != nil {
		fmt.Printf(" %s\n", ctx.styles.Warn.Render("failed"))
		fmt.Printf("    %s Could not detect conflicts: %v\n", ctx.styles.Warn.Render("Warning:"), err)
		// Continue anyway - conflict detection is advisory
		return nil, true
	}

	if len(conflicts) == 0 {
		fmt.Printf(" %s\n", ctx.styles.Success.Render("none"))
		return conflicts, true
	}

	fmt.Printf(" %s\n", ctx.styles.Warn.Render(fmt.Sprintf("found %d", len(conflicts))))
	printConflicts(conflicts, ctx.styles.Warn, ctx.styles.Info)

	// Check if we should proceed
	if !ctx.force && !ctx.dryRun {
		if !confirmProceed() {
			return conflicts, false
		}
	}
	return conflicts, true
}

// syncProject syncs a single project and returns the result.
//
// Sync strategy ("git if usable remote, else rsync"):
//   - If the local project directory is a git repo with at least one remote URL
//     configured, sync is performed via git pull (from remote) instead of rsync.
//     This is more efficient and handles merging better for version-controlled projects.
//   - If the project is NOT a git repo or has no remote, rsync is used as before.
//   - If git sync fails, the error is reported and we do NOT fall back to rsync
//     (falling back could cause data confusion between the two sync methods).
func syncProject(ctx *syncContext) syncResult {
	result := syncResult{name: ctx.name}

	fmt.Printf("\n  %s %s\n", ctx.styles.Label.Render("Project:"), ctx.name)

	// Build local path (handles both relative and absolute paths)
	localPath := resolveProjectPath(ctx.project.Path, ctx.baseDir)

	// Check if local directory exists; create it on first sync
	firstSync, err := ctx.ensureLocalDir(localPath)
	if err != nil {
		result.err = err
		fmt.Printf("    %s %v\n", ctx.styles.Error.Render("Error:"), result.err)
		return result
	}

	// Determine sync method: git if usable remote, else rsync.
	// On first sync the directory was just created (empty), so it cannot be a git repo yet — use rsync.
	useGit := !firstSync && gitops.HasUsableRemote(localPath)

	if useGit {
		result.method = syncMethodGit
		return syncProjectWithGit(ctx.name, localPath, ctx.sshEnv, ctx.dryRun, result, ctx.styles)
	}

	// --- rsync path (original behavior, unchanged) ---
	result.method = syncMethodRsync

	// Build remote path using unexpanded base dir so ~ expands on the remote side
	remotePath := sync.FormatRemotePath(ctx.storageHost, resolveProjectPath(ctx.project.Path, ctx.remoteBaseDir))

	// Merge exclusion patterns (global + per-project)
	excludes := ctx.buildExcludes()

	fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Local:"), localPath)
	fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Remote:"), remotePath)
	fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Method:"), "rsync")

	if len(excludes) > 0 {
		fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Excludes:"), strings.Join(excludes, ", "))
	}

	// Detect conflicts (skip on first sync - nothing local to conflict)
	if firstSync {
		fmt.Printf("    Checking for conflicts... %s\n", ctx.styles.Info.Render("skipped (first sync)"))
	} else {
		conflicts, proceed := ctx.detectAndHandleConflicts(localPath, excludes)
		result.conflicts = conflicts
		if !proceed {
			result.err = fmt.Errorf("sync cancelled by user")
			fmt.Printf("    %s\n", ctx.styles.Info.Render("Skipped"))
			return result
		}
	}

	// Pull from storage (remote -> local)
	fmt.Printf("    Pulling from storage...")
	pullOpts := sync.RsyncOptions{
		Source:     remotePath,
		Dest:       localPath,
		Excludes:   excludes,
		DryRun:     ctx.dryRun,
		SSHCommand: ctx.sshCommand,
	}

	pullResult, err := ctx.rsyncClient.Pull(context.Background(), pullOpts)
	if err != nil {
		fmt.Printf(" %s\n", ctx.styles.Error.Render("failed"))
		fmt.Printf("    %s Pull failed: %v\n", ctx.styles.Error.Render("Error:"), err)
		// Continue to try push anyway
	} else {
		result.pulled = true
		if ctx.dryRun {
			fmt.Printf(" %s\n", ctx.styles.Info.Render("(dry-run)"))
			if pullResult.Output != "" {
				printRsyncPreview(pullResult.Output, "Would pull:", ctx.styles.Info)
			}
		} else {
			fmt.Printf(" %s\n", ctx.styles.Success.Render("done"))
		}
	}

	// Push to storage (local -> remote)
	// Skip push on first sync - local was just created and populated from pull
	if firstSync {
		fmt.Printf("    Pushing to storage... %s\n", ctx.styles.Info.Render("skipped (first sync)"))
		result.pushed = true
	} else {
		fmt.Printf("    Pushing to storage...")
		pushOpts := sync.RsyncOptions{
			Source:     localPath,
			Dest:       remotePath,
			Excludes:   excludes,
			DryRun:     ctx.dryRun,
			SSHCommand: ctx.sshCommand,
		}

		pushResult, err := ctx.rsyncClient.Push(context.Background(), pushOpts)
		if err != nil {
			fmt.Printf(" %s\n", ctx.styles.Error.Render("failed"))
			fmt.Printf("    %s Push failed: %v\n", ctx.styles.Error.Render("Error:"), err)
		} else {
			result.pushed = true
			if ctx.dryRun {
				fmt.Printf(" %s\n", ctx.styles.Info.Render("(dry-run)"))
				if pushResult.Output != "" {
					printRsyncPreview(pushResult.Output, "Would push:", ctx.styles.Info)
				}
			} else {
				fmt.Printf(" %s\n", ctx.styles.Success.Render("done"))
			}
		}
	}

	return result
}

// syncProjectWithGit syncs a project using git pull on a working-set machine.
// This is used when the project has a usable git remote, making rsync unnecessary.
// On working-set machines, we pull from the remote to get the latest changes.
//
// If git sync fails, the error is reported and we do NOT fall back to rsync.
// Falling back could cause data confusion between the two sync methods.
func syncProjectWithGit(
	name string,
	localPath string,
	sshEnv string,
	dryRun bool,
	result syncResult,
	s Styles,
) syncResult {
	fmt.Printf("    %s %s\n", s.Info.Render("Local:"), localPath)
	fmt.Printf("    %s %s\n", s.Info.Render("Method:"), "git (usable remote detected)")

	if dryRun {
		fmt.Printf("    %s git pull would run here\n", s.Info.Render("(dry-run)"))
		result.pulled = true
		result.pushed = true
		return result
	}

	// Pull from remote
	fmt.Printf("    Pulling via git...")
	git := gitops.New(localPath)
	if sshEnv != "" {
		git.Env = []string{sshEnv}
	}

	if err := git.Pull(context.Background()); err != nil {
		fmt.Printf(" %s\n", s.Error.Render("failed"))
		fmt.Printf("    %s git pull failed: %v\n", s.Error.Render("Error:"), err)
		result.err = fmt.Errorf("git pull failed: %w", err)
		return result
	}

	fmt.Printf(" %s\n", s.Success.Render("done"))
	result.pulled = true
	result.pushed = true // Mark both so summary counts as "synced"
	return result
}

// printConflicts displays detected conflicts
func printConflicts(conflicts []sync.Conflict, warnStyle, infoStyle lipgloss.Style) {
	// Group by type
	bothModified := make([]sync.Conflict, 0)
	localOnly := make([]sync.Conflict, 0)
	remoteOnly := make([]sync.Conflict, 0)

	for _, c := range conflicts {
		switch c.Type {
		case sync.ConflictBothModified:
			bothModified = append(bothModified, c)
		case sync.ConflictLocalOnly:
			localOnly = append(localOnly, c)
		case sync.ConflictRemoteOnly:
			remoteOnly = append(remoteOnly, c)
		}
	}

	if len(bothModified) > 0 {
		fmt.Printf("    %s\n", warnStyle.Render("Modified on both sides:"))
		for _, c := range bothModified {
			fmt.Printf("      - %s\n", c.Path)
		}
	}

	if len(localOnly) > 0 {
		fmt.Printf("    %s\n", infoStyle.Render("Local only (will be pushed):"))
		for _, c := range localOnly {
			fmt.Printf("      + %s\n", c.Path)
		}
	}

	if len(remoteOnly) > 0 {
		fmt.Printf("    %s\n", infoStyle.Render("Remote only (will be pulled):"))
		for _, c := range remoteOnly {
			fmt.Printf("      + %s\n", c.Path)
		}
	}
}

// confirmProceed asks the user if they want to proceed with sync
func confirmProceed() bool {
	confirmed, err := confirmAction("    Proceed with sync?")
	if err != nil {
		return false
	}
	return confirmed
}

// printRsyncPreview prints a preview of what rsync would do
func printRsyncPreview(output string, header string, infoStyle lipgloss.Style) {
	lines := strings.Split(output, "\n")
	fileCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and metadata
		if line == "" ||
			strings.HasPrefix(line, "sending") ||
			strings.HasPrefix(line, "sent") ||
			strings.HasPrefix(line, "total size") ||
			strings.HasPrefix(line, "Number of") ||
			strings.Contains(line, "bytes/sec") {
			continue
		}
		// Skip directories
		if strings.HasSuffix(line, "/") {
			continue
		}
		fileCount++
	}

	if fileCount > 0 {
		fmt.Printf("      %s %d file(s)\n", infoStyle.Render(header), fileCount)
	}
}

// printSyncSummary prints a summary of all sync operations
func printSyncSummary(results []syncResult, successStyle, warnStyle, errorStyle, infoStyle lipgloss.Style) {
	successCount := 0
	partialCount := 0
	failedCount := 0
	skippedCount := 0
	totalConflicts := 0

	for _, r := range results {
		totalConflicts += len(r.conflicts)

		if r.err != nil {
			if strings.Contains(r.err.Error(), "cancelled") {
				skippedCount++
			} else {
				failedCount++
			}
		} else if r.pulled && r.pushed {
			successCount++
		} else if r.pulled || r.pushed {
			partialCount++
		} else {
			failedCount++
		}
	}

	fmt.Printf("  %s %d\n", successStyle.Render("Synced:"), successCount)
	if partialCount > 0 {
		fmt.Printf("  %s %d\n", warnStyle.Render("Partial:"), partialCount)
	}
	if skippedCount > 0 {
		fmt.Printf("  %s %d\n", infoStyle.Render("Skipped:"), skippedCount)
	}
	if failedCount > 0 {
		fmt.Printf("  %s %d\n", errorStyle.Render("Failed:"), failedCount)
	}
	if totalConflicts > 0 {
		fmt.Printf("  %s %d\n", warnStyle.Render("Conflicts detected:"), totalConflicts)
	}
}

// runSyncFromStorage orchestrates pushing projects from storage to a working-set machine
func runSyncFromStorage(cmd *cobra.Command, args []string, machineConfig *machine.MachineConfig, s Styles) error {

	// Check that working_set_hosts is configured
	if len(machineConfig.WorkingSetHosts) == 0 {
		fmt.Printf("%s No working-set hosts configured\n", s.Error.Render("Error:"))
		fmt.Printf("  Add working_set_hosts to ~/.config/overlord/machine.yaml:\n")
		fmt.Printf("    working_set_hosts:\n")
		fmt.Printf("      my-host: user@10.0.0.1\n")
		return fmt.Errorf("no working-set hosts configured")
	}

	// Determine target host
	targetName, targetAddr, err := resolveTargetHost(machineConfig, s.Label, s.Info)
	if err != nil {
		return err
	}

	fmt.Printf("%s Pushing to %s (%s)\n", s.Label.Render("Target:"), targetName, targetAddr)

	// Set up SSH multiplexing for connection reuse
	mux, err := ssh.NewMux()
	if err != nil {
		return fmt.Errorf("failed to create SSH multiplexer: %w", err)
	}
	defer mux.Close(context.Background())

	fmt.Printf("\n%s Establishing SSH connection to %s...\n", s.Label.Render("Setup:"), targetName)
	if err := mux.Connect(context.Background(), targetAddr); err != nil {
		return fmt.Errorf("failed to connect to %s: %w", targetName, err)
	}

	// Pull git registry first
	fmt.Printf("\n%s Pulling registry from git...\n", s.Label.Render("Step 1:"))

	gitRepoPath, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand git repo path: %w", err)
	}

	git := gitops.New(gitRepoPath)
	git.Env = []string{mux.GitSSHEnv()}
	if err := git.Pull(context.Background()); err != nil {
		fmt.Printf("  %s Failed to pull registry: %v\n", s.Warn.Render("Warning:"), err)
		fmt.Printf("  Continuing with local registry...\n")
	} else {
		fmt.Printf("  %s\n", s.Success.Render("Done"))
	}

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Determine which projects to sync
	var projectsToSync map[string]registry.Project

	if len(args) > 0 {
		projectName := args[0]
		result, err := reg.ResolveOne(projectName)
		if err != nil {
			return fmt.Errorf("project '%s' not found\nRun 'overlord list' to see all projects", projectName)
		}
		if result.Project.Sync.Status != registry.SyncActive {
			return fmt.Errorf("project '%s' is not active for sync\nRun 'overlord activate %s' to enable sync first", result.Name, result.Name)
		}
		projectsToSync = map[string]registry.Project{
			result.Name: result.Project,
		}
	} else {
		projectsToSync = getSyncActiveProjects(reg)
	}

	if len(projectsToSync) == 0 {
		fmt.Printf("%s No projects are active for sync\n", s.Info.Render("Info:"))
		fmt.Printf("  Run 'overlord activate <project>' to enable sync for a project\n")
		return nil
	}

	// Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Remote base directory: use unexpanded path so ~ expands on remote side
	remoteBaseDir := reg.Settings.BaseDir

	// Global exclusion patterns
	globalExcludes := reg.Settings.Sync.DefaultExclude

	// Create rsync instance
	rsync := sync.NewRsync()

	// SSH options for multiplexing
	sshCommand := mux.SSHCommand()
	sshArgs := mux.Wrap()
	sshEnv := mux.GitSSHEnv()

	// Sync each project
	fmt.Printf("\n%s Pushing %d project(s) to %s...\n", s.Label.Render("Step 2:"), len(projectsToSync), targetName)

	if syncOpts.dryRun {
		fmt.Printf("  %s\n", s.Warn.Render("(dry-run mode - no changes will be made)"))
	}

	results := make([]syncResult, 0, len(projectsToSync))

	// Sort project names for consistent output
	projectNames := make([]string, 0, len(projectsToSync))
	for name := range projectsToSync {
		projectNames = append(projectNames, name)
	}
	sort.Strings(projectNames)

	for _, name := range projectNames {
		project := projectsToSync[name]
		ctx := &hostSyncContext{
			name:           name,
			project:        project,
			baseDir:        baseDir,
			remoteBaseDir:  remoteBaseDir,
			targetAddr:     targetAddr,
			globalExcludes: globalExcludes,
			rsyncClient:    rsync,
			sshCommand:     sshCommand,
			sshArgs:        sshArgs,
			sshEnv:         sshEnv,
			dryRun:         syncOpts.dryRun,
			styles:         s,
		}
		results = append(results, syncProjectToHost(ctx))
	}

	// Print summary
	fmt.Printf("\n%s\n", s.Label.Render("Summary:"))
	printSyncSummary(results, s.Success, s.Warn, s.Error, s.Info)

	return nil
}

// resolveTargetHost determines the target working-set host from flag, auto-select, or picker.
// The returned addr may include a user prefix (e.g. "root@10.0.0.1").
func resolveTargetHost(machineConfig *machine.MachineConfig, labelStyle, infoStyle lipgloss.Style) (name, addr string, err error) {
	hosts := machineConfig.WorkingSetHosts

	// --host flag provided
	if syncOpts.host != "" {
		addr, ok := hosts[syncOpts.host]
		if !ok {
			available := make([]string, 0, len(hosts))
			for name := range hosts {
				available = append(available, name)
			}
			sort.Strings(available)
			return "", "", fmt.Errorf("host '%s' not found in working_set_hosts\nAvailable: %s", syncOpts.host, strings.Join(available, ", "))
		}
		return syncOpts.host, addr, nil
	}

	// Single host configured - use it automatically
	if len(hosts) == 1 {
		for name, addr := range hosts {
			return name, addr, nil
		}
	}

	// Multiple hosts, no flag - show interactive picker
	return pickTargetHost(hosts)
}

// pickTargetHost shows an interactive TUI picker for selecting a working-set host
func pickTargetHost(hosts map[string]string) (name, addr string, err error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		available := make([]string, 0, len(hosts))
		for name := range hosts {
			available = append(available, name)
		}
		sort.Strings(available)
		return "", "", fmt.Errorf("multiple working-set hosts configured, use --host to specify\nAvailable: %s", strings.Join(available, ", "))
	}

	// Build sorted host list
	entries := make([]hostEntry, 0, len(hosts))
	for name, addr := range hosts {
		entries = append(entries, hostEntry{name: name, addr: addr})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].name < entries[j].name
	})

	// Create picker model
	m := newHostPickerModel(entries)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", "", fmt.Errorf("picker error: %w", err)
	}

	result := finalModel.(hostPickerModel)
	if result.cancelled {
		return "", "", fmt.Errorf("cancelled")
	}
	if result.selected == nil {
		return "", "", fmt.Errorf("no host selected")
	}

	return result.selected.name, result.selected.addr, nil
}

// hostEntry represents a working-set host for the picker
type hostEntry struct {
	name string
	addr string // [user@]host or IP
}

// hostPickerModel is the Bubbletea model for the host picker
type hostPickerModel struct {
	textInput textinput.Model
	hosts     []hostEntry
	filtered  []hostEntry
	cursor    int
	selected  *hostEntry
	cancelled bool
}

func newHostPickerModel(hosts []hostEntry) hostPickerModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40

	return hostPickerModel{
		textInput: ti,
		hosts:     hosts,
		filtered:  hosts,
		cursor:    0,
	}
}

func (m hostPickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m hostPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.selected = &m.filtered[m.cursor]
			}
			return m, tea.Quit
		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter hosts based on input
	query := strings.ToLower(m.textInput.Value())
	if query == "" {
		m.filtered = m.hosts
	} else {
		m.filtered = make([]hostEntry, 0)
		for _, h := range m.hosts {
			if strings.Contains(strings.ToLower(h.name), query) || strings.Contains(h.addr, query) {
				m.filtered = append(m.filtered, h)
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}

	return m, cmd
}

func (m hostPickerModel) View() string {
	var b strings.Builder

	s := NewStyles()
	promptStyle := s.Prompt
	selectedStyle := s.Selected
	nameStyle := s.Name
	descStyle := s.Desc
	cursorStyle := s.Cursor

	b.WriteString(promptStyle.Render("Select target host: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	if len(m.filtered) == 0 {
		b.WriteString(descStyle.Render("  No matching hosts"))
		b.WriteString("\n")
	} else {
		// Calculate max name length for alignment
		maxNameLen := 0
		for _, h := range m.filtered {
			if len(h.name) > maxNameLen {
				maxNameLen = len(h.name)
			}
		}

		for i, h := range m.filtered {
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> "))
				b.WriteString(selectedStyle.Render(padRight(h.name, maxNameLen)))
			} else {
				b.WriteString("  ")
				b.WriteString(nameStyle.Render(padRight(h.name, maxNameLen)))
			}
			b.WriteString("  ")
			b.WriteString(descStyle.Render(h.addr))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(descStyle.Render("↑/↓ navigate • enter select • esc cancel"))

	return b.String()
}

// hostSyncContext bundles all parameters needed for pushing a project from
// storage to a working-set host. This replaces the 16-parameter syncProjectToHost signature.
type hostSyncContext struct {
	// Project info
	name    string
	project registry.Project

	// Path info
	baseDir       string
	remoteBaseDir string
	targetAddr    string // may include user prefix (e.g. "root@10.0.0.1")

	// Sync config
	globalExcludes []string
	rsyncClient    *sync.Rsync
	sshCommand     string
	sshArgs        []string // prepended to direct SSH commands (e.g. for ensureRemoteDir)
	sshEnv         string

	// Options
	dryRun bool

	// Styles
	styles Styles
}

// buildExcludes merges global and per-project exclusion patterns.
func (ctx *hostSyncContext) buildExcludes() []string {
	excludes := make([]string, 0, len(ctx.globalExcludes)+len(ctx.project.Sync.Exclude))
	excludes = append(excludes, ctx.globalExcludes...)
	excludes = append(excludes, ctx.project.Sync.Exclude...)
	return excludes
}

// ensureRemoteDir creates the remote project directory via SSH.
func (ctx *hostSyncContext) ensureRemoteDir(remoteProjectPath string) error {
	fmt.Printf("    Ensuring remote directory...")
	if err := ensureRemoteDirSSH(ctx.targetAddr, remoteProjectPath, ctx.sshArgs); err != nil {
		fmt.Printf(" %s\n", ctx.styles.Warn.Render("failed"))
		fmt.Printf("    %s Could not create remote directory: %v\n", ctx.styles.Warn.Render("Warning:"), err)
		// Continue anyway - rsync may create it, or fail with a clear error
		return err
	}
	fmt.Printf(" %s\n", ctx.styles.Success.Render("ok"))
	return nil
}

// syncProjectToHost pushes a single project from storage to a working-set host.
//
// Sync strategy ("git if usable remote, else rsync"):
//   - If the local project directory is a git repo with a usable remote,
//     sync is performed via git push instead of rsync.
//   - Otherwise, rsync is used as before.
//   - If git sync fails, the error is reported and we do NOT fall back to rsync.
func syncProjectToHost(ctx *hostSyncContext) syncResult {
	result := syncResult{name: ctx.name}

	fmt.Printf("\n  %s %s\n", ctx.styles.Label.Render("Project:"), ctx.name)

	// Build local path (expanded)
	localPath := resolveProjectPath(ctx.project.Path, ctx.baseDir)

	// Check if local directory exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		result.err = fmt.Errorf("local directory not found: %s", localPath)
		fmt.Printf("    %s %v\n", ctx.styles.Error.Render("Error:"), result.err)
		return result
	}

	// Determine sync method: git if usable remote, else rsync.
	if gitops.HasUsableRemote(localPath) {
		result.method = syncMethodGit
		return syncProjectToHostWithGit(ctx.name, localPath, ctx.sshEnv, ctx.dryRun, result, ctx.styles)
	}

	// --- rsync path (original behavior, unchanged) ---
	result.method = syncMethodRsync

	// Build remote path using unexpanded base dir so ~ expands on the remote side
	remoteProjectPath := resolveProjectPath(ctx.project.Path, ctx.remoteBaseDir)
	remotePath := sync.FormatRemotePath(ctx.targetAddr, remoteProjectPath)

	// Merge exclusion patterns (global + per-project)
	excludes := ctx.buildExcludes()

	fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Local:"), localPath)
	fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Remote:"), remotePath)
	fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Method:"), "rsync")

	if len(excludes) > 0 {
		fmt.Printf("    %s %s\n", ctx.styles.Info.Render("Excludes:"), strings.Join(excludes, ", "))
	}

	// Auto-create remote directory via SSH
	if !ctx.dryRun {
		_ = ctx.ensureRemoteDir(remoteProjectPath)
	}

	// Push to working-set (local -> remote)
	fmt.Printf("    Pushing to host...")
	pushOpts := sync.RsyncOptions{
		Source:     localPath,
		Dest:       remotePath,
		Excludes:   excludes,
		DryRun:     ctx.dryRun,
		SSHCommand: ctx.sshCommand,
	}

	pushResult, err := ctx.rsyncClient.Push(context.Background(), pushOpts)
	if err != nil {
		fmt.Printf(" %s\n", ctx.styles.Error.Render("failed"))
		fmt.Printf("    %s Push failed: %v\n", ctx.styles.Error.Render("Error:"), err)
		result.err = err
	} else {
		result.pushed = true
		result.pulled = true // Mark both so summary counts as "synced" not "partial"
		if ctx.dryRun {
			fmt.Printf(" %s\n", ctx.styles.Info.Render("(dry-run)"))
			if pushResult.Output != "" {
				printRsyncPreview(pushResult.Output, "Would push:", ctx.styles.Info)
			}
		} else {
			fmt.Printf(" %s\n", ctx.styles.Success.Render("done"))
		}
	}

	return result
}

// syncProjectToHostWithGit pushes a project using git push on a storage machine.
// This is used when the project has a usable git remote, making rsync unnecessary.
// On storage machines, we push to the remote so working-set machines can pull.
//
// If git sync fails, the error is reported and we do NOT fall back to rsync.
func syncProjectToHostWithGit(
	name string,
	localPath string,
	sshEnv string,
	dryRun bool,
	result syncResult,
	s Styles,
) syncResult {
	fmt.Printf("    %s %s\n", s.Info.Render("Local:"), localPath)
	fmt.Printf("    %s %s\n", s.Info.Render("Method:"), "git (usable remote detected)")

	if dryRun {
		fmt.Printf("    %s git push would run here\n", s.Info.Render("(dry-run)"))
		result.pulled = true
		result.pushed = true
		return result
	}

	// Push to remote
	fmt.Printf("    Pushing via git...")
	git := gitops.New(localPath)
	if sshEnv != "" {
		git.Env = []string{sshEnv}
	}

	if err := git.Push(context.Background()); err != nil {
		fmt.Printf(" %s\n", s.Error.Render("failed"))
		fmt.Printf("    %s git push failed: %v\n", s.Error.Render("Error:"), err)
		result.err = fmt.Errorf("git push failed: %w", err)
		return result
	}

	fmt.Printf(" %s\n", s.Success.Render("done"))
	result.pulled = true // Mark both so summary counts as "synced"
	result.pushed = true
	return result
}

// ensureRemoteDirSSH creates a directory on the remote host via SSH.
// host may include a user prefix (e.g. "root@10.0.0.1").
// sshArgs are prepended to the SSH command arguments for ControlMaster multiplexing.
func ensureRemoteDirSSH(host, remotePath string, sshArgs []string) error {
	args := make([]string, 0, len(sshArgs)+3)
	args = append(args, sshArgs...)
	args = append(args, host, "mkdir", "-p", remotePath)
	cmd := exec.Command("ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh mkdir failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

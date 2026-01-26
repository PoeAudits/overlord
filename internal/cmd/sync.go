package cmd

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/PoeAudits/overlord/internal/gitops"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/sync"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// syncFlags holds the flags for the sync command
type syncFlags struct {
	dryRun bool
	force  bool
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
  - Informs that sync should be initiated from working-set machines

If no project is specified, syncs all projects with sync status 'active'.
If a project name is provided, syncs only that specific project.

Exclusion patterns from both global settings (settings.sync.default_exclude)
and per-project settings (project.sync.exclude) are respected.

Examples:
  overlord sync                    # Sync all active projects
  overlord sync myproject          # Sync specific project
  overlord sync --dry-run          # Preview what would be synced
  overlord sync --force            # Sync even if conflicts detected`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSync,
}

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().BoolVar(&syncOpts.dryRun, "dry-run", false, "Preview sync without making changes")
	syncCmd.Flags().BoolVar(&syncOpts.force, "force", false, "Force sync even if conflicts are detected")
}

// syncResult tracks the result of syncing a single project
type syncResult struct {
	name      string
	pulled    bool
	pushed    bool
	conflicts []sync.Conflict
	err       error
}

// runSync handles the sync command logic
func runSync(cmd *cobra.Command, args []string) error {
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

	// 2. Handle storage role - sync is initiated from working-set machines
	if machineConfig.Role == machine.RoleStorage {
		fmt.Printf("%s This is a storage machine\n", labelStyle.Render("Info:"))
		fmt.Printf("  Sync operations should be initiated from working-set machines.\n")
		fmt.Printf("  Working-set machines will push/pull files to/from this machine.\n")
		return nil
	}

	// 3. Pull git registry first
	fmt.Printf("%s Pulling registry from git...\n", labelStyle.Render("Step 1:"))

	gitRepoPath, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand git repo path: %w", err)
	}

	git := gitops.New(gitRepoPath)
	if err := git.Pull(); err != nil {
		fmt.Printf("  %s Failed to pull registry: %v\n", warnStyle.Render("Warning:"), err)
		fmt.Printf("  Continuing with local registry...\n")
	} else {
		fmt.Printf("  %s\n", successStyle.Render("Done"))
	}

	// 4. Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// 5. Determine which projects to sync
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
		fmt.Printf("%s No projects are active for sync\n", infoStyle.Render("Info:"))
		fmt.Printf("  Run 'overlord activate <project>' to enable sync for a project\n")
		return nil
	}

	// 6. Expand base directory
	baseDir, err := expandPath(reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base directory: %w", err)
	}

	// 7. Get global exclusion patterns
	globalExcludes := reg.Settings.Sync.DefaultExclude

	// 8. Create rsync instance
	rsync := sync.NewRsync()

	// 9. Sync each project
	fmt.Printf("\n%s Syncing %d project(s)...\n", labelStyle.Render("Step 2:"), len(projectsToSync))

	if syncOpts.dryRun {
		fmt.Printf("  %s\n", warnStyle.Render("(dry-run mode - no changes will be made)"))
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
		result := syncProject(
			name,
			project,
			baseDir,
			machineConfig.StorageHost,
			globalExcludes,
			rsync,
			syncOpts.dryRun,
			syncOpts.force,
			labelStyle,
			successStyle,
			warnStyle,
			errorStyle,
			infoStyle,
		)
		results = append(results, result)
	}

	// 10. Print summary
	fmt.Printf("\n%s\n", labelStyle.Render("Summary:"))
	printSyncSummary(results, successStyle, warnStyle, errorStyle, infoStyle)

	// Print next steps if there were failures
	hasFailures := false
	for _, r := range results {
		if r.err != nil && !strings.Contains(r.err.Error(), "cancelled") {
			hasFailures = true
			break
		}
	}
	if hasFailures {
		fmt.Printf("\n%s Check error messages above for details\n", labelStyle.Render("Tip:"))
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

// syncProject syncs a single project and returns the result
func syncProject(
	name string,
	project registry.Project,
	baseDir string,
	storageHost string,
	globalExcludes []string,
	rsync *sync.Rsync,
	dryRun bool,
	force bool,
	labelStyle, successStyle, warnStyle, errorStyle, infoStyle lipgloss.Style,
) syncResult {
	result := syncResult{name: name}

	fmt.Printf("\n  %s %s\n", labelStyle.Render("Project:"), name)

	// Build local path (handles both relative and absolute paths)
	localPath := resolveProjectPath(project.Path, baseDir)

	// Check if local directory exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		result.err = fmt.Errorf("local directory not found: %s", localPath)
		fmt.Printf("    %s %v\n", errorStyle.Render("Error:"), result.err)
		return result
	}

	// Build remote path (same path on storage machine)
	remotePath := sync.FormatRemotePath(storageHost, resolveProjectPath(project.Path, baseDir))

	// Merge exclusion patterns (global + per-project)
	excludes := make([]string, 0, len(globalExcludes)+len(project.Sync.Exclude))
	excludes = append(excludes, globalExcludes...)
	excludes = append(excludes, project.Sync.Exclude...)

	fmt.Printf("    %s %s\n", infoStyle.Render("Local:"), localPath)
	fmt.Printf("    %s %s\n", infoStyle.Render("Remote:"), remotePath)

	if len(excludes) > 0 {
		fmt.Printf("    %s %s\n", infoStyle.Render("Excludes:"), strings.Join(excludes, ", "))
	}

	// Detect conflicts
	fmt.Printf("    Checking for conflicts...")
	conflicts, err := sync.DetectConflictsWithRsync(rsync, localPath, storageHost, resolveProjectPath(project.Path, baseDir), excludes)
	if err != nil {
		fmt.Printf(" %s\n", warnStyle.Render("failed"))
		fmt.Printf("    %s Could not detect conflicts: %v\n", warnStyle.Render("Warning:"), err)
		// Continue anyway - conflict detection is advisory
	} else {
		result.conflicts = conflicts
		if len(conflicts) > 0 {
			fmt.Printf(" %s\n", warnStyle.Render(fmt.Sprintf("found %d", len(conflicts))))
			printConflicts(conflicts, warnStyle, infoStyle)

			// Check if we should proceed
			if !force && !dryRun {
				if !confirmProceed() {
					result.err = fmt.Errorf("sync cancelled by user")
					fmt.Printf("    %s\n", infoStyle.Render("Skipped"))
					return result
				}
			}
		} else {
			fmt.Printf(" %s\n", successStyle.Render("none"))
		}
	}

	// Pull from storage (remote -> local)
	fmt.Printf("    Pulling from storage...")
	pullOpts := sync.RsyncOptions{
		Source:   remotePath,
		Dest:     localPath,
		Excludes: excludes,
		DryRun:   dryRun,
	}

	pullResult, err := rsync.Pull(pullOpts)
	if err != nil {
		fmt.Printf(" %s\n", errorStyle.Render("failed"))
		fmt.Printf("    %s Pull failed: %v\n", errorStyle.Render("Error:"), err)
		// Continue to try push anyway
	} else {
		result.pulled = true
		if dryRun {
			fmt.Printf(" %s\n", infoStyle.Render("(dry-run)"))
			if pullResult.Output != "" {
				printRsyncPreview(pullResult.Output, "Would pull:", infoStyle)
			}
		} else {
			fmt.Printf(" %s\n", successStyle.Render("done"))
		}
	}

	// Push to storage (local -> remote)
	fmt.Printf("    Pushing to storage...")
	pushOpts := sync.RsyncOptions{
		Source:   localPath,
		Dest:     remotePath,
		Excludes: excludes,
		DryRun:   dryRun,
	}

	pushResult, err := rsync.Push(pushOpts)
	if err != nil {
		fmt.Printf(" %s\n", errorStyle.Render("failed"))
		fmt.Printf("    %s Push failed: %v\n", errorStyle.Render("Error:"), err)
	} else {
		result.pushed = true
		if dryRun {
			fmt.Printf(" %s\n", infoStyle.Render("(dry-run)"))
			if pushResult.Output != "" {
				printRsyncPreview(pushResult.Output, "Would push:", infoStyle)
			}
		} else {
			fmt.Printf(" %s\n", successStyle.Render("done"))
		}
	}

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
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Not interactive - don't proceed
		return false
	}

	fmt.Printf("    Proceed with sync? [y/N]: ")
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
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

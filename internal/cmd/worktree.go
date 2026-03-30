package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/tmux"
	"github.com/PoeAudits/overlord/internal/worktree"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	// tmuxBaseWindow is the default window name created in new sessions.
	tmuxBaseWindow = "base"
	// tmuxAttachedSuffix indicates attached clients in tmux list output.
	tmuxAttachedSuffix = ":1"

	worktreeStatusMain      = "(main)"
	worktreeStatusAttached  = "(attached)"
	worktreeStatusDetached  = "(detached)"
	worktreeStatusNoSession = "(no session)"
)

// worktreeCmd is the parent command for worktree management
var worktreeCmd = &cobra.Command{
	Use:   "worktree",
	Short: "Manage git worktrees with tmux integration",
	Long: `Manage git worktrees with automatic tmux session creation.

Worktrees are created in ~/Overlord/worktrees/{project}/{branch}/ with
a tmux session named {project}--{branch} running opencode.

Commands work from any git repository (registered or not) and automatically
detect the project from the current directory.

Examples:
  overlord worktree new              # Create with auto-generated name
  overlord worktree new fix-auth     # Create with specific branch name
  overlord worktree list             # List worktrees for current project
  overlord worktree attach fix-auth  # Attach to worktree's session
  overlord worktree remove fix-auth  # Remove worktree, branch, and session`,
}

// worktreeNewCmd creates a new worktree with tmux session
var worktreeNewCmd = &cobra.Command{
	Use:   "new [branch]",
	Short: "Create a new worktree with tmux session",
	Long: `Create a new git worktree with an associated tmux session.

If no branch name is provided, one is auto-generated using adjective-noun
pairs (e.g., swift-fix, bright-task).

The worktree is created at ~/Overlord/worktrees/{project}/{branch}/ and
a tmux session named {project}--{branch} is created with opencode running.

Examples:
  overlord worktree new              # Auto-generate branch name
  overlord worktree new fix-auth     # Use specific branch name
  overlord worktree new feature/api  # Branch names with slashes work`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorktreeNew,
}

// worktreeListCmd lists worktrees for the current project
var worktreeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List worktrees for the current project",
	Long: `List all git worktrees for the current project.

Shows the branch name, path, and tmux session status for each worktree.

Examples:
  overlord worktree list
  overlord wt list`,
	Args: cobra.NoArgs,
	RunE: runWorktreeList,
}

// worktreeAttachCmd attaches to a worktree's tmux session
var worktreeAttachCmd = &cobra.Command{
	Use:   "attach [branch]",
	Short: "Attach to a worktree's tmux session",
	Long: `Attach to the tmux session for a worktree.

If no branch is specified and you're in a worktree, attaches to the
current worktree's session.

Examples:
  overlord worktree attach fix-auth  # Attach to specific worktree
  overlord worktree attach           # Attach to current (if in worktree)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWorktreeAttach,
}

// worktreeRemoveCmd removes a worktree, its branch, and tmux session
var worktreeRemoveCmd = &cobra.Command{
	Use:   "remove [branch]",
	Short: "Remove a worktree, its branch, and tmux session",
	Long: `Remove a git worktree along with its branch and tmux session.

If no branch is specified and you're in a worktree, removes the current
worktree (with confirmation).

Examples:
  overlord worktree remove fix-auth  # Remove specific worktree
  overlord worktree remove           # Remove current (if in worktree)`,
	Args:    cobra.MaximumNArgs(1),
	Aliases: []string{"rm"},
	RunE:    runWorktreeRemove,
}

func init() {
	rootCmd.AddCommand(worktreeCmd)
	worktreeCmd.AddCommand(worktreeNewCmd)
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeAttachCmd)
	worktreeCmd.AddCommand(worktreeRemoveCmd)

	// Add 'wt' as an alias for 'worktree'
	worktreeCmd.Aliases = []string{"wt"}
}

// worktreePath returns the path for a worktree
func worktreePath(project, branch string) (string, error) {
	baseDir, err := registry.ExpandPath(defaultWorktreeBaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand worktree base dir: %w", err)
	}
	return worktree.Path(baseDir, project, branch), nil
}

// collectExistingBranchNames merges branch names from refs and worktree metadata.
//
// refs/heads captures every local branch, including ones without a worktree.
// git worktree list captures branches currently attached to linked worktrees.
// We union both sources so generated names avoid collisions across both plain
// branches and active worktree branches.
func collectExistingBranchNames(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/heads")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	seen := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			seen[trimmed] = struct{}{}
		}
	}

	wtCmd := exec.Command("git", "worktree", "list", "--porcelain")
	wtCmd.Dir = repoPath
	wtOutput, err := wtCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	for _, entry := range worktree.ParseWorktreeList(string(wtOutput)) {
		if entry.Branch != "" {
			seen[entry.Branch] = struct{}{}
		}
	}

	branches := make([]string, 0, len(seen))
	for b := range seen {
		branches = append(branches, b)
	}
	return branches, nil
}

func resolveBranch(args []string, repoPath string) (string, error) {
	if len(args) > 0 {
		branch := args[0]
		if worktree.WorktreeExists(repoPath, branch) {
			return "", fmt.Errorf("worktree for branch '%s' already exists\nUse 'overlord worktree attach %s' to attach", branch, branch)
		}
		return branch, nil
	}

	existing, err := collectExistingBranchNames(repoPath)
	if err != nil {
		return "", err
	}

	return worktree.GenerateBranchName(existing), nil
}

// resolveBranchArg resolves branch name from args or current worktree context.
// Used by attach and remove commands.
func resolveBranchArg(args []string, info *worktree.Info, usage string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if info.IsWorktree {
		return info.CurrentBranch, nil
	}
	return "", fmt.Errorf("no branch specified and not in a worktree\n%s", usage)
}

func createWorktreeSession(s styles, info *worktree.Info, sessionName, wtPath string) (*tmux.Session, error) {
	fmt.Printf("  Creating tmux session...")
	session := tmux.NewSession(sessionName, wtPath)

	exists := false
	if ok, err := session.Exists(); err == nil {
		exists = ok
	} else {
		fmt.Printf("\n  %s failed to check tmux session: %v\n", s.Warn.Render("Warning:"), err)
	}

	if exists {
		fmt.Println(" already exists")
		return session, nil
	}

	if err := session.Create(); err != nil {
		rollbackCmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
		rollbackCmd.Dir = info.MainRepoPath
		_ = rollbackCmd.Run()
		return nil, fmt.Errorf("failed to create tmux session: %w", err)
	}

	sendCmd := exec.Command("tmux", "send-keys", "-t", sessionName+":"+tmuxBaseWindow, "opencode", "C-m")
	if err := sendCmd.Run(); err != nil {
		fmt.Printf("\n  %s failed to start opencode: %v\n", s.Warn.Render("Warning:"), err)
	}

	fmt.Println(" done")
	return session, nil
}

func worktreeSessionStatus(s styles, sessionName string) (string, bool, error) {
	sessionExists, err := tmux.NewSession(sessionName, "").Exists()
	if err != nil {
		return s.Dim.Render(worktreeStatusNoSession), false, err
	}
	if !sessionExists {
		return s.Dim.Render(worktreeStatusNoSession), false, nil
	}

	listCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_attached}")
	listOutput, err := listCmd.Output()
	if err == nil && strings.Contains(string(listOutput), sessionName+tmuxAttachedSuffix) {
		return s.Success.Render(worktreeStatusAttached), true, nil
	}

	return s.Info.Render(worktreeStatusDetached), true, nil
}

func formatWorktreeEntry(branch, path, status string) string {
	return fmt.Sprintf("  %-20s  %s  %s", branch, fileutil.FormatPathWithTilde(path), status)
}

func removeWorktreeResources(s styles, info *worktree.Info, branch, sessionName, wtPath string) error {
	fmt.Printf("  Killing tmux session...")
	killSession := exec.Command("tmux", "kill-session", "-t", sessionName)
	if killSession.Run() == nil {
		fmt.Println(" done")
	} else {
		fmt.Println(" not running")
	}

	fmt.Printf("  Removing worktree...")
	removeCmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	removeCmd.Dir = info.MainRepoPath
	if output, err := removeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	fmt.Println(" done")

	fmt.Printf("  Deleting branch...")
	deleteBranch := exec.Command("git", "branch", "-D", branch)
	deleteBranch.Dir = info.MainRepoPath
	if output, err := deleteBranch.CombinedOutput(); err != nil {
		fmt.Printf(" %s %s\n", s.Warn.Render("warning:"), strings.TrimSpace(string(output)))
	} else {
		fmt.Println(" done")
	}

	return nil
}

func cleanupEmptyWorktreeDir(wtPath string) {
	projectDir := filepath.Dir(wtPath)
	if entries, err := os.ReadDir(projectDir); err == nil && len(entries) == 0 {
		_ = os.Remove(projectDir)
	}
}

func detectWorktreeProject() (*worktree.Info, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return worktree.DetectProject(cwd)
}

func printWorktreeDetails(s styles, project, branch, wtPath, sessionName string) {
	fmt.Printf("%s %s\n", s.Label.Render("Project:"), project)
	fmt.Printf("%s %s\n", s.Label.Render("Branch:"), branch)
	fmt.Printf("%s %s\n", s.Label.Render("Path:"), s.Path.Render(wtPath))
	fmt.Printf("%s %s\n\n", s.Label.Render("Session:"), sessionName)
}

func createGitWorktree(repoPath, branch, wtPath string) error {
	fmt.Printf("  Creating worktree...")
	var gitCmd *exec.Cmd
	if worktree.BranchExists(repoPath, branch) {
		gitCmd = exec.Command("git", "worktree", "add", wtPath, branch)
	} else {
		gitCmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	}
	gitCmd.Dir = repoPath
	if output, err := gitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	fmt.Println(" done")
	return nil
}

func managedWorktreeEntries(entries []worktree.Entry, managedDir string) []worktree.Entry {
	managed := make([]worktree.Entry, 0, len(entries))
	for i, wt := range entries {
		if i == 0 || strings.HasPrefix(wt.Path, managedDir) {
			managed = append(managed, wt)
		}
	}
	return managed
}

func printManagedWorktreeEntries(s styles, projectName string, entries []worktree.Entry) int {
	activeSessions := 0
	for i, wt := range entries {
		if i == 0 {
			fmt.Println(formatWorktreeEntry(s.Bold.Render(wt.Branch), wt.Path, s.Dim.Render(worktreeStatusMain)))
			continue
		}
		sessionName := worktree.SessionName(projectName, wt.Branch)
		status, isActive, err := worktreeSessionStatus(s, sessionName)
		if err != nil {
			fmt.Printf("%s failed to check tmux session '%s': %v\n", s.Warn.Render("Warning:"), sessionName, err)
		}
		if isActive {
			activeSessions++
		}
		fmt.Println(formatWorktreeEntry(s.Name.Render(wt.Branch), wt.Path, status))
	}
	return activeSessions
}

func confirmWorktreeRemoval(s styles, branch, wtPath, sessionName string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return true, nil
	}

	fmt.Printf("%s %s\n", s.Label.Render("Branch:"), branch)
	fmt.Printf("%s %s\n", s.Label.Render("Path:"), s.Path.Render(wtPath))
	fmt.Printf("%s %s\n\n", s.Label.Render("Session:"), sessionName)

	confirmed, err := confirmStrict("Remove worktree, branch, and session?")
	if err != nil || !confirmed {
		return confirmed, err
	}
	return true, nil
}

func warnIfRemovingCurrentWorktree(s styles, wtPath, mainRepoPath string) {
	cwd, err := os.Getwd()
	if err == nil && strings.HasPrefix(cwd, wtPath) {
		fmt.Printf("\n%s You are currently in this worktree.\n", s.Warn.Render("Note:"))
		fmt.Printf("After removal, run: cd %s\n\n", mainRepoPath)
	}
}

func runWorktreeNew(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	info, err := detectWorktreeProject()
	if err != nil {
		return err
	}

	branch, err := resolveBranch(args, info.MainRepoPath)
	if err != nil {
		return err
	}

	// Get worktree path
	wtPath, err := worktreePath(info.ProjectName, branch)
	if err != nil {
		return err
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	sessionName := worktree.SessionName(info.ProjectName, branch)
	printWorktreeDetails(s, info.ProjectName, branch, wtPath, sessionName)

	if err := createGitWorktree(info.MainRepoPath, branch, wtPath); err != nil {
		return err
	}

	session, err := createWorktreeSession(s, info, sessionName, wtPath)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("%s\n", s.Success.Render("Worktree created successfully!"))
	fmt.Println()

	// Attach to session
	if err := session.AttachOrSwitch(); err != nil {
		fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
		fmt.Printf("Attach manually: tmux attach -t %s\n", sessionName)
	}

	return nil
}

func runWorktreeList(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	info, err := detectWorktreeProject()
	if err != nil {
		return err
	}

	// Get worktree list
	gitCmd := exec.Command("git", "worktree", "list", "--porcelain")
	gitCmd.Dir = info.MainRepoPath
	output, err := gitCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}
	entries := worktree.ParseWorktreeList(string(output))

	// Get worktree base path
	baseDir, err := registry.ExpandPath(defaultWorktreeBaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand worktree base dir: %w", err)
	}
	managedDir := filepath.Join(baseDir, info.ProjectName)
	managedEntries := managedWorktreeEntries(entries, managedDir)

	fmt.Printf("%s %s\n\n", s.Label.Render("Worktrees for"), s.Bold.Render(info.ProjectName))
	activeSessions := printManagedWorktreeEntries(s, info.ProjectName, managedEntries)

	if len(managedEntries) == 1 {
		fmt.Printf("  %s\n", s.Dim.Render("No worktrees found"))
	}

	fmt.Println()
	if activeSessions > 0 {
		fmt.Printf("%s %d active\n", s.Label.Render("Sessions:"), activeSessions)
	}

	return nil
}

func runWorktreeAttach(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// Detect project
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	info, err := worktree.DetectProject(cwd)
	if err != nil {
		return err
	}

	// Resolve branch from args or current worktree
	branch, err := resolveBranchArg(args, info, "Usage: overlord worktree attach <branch>")
	if err != nil {
		return err
	}

	sessionName := worktree.SessionName(info.ProjectName, branch)

	// Check if session exists
	sessionExists, err := tmux.NewSession(sessionName, "").Exists()
	if err != nil {
		return err
	}
	if !sessionExists {
		// Session doesn't exist - check if worktree exists
		if !worktree.WorktreeExists(info.MainRepoPath, branch) {
			return fmt.Errorf("no worktree found for branch '%s'\nCreate one with: overlord worktree new %s", branch, branch)
		}

		// Worktree exists but no session - create one
		wtPath, err := worktreePath(info.ProjectName, branch)
		if err != nil {
			return err
		}

		fmt.Printf("Creating session for existing worktree...\n")
		session := tmux.NewSession(sessionName, wtPath)
		if err := session.Create(); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		// Start opencode (best-effort)
		sendCmd := exec.Command("tmux", "send-keys", "-t", sessionName+":"+tmuxBaseWindow, "opencode", "C-m")
		_ = sendCmd.Run()
	}

	fmt.Printf("%s %s\n", s.Label.Render("Attaching to:"), sessionName)

	session := tmux.NewSession(sessionName, "")
	if err := session.AttachOrSwitch(); err != nil {
		return fmt.Errorf("failed to attach to session: %w", err)
	}

	return nil
}

func runWorktreeRemove(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	info, err := detectWorktreeProject()
	if err != nil {
		return err
	}

	// Resolve branch from args or current worktree
	branch, err := resolveBranchArg(args, info, "Usage: overlord worktree remove <branch>")
	if err != nil {
		return err
	}

	// Check if worktree exists
	if !worktree.WorktreeExists(info.MainRepoPath, branch) {
		return fmt.Errorf("no worktree found for branch '%s'", branch)
	}

	sessionName := worktree.SessionName(info.ProjectName, branch)
	wtPath, err := worktreePath(info.ProjectName, branch)
	if err != nil {
		return err
	}

	confirmed, err := confirmWorktreeRemoval(s, branch, wtPath, sessionName)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled")
		return nil
	}

	warnIfRemovingCurrentWorktree(s, wtPath, info.MainRepoPath)

	if err := removeWorktreeResources(s, info, branch, sessionName, wtPath); err != nil {
		return err
	}

	cleanupEmptyWorktreeDir(wtPath)

	fmt.Println()
	fmt.Printf("%s\n", s.Success.Render("Worktree removed successfully!"))

	return nil
}

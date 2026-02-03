package cmd

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// worktreeInfo holds information about the current git context
type worktreeInfo struct {
	projectName   string // name of the project (from directory name)
	mainRepoPath  string // path to the main working tree
	isWorktree    bool   // true if cwd is a worktree (not main repo)
	currentBranch string // current branch name (if in worktree)
}

// worktreeEntry represents a single worktree from git worktree list
type worktreeEntry struct {
	path   string
	branch string
	isMain bool
}

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

// detectProject detects the project from the current working directory
func detectProject() (*worktreeInfo, error) {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in a git repository
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = cwd
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("not in a git repository")
	}

	// Get the toplevel of the current worktree
	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git toplevel: %w", err)
	}
	currentToplevel := strings.TrimSpace(string(output))

	// Check if this is a worktree by looking at git worktree list
	cmd = exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = cwd
	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	worktrees := parseWorktreeList(string(output))
	if len(worktrees) == 0 {
		return nil, fmt.Errorf("failed to parse worktree list")
	}

	// Find the main worktree and check if current is a worktree
	var mainPath string
	var isWorktree bool
	var currentBranch string

	for _, wt := range worktrees {
		if wt.isMain {
			mainPath = wt.path
		}
		if wt.path == currentToplevel {
			isWorktree = !wt.isMain
			if isWorktree {
				currentBranch = wt.branch
			}
		}
	}

	if mainPath == "" {
		mainPath = currentToplevel
	}

	// Extract project name from main repo path
	projectName := filepath.Base(mainPath)

	return &worktreeInfo{
		projectName:   projectName,
		mainRepoPath:  mainPath,
		isWorktree:    isWorktree,
		currentBranch: currentBranch,
	}, nil
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) []worktreeEntry {
	var entries []worktreeEntry
	var current worktreeEntry
	isFirst := true

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.path != "" {
				// First entry is always the main worktree
				current.isMain = isFirst
				isFirst = false
				entries = append(entries, current)
				current = worktreeEntry{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.path = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			current.branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}

	// Don't forget the last entry
	if current.path != "" {
		current.isMain = isFirst
		entries = append(entries, current)
	}

	return entries
}

// generateBranchName generates a random adjective-noun branch name
func generateBranchName() (string, error) {
	adjIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(worktreeAdjectives))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random adjective: %w", err)
	}

	nounIdx, err := rand.Int(rand.Reader, big.NewInt(int64(len(worktreeNouns))))
	if err != nil {
		return "", fmt.Errorf("failed to generate random noun: %w", err)
	}

	return fmt.Sprintf("%s-%s", worktreeAdjectives[adjIdx.Int64()], worktreeNouns[nounIdx.Int64()]), nil
}

// worktreeSessionName returns the tmux session name for a project/branch
func worktreeSessionName(project, branch string) string {
	// Replace slashes in branch name with dashes for valid session names
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return fmt.Sprintf("%s--%s", project, safeBranch)
}

// worktreePath returns the path for a worktree
func worktreePath(project, branch string) (string, error) {
	baseDir, err := registry.ExpandPath(defaultWorktreeBaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand worktree base dir: %w", err)
	}
	// Replace slashes in branch name with dashes for valid directory names
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(baseDir, project, safeBranch), nil
}

// branchExists checks if a git branch already exists
func branchExists(repoPath, branch string) bool {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoPath
	return cmd.Run() == nil
}

// worktreeExists checks if a worktree already exists for a branch
func worktreeExists(repoPath, branch string) bool {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	worktrees := parseWorktreeList(string(output))
	for _, wt := range worktrees {
		if wt.branch == branch {
			return true
		}
	}
	return false
}

// ensureUniqueBranch ensures the branch name is unique, appending a counter if needed
func ensureUniqueBranch(repoPath, baseName string) string {
	name := baseName
	counter := 2

	for branchExists(repoPath, name) || worktreeExists(repoPath, name) {
		name = fmt.Sprintf("%s-%d", baseName, counter)
		counter++
	}

	return name
}

// resolveBranchArg resolves branch name from args or current worktree context.
// Used by attach and remove commands.
func resolveBranchArg(args []string, info *worktreeInfo, usage string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if info.isWorktree {
		return info.currentBranch, nil
	}
	return "", fmt.Errorf("no branch specified and not in a worktree\n%s", usage)
}

func runWorktreeNew(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// Detect project
	info, err := detectProject()
	if err != nil {
		return err
	}

	// Get or generate branch name
	var branch string
	if len(args) > 0 {
		branch = args[0]
		// Check if branch/worktree already exists
		if worktreeExists(info.mainRepoPath, branch) {
			return fmt.Errorf("worktree for branch '%s' already exists\nUse 'overlord worktree attach %s' to attach", branch, branch)
		}
	} else {
		// Generate a unique name
		baseName, err := generateBranchName()
		if err != nil {
			return err
		}
		branch = ensureUniqueBranch(info.mainRepoPath, baseName)
	}

	// Get worktree path
	wtPath, err := worktreePath(info.projectName, branch)
	if err != nil {
		return err
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	sessionName := worktreeSessionName(info.projectName, branch)

	fmt.Printf("%s %s\n", s.Label.Render("Project:"), info.projectName)
	fmt.Printf("%s %s\n", s.Label.Render("Branch:"), branch)
	fmt.Printf("%s %s\n", s.Label.Render("Path:"), s.Path.Render(wtPath))
	fmt.Printf("%s %s\n", s.Label.Render("Session:"), sessionName)
	fmt.Println()

	// Create worktree
	fmt.Printf("  Creating worktree...")
	var gitCmd *exec.Cmd
	if branchExists(info.mainRepoPath, branch) {
		// Branch exists, create worktree from it
		gitCmd = exec.Command("git", "worktree", "add", wtPath, branch)
	} else {
		// Create new branch with worktree
		gitCmd = exec.Command("git", "worktree", "add", "-b", branch, wtPath)
	}
	gitCmd.Dir = info.mainRepoPath
	if output, err := gitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree: %s", strings.TrimSpace(string(output)))
	}
	fmt.Println(" done")

	// Create tmux session
	fmt.Printf("  Creating tmux session...")
	session := &tmuxSession{
		Name: sessionName,
		Dir:  wtPath,
	}

	if session.exists() {
		fmt.Println(" already exists")
	} else {
		if err := session.create(); err != nil {
			// Rollback: remove worktree
			rollbackCmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
			rollbackCmd.Dir = info.mainRepoPath
			rollbackCmd.Run()
			return fmt.Errorf("failed to create tmux session: %w", err)
		}

		// Send opencode command to the base window
		sendCmd := exec.Command("tmux", "send-keys", "-t", sessionName+":base", "opencode", "C-m")
		if err := sendCmd.Run(); err != nil {
			fmt.Printf("\n  %s failed to start opencode: %v\n", s.Warn.Render("Warning:"), err)
		}

		fmt.Println(" done")
	}

	fmt.Println()
	fmt.Printf("%s\n", s.Success.Render("Worktree created successfully!"))
	fmt.Println()

	// Attach to session
	if err := session.attachOrSwitch(); err != nil {
		fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
		fmt.Printf("Attach manually: tmux attach -t %s\n", sessionName)
	}

	return nil
}

func runWorktreeList(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// Detect project
	info, err := detectProject()
	if err != nil {
		return err
	}

	// Get worktree list
	gitCmd := exec.Command("git", "worktree", "list", "--porcelain")
	gitCmd.Dir = info.mainRepoPath
	output, err := gitCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	worktrees := parseWorktreeList(string(output))

	// Get worktree base path
	baseDir, err := registry.ExpandPath(defaultWorktreeBaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand worktree base dir: %w", err)
	}
	projectWorktreeDir := filepath.Join(baseDir, info.projectName)

	fmt.Printf("%s %s\n\n", s.Label.Render("Worktrees for"), s.Bold.Render(info.projectName))

	// Filter to show only worktrees in our managed directory (plus main)
	hasWorktrees := false
	activeSessions := 0

	for _, wt := range worktrees {
		if wt.isMain {
			// Show main repo
			status := s.Dim.Render("(main)")
			fmt.Printf("  %-20s  %s  %s\n", s.Bold.Render(wt.branch), formatPathWithTilde(wt.path), status)
			continue
		}

		// Only show worktrees in our managed directory
		if !strings.HasPrefix(wt.path, projectWorktreeDir) {
			continue
		}

		hasWorktrees = true
		sessionName := worktreeSessionName(info.projectName, wt.branch)

		// Check session status
		var status string
		checkSession := exec.Command("tmux", "has-session", "-t", sessionName)
		if checkSession.Run() == nil {
			// Check if attached
			listCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}:#{session_attached}")
			listOutput, _ := listCmd.Output()
			if strings.Contains(string(listOutput), sessionName+":1") {
				status = s.Success.Render("(attached)")
			} else {
				status = s.Info.Render("(detached)")
			}
			activeSessions++
		} else {
			status = s.Dim.Render("(no session)")
		}

		fmt.Printf("  %-20s  %s  %s\n", s.Name.Render(wt.branch), formatPathWithTilde(wt.path), status)
	}

	if !hasWorktrees {
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
	info, err := detectProject()
	if err != nil {
		return err
	}

	// Resolve branch from args or current worktree
	branch, err := resolveBranchArg(args, info, "Usage: overlord worktree attach <branch>")
	if err != nil {
		return err
	}

	sessionName := worktreeSessionName(info.projectName, branch)

	// Check if session exists
	checkSession := exec.Command("tmux", "has-session", "-t", sessionName)
	if checkSession.Run() != nil {
		// Session doesn't exist - check if worktree exists
		if !worktreeExists(info.mainRepoPath, branch) {
			return fmt.Errorf("no worktree found for branch '%s'\nCreate one with: overlord worktree new %s", branch, branch)
		}

		// Worktree exists but no session - create one
		wtPath, err := worktreePath(info.projectName, branch)
		if err != nil {
			return err
		}

		fmt.Printf("Creating session for existing worktree...\n")
		session := &tmuxSession{
			Name: sessionName,
			Dir:  wtPath,
		}

		if err := session.create(); err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		// Start opencode
		sendCmd := exec.Command("tmux", "send-keys", "-t", sessionName+":base", "opencode", "C-m")
		sendCmd.Run()
	}

	fmt.Printf("%s %s\n", s.Label.Render("Attaching to:"), sessionName)

	session := &tmuxSession{Name: sessionName}
	if err := session.attachOrSwitch(); err != nil {
		return fmt.Errorf("failed to attach to session: %w", err)
	}

	return nil
}

func runWorktreeRemove(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// Detect project
	info, err := detectProject()
	if err != nil {
		return err
	}

	// Resolve branch from args or current worktree
	branch, err := resolveBranchArg(args, info, "Usage: overlord worktree remove <branch>")
	if err != nil {
		return err
	}

	// Check if worktree exists
	if !worktreeExists(info.mainRepoPath, branch) {
		return fmt.Errorf("no worktree found for branch '%s'", branch)
	}

	sessionName := worktreeSessionName(info.projectName, branch)
	wtPath, err := worktreePath(info.projectName, branch)
	if err != nil {
		return err
	}

	// Confirm removal
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Printf("%s %s\n", s.Label.Render("Branch:"), branch)
		fmt.Printf("%s %s\n", s.Label.Render("Path:"), s.Path.Render(wtPath))
		fmt.Printf("%s %s\n", s.Label.Render("Session:"), sessionName)
		fmt.Println()

		confirmed, err := confirmAction("Remove worktree, branch, and session?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// If we're in the worktree being removed, warn user
	cwd, _ := os.Getwd()
	if strings.HasPrefix(cwd, wtPath) {
		fmt.Printf("\n%s You are currently in this worktree.\n", s.Warn.Render("Note:"))
		fmt.Printf("After removal, run: cd %s\n\n", info.mainRepoPath)
	}

	// Kill tmux session if exists
	fmt.Printf("  Killing tmux session...")
	killSession := exec.Command("tmux", "kill-session", "-t", sessionName)
	if killSession.Run() == nil {
		fmt.Println(" done")
	} else {
		fmt.Println(" not running")
	}

	// Remove worktree
	fmt.Printf("  Removing worktree...")
	removeCmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	removeCmd.Dir = info.mainRepoPath
	if output, err := removeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %s", strings.TrimSpace(string(output)))
	}
	fmt.Println(" done")

	// Delete branch
	fmt.Printf("  Deleting branch...")
	deleteBranch := exec.Command("git", "branch", "-D", branch)
	deleteBranch.Dir = info.mainRepoPath
	if output, err := deleteBranch.CombinedOutput(); err != nil {
		fmt.Printf(" %s %s\n", s.Warn.Render("warning:"), strings.TrimSpace(string(output)))
	} else {
		fmt.Println(" done")
	}

	// Clean up empty directories
	projectDir := filepath.Dir(wtPath)
	if entries, err := os.ReadDir(projectDir); err == nil && len(entries) == 0 {
		os.Remove(projectDir)
	}

	fmt.Println()
	fmt.Printf("%s\n", s.Success.Render("Worktree removed successfully!"))

	return nil
}

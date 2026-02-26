package worktree

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os/exec"
	"path/filepath"
	"strings"
)

// Info describes the detected git context for worktree operations.
type Info struct {
	ProjectName   string
	MainRepoPath  string
	IsWorktree    bool
	CurrentBranch string
}

// Entry represents a parsed line from `git worktree list --porcelain`.
type Entry struct {
	Path   string
	Branch string
	Head   string
	IsBare bool
}

// adjectives provides the adjective pool for generated branch names.
var adjectives = []string{
	"swift", "bright", "clever", "smooth", "quick",
	"clean", "sharp", "neat", "cool", "fast",
	"bold", "calm", "clear", "crisp", "fresh",
	"light", "prime", "ready", "smart", "steady",
}

// nouns provides the noun pool for generated branch names.
var nouns = []string{
	"fix", "task", "work", "dev", "patch",
	"branch", "code", "build", "test", "run",
	"flow", "sync", "push", "pull", "draft",
	"spike", "probe", "check", "scan", "forge",
}

// DetectProject determines git context information for the provided cwd.
func DetectProject(cwd string) (*Info, error) {
	if strings.TrimSpace(cwd) == "" {
		return nil, fmt.Errorf("cwd must not be empty")
	}

	checkCmd := exec.Command("git", "rev-parse", "--git-dir")
	checkCmd.Dir = cwd
	if err := checkCmd.Run(); err != nil {
		return nil, fmt.Errorf("not a git repository: %w", err)
	}

	topCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	topCmd.Dir = cwd
	topOutput, err := topCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to determine repository toplevel: %w", err)
	}
	currentTop := strings.TrimSpace(string(topOutput))

	wtCmd := exec.Command("git", "worktree", "list", "--porcelain")
	wtCmd.Dir = cwd
	wtOutput, err := wtCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}
	entries := ParseWorktreeList(string(wtOutput))

	mainPath := currentTop
	if len(entries) > 0 && entries[0].Path != "" {
		mainPath = entries[0].Path
	}

	info := &Info{
		ProjectName:  filepath.Base(mainPath),
		MainRepoPath: mainPath,
	}

	for _, entry := range entries {
		if entry.Path == currentTop {
			if entry.Path != mainPath {
				info.IsWorktree = true
			}
			info.CurrentBranch = entry.Branch
			return info, nil
		}
	}

	// If the current toplevel wasn't listed, treat it as main.
	return info, nil
}

// ParseWorktreeList parses git worktree output into structured entries.
func ParseWorktreeList(output string) []Entry {
	var entries []Entry
	lines := strings.Split(output, "\n")
	current := Entry{}

	flush := func() {
		if current.Path == "" {
			return
		}
		entries = append(entries, current)
		current = Entry{}
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if current.Path != "" {
				flush()
			}
			current.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			continue
		}

		if current.Path == "" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case strings.HasPrefix(line, "branch refs/heads/"):
			current.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
		case strings.HasPrefix(line, "branch "):
			branchVal := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			current.Branch = strings.TrimPrefix(branchVal, "refs/heads/")
		case line == "detached":
			current.Branch = ""
		case line == "bare":
			current.IsBare = true
		}
	}

	flush()
	return entries
}

// GenerateBranchName creates an adjective-noun branch name avoiding collisions.
func GenerateBranchName(existingBranches []string) string {
	base := randomWord(adjectives)
	noun := randomWord(nouns)
	name := strings.Trim(strings.Join([]string{base, noun}, "-"), "-")
	if name == "" {
		name = "worktree"
	}
	return EnsureUniqueBranch(name, existingBranches)
}

// SessionName returns the tmux session identifier for a project and branch.
func SessionName(project, branch string) string {
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return fmt.Sprintf("%s--%s", project, safeBranch)
}

// Path builds the filesystem path for a worktree under the provided base dir.
func Path(baseDir, project, branch string) string {
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(baseDir, project, safeBranch)
}

// branchSuffixMax is the upper bound for suffix iteration in EnsureUniqueBranch.
const branchSuffixMax = 1000

// EnsureUniqueBranch appends a numeric suffix when the base name is taken.
func EnsureUniqueBranch(name string, existingBranches []string) string {
	set := make(map[string]struct{}, len(existingBranches))
	for _, b := range existingBranches {
		set[b] = struct{}{}
	}
	if _, ok := set[name]; !ok {
		return name
	}
	counter := 2
	for i := 0; i < branchSuffixMax; i++ {
		candidate := fmt.Sprintf("%s-%d", name, counter)
		if _, exists := set[candidate]; !exists {
			return candidate
		}
		counter++
	}
	// Exhausted suffix space — return last candidate.
	return fmt.Sprintf("%s-%d", name, counter)
}

// BranchExists returns true if the branch exists in the provided repo path.
func BranchExists(repoPath, branch string) bool {
	branch = strings.TrimSpace(branch)
	if repoPath == "" || branch == "" {
		return false
	}
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = repoPath
	return cmd.Run() == nil
}

// WorktreeExists returns true if a worktree exists for the branch in repo path.
func WorktreeExists(repoPath, branch string) bool {
	branch = strings.TrimSpace(branch)
	if repoPath == "" || branch == "" {
		return false
	}
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	entries := ParseWorktreeList(string(output))
	for _, entry := range entries {
		if entry.Branch == branch {
			return true
		}
	}
	return false
}

func randomWord(words []string) string {
	if len(words) == 0 {
		return ""
	}
	if len(words) == 1 {
		return words[0]
	}
	max := big.NewInt(int64(len(words)))
	idx, err := rand.Int(rand.Reader, max)
	if err != nil {
		return words[0]
	}
	return words[idx.Int64()]
}

package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected []Entry
	}{
		{
			name:     "empty output",
			input:    "",
			expected: nil,
		},
		{
			name: "mixed entries",
			input: strings.TrimSpace(`
worktree /projects/main
HEAD 1111111
branch refs/heads/main

worktree /projects/main/wt/feature
HEAD 2222222
branch refs/heads/feature

worktree /projects/main/wt/detached
HEAD 3333333
detached

worktree /projects/main/wt/bare
bare
HEAD 4444444
branch refs/heads/rewrite
`),
			expected: []Entry{
				{Path: "/projects/main", Branch: "main", Head: "1111111"},
				{Path: "/projects/main/wt/feature", Branch: "feature", Head: "2222222"},
				{Path: "/projects/main/wt/detached", Branch: "", Head: "3333333"},
				{Path: "/projects/main/wt/bare", Branch: "rewrite", Head: "4444444", IsBare: true},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entries := ParseWorktreeList(tc.input)
			if len(entries) != len(tc.expected) {
				t.Fatalf("expected %d entries, got %d", len(tc.expected), len(entries))
			}
			for i := range entries {
				if entries[i] != tc.expected[i] {
					t.Fatalf("entry %d mismatch: got %+v want %+v", i, entries[i], tc.expected[i])
				}
			}
		})
	}
}

func TestGenerateBranchName_UsesWordListsAndAvoidsCollisions(t *testing.T) {
	withWordLists(t, []string{"onyx"}, []string{"spark"})

	name := GenerateBranchName(nil)
	if name != "onyx-spark" {
		t.Fatalf("expected base adjective-noun pair, got %s", name)
	}

	existing := []string{"onyx-spark"}
	unique := GenerateBranchName(existing)
	if unique != "onyx-spark-2" {
		t.Fatalf("expected suffixed branch when collisions present, got %s", unique)
	}
}

func TestGenerateBranchName_UsesRealWordLists(t *testing.T) {
	// Verify GenerateBranchName(nil) produces a valid adjective-noun pair
	// from the actual adjectives/nouns word lists (no override).
	for i := 0; i < 10; i++ {
		name := GenerateBranchName(nil)
		parts := strings.Split(name, "-")
		if len(parts) != 2 {
			t.Fatalf("GenerateBranchName()=%q expected adjective-noun format", name)
		}
		foundAdj := false
		for _, adj := range adjectives {
			if adj == parts[0] {
				foundAdj = true
				break
			}
		}
		if !foundAdj {
			t.Fatalf("adjective %q not found in adjectives list", parts[0])
		}
		foundNoun := false
		for _, noun := range nouns {
			if noun == parts[1] {
				foundNoun = true
				break
			}
		}
		if !foundNoun {
			t.Fatalf("noun %q not found in nouns list", parts[1])
		}
	}
}

func TestSessionName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		project string
		branch  string
		want    string
	}{
		{project: "demo", branch: "feature", want: "demo--feature"},
		{project: "demo", branch: "feature/api", want: "demo--feature-api"},
		{project: "demo", branch: "release_candidate", want: "demo--release_candidate"},
	}

	for _, tc := range cases {
		got := SessionName(tc.project, tc.branch)
		if got != tc.want {
			t.Fatalf("SessionName(%s,%s)=%s want %s", tc.project, tc.branch, got, tc.want)
		}
	}
}

func TestPath(t *testing.T) {
	base := t.TempDir()
	want := filepath.Join(base, "demo", "feature-api")
	got := Path(base, "demo", "feature/api")
	if got != want {
		t.Fatalf("Path()=%s want %s", got, want)
	}
}

func TestEnsureUniqueBranch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		base     string
		existing []string
		want     string
	}{
		{name: "unused", base: "alpha", existing: []string{"beta"}, want: "alpha"},
		{name: "single collision", base: "alpha", existing: []string{"alpha"}, want: "alpha-2"},
		{name: "multiple collisions", base: "alpha", existing: []string{"alpha", "alpha-2", "alpha-3"}, want: "alpha-4"},
	}

	for _, tc := range cases {
		got := EnsureUniqueBranch(tc.base, tc.existing)
		if got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestWordListsNoDuplicates(t *testing.T) {
	checkUnique := func(name string, values []string) {
		set := make(map[string]struct{})
		for _, v := range values {
			if _, dup := set[v]; dup {
				t.Fatalf("%s list contains duplicate value %s", name, v)
			}
			set[v] = struct{}{}
		}
	}

	checkUnique("adjectives", adjectives)
	checkUnique("nouns", nouns)
}

func TestBranchAndWorktreeQueries(t *testing.T) {
	repo := createTestRepo(t)
	runGit(t, repo, "branch", "feature")
	if !BranchExists(repo, "feature") {
		t.Fatal("expected BranchExists to detect existing branch")
	}
	if BranchExists(repo, "missing") {
		t.Fatal("expected BranchExists to return false for unknown branch")
	}

	wtDir := filepath.Join(t.TempDir(), "feature-wt")
	runGit(t, repo, "worktree", "add", wtDir, "feature")
	if !WorktreeExists(repo, "feature") {
		t.Fatal("expected WorktreeExists to detect active worktree")
	}
	if WorktreeExists(repo, "other") {
		t.Fatal("expected WorktreeExists to return false for missing worktree")
	}
}

func TestDetectProject(t *testing.T) {
	repo := createTestRepo(t)
	info, err := DetectProject(repo)
	if err != nil {
		t.Fatalf("DetectProject main repo failed: %v", err)
	}
	if info.ProjectName != filepath.Base(repo) {
		t.Fatalf("expected project %s got %s", filepath.Base(repo), info.ProjectName)
	}
	if info.MainRepoPath != repo {
		t.Fatalf("expected main repo path %s got %s", repo, info.MainRepoPath)
	}
	if info.IsWorktree {
		t.Fatal("main repo should not be marked as worktree")
	}

	wtDir := filepath.Join(t.TempDir(), "feature-wt")
	runGit(t, repo, "worktree", "add", "-b", "feature", wtDir)
	wtInfo, err := DetectProject(wtDir)
	if err != nil {
		t.Fatalf("DetectProject worktree failed: %v", err)
	}
	if !wtInfo.IsWorktree {
		t.Fatal("expected worktree flag to be true")
	}
	if wtInfo.CurrentBranch != "feature" {
		t.Fatalf("expected current branch feature got %s", wtInfo.CurrentBranch)
	}
	if wtInfo.MainRepoPath != repo {
		t.Fatalf("expected main repo path %s got %s", repo, wtInfo.MainRepoPath)
	}
}

func withWordLists(t *testing.T, adjs, nns []string) {
	t.Helper()
	origAdj := adjectives
	origNouns := nouns
	adjectives = append([]string{}, adjs...)
	nouns = append([]string{}, nns...)
	t.Cleanup(func() {
		adjectives = origAdj
		nouns = origNouns
	})
}

func createTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "ci@example.com")
	runGit(t, dir, "config", "user.name", "CI")
	file := filepath.Join(dir, "README.md")
	if err := os.WriteFile(file, []byte("test"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

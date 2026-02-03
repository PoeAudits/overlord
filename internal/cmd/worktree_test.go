package cmd

import (
	"strings"
	"testing"
)

func TestWorktreeSessionName(t *testing.T) {
	tests := []struct {
		project string
		branch  string
		want    string
	}{
		{"my-project", "fix-auth", "my-project--fix-auth"},
		{"overlord", "swift-fix", "overlord--swift-fix"},
		{"api", "feature/new-endpoint", "api--feature-new-endpoint"},
		{"app", "bugfix/issue-123", "app--bugfix-issue-123"},
	}

	for _, tt := range tests {
		t.Run(tt.project+"_"+tt.branch, func(t *testing.T) {
			got := worktreeSessionName(tt.project, tt.branch)
			if got != tt.want {
				t.Errorf("worktreeSessionName(%q, %q) = %q, want %q", tt.project, tt.branch, got, tt.want)
			}
		})
	}
}

func TestGenerateBranchName(t *testing.T) {
	// Test that generated names follow the adjective-noun pattern
	for i := 0; i < 10; i++ {
		name, err := generateBranchName()
		if err != nil {
			t.Fatalf("generateBranchName() error = %v", err)
		}

		parts := strings.Split(name, "-")
		if len(parts) != 2 {
			t.Errorf("generateBranchName() = %q, expected adjective-noun format", name)
			continue
		}

		// Check that adjective is in the list
		adjFound := false
		for _, adj := range worktreeAdjectives {
			if adj == parts[0] {
				adjFound = true
				break
			}
		}
		if !adjFound {
			t.Errorf("generateBranchName() adjective %q not in worktreeAdjectives", parts[0])
		}

		// Check that noun is in the list
		nounFound := false
		for _, noun := range worktreeNouns {
			if noun == parts[1] {
				nounFound = true
				break
			}
		}
		if !nounFound {
			t.Errorf("generateBranchName() noun %q not in worktreeNouns", parts[1])
		}
	}
}

func TestParseWorktreeList(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []worktreeEntry
	}{
		{
			name: "single main worktree",
			input: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

`,
			want: []worktreeEntry{
				{path: "/home/user/project", branch: "main", isMain: true},
			},
		},
		{
			name: "main with one worktree",
			input: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/worktrees/project/fix-auth
HEAD def456
branch refs/heads/fix-auth

`,
			want: []worktreeEntry{
				{path: "/home/user/project", branch: "main", isMain: true},
				{path: "/home/user/worktrees/project/fix-auth", branch: "fix-auth", isMain: false},
			},
		},
		{
			name: "multiple worktrees",
			input: `worktree /home/user/project
HEAD abc123
branch refs/heads/main

worktree /home/user/worktrees/project/swift-fix
HEAD def456
branch refs/heads/swift-fix

worktree /home/user/worktrees/project/bright-task
HEAD ghi789
branch refs/heads/bright-task

`,
			want: []worktreeEntry{
				{path: "/home/user/project", branch: "main", isMain: true},
				{path: "/home/user/worktrees/project/swift-fix", branch: "swift-fix", isMain: false},
				{path: "/home/user/worktrees/project/bright-task", branch: "bright-task", isMain: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseWorktreeList(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("parseWorktreeList() returned %d entries, want %d", len(got), len(tt.want))
			}

			for i, entry := range got {
				if entry.path != tt.want[i].path {
					t.Errorf("entry[%d].path = %q, want %q", i, entry.path, tt.want[i].path)
				}
				if entry.branch != tt.want[i].branch {
					t.Errorf("entry[%d].branch = %q, want %q", i, entry.branch, tt.want[i].branch)
				}
				if entry.isMain != tt.want[i].isMain {
					t.Errorf("entry[%d].isMain = %v, want %v", i, entry.isMain, tt.want[i].isMain)
				}
			}
		})
	}
}

func TestWorktreeAdjectives(t *testing.T) {
	if len(worktreeAdjectives) != 20 {
		t.Errorf("worktreeAdjectives has %d items, want 20", len(worktreeAdjectives))
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, adj := range worktreeAdjectives {
		if seen[adj] {
			t.Errorf("duplicate adjective: %q", adj)
		}
		seen[adj] = true
	}
}

func TestWorktreeNouns(t *testing.T) {
	if len(worktreeNouns) != 20 {
		t.Errorf("worktreeNouns has %d items, want 20", len(worktreeNouns))
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, noun := range worktreeNouns {
		if seen[noun] {
			t.Errorf("duplicate noun: %q", noun)
		}
		seen[noun] = true
	}
}

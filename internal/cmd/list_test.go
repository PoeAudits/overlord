package cmd

import (
	"testing"
	"time"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestHasAllTags(t *testing.T) {
	tests := []struct {
		name         string
		projectTags  []string
		requiredTags []string
		want         bool
	}{
		{
			name:         "all tags present",
			projectTags:  []string{"api", "backend", "go"},
			requiredTags: []string{"api", "backend"},
			want:         true,
		},
		{
			name:         "missing tag",
			projectTags:  []string{"api", "backend"},
			requiredTags: []string{"api", "frontend"},
			want:         false,
		},
		{
			name:         "empty required tags",
			projectTags:  []string{"api", "backend"},
			requiredTags: []string{},
			want:         true,
		},
		{
			name:         "empty project tags",
			projectTags:  []string{},
			requiredTags: []string{"api"},
			want:         false,
		},
		{
			name:         "both empty",
			projectTags:  []string{},
			requiredTags: []string{},
			want:         true,
		},
		{
			name:         "single tag match",
			projectTags:  []string{"cli"},
			requiredTags: []string{"cli"},
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAllTags(tt.projectTags, tt.requiredTags)
			if got != tt.want {
				t.Errorf("hasAllTags(%v, %v) = %v, want %v", tt.projectTags, tt.requiredTags, got, tt.want)
			}
		})
	}
}

func TestGetSyncedProjects(t *testing.T) {
	tests := []struct {
		name     string
		registry *registry.Registry
		want     int
	}{
		{
			name: "no projects",
			registry: &registry.Registry{
				Version: 2,
				Settings: registry.Settings{
					BaseDir:     "~/Overlord",
					ThoughtsDir: "~/thoughts",
				},
				Projects: map[string]registry.Project{},
			},
			want: 0,
		},
		{
			name: "active project without sync returns nothing",
			registry: &registry.Registry{
				Version: 2,
				Settings: registry.Settings{
					BaseDir:     "~/Overlord",
					ThoughtsDir: "~/thoughts",
				},
				Projects: map[string]registry.Project{
					"project1": {
						Path:        "projects/web/project1",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Active but not synced",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncInactive},
					},
				},
			},
			want: 0,
		},
		{
			name: "active project with sync active is returned",
			registry: &registry.Registry{
				Version: 2,
				Settings: registry.Settings{
					BaseDir:     "~/Overlord",
					ThoughtsDir: "~/thoughts",
				},
				Projects: map[string]registry.Project{
					"project1": {
						Path:        "projects/web/project1",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Active and synced",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
				},
			},
			want: 1,
		},
		{
			name: "archived project with sync active is NOT returned",
			registry: &registry.Registry{
				Version: 2,
				Settings: registry.Settings{
					BaseDir:     "~/Overlord",
					ThoughtsDir: "~/thoughts",
				},
				Projects: map[string]registry.Project{
					"project1": {
						Path:        "projects/web/project1",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Archived but sync active",
						Status:      registry.Status{State: registry.StateArchived},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
				},
			},
			want: 0,
		},
		{
			name: "mixed projects - only active+synced returned",
			registry: &registry.Registry{
				Version: 2,
				Settings: registry.Settings{
					BaseDir:     "~/Overlord",
					ThoughtsDir: "~/thoughts",
				},
				Projects: map[string]registry.Project{
					"synced": {
						Path:        "projects/web/synced",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Active and synced",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
					"active-only": {
						Path:        "projects/web/active-only",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Active but not synced",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{},
					},
					"archived-synced": {
						Path:        "projects/web/archived-synced",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Archived with sync",
						Status:      registry.Status{State: registry.StateArchived},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSyncedProjects(tt.registry)
			if len(got) != tt.want {
				t.Errorf("getSyncedProjects() returned %d projects, want %d", len(got), tt.want)
			}
		})
	}
}

func TestRecentCutoff(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantDur time.Duration
		wantErr bool
	}{
		{
			name:    "7 days",
			input:   "7d",
			wantDur: 7 * 24 * time.Hour,
		},
		{
			name:    "30 days",
			input:   "30d",
			wantDur: 30 * 24 * time.Hour,
		},
		{
			name:    "2 weeks",
			input:   "2w",
			wantDur: 2 * 7 * 24 * time.Hour,
		},
		{
			name:    "24 hours",
			input:   "24h",
			wantDur: 24 * time.Hour,
		},
		{
			name:    "plain number defaults to days",
			input:   "14",
			wantDur: 14 * 24 * time.Hour,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "zero duration",
			input:   "0d",
			wantErr: true,
		},
		{
			name:    "negative number",
			input:   "-5d",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := recentCutoff(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("recentCutoff(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// The cutoff should be approximately now minus the duration
			expected := time.Now().UTC().Add(-tt.wantDur)
			diff := got.Sub(expected)
			if diff < -time.Second || diff > time.Second {
				t.Errorf("recentCutoff(%q) = %v, expected ~%v (diff: %v)", tt.input, got, expected, diff)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		offset   time.Duration
		expected string
	}{
		{
			name:     "just now",
			offset:   30 * time.Second,
			expected: "just now",
		},
		{
			name:     "minutes ago",
			offset:   45 * time.Minute,
			expected: "45m ago",
		},
		{
			name:     "hours ago",
			offset:   3 * time.Hour,
			expected: "3h ago",
		},
		{
			name:     "days ago",
			offset:   3 * 24 * time.Hour,
			expected: "3d ago",
		},
		{
			name:     "1 week ago",
			offset:   8 * 24 * time.Hour,
			expected: "1w ago",
		},
		{
			name:     "multiple weeks ago",
			offset:   21 * 24 * time.Hour,
			expected: "3w ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rfcTime := time.Now().Add(-tt.offset).Format(time.RFC3339)
			got := relativeTime(rfcTime)
			if got != tt.expected {
				t.Errorf("relativeTime() = %q, want %q", got, tt.expected)
			}
		})
	}

	// Test invalid input
	t.Run("empty string", func(t *testing.T) {
		got := relativeTime("")
		if got != "" {
			t.Errorf("relativeTime(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		got := relativeTime("not-a-date")
		if got != "" {
			t.Errorf("relativeTime(\"not-a-date\") = %q, want \"\"", got)
		}
	})
}

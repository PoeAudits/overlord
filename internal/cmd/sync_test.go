package cmd

import (
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestGetSyncActiveProjects(t *testing.T) {
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
			name: "no sync-active projects",
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
						Description: "Test project 1",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncInactive},
					},
					"project2": {
						Path:        "projects/web/project2",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Test project 2",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{}, // Empty = inactive
					},
				},
			},
			want: 0,
		},
		{
			name: "one sync-active project",
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
						Description: "Test project 1",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
					"project2": {
						Path:        "projects/web/project2",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Test project 2",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncInactive},
					},
				},
			},
			want: 1,
		},
		{
			name: "multiple sync-active projects",
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
						Description: "Test project 1",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
					"project2": {
						Path:        "projects/web/project2",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Test project 2",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
					"project3": {
						Path:        "projects/web/project3",
						Category:    registry.CategoryWeb,
						Lang:        registry.LanguageGo,
						Created:     "2024-01-01",
						Description: "Test project 3",
						Status:      registry.Status{State: registry.StateActive},
						Sync:        registry.ProjectSync{Status: registry.SyncInactive},
					},
				},
			},
			want: 2,
		},
		{
			name: "archived project with sync active is still returned",
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
						Description: "Test project 1",
						Status:      registry.Status{State: registry.StateArchived},
						Sync:        registry.ProjectSync{Status: registry.SyncActive},
					},
				},
			},
			want: 1, // Sync status is independent of archive status
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSyncActiveProjects(tt.registry)
			if len(got) != tt.want {
				t.Errorf("getSyncActiveProjects() returned %d projects, want %d", len(got), tt.want)
			}
		})
	}
}

func TestGetSyncActiveProjects_ReturnsCorrectProjects(t *testing.T) {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: map[string]registry.Project{
			"active-sync": {
				Path:        "projects/web/active-sync",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Active sync project",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"inactive-sync": {
				Path:        "projects/web/inactive-sync",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Inactive sync project",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
		},
	}

	got := getSyncActiveProjects(reg)

	// Should only contain active-sync
	if len(got) != 1 {
		t.Fatalf("expected 1 project, got %d", len(got))
	}

	if _, ok := got["active-sync"]; !ok {
		t.Error("expected 'active-sync' to be in results")
	}

	if _, ok := got["inactive-sync"]; ok {
		t.Error("'inactive-sync' should not be in results")
	}
}

func TestSyncResult(t *testing.T) {
	// Test that syncResult correctly tracks state
	result := syncResult{
		name:   "test-project",
		pulled: true,
		pushed: true,
	}

	if result.name != "test-project" {
		t.Errorf("expected name 'test-project', got '%s'", result.name)
	}

	if !result.pulled {
		t.Error("expected pulled to be true")
	}

	if !result.pushed {
		t.Error("expected pushed to be true")
	}

	if result.err != nil {
		t.Error("expected err to be nil")
	}

	if len(result.conflicts) != 0 {
		t.Error("expected no conflicts")
	}
}

func TestSyncFlags(t *testing.T) {
	// Test default flag values
	opts := syncFlags{}

	if opts.dryRun {
		t.Error("expected dryRun to default to false")
	}

	if opts.force {
		t.Error("expected force to default to false")
	}
}

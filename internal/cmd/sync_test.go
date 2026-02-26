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
			got := syncActiveProjects(tt.registry)
			if len(got) != tt.want {
				t.Errorf("syncActiveProjects() returned %d projects, want %d", len(got), tt.want)
			}
		})
	}
}

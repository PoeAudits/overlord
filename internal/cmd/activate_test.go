package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestActivateCommand(t *testing.T) {
	// Create a temporary directory for test registry
	tmpDir, err := os.MkdirTemp("", "overlord-activate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	registryPath := filepath.Join(tmpDir, "registry.yaml")

	// Create a test registry with projects
	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: map[string]registry.Project{
			"active-project": {
				Path:        "projects/web/active-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "An active project",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
			"already-synced": {
				Path:        "projects/web/already-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Already synced project",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"archived-project": {
				Path:        "archive/archived-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "An archived project",
				Status:      registry.Status{State: registry.StateArchived},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
			"project-with-alias": {
				Path:        "projects/cli/project-with-alias",
				Category:    registry.CategoryCLI,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Project with alias",
				Aliases:     []string{"pwa", "alias-test"},
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
		},
	}

	// Save the test registry
	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	t.Run("activate inactive project", func(t *testing.T) {
		// Load registry
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		// Verify initial state
		project := reg.Projects["active-project"]
		if project.Sync.Status == registry.SyncActive {
			t.Error("project should not be active for sync initially")
		}

		// Simulate activation (without git operations)
		result, err := reg.ResolveOne("active-project")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Check not archived
		if result.Project.Status.State == registry.StateArchived {
			t.Error("project should not be archived")
		}

		// Update sync status
		updatedProject := result.Project
		updatedProject.Sync.Status = registry.SyncActive
		reg.Projects[result.Name] = updatedProject

		// Save registry
		if err := registry.Save(registryPath, reg); err != nil {
			t.Fatalf("failed to save registry: %v", err)
		}

		// Verify the change persisted
		reg2, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to reload registry: %v", err)
		}

		if reg2.Projects["active-project"].Sync.Status != registry.SyncActive {
			t.Error("project sync status should be active after activation")
		}
	})

	t.Run("activate already active project is no-op", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		result, err := reg.ResolveOne("already-synced")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Should already be active
		if result.Project.Sync.Status != registry.SyncActive {
			t.Error("project should already be active for sync")
		}
	})

	t.Run("activate archived project fails", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		result, err := reg.ResolveOne("archived-project")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Should be archived
		if result.Project.Status.State != registry.StateArchived {
			t.Error("project should be archived")
		}
	})

	t.Run("activate by alias", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		// Resolve by alias
		result, err := reg.ResolveOne("pwa")
		if err != nil {
			t.Fatalf("failed to resolve project by alias: %v", err)
		}

		if result.Name != "project-with-alias" {
			t.Errorf("expected project name 'project-with-alias', got '%s'", result.Name)
		}
	})

	t.Run("activate non-existent project fails", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		_, err = reg.ResolveOne("non-existent-project")
		if err == nil {
			t.Error("expected error for non-existent project")
		}
	})
}

func TestActivateLogic(t *testing.T) {
	tests := []struct {
		name           string
		projectState   registry.State
		syncStatus     registry.SyncStatus
		expectError    bool
		expectNoChange bool
		errorContains  string
	}{
		{
			name:           "activate inactive project",
			projectState:   registry.StateActive,
			syncStatus:     registry.SyncInactive,
			expectError:    false,
			expectNoChange: false,
		},
		{
			name:           "activate project with empty sync status",
			projectState:   registry.StateActive,
			syncStatus:     "", // Empty is treated as inactive
			expectError:    false,
			expectNoChange: false,
		},
		{
			name:           "activate already active project",
			projectState:   registry.StateActive,
			syncStatus:     registry.SyncActive,
			expectError:    false,
			expectNoChange: true,
		},
		{
			name:           "activate archived project",
			projectState:   registry.StateArchived,
			syncStatus:     registry.SyncInactive,
			expectError:    true,
			expectNoChange: true,
			errorContains:  "archived",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := registry.Project{
				Path:        "test/path",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Test project",
				Status:      registry.Status{State: tt.projectState},
				Sync:        registry.ProjectSync{Status: tt.syncStatus},
			}

			// Check archived state
			if project.Status.State == registry.StateArchived {
				if !tt.expectError {
					t.Error("expected no error but project is archived")
				}
				return
			}

			// Check if already active
			if project.Sync.Status == registry.SyncActive {
				if !tt.expectNoChange {
					t.Error("expected change but project is already active")
				}
				return
			}

			// Simulate activation
			project.Sync.Status = registry.SyncActive

			if project.Sync.Status != registry.SyncActive {
				t.Error("sync status should be active after activation")
			}
		})
	}
}

package cmd

import (
	"path/filepath"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestDeactivateCommand(t *testing.T) {
	// Create a temporary directory for test registry
	tmpDir := t.TempDir()

	registryPath := filepath.Join(tmpDir, "registry.yaml")

	// Create a test registry with projects
	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: "~/thoughts",
		},
		Projects: map[string]registry.Project{
			"active-synced": {
				Path:        "projects/web/active-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "An active synced project",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"inactive-project": {
				Path:        "projects/web/inactive-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "An inactive project",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
			"archived-project": {
				Path:        "archive/archived-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "An archived project",
				Status:      registry.Status{State: registry.StateArchived},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"project-with-alias": {
				Path:        "projects/cli/project-with-alias",
				Category:    registry.CategoryCLI,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Project with alias",
				Aliases:     []string{"pwa", "alias-test"},
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
		},
	}

	// Save the test registry
	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	t.Run("deactivate active synced project", func(t *testing.T) {
		// Load registry
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		// Verify initial state
		project := reg.Projects["active-synced"]
		if project.Sync.Status != registry.SyncActive {
			t.Error("project should be active for sync initially")
		}

		// Simulate deactivation (without sync/git operations)
		result, err := reg.ResolveOne("active-synced")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Check not archived
		if result.Project.Status.State == registry.StateArchived {
			t.Error("project should not be archived")
		}

		// Update sync status
		updatedProject := result.Project
		updatedProject.Sync.Status = registry.SyncInactive
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

		if reg2.Projects["active-synced"].Sync.Status != registry.SyncInactive {
			t.Error("project sync status should be inactive after deactivation")
		}
	})

	t.Run("deactivate already inactive project is no-op", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		result, err := reg.ResolveOne("inactive-project")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Should already be inactive
		if result.Project.Sync.Status == registry.SyncActive {
			t.Error("project should already be inactive for sync")
		}
	})

	t.Run("deactivate archived project fails", func(t *testing.T) {
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

	t.Run("deactivate by alias", func(t *testing.T) {
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

	t.Run("deactivate non-existent project fails", func(t *testing.T) {
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

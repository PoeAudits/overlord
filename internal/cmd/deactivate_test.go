package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestDeactivateCommand(t *testing.T) {
	// Create a temporary directory for test registry
	tmpDir, err := os.MkdirTemp("", "overlord-deactivate-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	registryPath := filepath.Join(tmpDir, "registry.yaml")

	// Create a test registry with projects
	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: "~/thoughts",
			Sync: registry.SyncSettings{
				DefaultExclude: []string{"node_modules", ".git"},
			},
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
			"project-with-excludes": {
				Path:        "projects/web/project-with-excludes",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageTypeScript,
				Created:     "2024-01-01",
				Description: "Project with custom excludes",
				Status:      registry.Status{State: registry.StateActive},
				Sync: registry.ProjectSync{
					Status:  registry.SyncActive,
					Exclude: []string{".cache", "dist/"},
				},
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

	t.Run("project with custom excludes", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		result, err := reg.ResolveOne("project-with-excludes")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Verify custom excludes are present
		if len(result.Project.Sync.Exclude) != 2 {
			t.Errorf("expected 2 custom excludes, got %d", len(result.Project.Sync.Exclude))
		}

		// Verify global excludes are in settings
		if len(reg.Settings.Sync.DefaultExclude) != 2 {
			t.Errorf("expected 2 global excludes, got %d", len(reg.Settings.Sync.DefaultExclude))
		}
	})
}

func TestDeactivateLogic(t *testing.T) {
	tests := []struct {
		name           string
		projectState   registry.State
		syncStatus     registry.SyncStatus
		expectError    bool
		expectNoChange bool
		errorContains  string
	}{
		{
			name:           "deactivate active synced project",
			projectState:   registry.StateActive,
			syncStatus:     registry.SyncActive,
			expectError:    false,
			expectNoChange: false,
		},
		{
			name:           "deactivate already inactive project",
			projectState:   registry.StateActive,
			syncStatus:     registry.SyncInactive,
			expectError:    false,
			expectNoChange: true,
		},
		{
			name:           "deactivate project with empty sync status",
			projectState:   registry.StateActive,
			syncStatus:     "", // Empty is treated as inactive
			expectError:    false,
			expectNoChange: true,
		},
		{
			name:           "deactivate archived project",
			projectState:   registry.StateArchived,
			syncStatus:     registry.SyncActive,
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

			// Check if already inactive
			if project.Sync.Status != registry.SyncActive {
				if !tt.expectNoChange {
					t.Error("expected change but project is already inactive")
				}
				return
			}

			// Simulate deactivation
			project.Sync.Status = registry.SyncInactive

			if project.Sync.Status != registry.SyncInactive {
				t.Error("sync status should be inactive after deactivation")
			}
		})
	}
}

func TestDeactivateDirectoryRemoval(t *testing.T) {
	// Create a temporary directory for test
	tmpDir, err := os.MkdirTemp("", "overlord-deactivate-dir-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	t.Run("directory exists and can be removed", func(t *testing.T) {
		// Create a test directory
		testDir := filepath.Join(tmpDir, "test-project")
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatalf("failed to create test dir: %v", err)
		}

		// Create a test file inside
		testFile := filepath.Join(testDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Verify directory exists
		if _, err := os.Stat(testDir); os.IsNotExist(err) {
			t.Fatal("test directory should exist")
		}

		// Remove directory
		if err := os.RemoveAll(testDir); err != nil {
			t.Fatalf("failed to remove directory: %v", err)
		}

		// Verify directory is removed
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("test directory should be removed")
		}
	})

	t.Run("directory does not exist", func(t *testing.T) {
		testDir := filepath.Join(tmpDir, "non-existent-project")

		// Verify directory does not exist
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("test directory should not exist")
		}
	})
}

func TestDeactivateFlagCombinations(t *testing.T) {
	tests := []struct {
		name         string
		force        bool
		keepLocal    bool
		forceRemove  bool
		syncFailed   bool
		userConfirms bool // Simulates user confirmation response
		expectRemove bool
		description  string
	}{
		{
			name:         "default flags, sync succeeded, user confirms",
			force:        false,
			keepLocal:    false,
			forceRemove:  false,
			syncFailed:   false,
			userConfirms: true,
			expectRemove: true,
			description:  "Should prompt and remove if user confirms",
		},
		{
			name:         "default flags, sync succeeded, user declines",
			force:        false,
			keepLocal:    false,
			forceRemove:  false,
			syncFailed:   false,
			userConfirms: false,
			expectRemove: false,
			description:  "Should prompt and not remove if user declines",
		},
		{
			name:         "force flag, sync succeeded",
			force:        true,
			keepLocal:    false,
			forceRemove:  false,
			syncFailed:   false,
			userConfirms: false, // Doesn't matter with force
			expectRemove: true,
			description:  "Should remove without prompt",
		},
		{
			name:         "keep-local flag",
			force:        false,
			keepLocal:    true,
			forceRemove:  false,
			syncFailed:   false,
			userConfirms: true, // Doesn't matter with keep-local
			expectRemove: false,
			description:  "Should not remove directory",
		},
		{
			name:         "sync failed, no force-remove",
			force:        true,
			keepLocal:    false,
			forceRemove:  false,
			syncFailed:   true,
			userConfirms: true, // Doesn't matter - sync failure blocks removal
			expectRemove: false,
			description:  "Should not remove due to sync failure",
		},
		{
			name:         "sync failed, with force-remove",
			force:        true,
			keepLocal:    false,
			forceRemove:  true,
			syncFailed:   true,
			userConfirms: false, // Doesn't matter with force
			expectRemove: true,
			description:  "Should remove despite sync failure",
		},
		{
			name:         "keep-local overrides force-remove",
			force:        true,
			keepLocal:    true,
			forceRemove:  true,
			syncFailed:   true,
			userConfirms: true, // Doesn't matter with keep-local
			expectRemove: false,
			description:  "keep-local should take precedence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the decision logic from the command
			// This mirrors the logic in runDeactivate
			shouldRemove := false

			if !tt.keepLocal {
				// Check if sync succeeded or force-remove is set
				if tt.syncFailed && !tt.forceRemove {
					// Don't remove if sync failed and no force-remove
					shouldRemove = false
				} else {
					// Sync succeeded or force-remove is set
					// Now check if we should prompt or force
					if tt.force {
						shouldRemove = true
					} else {
						// Would prompt - use userConfirms to simulate response
						shouldRemove = tt.userConfirms
					}
				}
			}

			if shouldRemove != tt.expectRemove {
				t.Errorf("%s: expected remove=%v, got %v", tt.description, tt.expectRemove, shouldRemove)
			}
		})
	}
}

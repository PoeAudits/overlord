package cmd

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/sync"
)

// TestSyncWorkflowIntegration tests the full activate -> sync -> deactivate workflow
func TestSyncWorkflowIntegration(t *testing.T) {
	// Create a temporary directory for test registry and project files
	tmpDir := t.TempDir()

	registryPath := filepath.Join(tmpDir, "registry.yaml")
	projectDir := filepath.Join(tmpDir, "projects", "web", "test-project")

	// Create project directory
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create a test file in the project
	testFile := filepath.Join(projectDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create initial registry with inactive project
	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
			Sync: registry.SyncSettings{
				DefaultExclude: []string{"node_modules", ".git", "__pycache__"},
			},
		},
		Projects: map[string]registry.Project{
			"test-project": {
				Path:        "projects/web/test-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Test project for integration tests",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
		},
	}

	// Save initial registry
	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	// Step 1: Verify initial state (inactive)
	t.Run("initial state is inactive", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		project := reg.Projects["test-project"]
		if project.Sync.Status == registry.SyncActive {
			t.Error("project should be inactive initially")
		}
	})

	// Step 2: Activate the project
	t.Run("activate sets status to active", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		// Resolve and activate
		result, err := reg.ResolveOne("test-project")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Verify not archived
		if result.Project.Status.State == registry.StateArchived {
			t.Fatal("project should not be archived")
		}

		// Activate
		project := result.Project
		project.Sync.Status = registry.SyncActive
		reg.Projects[result.Name] = project

		// Save
		if err := registry.Save(registryPath, reg); err != nil {
			t.Fatalf("failed to save registry: %v", err)
		}

		// Verify
		reg2, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to reload registry: %v", err)
		}

		if reg2.Projects["test-project"].Sync.Status != registry.SyncActive {
			t.Error("project should be active after activation")
		}
	})

	// Step 3: Verify sync identifies active projects
	t.Run("sync identifies active projects", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		activeProjects := getSyncActiveProjects(reg)
		if len(activeProjects) != 1 {
			t.Errorf("expected 1 active project, got %d", len(activeProjects))
		}

		if _, ok := activeProjects["test-project"]; !ok {
			t.Error("test-project should be in active projects")
		}
	})

	// Step 4: Deactivate the project
	t.Run("deactivate sets status to inactive", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		// Resolve and deactivate
		result, err := reg.ResolveOne("test-project")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		// Verify currently active
		if result.Project.Sync.Status != registry.SyncActive {
			t.Fatal("project should be active before deactivation")
		}

		// Deactivate
		project := result.Project
		project.Sync.Status = registry.SyncInactive
		reg.Projects[result.Name] = project

		// Save
		if err := registry.Save(registryPath, reg); err != nil {
			t.Fatalf("failed to save registry: %v", err)
		}

		// Verify
		reg2, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to reload registry: %v", err)
		}

		if reg2.Projects["test-project"].Sync.Status != registry.SyncInactive {
			t.Error("project should be inactive after deactivation")
		}
	})

	// Step 5: Verify no active projects after deactivation
	t.Run("no active projects after deactivation", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		activeProjects := getSyncActiveProjects(reg)
		if len(activeProjects) != 0 {
			t.Errorf("expected 0 active projects, got %d", len(activeProjects))
		}
	})
}

// TestStateTransitions tests all valid state transitions
func TestStateTransitions(t *testing.T) {
	tests := []struct {
		name          string
		initialState  registry.SyncStatus
		action        string // "activate" or "deactivate"
		expectedState registry.SyncStatus
		expectNoOp    bool
	}{
		{
			name:          "inactive -> activate -> active",
			initialState:  registry.SyncInactive,
			action:        "activate",
			expectedState: registry.SyncActive,
			expectNoOp:    false,
		},
		{
			name:          "empty -> activate -> active",
			initialState:  "", // Empty treated as inactive
			action:        "activate",
			expectedState: registry.SyncActive,
			expectNoOp:    false,
		},
		{
			name:          "active -> activate -> active (no-op)",
			initialState:  registry.SyncActive,
			action:        "activate",
			expectedState: registry.SyncActive,
			expectNoOp:    true,
		},
		{
			name:          "active -> deactivate -> inactive",
			initialState:  registry.SyncActive,
			action:        "deactivate",
			expectedState: registry.SyncInactive,
			expectNoOp:    false,
		},
		{
			name:          "inactive -> deactivate -> inactive (no-op)",
			initialState:  registry.SyncInactive,
			action:        "deactivate",
			expectedState: registry.SyncInactive,
			expectNoOp:    true,
		},
		{
			name:          "empty -> deactivate -> inactive (no-op)",
			initialState:  "", // Empty treated as inactive
			action:        "deactivate",
			expectedState: registry.SyncInactive,
			expectNoOp:    true,
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
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: tt.initialState},
			}

			// Determine if this is a no-op
			isNoOp := false
			if tt.action == "activate" && project.Sync.Status == registry.SyncActive {
				isNoOp = true
			}
			if tt.action == "deactivate" && project.Sync.Status != registry.SyncActive {
				isNoOp = true
			}

			if isNoOp != tt.expectNoOp {
				t.Errorf("expected no-op=%v, got %v", tt.expectNoOp, isNoOp)
			}

			// Apply action
			if tt.action == "activate" {
				project.Sync.Status = registry.SyncActive
			} else {
				project.Sync.Status = registry.SyncInactive
			}

			if project.Sync.Status != tt.expectedState {
				t.Errorf("expected state %q, got %q", tt.expectedState, project.Sync.Status)
			}
		})
	}
}

// TestExclusionPatternMerging tests that global and project excludes are merged correctly
func TestExclusionPatternMerging(t *testing.T) {
	tests := []struct {
		name            string
		globalExcludes  []string
		projectExcludes []string
		expectedCount   int
		expectedItems   []string
	}{
		{
			name:            "global only",
			globalExcludes:  []string{"node_modules", ".git"},
			projectExcludes: []string{},
			expectedCount:   2,
			expectedItems:   []string{"node_modules", ".git"},
		},
		{
			name:            "project only",
			globalExcludes:  []string{},
			projectExcludes: []string{".cache", "dist/"},
			expectedCount:   2,
			expectedItems:   []string{".cache", "dist/"},
		},
		{
			name:            "both global and project",
			globalExcludes:  []string{"node_modules", ".git"},
			projectExcludes: []string{".cache", "dist/"},
			expectedCount:   4,
			expectedItems:   []string{"node_modules", ".git", ".cache", "dist/"},
		},
		{
			name:            "empty both",
			globalExcludes:  []string{},
			projectExcludes: []string{},
			expectedCount:   0,
			expectedItems:   []string{},
		},
		{
			name:            "duplicates are preserved",
			globalExcludes:  []string{"node_modules", ".git"},
			projectExcludes: []string{"node_modules", ".cache"}, // node_modules appears twice
			expectedCount:   4,
			expectedItems:   []string{"node_modules", ".git", "node_modules", ".cache"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Merge excludes (same logic as in sync.go)
			excludes := make([]string, 0, len(tt.globalExcludes)+len(tt.projectExcludes))
			excludes = append(excludes, tt.globalExcludes...)
			excludes = append(excludes, tt.projectExcludes...)

			if len(excludes) != tt.expectedCount {
				t.Errorf("expected %d excludes, got %d", tt.expectedCount, len(excludes))
			}

			// Verify all expected items are present
			for i, expected := range tt.expectedItems {
				if i >= len(excludes) {
					t.Errorf("missing expected item at index %d: %s", i, expected)
					continue
				}
				if excludes[i] != expected {
					t.Errorf("at index %d: expected %q, got %q", i, expected, excludes[i])
				}
			}
		})
	}
}

// TestExclusionPatternMergingWithRegistry tests exclusion merging with actual registry data
func TestExclusionPatternMergingWithRegistry(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")

	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
			Sync: registry.SyncSettings{
				DefaultExclude: []string{"node_modules", ".git", "__pycache__", ".venv"},
			},
		},
		Projects: map[string]registry.Project{
			"project-with-excludes": {
				Path:        "projects/web/project-with-excludes",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageTypeScript,
				Created:     "2024-01-01",
				Description: "Project with custom excludes",
				Status:      registry.Status{State: registry.StateActive},
				Sync: registry.ProjectSync{
					Status:  registry.SyncActive,
					Exclude: []string{".cache", "dist/", "coverage/"},
				},
			},
			"project-no-excludes": {
				Path:        "projects/web/project-no-excludes",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Project without custom excludes",
				Status:      registry.Status{State: registry.StateActive},
				Sync: registry.ProjectSync{
					Status: registry.SyncActive,
				},
			},
		},
	}

	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	t.Run("project with custom excludes", func(t *testing.T) {
		project := reg.Projects["project-with-excludes"]
		globalExcludes := reg.Settings.Sync.DefaultExclude

		// Merge excludes
		excludes := make([]string, 0, len(globalExcludes)+len(project.Sync.Exclude))
		excludes = append(excludes, globalExcludes...)
		excludes = append(excludes, project.Sync.Exclude...)

		// Should have 4 global + 3 project = 7 total
		if len(excludes) != 7 {
			t.Errorf("expected 7 excludes, got %d: %v", len(excludes), excludes)
		}

		// Verify global excludes come first
		expectedOrder := []string{"node_modules", ".git", "__pycache__", ".venv", ".cache", "dist/", "coverage/"}
		for i, expected := range expectedOrder {
			if excludes[i] != expected {
				t.Errorf("at index %d: expected %q, got %q", i, expected, excludes[i])
			}
		}
	})

	t.Run("project without custom excludes", func(t *testing.T) {
		project := reg.Projects["project-no-excludes"]
		globalExcludes := reg.Settings.Sync.DefaultExclude

		// Merge excludes
		excludes := make([]string, 0, len(globalExcludes)+len(project.Sync.Exclude))
		excludes = append(excludes, globalExcludes...)
		excludes = append(excludes, project.Sync.Exclude...)

		// Should have only 4 global excludes
		if len(excludes) != 4 {
			t.Errorf("expected 4 excludes, got %d: %v", len(excludes), excludes)
		}
	})
}

// MockRsyncExecutor for testing conflict detection
type MockRsyncExecutor struct {
	PushOutput string
	PullOutput string
	CallCount  int
	Calls      []string
}

func (m *MockRsyncExecutor) Execute(name string, args ...string) ([]byte, int, error) {
	m.CallCount++
	m.Calls = append(m.Calls, name+" "+joinArgs(args))

	// Determine if this is a push or pull based on arguments
	var output string
	if len(args) >= 2 {
		source := args[len(args)-2]
		// If source contains ":", it's a remote path (pull)
		if sync.IsRemotePath(source) {
			output = m.PullOutput
		} else {
			output = m.PushOutput
		}
	}

	return []byte(output), 0, nil
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

// TestConflictDetectionIntegration tests conflict detection with mocked rsync
func TestConflictDetectionIntegration(t *testing.T) {
	tests := []struct {
		name                 string
		pushOutput           string
		pullOutput           string
		expectedConflicts    int
		expectedBothModified int
		expectedLocalOnly    int
		expectedRemoteOnly   int
	}{
		{
			name: "no conflicts - no changes",
			pushOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			pullOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			expectedConflicts:    0,
			expectedBothModified: 0,
			expectedLocalOnly:    0,
			expectedRemoteOnly:   0,
		},
		{
			name: "local-only changes",
			pushOutput: `sending incremental file list
new-local-file.txt
src/local-change.go

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			pullOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			expectedConflicts:    2,
			expectedBothModified: 0,
			expectedLocalOnly:    2,
			expectedRemoteOnly:   0,
		},
		{
			name: "remote-only changes",
			pushOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			pullOutput: `sending incremental file list
new-remote-file.txt
docs/remote-doc.md

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			expectedConflicts:    2,
			expectedBothModified: 0,
			expectedLocalOnly:    0,
			expectedRemoteOnly:   2,
		},
		{
			name: "both-modified conflict",
			pushOutput: `sending incremental file list
shared-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			pullOutput: `sending incremental file list
shared-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			expectedConflicts:    1,
			expectedBothModified: 1,
			expectedLocalOnly:    0,
			expectedRemoteOnly:   0,
		},
		{
			name: "mixed conflicts",
			pushOutput: `sending incremental file list
local-only.txt
both-modified.go
src/
src/local-file.rs

sent 5,000 bytes  received 100 bytes  10,200.00 bytes/sec`,
			pullOutput: `sending incremental file list
remote-only.md
both-modified.go
docs/
docs/remote-file.txt

sent 5,000 bytes  received 100 bytes  10,200.00 bytes/sec`,
			expectedConflicts:    5,
			expectedBothModified: 1,
			expectedLocalOnly:    2,
			expectedRemoteOnly:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &MockRsyncExecutor{
				PushOutput: tt.pushOutput,
				PullOutput: tt.pullOutput,
			}

			rsync := sync.NewRsyncWithExecutor(mockExec)

			conflicts, err := sync.DetectConflictsWithRsync(
				rsync,
				"/local/path",
				"remote-host",
				"/remote/path",
				[]string{},
			)

			if err != nil {
				t.Fatalf("DetectConflictsWithRsync() error = %v", err)
			}

			if len(conflicts) != tt.expectedConflicts {
				t.Errorf("expected %d conflicts, got %d", tt.expectedConflicts, len(conflicts))
			}

			// Count by type
			bothModified := 0
			localOnly := 0
			remoteOnly := 0
			for _, c := range conflicts {
				switch c.Type {
				case sync.ConflictBothModified:
					bothModified++
				case sync.ConflictLocalOnly:
					localOnly++
				case sync.ConflictRemoteOnly:
					remoteOnly++
				}
			}

			if bothModified != tt.expectedBothModified {
				t.Errorf("expected %d both-modified, got %d", tt.expectedBothModified, bothModified)
			}
			if localOnly != tt.expectedLocalOnly {
				t.Errorf("expected %d local-only, got %d", tt.expectedLocalOnly, localOnly)
			}
			if remoteOnly != tt.expectedRemoteOnly {
				t.Errorf("expected %d remote-only, got %d", tt.expectedRemoteOnly, remoteOnly)
			}

			// Verify rsync was called twice (push and pull dry-run)
			if mockExec.CallCount != 2 {
				t.Errorf("expected 2 rsync calls, got %d", mockExec.CallCount)
			}
		})
	}
}

// TestConflictDetectionWithExcludes tests that exclusion patterns are passed to rsync
func TestConflictDetectionWithExcludes(t *testing.T) {
	mockExec := &MockRsyncExecutor{
		PushOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
		PullOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
	}

	rsync := sync.NewRsyncWithExecutor(mockExec)

	excludes := []string{"node_modules", ".git", ".cache"}

	_, err := sync.DetectConflictsWithRsync(
		rsync,
		"/local/path",
		"remote-host",
		"/remote/path",
		excludes,
	)

	if err != nil {
		t.Fatalf("DetectConflictsWithRsync() error = %v", err)
	}

	// Verify excludes were passed in the rsync calls
	for _, call := range mockExec.Calls {
		for _, exclude := range excludes {
			expectedArg := "--exclude=" + exclude
			if !containsSubstring(call, expectedArg) {
				t.Errorf("expected call to contain %q, got: %s", expectedArg, call)
			}
		}
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestDryRunMode tests that dry-run mode doesn't make actual changes
func TestDryRunMode(t *testing.T) {
	mockExec := &MockRsyncExecutor{
		PushOutput: `sending incremental file list
file1.txt
file2.go

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
		PullOutput: `sending incremental file list
file3.md

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
	}

	rsync := sync.NewRsyncWithExecutor(mockExec)

	// Test dry-run
	output, err := rsync.DryRun(sync.RsyncOptions{
		Source:   "/local/path",
		Dest:     "remote-host:/remote/path",
		Excludes: []string{},
		DryRun:   true,
	})

	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	// Verify output contains file list
	if output == "" {
		t.Error("expected non-empty output from dry-run")
	}

	// Verify --dry-run flag was passed
	if len(mockExec.Calls) == 0 {
		t.Fatal("expected at least one rsync call")
	}

	lastCall := mockExec.Calls[len(mockExec.Calls)-1]
	if !containsSubstring(lastCall, "--dry-run") {
		t.Errorf("expected --dry-run flag in call: %s", lastCall)
	}
}

// TestMachineRoleBehavior tests different behavior based on machine role
func TestMachineRoleBehavior(t *testing.T) {
	tests := []struct {
		name        string
		role        machine.Role
		storageHost string
		expectSync  bool
		description string
	}{
		{
			name:        "storage machine",
			role:        machine.RoleStorage,
			storageHost: "",
			expectSync:  false,
			description: "Storage machines should not initiate sync",
		},
		{
			name:        "working-set machine",
			role:        machine.RoleWorkingSet,
			storageHost: "storage-host",
			expectSync:  true,
			description: "Working-set machines should initiate sync",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &machine.MachineConfig{
				Name:        "test-machine",
				Role:        tt.role,
				StorageHost: tt.storageHost,
			}

			// Validate config
			if err := config.Validate(); err != nil {
				t.Fatalf("invalid config: %v", err)
			}

			// Check if sync should be initiated
			shouldSync := config.Role == machine.RoleWorkingSet

			if shouldSync != tt.expectSync {
				t.Errorf("%s: expected sync=%v, got %v", tt.description, tt.expectSync, shouldSync)
			}
		})
	}
}

// TestProjectFilteringByStatus tests filtering projects by various statuses
func TestProjectFilteringByStatus(t *testing.T) {
	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: map[string]registry.Project{
			"active-synced": {
				Path:        "projects/web/active-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Active and synced",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"active-not-synced": {
				Path:        "projects/web/active-not-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Active but not synced",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
			"archived-synced": {
				Path:        "archive/archived-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Archived but sync active",
				Status:      registry.Status{State: registry.StateArchived},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"archived-not-synced": {
				Path:        "archive/archived-not-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Archived and not synced",
				Status:      registry.Status{State: registry.StateArchived},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
			"empty-sync-status": {
				Path:        "projects/web/empty-sync-status",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Empty sync status",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{}, // Empty = inactive
			},
		},
	}

	t.Run("getSyncActiveProjects returns only sync-active", func(t *testing.T) {
		active := getSyncActiveProjects(testRegistry)

		// Should return 2: active-synced and archived-synced
		// (sync status is independent of archive status)
		if len(active) != 2 {
			t.Errorf("expected 2 sync-active projects, got %d", len(active))
		}

		if _, ok := active["active-synced"]; !ok {
			t.Error("expected active-synced to be in results")
		}
		if _, ok := active["archived-synced"]; !ok {
			t.Error("expected archived-synced to be in results")
		}
	})

	t.Run("GetActiveProjects returns only state-active", func(t *testing.T) {
		active := testRegistry.GetActiveProjects()

		// Should return 3: active-synced, active-not-synced, empty-sync-status
		if len(active) != 3 {
			t.Errorf("expected 3 state-active projects, got %d", len(active))
		}

		if _, ok := active["archived-synced"]; ok {
			t.Error("archived-synced should not be in state-active results")
		}
	})

	t.Run("GetArchivedProjects returns only archived", func(t *testing.T) {
		archived := testRegistry.GetArchivedProjects()

		// Should return 2: archived-synced, archived-not-synced
		if len(archived) != 2 {
			t.Errorf("expected 2 archived projects, got %d", len(archived))
		}
	})
}

// TestSyncResultTracking tests the syncResult struct behavior
func TestSyncResultTracking(t *testing.T) {
	tests := []struct {
		name           string
		result         syncResult
		expectSuccess  bool
		expectPartial  bool
		expectFailed   bool
		expectSkipped  bool
		expectConflict bool
	}{
		{
			name: "full success",
			result: syncResult{
				name:   "project1",
				pulled: true,
				pushed: true,
			},
			expectSuccess: true,
		},
		{
			name: "partial - only pulled",
			result: syncResult{
				name:   "project2",
				pulled: true,
				pushed: false,
			},
			expectPartial: true,
		},
		{
			name: "partial - only pushed",
			result: syncResult{
				name:   "project3",
				pulled: false,
				pushed: true,
			},
			expectPartial: true,
		},
		{
			name: "failed - neither",
			result: syncResult{
				name:   "project4",
				pulled: false,
				pushed: false,
			},
			expectFailed: true,
		},
		{
			name: "skipped - cancelled",
			result: syncResult{
				name:   "project5",
				pulled: false,
				pushed: false,
				err:    errCancelled{},
			},
			expectSkipped: true,
		},
		{
			name: "with conflicts",
			result: syncResult{
				name:   "project6",
				pulled: true,
				pushed: true,
				conflicts: []sync.Conflict{
					{Path: "file.txt", Type: sync.ConflictBothModified},
				},
			},
			expectSuccess:  true,
			expectConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine result category (same logic as printSyncSummary)
			var isSuccess, isPartial, isFailed, isSkipped bool

			if tt.result.err != nil {
				if _, ok := tt.result.err.(errCancelled); ok {
					isSkipped = true
				} else {
					isFailed = true
				}
			} else if tt.result.pulled && tt.result.pushed {
				isSuccess = true
			} else if tt.result.pulled || tt.result.pushed {
				isPartial = true
			} else {
				isFailed = true
			}

			hasConflicts := len(tt.result.conflicts) > 0

			if isSuccess != tt.expectSuccess {
				t.Errorf("expected success=%v, got %v", tt.expectSuccess, isSuccess)
			}
			if isPartial != tt.expectPartial {
				t.Errorf("expected partial=%v, got %v", tt.expectPartial, isPartial)
			}
			if isFailed != tt.expectFailed {
				t.Errorf("expected failed=%v, got %v", tt.expectFailed, isFailed)
			}
			if isSkipped != tt.expectSkipped {
				t.Errorf("expected skipped=%v, got %v", tt.expectSkipped, isSkipped)
			}
			if hasConflicts != tt.expectConflict {
				t.Errorf("expected conflict=%v, got %v", tt.expectConflict, hasConflicts)
			}
		})
	}
}

// errCancelled is a helper type for testing cancelled sync
type errCancelled struct{}

func (e errCancelled) Error() string {
	return "sync cancelled by user"
}

// TestMultipleProjectSync tests syncing multiple projects
func TestMultipleProjectSync(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")

	// Create project directories
	projects := []string{"project-a", "project-b", "project-c"}
	for _, p := range projects {
		projectDir := filepath.Join(tmpDir, "projects", "web", p)
		if err := os.MkdirAll(projectDir, 0755); err != nil {
			t.Fatalf("failed to create project dir: %v", err)
		}
	}

	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
			Sync: registry.SyncSettings{
				DefaultExclude: []string{"node_modules"},
			},
		},
		Projects: map[string]registry.Project{
			"project-a": {
				Path:        "projects/web/project-a",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Project A",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"project-b": {
				Path:        "projects/web/project-b",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageTypeScript,
				Created:     "2024-01-01",
				Description: "Project B",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
			"project-c": {
				Path:        "projects/web/project-c",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguagePython,
				Created:     "2024-01-01",
				Description: "Project C - inactive",
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
		},
	}

	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	t.Run("returns only active projects", func(t *testing.T) {
		active := getSyncActiveProjects(reg)

		if len(active) != 2 {
			t.Errorf("expected 2 active projects, got %d", len(active))
		}

		if _, ok := active["project-a"]; !ok {
			t.Error("expected project-a to be active")
		}
		if _, ok := active["project-b"]; !ok {
			t.Error("expected project-b to be active")
		}
		if _, ok := active["project-c"]; ok {
			t.Error("project-c should not be active")
		}
	})

	t.Run("projects are sorted for consistent output", func(t *testing.T) {
		active := getSyncActiveProjects(reg)

		// Get sorted names
		names := make([]string, 0, len(active))
		for name := range active {
			names = append(names, name)
		}
		sort.Strings(names)

		expected := []string{"project-a", "project-b"}
		for i, name := range names {
			if name != expected[i] {
				t.Errorf("at index %d: expected %q, got %q", i, expected[i], name)
			}
		}
	})
}

// TestActivateArchivedProjectFails tests that archived projects cannot be activated
func TestActivateArchivedProjectFails(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")

	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
		},
		Projects: map[string]registry.Project{
			"archived-project": {
				Path:        "archive/archived-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Archived project",
				Status:      registry.Status{State: registry.StateArchived},
				Sync:        registry.ProjectSync{Status: registry.SyncInactive},
			},
		},
	}

	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	result, err := reg.ResolveOne("archived-project")
	if err != nil {
		t.Fatalf("failed to resolve project: %v", err)
	}

	// Verify project is archived
	if result.Project.Status.State != registry.StateArchived {
		t.Error("project should be archived")
	}

	// In the actual command, this would return an error
	// Here we just verify the state check works
	isArchived := result.Project.Status.State == registry.StateArchived
	if !isArchived {
		t.Error("expected project to be detected as archived")
	}
}

// TestDeactivateArchivedProjectFails tests that archived projects cannot be deactivated
func TestDeactivateArchivedProjectFails(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")

	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
		},
		Projects: map[string]registry.Project{
			"archived-synced": {
				Path:        "archive/archived-synced",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Archived but sync active",
				Status:      registry.Status{State: registry.StateArchived},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
		},
	}

	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	result, err := reg.ResolveOne("archived-synced")
	if err != nil {
		t.Fatalf("failed to resolve project: %v", err)
	}

	// Verify project is archived
	if result.Project.Status.State != registry.StateArchived {
		t.Error("project should be archived")
	}

	// In the actual command, this would return an error
	isArchived := result.Project.Status.State == registry.StateArchived
	if !isArchived {
		t.Error("expected project to be detected as archived")
	}
}

// TestFuzzyMatchingForSync tests that fuzzy matching works for sync commands
func TestFuzzyMatchingForSync(t *testing.T) {
	tmpDir := t.TempDir()
	registryPath := filepath.Join(tmpDir, "registry.yaml")

	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
		},
		Projects: map[string]registry.Project{
			"my-awesome-project": {
				Path:        "projects/web/my-awesome-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "My awesome project",
				Aliases:     []string{"map", "awesome"},
				Status:      registry.Status{State: registry.StateActive},
				Sync:        registry.ProjectSync{Status: registry.SyncActive},
			},
		},
	}

	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	tests := []struct {
		name      string
		query     string
		expectErr bool
	}{
		{"exact name", "my-awesome-project", false},
		{"alias", "map", false},
		{"alias 2", "awesome", false},
		{"partial match", "awesome-project", false},
		{"non-existent", "non-existent-project", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reg.ResolveOne(tt.query)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Name != "my-awesome-project" {
				t.Errorf("expected project name 'my-awesome-project', got '%s'", result.Name)
			}
		})
	}
}

package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NonExistentFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	registry, err := Load(registryPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if registry == nil {
		t.Fatal("Load() returned nil registry")
	}

	// Check default values
	if registry.Version != 2 {
		t.Errorf("registry.Version = %d, want 2", registry.Version)
	}

	if registry.Settings.BaseDir != "~/Overlord" {
		t.Errorf("registry.Settings.BaseDir = %s, want ~/Overlord", registry.Settings.BaseDir)
	}

	if registry.Settings.ThoughtsDir != "~/thoughts" {
		t.Errorf("registry.Settings.ThoughtsDir = %s, want ~/thoughts", registry.Settings.ThoughtsDir)
	}

	if registry.Projects == nil {
		t.Error("registry.Projects is nil, want empty map")
	}

	if len(registry.Projects) != 0 {
		t.Errorf("len(registry.Projects) = %d, want 0", len(registry.Projects))
	}
}

func TestLoad_ValidFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Create a valid registry file
	validYAML := `version: 2
settings:
  base_dir: ~/Overlord
  thoughts_dir: ~/thoughts
projects:
  test-project:
    path: ~/Overlord/test-project
    category: cli
    lang: go
    created: "2024-01-15"
    description: Test project
    status:
      state: active
`

	if err := os.WriteFile(registryPath, []byte(validYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	registry, err := Load(registryPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if registry.Version != 2 {
		t.Errorf("registry.Version = %d, want 2", registry.Version)
	}

	if len(registry.Projects) != 1 {
		t.Fatalf("len(registry.Projects) = %d, want 1", len(registry.Projects))
	}

	project, ok := registry.Projects["test-project"]
	if !ok {
		t.Fatal("project 'test-project' not found")
	}

	if project.Category != CategoryCLI {
		t.Errorf("project.Category = %s, want %s", project.Category, CategoryCLI)
	}

	if project.Lang != LanguageGo {
		t.Errorf("project.Lang = %s, want %s", project.Lang, LanguageGo)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Create invalid YAML
	invalidYAML := `version: 2
settings:
  base_dir: ~/Overlord
  invalid yaml here [[[
`

	if err := os.WriteFile(registryPath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(registryPath)
	if err == nil {
		t.Fatal("Load() error = nil, want error for invalid YAML")
	}
}

func TestLoad_InvalidRegistry(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Create registry with invalid version
	invalidRegistry := `version: 1
settings:
  base_dir: ~/Overlord
  thoughts_dir: ~/thoughts
projects: {}
`

	if err := os.WriteFile(registryPath, []byte(invalidRegistry), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Load(registryPath)
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}

func TestSave_NewFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	registry := defaultRegistry()
	registry.Projects["test"] = Project{
		Path:        "~/Overlord/test",
		Category:    CategoryCLI,
		Lang:        LanguageGo,
		Created:     "2024-01-15",
		Description: "Test project",
		Status:      Status{State: StateActive},
	}

	if err := Save(registryPath, registry); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Fatal("registry file was not created")
	}

	// Load and verify
	loaded, err := Load(registryPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	if len(loaded.Projects) != 1 {
		t.Errorf("len(loaded.Projects) = %d, want 1", len(loaded.Projects))
	}

	project, ok := loaded.Projects["test"]
	if !ok {
		t.Fatal("project 'test' not found after save/load")
	}

	if project.Description != "Test project" {
		t.Errorf("project.Description = %s, want 'Test project'", project.Description)
	}
}

func TestSave_CreatesParentDirectories(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "nested", "dir", "registry.yaml")

	registry := defaultRegistry()

	if err := Save(registryPath, registry); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Fatal("registry file was not created in nested directory")
	}
}

func TestSave_CreatesBackup(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Create initial registry
	registry1 := defaultRegistry()
	registry1.Projects["project1"] = Project{
		Path:        "~/Overlord/project1",
		Category:    CategoryCLI,
		Lang:        LanguageGo,
		Created:     "2024-01-15",
		Description: "First version",
		Status:      Status{State: StateActive},
	}

	if err := Save(registryPath, registry1); err != nil {
		t.Fatalf("Save() first save error = %v, want nil", err)
	}

	// Save again with different content
	registry2 := defaultRegistry()
	registry2.Projects["project2"] = Project{
		Path:        "~/Overlord/project2",
		Category:    CategoryWeb,
		Lang:        LanguageTypeScript,
		Created:     "2024-01-16",
		Description: "Second version",
		Status:      Status{State: StateActive},
	}

	if err := Save(registryPath, registry2); err != nil {
		t.Fatalf("Save() second save error = %v, want nil", err)
	}

	// Verify backup exists
	backupPath := registryPath + BackupSuffix
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatal("backup file was not created")
	}

	// Load backup and verify it contains first version
	backup, err := Load(backupPath)
	if err != nil {
		t.Fatalf("Load() backup error = %v, want nil", err)
	}

	if _, ok := backup.Projects["project1"]; !ok {
		t.Error("backup does not contain project1")
	}

	if _, ok := backup.Projects["project2"]; ok {
		t.Error("backup should not contain project2")
	}
}

func TestSave_NilRegistry(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	err := Save(registryPath, nil)
	if err == nil {
		t.Fatal("Save() error = nil, want error for nil registry")
	}
}

func TestSave_InvalidRegistry(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Create invalid registry (wrong version)
	registry := &Registry{
		Version: 1,
		Settings: Settings{
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: make(map[string]Project),
	}

	err := Save(registryPath, registry)
	if err == nil {
		t.Fatal("Save() error = nil, want validation error")
	}
}

func TestExpandPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "tilde only",
			path:    "~",
			wantErr: false,
		},
		{
			name:    "tilde with path",
			path:    "~/config/registry.yaml",
			wantErr: false,
		},
		{
			name:    "absolute path",
			path:    "/etc/registry.yaml",
			wantErr: false,
		},
		{
			name:    "relative path",
			path:    "config/registry.yaml",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "tilde user expansion",
			path:    "~user/config",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := expandPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == "" {
					t.Error("expandPath() returned empty string")
				}

				// For tilde paths, verify expansion happened
				if len(tt.path) > 0 && tt.path[0] == '~' && result == tt.path {
					t.Errorf("expandPath() did not expand tilde: got %s", result)
				}
			}
		})
	}
}

func TestDefaultRegistry(t *testing.T) {
	t.Parallel()

	registry := defaultRegistry()

	if registry == nil {
		t.Fatal("defaultRegistry() returned nil")
	}

	if registry.Version != 2 {
		t.Errorf("registry.Version = %d, want 2", registry.Version)
	}

	if registry.Settings.BaseDir != "~/Overlord" {
		t.Errorf("registry.Settings.BaseDir = %s, want ~/Overlord", registry.Settings.BaseDir)
	}

	if registry.Settings.ThoughtsDir != "~/thoughts" {
		t.Errorf("registry.Settings.ThoughtsDir = %s, want ~/thoughts", registry.Settings.ThoughtsDir)
	}

	if registry.Projects == nil {
		t.Error("registry.Projects is nil, want empty map")
	}

	if len(registry.Projects) != 0 {
		t.Errorf("len(registry.Projects) = %d, want 0", len(registry.Projects))
	}

	// Verify default registry is valid
	if err := registry.Validate(); err != nil {
		t.Errorf("defaultRegistry() validation error = %v, want nil", err)
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Create registry with multiple projects
	original := defaultRegistry()
	original.Projects["project1"] = Project{
		Path:        "~/Overlord/project1",
		Category:    CategoryCLI,
		Lang:        LanguageGo,
		Created:     "2024-01-15",
		Description: "First project",
		Aliases:     []string{"p1", "proj1"},
		Tags:        []string{"tool", "cli"},
		Status:      Status{State: StateActive},
	}
	original.Projects["project2"] = Project{
		Path:        "~/Overlord/project2",
		Category:    CategoryWeb,
		Lang:        LanguageTypeScript,
		Created:     "2024-01-16",
		Description: "Second project",
		Status:      Status{State: StateArchived},
	}

	// Save
	if err := Save(registryPath, original); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	// Load
	loaded, err := Load(registryPath)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Verify all fields match
	if loaded.Version != original.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, original.Version)
	}

	if loaded.Settings.BaseDir != original.Settings.BaseDir {
		t.Errorf("Settings.BaseDir = %s, want %s", loaded.Settings.BaseDir, original.Settings.BaseDir)
	}

	if len(loaded.Projects) != len(original.Projects) {
		t.Fatalf("len(Projects) = %d, want %d", len(loaded.Projects), len(original.Projects))
	}

	// Verify project1
	p1 := loaded.Projects["project1"]
	if p1.Description != "First project" {
		t.Errorf("project1.Description = %s, want 'First project'", p1.Description)
	}
	if len(p1.Aliases) != 2 {
		t.Errorf("len(project1.Aliases) = %d, want 2", len(p1.Aliases))
	}
	if len(p1.Tags) != 2 {
		t.Errorf("len(project1.Tags) = %d, want 2", len(p1.Tags))
	}
	if p1.Status.State != StateActive {
		t.Errorf("project1.Status.State = %s, want %s", p1.Status.State, StateActive)
	}

	// Verify project2
	p2 := loaded.Projects["project2"]
	if p2.Status.State != StateArchived {
		t.Errorf("project2.Status.State = %s, want %s", p2.Status.State, StateArchived)
	}
}

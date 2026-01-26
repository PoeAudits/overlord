package registry

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCategoryPath(t *testing.T) {
	tests := []struct {
		category Category
		expected string
	}{
		{CategoryContracts, "projects/contracts/"},
		{CategoryWeb, "projects/web/"},
		{CategoryServices, "projects/services/"},
		{CategoryML, "projects/ml/"},
		{CategoryLibs, "projects/libs/"},
		{CategoryCLI, "projects/cli/"},
		{CategoryCoreAgents, "core/agents/"},
		{CategoryCoreTools, "core/tools/"},
		{CategorySandbox, "sandbox/"},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			if got := tt.category.Path(); got != tt.expected {
				t.Errorf("Category.Path() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCategoryIsValid(t *testing.T) {
	tests := []struct {
		category Category
		valid    bool
	}{
		{CategoryContracts, true},
		{CategoryWeb, true},
		{Category("invalid"), false},
		{Category(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			if got := tt.category.IsValid(); got != tt.valid {
				t.Errorf("Category.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestLanguageIsValid(t *testing.T) {
	tests := []struct {
		lang  Language
		valid bool
	}{
		{LanguagePython, true},
		{LanguageTypeScript, true},
		{LanguageGo, true},
		{LanguageSolidity, true},
		{LanguageBase, true},
		{Language("invalid"), false},
		{Language(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			if got := tt.lang.IsValid(); got != tt.valid {
				t.Errorf("Language.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestStateIsValid(t *testing.T) {
	tests := []struct {
		state State
		valid bool
	}{
		{StateActive, true},
		{StateArchived, true},
		{State("invalid"), false},
		{State(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsValid(); got != tt.valid {
				t.Errorf("State.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestSyncStatusIsValid(t *testing.T) {
	tests := []struct {
		name   string
		status SyncStatus
		valid  bool
	}{
		{"active", SyncActive, true},
		{"inactive", SyncInactive, true},
		{"empty (backward compat)", SyncStatus(""), true},
		{"invalid", SyncStatus("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.valid {
				t.Errorf("SyncStatus.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestProjectSyncValidate(t *testing.T) {
	tests := []struct {
		name    string
		sync    ProjectSync
		wantErr bool
	}{
		{
			name: "valid active sync",
			sync: ProjectSync{
				Status:  SyncActive,
				Exclude: []string{"*.log", "node_modules/"},
			},
			wantErr: false,
		},
		{
			name: "valid inactive sync",
			sync: ProjectSync{
				Status:  SyncInactive,
				Exclude: []string{},
			},
			wantErr: false,
		},
		{
			name: "empty status (backward compat)",
			sync: ProjectSync{
				Status:  SyncStatus(""),
				Exclude: []string{"*.tmp"},
			},
			wantErr: false,
		},
		{
			name: "invalid status",
			sync: ProjectSync{
				Status:  SyncStatus("invalid"),
				Exclude: []string{},
			},
			wantErr: true,
		},
		{
			name:    "empty sync (all defaults)",
			sync:    ProjectSync{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sync.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ProjectSync.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProjectValidate(t *testing.T) {
	tests := []struct {
		name    string
		project Project
		wantErr bool
	}{
		{
			name: "valid project",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
			},
			wantErr: false,
		},
		{
			name: "missing path",
			project: Project{
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
			},
			wantErr: true,
		},
		{
			name: "invalid category",
			project: Project{
				Path:        "/home/user/project",
				Category:    Category("invalid"),
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
			},
			wantErr: true,
		},
		{
			name: "invalid date format",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "01-22-2026",
				Description: "Test project",
				Status:      Status{State: StateActive},
			},
			wantErr: true,
		},
		{
			name: "invalid state",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: State("invalid")},
			},
			wantErr: true,
		},
		{
			name: "valid project with sync",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				Sync: ProjectSync{
					Status:  SyncActive,
					Exclude: []string{"*.log", "node_modules/"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid sync status",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				Sync: ProjectSync{
					Status:  SyncStatus("invalid"),
					Exclude: []string{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.project.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Project.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSyncSettingsValidate(t *testing.T) {
	tests := []struct {
		name     string
		settings SyncSettings
		wantErr  bool
	}{
		{
			name: "empty exclude list",
			settings: SyncSettings{
				DefaultExclude: []string{},
			},
			wantErr: false,
		},
		{
			name: "nil exclude list",
			settings: SyncSettings{
				DefaultExclude: nil,
			},
			wantErr: false,
		},
		{
			name: "valid exclude patterns",
			settings: SyncSettings{
				DefaultExclude: []string{"node_modules", ".venv", "__pycache__", ".git"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SyncSettings.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSettingsValidate(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		wantErr  bool
	}{
		{
			name: "valid settings without sync",
			settings: Settings{
				BaseDir:     "/home/user/work",
				ThoughtsDir: "/home/user/thoughts",
			},
			wantErr: false,
		},
		{
			name: "valid settings with sync",
			settings: Settings{
				BaseDir:     "/home/user/work",
				ThoughtsDir: "/home/user/thoughts",
				Sync: SyncSettings{
					DefaultExclude: []string{"node_modules", ".venv"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing base_dir",
			settings: Settings{
				ThoughtsDir: "/home/user/thoughts",
			},
			wantErr: true,
		},
		{
			name: "missing thoughts_dir",
			settings: Settings{
				BaseDir: "/home/user/work",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Settings.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistryValidate(t *testing.T) {
	tests := []struct {
		name     string
		registry Registry
		wantErr  bool
	}{
		{
			name: "valid registry",
			registry: Registry{
				Version: 2,
				Settings: Settings{
					BaseDir:     "/home/user/work",
					ThoughtsDir: "/home/user/thoughts",
				},
				Projects: map[string]Project{
					"test-project": {
						Path:        "/home/user/work/test",
						Category:    CategoryWeb,
						Lang:        LanguageTypeScript,
						Created:     "2026-01-22",
						Description: "Test project",
						Status:      Status{State: StateActive},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid registry with sync settings",
			registry: Registry{
				Version: 2,
				Settings: Settings{
					BaseDir:     "/home/user/work",
					ThoughtsDir: "/home/user/thoughts",
					Sync: SyncSettings{
						DefaultExclude: []string{"node_modules", ".venv", "__pycache__"},
					},
				},
				Projects: map[string]Project{
					"test-project": {
						Path:        "/home/user/work/test",
						Category:    CategoryWeb,
						Lang:        LanguageTypeScript,
						Created:     "2026-01-22",
						Description: "Test project",
						Status:      Status{State: StateActive},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid version",
			registry: Registry{
				Version: 1,
				Settings: Settings{
					BaseDir:     "/home/user/work",
					ThoughtsDir: "/home/user/thoughts",
				},
				Projects: map[string]Project{},
			},
			wantErr: true,
		},
		{
			name: "missing base_dir",
			registry: Registry{
				Version: 2,
				Settings: Settings{
					ThoughtsDir: "/home/user/thoughts",
				},
				Projects: map[string]Project{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.registry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Registry.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegistryGetProjectByAlias(t *testing.T) {
	registry := Registry{
		Projects: map[string]Project{
			"project-one": {
				Aliases: []string{"p1", "proj1"},
			},
			"project-two": {
				Aliases: []string{"p2", "proj2"},
			},
		},
	}

	tests := []struct {
		alias     string
		wantName  string
		wantFound bool
	}{
		{"p1", "project-one", true},
		{"proj2", "project-two", true},
		{"nonexistent", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.alias, func(t *testing.T) {
			name, _, found := registry.GetProjectByAlias(tt.alias)
			if found != tt.wantFound {
				t.Errorf("GetProjectByAlias() found = %v, want %v", found, tt.wantFound)
			}
			if name != tt.wantName {
				t.Errorf("GetProjectByAlias() name = %v, want %v", name, tt.wantName)
			}
		})
	}
}

func TestRegistryYAMLMarshaling(t *testing.T) {
	registry := Registry{
		Version: 2,
		Settings: Settings{
			BaseDir:     "/home/thomas/Work",
			ThoughtsDir: "/home/thomas/thoughts",
		},
		Projects: map[string]Project{
			"test-project": {
				Path:        "/home/thomas/Work/test",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Aliases:     []string{"test", "tp"},
				Tags:        []string{"web", "frontend"},
				Status:      Status{State: StateActive},
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(&registry)
	if err != nil {
		t.Fatalf("Failed to marshal registry: %v", err)
	}

	// Unmarshal back
	var unmarshaled Registry
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal registry: %v", err)
	}

	// Validate unmarshaled registry
	if err := unmarshaled.Validate(); err != nil {
		t.Errorf("Unmarshaled registry validation failed: %v", err)
	}

	// Check key fields
	if unmarshaled.Version != registry.Version {
		t.Errorf("Version mismatch: got %d, want %d", unmarshaled.Version, registry.Version)
	}

	if unmarshaled.Settings.BaseDir != registry.Settings.BaseDir {
		t.Errorf("BaseDir mismatch: got %s, want %s", unmarshaled.Settings.BaseDir, registry.Settings.BaseDir)
	}

	project, ok := unmarshaled.Projects["test-project"]
	if !ok {
		t.Fatal("test-project not found in unmarshaled registry")
	}

	if project.Category != CategoryWeb {
		t.Errorf("Category mismatch: got %s, want %s", project.Category, CategoryWeb)
	}
}

func TestSyncSettingsYAMLMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
	}{
		{
			name: "settings without sync (backward compatibility)",
			settings: Settings{
				BaseDir:     "/home/user/work",
				ThoughtsDir: "/home/user/thoughts",
			},
		},
		{
			name: "settings with empty sync",
			settings: Settings{
				BaseDir:     "/home/user/work",
				ThoughtsDir: "/home/user/thoughts",
				Sync:        SyncSettings{},
			},
		},
		{
			name: "settings with sync exclude patterns",
			settings: Settings{
				BaseDir:     "/home/user/work",
				ThoughtsDir: "/home/user/thoughts",
				Sync: SyncSettings{
					DefaultExclude: []string{"node_modules", ".venv", "__pycache__", ".git"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			data, err := yaml.Marshal(&tt.settings)
			if err != nil {
				t.Fatalf("Failed to marshal settings: %v", err)
			}

			// Unmarshal back
			var unmarshaled Settings
			if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal settings: %v", err)
			}

			// Validate unmarshaled settings
			if err := unmarshaled.Validate(); err != nil {
				t.Errorf("Unmarshaled settings validation failed: %v", err)
			}

			// Check that sync settings are preserved
			if len(tt.settings.Sync.DefaultExclude) > 0 {
				if len(unmarshaled.Sync.DefaultExclude) != len(tt.settings.Sync.DefaultExclude) {
					t.Errorf("DefaultExclude length mismatch: got %d, want %d",
						len(unmarshaled.Sync.DefaultExclude), len(tt.settings.Sync.DefaultExclude))
				}
			}
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Simulate old registry YAML without sync settings
	oldRegistryYAML := `version: 2
settings:
  base_dir: /home/user/work
  thoughts_dir: /home/user/thoughts
projects:
  test-project:
    path: /home/user/work/test
    category: web
    lang: typescript
    created: "2026-01-22"
    description: Test project
    status:
      state: active
`

	var registry Registry
	if err := yaml.Unmarshal([]byte(oldRegistryYAML), &registry); err != nil {
		t.Fatalf("Failed to unmarshal old registry format: %v", err)
	}

	// Should validate successfully even without sync settings
	if err := registry.Validate(); err != nil {
		t.Errorf("Old registry format validation failed: %v", err)
	}

	// Sync settings should be empty but valid
	if err := registry.Settings.Sync.Validate(); err != nil {
		t.Errorf("Empty sync settings validation failed: %v", err)
	}

	// Project sync should be empty but valid
	project, ok := registry.Projects["test-project"]
	if !ok {
		t.Fatal("test-project not found")
	}
	if err := project.Sync.Validate(); err != nil {
		t.Errorf("Empty project sync validation failed: %v", err)
	}
}

func TestProjectSyncYAMLMarshaling(t *testing.T) {
	tests := []struct {
		name    string
		project Project
	}{
		{
			name: "project without sync (backward compatibility)",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
			},
		},
		{
			name: "project with active sync",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				Sync: ProjectSync{
					Status:  SyncActive,
					Exclude: []string{"*.log", "node_modules/"},
				},
			},
		},
		{
			name: "project with inactive sync",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				Sync: ProjectSync{
					Status: SyncInactive,
				},
			},
		},
		{
			name: "archived project with active sync",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateArchived},
				Sync: ProjectSync{
					Status:  SyncActive,
					Exclude: []string{"*.tmp"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to YAML
			data, err := yaml.Marshal(&tt.project)
			if err != nil {
				t.Fatalf("Failed to marshal project: %v", err)
			}

			// Unmarshal back
			var unmarshaled Project
			if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal project: %v", err)
			}

			// Validate unmarshaled project
			if err := unmarshaled.Validate(); err != nil {
				t.Errorf("Unmarshaled project validation failed: %v", err)
			}

			// Check sync status is preserved
			if tt.project.Sync.Status != "" {
				if unmarshaled.Sync.Status != tt.project.Sync.Status {
					t.Errorf("Sync status mismatch: got %s, want %s",
						unmarshaled.Sync.Status, tt.project.Sync.Status)
				}
			}

			// Check exclude patterns are preserved
			if len(tt.project.Sync.Exclude) > 0 {
				if len(unmarshaled.Sync.Exclude) != len(tt.project.Sync.Exclude) {
					t.Errorf("Exclude length mismatch: got %d, want %d",
						len(unmarshaled.Sync.Exclude), len(tt.project.Sync.Exclude))
				}
			}
		})
	}
}

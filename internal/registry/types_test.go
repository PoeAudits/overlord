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

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
		{CategoryConfig, "config/"},
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
		{CategoryConfig, true},
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
				Status: SyncActive,
			},
			wantErr: false,
		},
		{
			name: "valid inactive sync",
			sync: ProjectSync{
				Status: SyncInactive,
			},
			wantErr: false,
		},
		{
			name: "empty status (backward compat)",
			sync: ProjectSync{
				Status: SyncStatus(""),
			},
			wantErr: false,
		},
		{
			name: "invalid status",
			sync: ProjectSync{
				Status: SyncStatus("invalid"),
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
					Status: SyncActive,
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
					Status: SyncStatus("invalid"),
				},
			},
			wantErr: true,
		},
		{
			name: "valid project with last_opened",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				LastOpened:  "2026-02-01T12:00:00Z",
			},
			wantErr: false,
		},
		{
			name: "valid project without last_opened (empty)",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				LastOpened:  "",
			},
			wantErr: false,
		},
		{
			name: "invalid last_opened format",
			project: Project{
				Path:        "/home/user/project",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-22",
				Description: "Test project",
				Status:      Status{State: StateActive},
				LastOpened:  "2026-02-01",
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
			name, _, found := registry.ProjectByAlias(tt.alias)
			if found != tt.wantFound {
				t.Errorf("ProjectByAlias() found = %v, want %v", found, tt.wantFound)
			}
			if name != tt.wantName {
				t.Errorf("ProjectByAlias() name = %v, want %v", name, tt.wantName)
			}
		})
	}
}

func TestFindProjectByPath(t *testing.T) {
	reg := Registry{
		Projects: map[string]Project{
			"overlord": {
				Path: "core/tools/overlord",
			},
			"myapp": {
				Path: "projects/web/myapp",
			},
			"api-service": {
				Path: "projects/services/api-service",
			},
			"absolute-project": {
				Path: "/opt/custom/absolute-project",
			},
		},
	}

	baseDir := "/home/user/Overlord"

	tests := []struct {
		name      string
		absPath   string
		wantName  string
		wantFound bool
	}{
		{
			name:      "exact match on project root",
			absPath:   "/home/user/Overlord/projects/web/myapp",
			wantName:  "myapp",
			wantFound: true,
		},
		{
			name:      "subdirectory of project",
			absPath:   "/home/user/Overlord/projects/web/myapp/src/components",
			wantName:  "myapp",
			wantFound: true,
		},
		{
			name:      "deeply nested subdirectory",
			absPath:   "/home/user/Overlord/core/tools/overlord/internal/cmd",
			wantName:  "overlord",
			wantFound: true,
		},
		{
			name:      "no match - outside all projects",
			absPath:   "/home/user/other-dir",
			wantFound: false,
		},
		{
			name:      "no match - partial directory name",
			absPath:   "/home/user/Overlord/projects/web/myapp-extended",
			wantFound: false,
		},
		{
			name:      "no match - base dir only",
			absPath:   "/home/user/Overlord",
			wantFound: false,
		},
		{
			name:      "absolute path project - exact match",
			absPath:   "/opt/custom/absolute-project",
			wantName:  "absolute-project",
			wantFound: true,
		},
		{
			name:      "absolute path project - subdirectory",
			absPath:   "/opt/custom/absolute-project/src",
			wantName:  "absolute-project",
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, _, found := reg.FindProjectByPath(tt.absPath, baseDir)
			if found != tt.wantFound {
				t.Errorf("FindProjectByPath() found = %v, want %v", found, tt.wantFound)
			}
			if found && name != tt.wantName {
				t.Errorf("FindProjectByPath() name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestFindProjectByPath_MostSpecificWins(t *testing.T) {
	// When projects are nested, the most specific (longest path) should win
	reg := Registry{
		Projects: map[string]Project{
			"parent": {
				Path: "projects/web",
			},
			"child": {
				Path: "projects/web/child-app",
			},
		},
	}

	baseDir := "/home/user/Overlord"

	// Inside child-app - should match child, not parent
	name, _, found := reg.FindProjectByPath("/home/user/Overlord/projects/web/child-app/src", baseDir)
	if !found {
		t.Fatal("expected to find a project")
	}
	if name != "child" {
		t.Errorf("FindProjectByPath() name = %q, want %q (most specific match)", name, "child")
	}

	// Inside parent but outside child - should match parent
	name, _, found = reg.FindProjectByPath("/home/user/Overlord/projects/web/other-dir", baseDir)
	if !found {
		t.Fatal("expected to find a project")
	}
	if name != "parent" {
		t.Errorf("FindProjectByPath() name = %q, want %q", name, "parent")
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

func TestSyncMethodIsValid(t *testing.T) {
	tests := []struct {
		name   string
		method SyncMethod
		valid  bool
	}{
		{"syncthing", SyncMethodSyncthing, true},
		{"empty (backward compat)", SyncMethod(""), true},
		{"invalid", SyncMethod("invalid"), false},
		{"rsync", SyncMethod("rsync"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.method.IsValid(); got != tt.valid {
				t.Errorf("SyncMethod.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestRegistry_ProjectByName(t *testing.T) {
	reg := Registry{
		Projects: map[string]Project{
			"alpha": {
				Path:        "/home/user/alpha",
				Category:    CategoryCLI,
				Lang:        LanguageGo,
				Created:     "2026-01-01",
				Description: "Alpha project",
				Status:      Status{State: StateActive},
			},
			"beta": {
				Path:        "/home/user/beta",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2026-01-02",
				Description: "Beta project",
				Status:      Status{State: StateArchived},
			},
		},
	}

	tests := []struct {
		name      string
		query     string
		wantFound bool
		wantPath  string
	}{
		{
			name:      "exact name match returns project",
			query:     "alpha",
			wantFound: true,
			wantPath:  "/home/user/alpha",
		},
		{
			name:      "second exact name match",
			query:     "beta",
			wantFound: true,
			wantPath:  "/home/user/beta",
		},
		{
			name:      "name not found returns not ok",
			query:     "nonexistent",
			wantFound: false,
		},
		{
			name:      "case sensitive - uppercase not found",
			query:     "Alpha",
			wantFound: false,
		},
		{
			name:      "empty string not found",
			query:     "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, ok := reg.ProjectByName(tt.query)
			if ok != tt.wantFound {
				t.Errorf("ProjectByName(%q) found = %v, want %v", tt.query, ok, tt.wantFound)
			}
			if tt.wantFound && project.Path != tt.wantPath {
				t.Errorf("ProjectByName(%q) path = %q, want %q", tt.query, project.Path, tt.wantPath)
			}
		})
	}
}

func TestRegistry_ProjectByName_EmptyRegistry(t *testing.T) {
	reg := Registry{
		Projects: map[string]Project{},
	}

	_, ok := reg.ProjectByName("anything")
	if ok {
		t.Error("ProjectByName() on empty registry returned ok=true, want false")
	}
}

func TestRegistry_ActiveProjects(t *testing.T) {
	active := Project{
		Path:        "/home/user/active",
		Category:    CategoryCLI,
		Lang:        LanguageGo,
		Created:     "2026-01-01",
		Description: "Active project",
		Status:      Status{State: StateActive},
	}
	archived := Project{
		Path:        "/home/user/archived",
		Category:    CategoryWeb,
		Lang:        LanguageTypeScript,
		Created:     "2026-01-02",
		Description: "Archived project",
		Status:      Status{State: StateArchived},
	}

	tests := []struct {
		name         string
		projects     map[string]Project
		wantNames    []string
		wantNotNames []string
	}{
		{
			name:      "empty registry returns empty map",
			projects:  map[string]Project{},
			wantNames: []string{},
		},
		{
			name: "all active projects returned",
			projects: map[string]Project{
				"proj-a": active,
				"proj-b": active,
			},
			wantNames: []string{"proj-a", "proj-b"},
		},
		{
			name: "archived projects excluded",
			projects: map[string]Project{
				"proj-archived": archived,
			},
			wantNames:    []string{},
			wantNotNames: []string{"proj-archived"},
		},
		{
			name: "mixed returns only active",
			projects: map[string]Project{
				"proj-active":   active,
				"proj-archived": archived,
			},
			wantNames:    []string{"proj-active"},
			wantNotNames: []string{"proj-archived"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := Registry{Projects: tt.projects}
			got := reg.ActiveProjects()

			for _, name := range tt.wantNames {
				if _, ok := got[name]; !ok {
					t.Errorf("ActiveProjects() missing expected project %q", name)
				}
			}
			for _, name := range tt.wantNotNames {
				if _, ok := got[name]; ok {
					t.Errorf("ActiveProjects() contains unexpected project %q", name)
				}
			}
			// Verify count for empty-expected cases
			if len(tt.wantNames) == 0 && len(got) != 0 {
				t.Errorf("ActiveProjects() returned %d projects, want 0", len(got))
			}
		})
	}
}

func TestRegistry_ArchivedProjects(t *testing.T) {
	active := Project{
		Path:        "/home/user/active",
		Category:    CategoryCLI,
		Lang:        LanguageGo,
		Created:     "2026-01-01",
		Description: "Active project",
		Status:      Status{State: StateActive},
	}
	archived := Project{
		Path:        "/home/user/archived",
		Category:    CategoryWeb,
		Lang:        LanguageTypeScript,
		Created:     "2026-01-02",
		Description: "Archived project",
		Status:      Status{State: StateArchived},
	}

	tests := []struct {
		name         string
		projects     map[string]Project
		wantNames    []string
		wantNotNames []string
	}{
		{
			name:      "empty registry returns empty map",
			projects:  map[string]Project{},
			wantNames: []string{},
		},
		{
			name: "all archived projects returned",
			projects: map[string]Project{
				"proj-a": archived,
				"proj-b": archived,
			},
			wantNames: []string{"proj-a", "proj-b"},
		},
		{
			name: "active projects excluded",
			projects: map[string]Project{
				"proj-active": active,
			},
			wantNames:    []string{},
			wantNotNames: []string{"proj-active"},
		},
		{
			name: "mixed returns only archived",
			projects: map[string]Project{
				"proj-active":   active,
				"proj-archived": archived,
			},
			wantNames:    []string{"proj-archived"},
			wantNotNames: []string{"proj-active"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := Registry{Projects: tt.projects}
			got := reg.ArchivedProjects()

			for _, name := range tt.wantNames {
				if _, ok := got[name]; !ok {
					t.Errorf("ArchivedProjects() missing expected project %q", name)
				}
			}
			for _, name := range tt.wantNotNames {
				if _, ok := got[name]; ok {
					t.Errorf("ArchivedProjects() contains unexpected project %q", name)
				}
			}
			// Verify count for empty-expected cases
			if len(tt.wantNames) == 0 && len(got) != 0 {
				t.Errorf("ArchivedProjects() returned %d projects, want 0", len(got))
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
					Status: SyncActive,
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
					Status: SyncActive,
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

		})
	}
}

package registry

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Category represents the project category type
type Category string

// Category constants
const (
	CategoryContracts  Category = "contracts"
	CategoryWeb        Category = "web"
	CategoryServices   Category = "services"
	CategoryML         Category = "ml"
	CategoryLibs       Category = "libs"
	CategoryCLI        Category = "cli"
	CategoryCoreAgents Category = "core-agents"
	CategoryCoreTools  Category = "core-tools"
	CategoryCoreEvals  Category = "core-evals"
	CategorySandbox    Category = "sandbox"
	CategoryConfig     Category = "config"
)

// categoryPaths maps each category to its filesystem path.
var categoryPaths = map[Category]string{
	CategoryContracts:  "projects/contracts/",
	CategoryWeb:        "projects/web/",
	CategoryServices:   "projects/services/",
	CategoryML:         "projects/ml/",
	CategoryLibs:       "projects/libs/",
	CategoryCLI:        "projects/cli/",
	CategoryCoreAgents: "core/agents/",
	CategoryCoreTools:  "core/tools/",
	CategoryCoreEvals:  "core/evals/",
	CategorySandbox:    "sandbox/",
	CategoryConfig:     "config/",
}

// ValidCategories returns all valid category values
func ValidCategories() []Category {
	return []Category{
		CategoryContracts,
		CategoryWeb,
		CategoryServices,
		CategoryML,
		CategoryLibs,
		CategoryCLI,
		CategoryCoreAgents,
		CategoryCoreTools,
		CategoryCoreEvals,
		CategorySandbox,
		CategoryConfig,
	}
}

// IsValid checks if the category is valid
func (c Category) IsValid() bool {
	for _, valid := range ValidCategories() {
		if c == valid {
			return true
		}
	}
	return false
}

// Path returns the filesystem path for the category
func (c Category) Path() string {
	return categoryPaths[c]
}

// Language represents the programming language type
type Language string

// Language constants
const (
	LanguagePython     Language = "python"
	LanguageTypeScript Language = "typescript"
	LanguageGo         Language = "go"
	LanguageSolidity   Language = "solidity"
	LanguageBase       Language = "base"
)

// ValidLanguages returns all valid language values
func ValidLanguages() []Language {
	return []Language{
		LanguagePython,
		LanguageTypeScript,
		LanguageGo,
		LanguageSolidity,
		LanguageBase,
	}
}

// IsValid checks if the language is valid
func (l Language) IsValid() bool {
	for _, valid := range ValidLanguages() {
		if l == valid {
			return true
		}
	}
	return false
}

// State represents the project state
type State string

// State constants
const (
	StateActive   State = "active"
	StateArchived State = "archived"
)

// IsValid checks if the state is valid
func (s State) IsValid() bool {
	return s == StateActive || s == StateArchived
}

// Status represents the project status
type Status struct {
	State State `yaml:"state"`
}

// Validate validates the status
func (s *Status) Validate() error {
	if !s.State.IsValid() {
		return fmt.Errorf("invalid state: %s (must be 'active' or 'archived')", s.State)
	}
	return nil
}

// Project represents a project entry in the registry
type Project struct {
	Path        string      `yaml:"path"`
	Category    Category    `yaml:"category"`
	Lang        Language    `yaml:"lang"`
	Created     string      `yaml:"created"`
	Description string      `yaml:"description"`
	Aliases     []string    `yaml:"aliases,omitempty"`
	Tags        []string    `yaml:"tags,omitempty"`
	Status      Status      `yaml:"status"`
	LastOpened  string      `yaml:"last_opened,omitempty"`
}

// Validate validates the project fields
func (p *Project) Validate() error {
	var errs []string

	// Required fields
	if p.Path == "" {
		errs = append(errs, "path is required")
	}

	if !p.Category.IsValid() {
		errs = append(errs, fmt.Sprintf("invalid category: %s", p.Category))
	}

	if !p.Lang.IsValid() {
		errs = append(errs, fmt.Sprintf("invalid language: %s", p.Lang))
	}

	if p.Created == "" {
		errs = append(errs, "created date is required")
	} else {
		// Validate date format YYYY-MM-DD
		if _, err := time.Parse("2006-01-02", p.Created); err != nil {
			errs = append(errs, fmt.Sprintf("invalid created date format: %s (expected YYYY-MM-DD)", p.Created))
		}
	}

	if p.Description == "" {
		errs = append(errs, "description is required")
	}

	// Validate status
	if err := p.Status.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate last_opened if present (must be RFC3339)
	if p.LastOpened != "" {
		if _, err := time.Parse(time.RFC3339, p.LastOpened); err != nil {
			errs = append(errs, fmt.Sprintf("invalid last_opened format: %s (expected RFC3339)", p.LastOpened))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("project validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// Settings represents the registry settings
type Settings struct {
	BaseDir string `yaml:"base_dir"`
}

// Validate validates the settings
func (s *Settings) Validate() error {
	if s.BaseDir == "" {
		return fmt.Errorf("settings validation failed: base_dir is required")
	}
	return nil
}

// Registry represents the complete registry structure
type Registry struct {
	Version  int                `yaml:"version"`
	Settings Settings           `yaml:"settings"`
	Projects map[string]Project `yaml:"projects"`
}

// Validate validates the entire registry
func (r *Registry) Validate() error {
	var errs []string

	// Validate version
	if r.Version != 2 {
		errs = append(errs, fmt.Sprintf("unsupported version: %d (expected 2)", r.Version))
	}

	// Validate settings
	if err := r.Settings.Validate(); err != nil {
		errs = append(errs, err.Error())
	}

	// Validate each project
	for name, project := range r.Projects {
		if err := project.Validate(); err != nil {
			errs = append(errs, fmt.Sprintf("project '%s': %s", name, err.Error()))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("registry validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// ProjectByName retrieves a project by name.
func (r *Registry) ProjectByName(name string) (Project, bool) {
	project, ok := r.Projects[name]
	return project, ok
}

// ProjectByAlias retrieves a project by alias.
func (r *Registry) ProjectByAlias(alias string) (string, Project, bool) {
	for name, project := range r.Projects {
		for _, a := range project.Aliases {
			if a == alias {
				return name, project, true
			}
		}
	}
	return "", Project{}, false
}

// ActiveProjects retrieves all active projects.
func (r *Registry) ActiveProjects() map[string]Project {
	result := make(map[string]Project, len(r.Projects))
	for name, project := range r.Projects {
		if project.Status.State == StateActive {
			result[name] = project
		}
	}
	return result
}

// ArchivedProjects retrieves all archived projects.
func (r *Registry) ArchivedProjects() map[string]Project {
	result := make(map[string]Project, len(r.Projects))
	for name, project := range r.Projects {
		if project.Status.State == StateArchived {
			result[name] = project
		}
	}
	return result
}

// FindProjectByPath finds a project whose resolved root contains the given absolute path.
// baseDir should be the expanded (absolute) base directory from settings.
// Returns the project name, the project, and whether a match was found.
// If multiple projects match (nested directories), the most specific match (longest path) wins.
func (r *Registry) FindProjectByPath(absPath, baseDir string) (string, Project, bool) {
	absPath = filepath.Clean(absPath)
	baseDir = filepath.Clean(baseDir)

	var bestName string
	var bestProject Project
	bestLen := 0

	for name, project := range r.Projects {
		var projectRoot string
		if filepath.IsAbs(project.Path) {
			projectRoot = filepath.Clean(project.Path)
		} else {
			projectRoot = filepath.Clean(filepath.Join(baseDir, project.Path))
		}

		// Check if absPath is within or equal to projectRoot
		if absPath == projectRoot || strings.HasPrefix(absPath, projectRoot+string(filepath.Separator)) {
			if len(projectRoot) > bestLen {
				bestLen = len(projectRoot)
				bestName = name
				bestProject = project
			}
		}
	}

	return bestName, bestProject, bestLen > 0
}

package registry

import (
	"fmt"
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
	CategorySandbox    Category = "sandbox"
)

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
		CategorySandbox,
	}
}

// IsValid checks if the category is valid
func (c Category) IsValid() bool {
	switch c {
	case CategoryContracts, CategoryWeb, CategoryServices, CategoryML,
		CategoryLibs, CategoryCLI, CategoryCoreAgents, CategoryCoreTools,
		CategorySandbox:
		return true
	}
	return false
}

// Path returns the filesystem path for the category
func (c Category) Path() string {
	switch c {
	case CategoryContracts:
		return "projects/contracts/"
	case CategoryWeb:
		return "projects/web/"
	case CategoryServices:
		return "projects/services/"
	case CategoryML:
		return "projects/ml/"
	case CategoryLibs:
		return "projects/libs/"
	case CategoryCLI:
		return "projects/cli/"
	case CategoryCoreAgents:
		return "core/agents/"
	case CategoryCoreTools:
		return "core/tools/"
	case CategorySandbox:
		return "sandbox/"
	default:
		return ""
	}
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
	switch l {
	case LanguagePython, LanguageTypeScript, LanguageGo, LanguageSolidity, LanguageBase:
		return true
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
	Path        string   `yaml:"path"`
	Category    Category `yaml:"category"`
	Lang        Language `yaml:"lang"`
	Created     string   `yaml:"created"`
	Description string   `yaml:"description"`
	Aliases     []string `yaml:"aliases,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
	Status      Status   `yaml:"status"`
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

	if len(errs) > 0 {
		return fmt.Errorf("project validation failed: %s", strings.Join(errs, "; "))
	}

	return nil
}

// Settings represents the registry settings
type Settings struct {
	BaseDir     string `yaml:"base_dir"`
	ThoughtsDir string `yaml:"thoughts_dir"`
}

// Validate validates the settings
func (s *Settings) Validate() error {
	var errs []string

	if s.BaseDir == "" {
		errs = append(errs, "base_dir is required")
	}

	if s.ThoughtsDir == "" {
		errs = append(errs, "thoughts_dir is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("settings validation failed: %s", strings.Join(errs, "; "))
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

// GetProject retrieves a project by name
func (r *Registry) GetProject(name string) (Project, bool) {
	project, ok := r.Projects[name]
	return project, ok
}

// GetProjectByAlias retrieves a project by alias
func (r *Registry) GetProjectByAlias(alias string) (string, Project, bool) {
	for name, project := range r.Projects {
		for _, a := range project.Aliases {
			if a == alias {
				return name, project, true
			}
		}
	}
	return "", Project{}, false
}

// GetProjectsByCategory retrieves all projects in a category
func (r *Registry) GetProjectsByCategory(category Category) map[string]Project {
	result := make(map[string]Project)
	for name, project := range r.Projects {
		if project.Category == category {
			result[name] = project
		}
	}
	return result
}

// GetProjectsByTag retrieves all projects with a specific tag
func (r *Registry) GetProjectsByTag(tag string) map[string]Project {
	result := make(map[string]Project)
	for name, project := range r.Projects {
		for _, t := range project.Tags {
			if t == tag {
				result[name] = project
				break
			}
		}
	}
	return result
}

// GetProjectsByLanguage retrieves all projects using a specific language
func (r *Registry) GetProjectsByLanguage(lang Language) map[string]Project {
	result := make(map[string]Project)
	for name, project := range r.Projects {
		if project.Lang == lang {
			result[name] = project
		}
	}
	return result
}

// GetActiveProjects retrieves all active projects
func (r *Registry) GetActiveProjects() map[string]Project {
	result := make(map[string]Project)
	for name, project := range r.Projects {
		if project.Status.State == StateActive {
			result[name] = project
		}
	}
	return result
}

// GetArchivedProjects retrieves all archived projects
func (r *Registry) GetArchivedProjects() map[string]Project {
	result := make(map[string]Project)
	for name, project := range r.Projects {
		if project.Status.State == StateArchived {
			result[name] = project
		}
	}
	return result
}

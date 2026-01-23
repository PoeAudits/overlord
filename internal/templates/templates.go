// Package templates provides embedded project templates and rendering utilities.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"text/template"
	"time"
)

//go:embed all:templates
var templateFS embed.FS

// TemplateData contains variables for template rendering.
type TemplateData struct {
	ProjectName string
	Description string
	Date        string // YYYY-MM-DD format
	Category    string
	Language    string
}

// NewTemplateData creates a new TemplateData with the current date.
func NewTemplateData(projectName, description, category, language string) TemplateData {
	return TemplateData{
		ProjectName: projectName,
		Description: description,
		Date:        time.Now().Format("2006-01-02"),
		Category:    category,
		Language:    language,
	}
}

// TemplateType represents the type of template file.
type TemplateType string

// Template type constants.
const (
	TemplateMakefile  TemplateType = "Makefile"
	TemplateREADME    TemplateType = "README.md"
	TemplateAGENTS    TemplateType = "AGENTS.md"
	TemplateOpenCode  TemplateType = "opencode.jsonc"
	TemplateTmuxLocal TemplateType = ".tmux.local"
)

// ValidTemplateTypes returns all valid template types.
func ValidTemplateTypes() []TemplateType {
	return []TemplateType{
		TemplateMakefile,
		TemplateREADME,
		TemplateAGENTS,
		TemplateOpenCode,
		TemplateTmuxLocal,
	}
}

// Language represents a supported language for templates.
type Language string

// Language constants.
const (
	LangGo         Language = "go"
	LangPython     Language = "python"
	LangTypeScript Language = "typescript"
	LangSolidity   Language = "solidity"
	LangBase       Language = "base"
)

// ValidLanguages returns all valid language values.
func ValidLanguages() []Language {
	return []Language{
		LangGo,
		LangPython,
		LangTypeScript,
		LangSolidity,
		LangBase,
	}
}

// IsValid checks if the language is valid.
func (l Language) IsValid() bool {
	switch l {
	case LangGo, LangPython, LangTypeScript, LangSolidity, LangBase:
		return true
	}
	return false
}

// getTemplatePath returns the path to a template file within the embedded FS.
// It handles the fallback logic: language-specific -> base -> shared.
func getTemplatePath(lang Language, templateType TemplateType) (string, error) {
	// Special case for .tmux.local - always use shared
	if templateType == TemplateTmuxLocal {
		return "templates/shared/.tmux.local", nil
	}

	// Determine the filename (with or without .tmpl extension)
	filename := string(templateType)
	tmplFilename := filename + ".tmpl"

	// Special case for opencode.jsonc - it's in a subdirectory
	if templateType == TemplateOpenCode {
		filename = ".opencode/opencode.jsonc"
		tmplFilename = ".opencode/opencode.jsonc.tmpl"
	}

	// Try language-specific directory first
	langDir := "templates/" + string(lang)
	paths := []string{
		path.Join(langDir, tmplFilename),
		path.Join(langDir, filename),
	}

	// Fall back to base directory
	baseDir := "templates/base"
	paths = append(paths,
		path.Join(baseDir, tmplFilename),
		path.Join(baseDir, filename),
	)

	// Find the first existing path
	for _, p := range paths {
		if _, err := templateFS.Open(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("template not found: %s for language %s", templateType, lang)
}

// GetTemplate returns the raw content of a template file.
// It uses fallback logic: language-specific -> base -> shared.
func GetTemplate(lang Language, templateType TemplateType) (string, error) {
	if !lang.IsValid() {
		return "", fmt.Errorf("invalid language: %s", lang)
	}

	templatePath, err := getTemplatePath(lang, templateType)
	if err != nil {
		return "", err
	}

	content, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	return string(content), nil
}

// RenderTemplate renders a template with the given data.
// Templates with .tmpl extension are processed with text/template.
// Other templates are returned as-is.
func RenderTemplate(lang Language, templateType TemplateType, data TemplateData) (string, error) {
	content, err := GetTemplate(lang, templateType)
	if err != nil {
		return "", err
	}

	templatePath, err := getTemplatePath(lang, templateType)
	if err != nil {
		return "", err
	}

	// Only process .tmpl files with text/template
	if !strings.HasSuffix(templatePath, ".tmpl") {
		return content, nil
	}

	tmpl, err := template.New(string(templateType)).Parse(content)
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templateType, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templateType, err)
	}

	return buf.String(), nil
}

// GetAllTemplates returns all templates for a language.
// Returns a map of template type to rendered content.
func GetAllTemplates(lang Language, data TemplateData) (map[TemplateType]string, error) {
	result := make(map[TemplateType]string)

	for _, tt := range ValidTemplateTypes() {
		content, err := RenderTemplate(lang, tt, data)
		if err != nil {
			return nil, fmt.Errorf("failed to get template %s: %w", tt, err)
		}
		result[tt] = content
	}

	return result, nil
}

// ListTemplates returns a list of all available template files.
func ListTemplates() ([]string, error) {
	var templates []string

	err := fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			templates = append(templates, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list templates: %w", err)
	}

	return templates, nil
}

// OutputPath returns the output path for a template type.
// This is the path where the template should be written in a project.
func (t TemplateType) OutputPath() string {
	switch t {
	case TemplateOpenCode:
		return ".opencode/opencode.jsonc"
	default:
		return string(t)
	}
}

// NeedsRendering returns true if the template type requires variable substitution.
func (t TemplateType) NeedsRendering() bool {
	switch t {
	case TemplateREADME, TemplateAGENTS, TemplateOpenCode:
		return true
	default:
		return false
	}
}

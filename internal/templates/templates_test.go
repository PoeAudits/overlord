package templates

import (
	"strings"
	"testing"
)

func TestLanguageIsValid(t *testing.T) {
	tests := []struct {
		lang  Language
		valid bool
	}{
		{LangGo, true},
		{LangPython, true},
		{LangTypeScript, true},
		{LangSolidity, true},
		{LangBase, true},
		{Language("invalid"), false},
		{Language(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			if got := tt.lang.IsValid(); got != tt.valid {
				t.Errorf("Language(%q).IsValid() = %v, want %v", tt.lang, got, tt.valid)
			}
		})
	}
}

func TestValidLanguages(t *testing.T) {
	langs := ValidLanguages()
	if len(langs) != 5 {
		t.Errorf("ValidLanguages() returned %d languages, want 5", len(langs))
	}

	// Verify all returned languages are valid
	for _, lang := range langs {
		if !lang.IsValid() {
			t.Errorf("ValidLanguages() returned invalid language: %s", lang)
		}
	}
}

func TestValidTemplateTypes(t *testing.T) {
	types := ValidTemplateTypes()
	if len(types) != 5 {
		t.Errorf("ValidTemplateTypes() returned %d types, want 5", len(types))
	}

	expected := map[TemplateType]bool{
		TemplateMakefile:  true,
		TemplateREADME:    true,
		TemplateAGENTS:    true,
		TemplateOpenCode:  true,
		TemplateTmuxLocal: true,
	}

	for _, tt := range types {
		if !expected[tt] {
			t.Errorf("ValidTemplateTypes() returned unexpected type: %s", tt)
		}
	}
}

func TestGetTemplate(t *testing.T) {
	tests := []struct {
		name         string
		lang         Language
		templateType TemplateType
		wantContains string
		wantErr      bool
	}{
		{
			name:         "go makefile",
			lang:         LangGo,
			templateType: TemplateMakefile,
			wantContains: "go build",
			wantErr:      false,
		},
		{
			name:         "python makefile",
			lang:         LangPython,
			templateType: TemplateMakefile,
			wantContains: "uv sync",
			wantErr:      false,
		},
		{
			name:         "typescript makefile",
			lang:         LangTypeScript,
			templateType: TemplateMakefile,
			wantContains: "pnpm",
			wantErr:      false,
		},
		{
			name:         "solidity makefile",
			lang:         LangSolidity,
			templateType: TemplateMakefile,
			wantContains: "forge",
			wantErr:      false,
		},
		{
			name:         "base makefile",
			lang:         LangBase,
			templateType: TemplateMakefile,
			wantContains: "install:",
			wantErr:      false,
		},
		{
			name:         "shared tmux.local",
			lang:         LangGo,
			templateType: TemplateTmuxLocal,
			wantContains: "TMUX_SESSION",
			wantErr:      false,
		},
		{
			name:         "base readme template",
			lang:         LangBase,
			templateType: TemplateREADME,
			wantContains: "{{.ProjectName}}",
			wantErr:      false,
		},
		{
			name:         "go agents template",
			lang:         LangGo,
			templateType: TemplateAGENTS,
			wantContains: "make test",
			wantErr:      false,
		},
		{
			name:         "invalid language",
			lang:         Language("invalid"),
			templateType: TemplateMakefile,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := GetTemplate(tt.lang, tt.templateType)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.Contains(content, tt.wantContains) {
				t.Errorf("GetTemplate() content does not contain %q", tt.wantContains)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	data := TemplateData{
		ProjectName: "test-project",
		Description: "A test project",
		Date:        "2024-01-15",
		Category:    "cli",
		Language:    "go",
	}

	tests := []struct {
		name         string
		lang         Language
		templateType TemplateType
		wantContains string
		wantErr      bool
	}{
		{
			name:         "render readme",
			lang:         LangBase,
			templateType: TemplateREADME,
			wantContains: "test-project",
			wantErr:      false,
		},
		{
			name:         "render agents",
			lang:         LangGo,
			templateType: TemplateAGENTS,
			wantContains: "test-project",
			wantErr:      false,
		},
		{
			name:         "render opencode",
			lang:         LangBase,
			templateType: TemplateOpenCode,
			wantContains: "test-project",
			wantErr:      false,
		},
		{
			name:         "makefile not rendered",
			lang:         LangGo,
			templateType: TemplateMakefile,
			wantContains: "go build",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := RenderTemplate(tt.lang, tt.templateType, data)
			if (err != nil) != tt.wantErr {
				t.Errorf("RenderTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.Contains(content, tt.wantContains) {
				t.Errorf("RenderTemplate() content does not contain %q\nGot: %s", tt.wantContains, content)
			}
		})
	}
}

func TestRenderTemplateVariableSubstitution(t *testing.T) {
	data := TemplateData{
		ProjectName: "my-awesome-project",
		Description: "An awesome description",
		Date:        "2024-06-15",
		Category:    "services",
		Language:    "python",
	}

	content, err := RenderTemplate(LangBase, TemplateREADME, data)
	if err != nil {
		t.Fatalf("RenderTemplate() error = %v", err)
	}

	// Verify key variables are substituted (README uses ProjectName and Description)
	checks := []struct {
		name     string
		contains string
	}{
		{"project name", "my-awesome-project"},
		{"description", "An awesome description"},
	}

	for _, check := range checks {
		if !strings.Contains(content, check.contains) {
			t.Errorf("Rendered content missing %s: %q", check.name, check.contains)
		}
	}

	// Verify no unsubstituted variables remain
	if strings.Contains(content, "{{.") {
		t.Error("Rendered content contains unsubstituted template variables")
	}
}

func TestNewTemplateData(t *testing.T) {
	data := NewTemplateData("test", "desc", "cli", "go")

	if data.ProjectName != "test" {
		t.Errorf("ProjectName = %q, want %q", data.ProjectName, "test")
	}
	if data.Description != "desc" {
		t.Errorf("Description = %q, want %q", data.Description, "desc")
	}
	if data.Category != "cli" {
		t.Errorf("Category = %q, want %q", data.Category, "cli")
	}
	if data.Language != "go" {
		t.Errorf("Language = %q, want %q", data.Language, "go")
	}
	if data.Date == "" {
		t.Error("Date should not be empty")
	}
}

func TestGetAllTemplates(t *testing.T) {
	data := TemplateData{
		ProjectName: "test-project",
		Description: "A test project",
		Date:        "2024-01-15",
		Category:    "cli",
		Language:    "go",
	}

	templates, err := GetAllTemplates(LangGo, data)
	if err != nil {
		t.Fatalf("GetAllTemplates() error = %v", err)
	}

	// Verify all template types are present
	for _, tt := range ValidTemplateTypes() {
		if _, ok := templates[tt]; !ok {
			t.Errorf("GetAllTemplates() missing template type: %s", tt)
		}
	}
}

func TestListTemplates(t *testing.T) {
	templates, err := ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}

	if len(templates) == 0 {
		t.Error("ListTemplates() returned empty list")
	}

	// Verify some expected templates exist
	expectedPaths := []string{
		"templates/go/Makefile",
		"templates/python/Makefile",
		"templates/base/README.md.tmpl",
		"templates/shared/.tmux.local",
	}

	for _, expected := range expectedPaths {
		found := false
		for _, tmpl := range templates {
			if tmpl == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListTemplates() missing expected template: %s", expected)
		}
	}
}

func TestTemplateTypeOutputPath(t *testing.T) {
	tests := []struct {
		templateType TemplateType
		want         string
	}{
		{TemplateMakefile, "Makefile"},
		{TemplateREADME, "README.md"},
		{TemplateAGENTS, "AGENTS.md"},
		{TemplateOpenCode, ".opencode/opencode.jsonc"},
		{TemplateTmuxLocal, ".tmux.local"},
	}

	for _, tt := range tests {
		t.Run(string(tt.templateType), func(t *testing.T) {
			if got := tt.templateType.OutputPath(); got != tt.want {
				t.Errorf("OutputPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTemplateTypeNeedsRendering(t *testing.T) {
	tests := []struct {
		templateType TemplateType
		want         bool
	}{
		{TemplateMakefile, false},
		{TemplateREADME, true},
		{TemplateAGENTS, true},
		{TemplateOpenCode, true},
		{TemplateTmuxLocal, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.templateType), func(t *testing.T) {
			if got := tt.templateType.NeedsRendering(); got != tt.want {
				t.Errorf("NeedsRendering() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFallbackToBase(t *testing.T) {
	// Go doesn't have its own README.md.tmpl, should fall back to base
	content, err := GetTemplate(LangGo, TemplateREADME)
	if err != nil {
		t.Fatalf("GetTemplate() error = %v", err)
	}

	if !strings.Contains(content, "{{.ProjectName}}") {
		t.Error("Expected base README template with template variables")
	}
}

func TestAllLanguagesHaveMakefile(t *testing.T) {
	for _, lang := range ValidLanguages() {
		t.Run(string(lang), func(t *testing.T) {
			content, err := GetTemplate(lang, TemplateMakefile)
			if err != nil {
				t.Fatalf("GetTemplate() error = %v", err)
			}

			// All Makefiles should have standard targets
			requiredTargets := []string{"install", "build", "test", "clean"}
			for _, target := range requiredTargets {
				if !strings.Contains(content, target) {
					t.Errorf("Makefile for %s missing target: %s", lang, target)
				}
			}
		})
	}
}

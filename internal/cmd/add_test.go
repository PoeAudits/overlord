package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestResolveAddArgs(t *testing.T) {
	// Save and restore addOpts.name
	origName := addOpts.name
	defer func() { addOpts.name = origName }()

	t.Run("zero args uses cwd", func(t *testing.T) {
		addOpts.name = ""
		name, path, err := resolveAddArgs([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "." {
			t.Errorf("path = %q, want %q", path, ".")
		}
		// Name should be basename of cwd
		cwd, _ := os.Getwd()
		expectedName := filepath.Base(cwd)
		if name != expectedName {
			t.Errorf("name = %q, want %q", name, expectedName)
		}
	})

	t.Run("one arg uses arg as path", func(t *testing.T) {
		addOpts.name = ""
		tmpDir := t.TempDir()
		name, path, err := resolveAddArgs([]string{tmpDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != tmpDir {
			t.Errorf("path = %q, want %q", path, tmpDir)
		}
		expectedName := filepath.Base(tmpDir)
		if name != expectedName {
			t.Errorf("name = %q, want %q", name, expectedName)
		}
	})

	t.Run("two args legacy form", func(t *testing.T) {
		addOpts.name = ""
		name, path, err := resolveAddArgs([]string{"my-project", "/some/path"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "my-project" {
			t.Errorf("name = %q, want %q", name, "my-project")
		}
		if path != "/some/path" {
			t.Errorf("path = %q, want %q", path, "/some/path")
		}
	})

	t.Run("name flag overrides auto-derived name", func(t *testing.T) {
		addOpts.name = "custom-name"
		name, path, err := resolveAddArgs([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if path != "." {
			t.Errorf("path = %q, want %q", path, ".")
		}
		if name != "custom-name" {
			t.Errorf("name = %q, want %q", name, "custom-name")
		}
	})

	t.Run("name flag overrides with one arg", func(t *testing.T) {
		addOpts.name = "override"
		tmpDir := t.TempDir()
		name, _, err := resolveAddArgs([]string{tmpDir})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "override" {
			t.Errorf("name = %q, want %q", name, "override")
		}
	})

	t.Run("name flag ignored with two args", func(t *testing.T) {
		addOpts.name = "should-be-ignored"
		name, path, err := resolveAddArgs([]string{"explicit-name", "/explicit/path"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Two-arg form always uses args[0] as name
		if name != "explicit-name" {
			t.Errorf("name = %q, want %q", name, "explicit-name")
		}
		if path != "/explicit/path" {
			t.Errorf("path = %q, want %q", path, "/explicit/path")
		}
	})
}

func TestDetectLanguage(t *testing.T) {
	t.Run("detects Go", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0o644)

		lang, err := detectLanguage(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lang != registry.LanguageGo {
			t.Errorf("lang = %q, want %q", lang, registry.LanguageGo)
		}
	})

	t.Run("detects Python from pyproject.toml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0o644)

		lang, err := detectLanguage(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lang != registry.LanguagePython {
			t.Errorf("lang = %q, want %q", lang, registry.LanguagePython)
		}
	})

	t.Run("detects Python from requirements.txt", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask"), 0o644)

		lang, err := detectLanguage(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lang != registry.LanguagePython {
			t.Errorf("lang = %q, want %q", lang, registry.LanguagePython)
		}
	})

	t.Run("detects TypeScript", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

		lang, err := detectLanguage(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lang != registry.LanguageTypeScript {
			t.Errorf("lang = %q, want %q", lang, registry.LanguageTypeScript)
		}
	})

	t.Run("detects Solidity from foundry.toml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "foundry.toml"), []byte("[profile]"), 0o644)

		lang, err := detectLanguage(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if lang != registry.LanguageSolidity {
			t.Errorf("lang = %q, want %q", lang, registry.LanguageSolidity)
		}
	})

	t.Run("errors on empty directory", func(t *testing.T) {
		dir := t.TempDir()

		_, err := detectLanguage(dir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "could not auto-detect language") {
			t.Errorf("error should mention auto-detect failure, got: %v", err)
		}
		if !strings.Contains(err.Error(), "--py") {
			t.Errorf("error should suggest language flags, got: %v", err)
		}
		if !strings.Contains(err.Error(), "--base") {
			t.Errorf("error should suggest --base flag, got: %v", err)
		}
	})
}

func TestDetectCategory(t *testing.T) {
	t.Run("detects category from path", func(t *testing.T) {
		tests := []struct {
			name     string
			relPath  string
			expected registry.Category
		}{
			{"contracts", "projects/contracts/my-contract", registry.CategoryContracts},
			{"web", "projects/web/my-app", registry.CategoryWeb},
			{"services", "projects/services/my-api", registry.CategoryServices},
			{"ml", "projects/ml/my-model", registry.CategoryML},
			{"libs", "projects/libs/my-lib", registry.CategoryLibs},
			{"cli", "projects/cli/my-tool", registry.CategoryCLI},
			{"core-agents", "core/agents/my-agent", registry.CategoryCoreAgents},
			{"core-tools", "core/tools/my-tool", registry.CategoryCoreTools},
			{"sandbox", "sandbox/experiment", registry.CategorySandbox},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				baseDir := t.TempDir()
				projectPath := filepath.Join(baseDir, tt.relPath)
				os.MkdirAll(projectPath, 0o755)

				cat, err := detectCategory(projectPath, baseDir)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cat != tt.expected {
					t.Errorf("category = %q, want %q", cat, tt.expected)
				}
			})
		}
	})

	t.Run("error when outside base dir", func(t *testing.T) {
		baseDir := t.TempDir()
		outsidePath := t.TempDir()

		_, err := detectCategory(outsidePath, baseDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "not under base directory") {
			t.Errorf("error should mention base directory, got: %v", err)
		}
		if !strings.Contains(errMsg, "--category") {
			t.Errorf("error should suggest --category flag, got: %v", err)
		}
	})

	t.Run("error when under base dir but no category match", func(t *testing.T) {
		baseDir := t.TempDir()
		unknownPath := filepath.Join(baseDir, "unknown", "my-project")
		os.MkdirAll(unknownPath, 0o755)

		_, err := detectCategory(unknownPath, baseDir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		errMsg := err.Error()
		if !strings.Contains(errMsg, "cannot auto-detect category") {
			t.Errorf("error should mention auto-detect failure, got: %v", err)
		}
		if !strings.Contains(errMsg, "--category") {
			t.Errorf("error should suggest --category flag, got: %v", err)
		}
	})
}

func TestGetLanguageFromFlags(t *testing.T) {
	// Save and restore all flags
	orig := addOpts
	defer func() { addOpts = orig }()

	tests := []struct {
		name     string
		py       bool
		ts       bool
		goLang   bool
		sol      bool
		base     bool
		wantLang registry.Language
		wantOk   bool
	}{
		{name: "no flags", wantLang: "", wantOk: false},
		{name: "python", py: true, wantLang: registry.LanguagePython, wantOk: true},
		{name: "typescript", ts: true, wantLang: registry.LanguageTypeScript, wantOk: true},
		{name: "go", goLang: true, wantLang: registry.LanguageGo, wantOk: true},
		{name: "solidity", sol: true, wantLang: registry.LanguageSolidity, wantOk: true},
		{name: "base", base: true, wantLang: registry.LanguageBase, wantOk: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addOpts.langPy = tt.py
			addOpts.langTs = tt.ts
			addOpts.langGo = tt.goLang
			addOpts.langSol = tt.sol
			addOpts.langBase = tt.base

			lang, ok := languageFromFlags(addOpts.langPy, addOpts.langTs, addOpts.langGo, addOpts.langSol, addOpts.langBase)
			if ok != tt.wantOk {
				t.Errorf("ok = %v, want %v", ok, tt.wantOk)
			}
			if lang != tt.wantLang {
				t.Errorf("lang = %q, want %q", lang, tt.wantLang)
			}
		})
	}
}

func TestValidateAddFlags(t *testing.T) {
	// Save and restore all flags
	orig := addOpts
	defer func() { addOpts = orig }()

	t.Run("no flags is valid", func(t *testing.T) {
		addOpts = addFlags{}
		err := validateAddFlags(nil, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("single language flag is valid", func(t *testing.T) {
		addOpts = addFlags{langPy: true}
		err := validateAddFlags(nil, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("multiple language flags error", func(t *testing.T) {
		addOpts = addFlags{langPy: true, langTs: true}
		err := validateAddFlags(nil, []string{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "only one language flag") {
			t.Errorf("error should mention language flag constraint, got: %v", err)
		}
	})

	t.Run("base with another flag errors", func(t *testing.T) {
		addOpts = addFlags{langBase: true, langGo: true}
		err := validateAddFlags(nil, []string{})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

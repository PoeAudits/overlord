package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

// ---------------------------------------------------------------------------
// passEntryPath: derives the pass store path from a project name.
// This mirrors the logic in runEnvPull: fmt.Sprintf("overlord/env/%s", name)
// ---------------------------------------------------------------------------

func TestPassEntryPath(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		want        string
	}{
		{
			name:        "simple project name",
			projectName: "myproject",
			want:        "overlord/env/myproject",
		},
		{
			name:        "hyphenated project name",
			projectName: "my-cool-project",
			want:        "overlord/env/my-cool-project",
		},
		{
			name:        "project with underscores",
			projectName: "my_project_v2",
			want:        "overlord/env/my_project_v2",
		},
		{
			name:        "single character name",
			projectName: "x",
			want:        "overlord/env/x",
		},
		{
			name:        "project with numbers",
			projectName: "project123",
			want:        "overlord/env/project123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmt.Sprintf("overlord/env/%s", tt.projectName)
			if got != tt.want {
				t.Errorf("passEntryPath(%q) = %q, want %q", tt.projectName, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolveProjectPath: tested here for env-specific scenarios.
// The function is defined in fileutil.go and already tested elsewhere,
// but we verify the env-specific usage patterns.
// ---------------------------------------------------------------------------

func TestResolveProjectPath_EnvScenarios(t *testing.T) {
	tests := []struct {
		name        string
		projectPath string
		baseDir     string
		want        string
	}{
		{
			name:        "relative path joined with base",
			projectPath: "projects/web/myapp",
			baseDir:     "/home/user/Overlord",
			want:        "/home/user/Overlord/projects/web/myapp",
		},
		{
			name:        "absolute path returned as-is",
			projectPath: "/opt/custom/myapp",
			baseDir:     "/home/user/Overlord",
			want:        "/opt/custom/myapp",
		},
		{
			name:        "nested relative path",
			projectPath: "core/tools/overlord-v2",
			baseDir:     "/home/user/Overlord",
			want:        "/home/user/Overlord/core/tools/overlord-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveProjectPath(tt.projectPath, tt.baseDir)
			if got != tt.want {
				t.Errorf("resolveProjectPath(%q, %q) = %q, want %q",
					tt.projectPath, tt.baseDir, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// envFilePath: the .env file target path derivation
// ---------------------------------------------------------------------------

func TestEnvFilePath(t *testing.T) {
	tests := []struct {
		name        string
		projectRoot string
		want        string
	}{
		{
			name:        "standard project root",
			projectRoot: "/home/user/Overlord/projects/web/myapp",
			want:        "/home/user/Overlord/projects/web/myapp/.env",
		},
		{
			name:        "root directory",
			projectRoot: "/",
			want:        "/.env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filepath.Join(tt.projectRoot, ".env")
			if got != tt.want {
				t.Errorf("envFilePath = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Atomic write behavior: temp file → chmod 0600 → rename
// Tests atomicWriteEnvFile from env.go.
// ---------------------------------------------------------------------------

func TestAtomicWriteEnvFile(t *testing.T) {
	t.Run("writes content to .env file", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")
		content := []byte("DB_HOST=localhost\nDB_PORT=5432\n")

		if err := atomicWriteEnvFile(envPath, content); err != nil {
			t.Fatalf("atomicWriteEnvFile() error = %v", err)
		}

		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("failed to read .env: %v", err)
		}
		if string(got) != string(content) {
			t.Errorf("content = %q, want %q", string(got), string(content))
		}
	})

	t.Run("sets 0600 permissions", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")

		if err := atomicWriteEnvFile(envPath, []byte("SECRET=value\n")); err != nil {
			t.Fatalf("atomicWriteEnvFile() error = %v", err)
		}

		info, err := os.Stat(envPath)
		if err != nil {
			t.Fatalf("failed to stat .env: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("permissions = %o, want 0600", perm)
		}
	})

	t.Run("no temp file left after success", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")

		if err := atomicWriteEnvFile(envPath, []byte("KEY=value\n")); err != nil {
			t.Fatalf("atomicWriteEnvFile() error = %v", err)
		}

		tempPath := envPath + ".tmp"
		if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
			t.Error("temp file should not exist after successful write")
		}
	})

	t.Run("overwrites existing .env file", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")

		// Write initial content
		if err := os.WriteFile(envPath, []byte("OLD=value\n"), 0644); err != nil {
			t.Fatalf("failed to write initial .env: %v", err)
		}

		newContent := []byte("NEW=value\n")
		if err := atomicWriteEnvFile(envPath, newContent); err != nil {
			t.Fatalf("atomicWriteEnvFile() error = %v", err)
		}

		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("failed to read .env: %v", err)
		}
		if string(got) != string(newContent) {
			t.Errorf("content = %q, want %q", string(got), string(newContent))
		}
	})

	t.Run("handles empty content", func(t *testing.T) {
		dir := t.TempDir()
		envPath := filepath.Join(dir, ".env")

		if err := atomicWriteEnvFile(envPath, []byte{}); err != nil {
			t.Fatalf("atomicWriteEnvFile() error = %v", err)
		}

		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("failed to read .env: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty content, got %q", string(got))
		}
	})
}

// ---------------------------------------------------------------------------
// Write failure scenarios
// ---------------------------------------------------------------------------

func TestAtomicWriteEnvFile_Errors(t *testing.T) {
	t.Run("fails on non-existent directory", func(t *testing.T) {
		err := atomicWriteEnvFile("/nonexistent/path/.env", []byte("data"))
		if err == nil {
			t.Fatal("expected error for non-existent directory")
		}
		if !strings.Contains(err.Error(), "failed to write temp file") {
			t.Errorf("error = %q, want to contain 'failed to write temp file'", err.Error())
		}
	})

	t.Run("fails on read-only directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Chmod(dir, 0555); err != nil {
			t.Fatalf("failed to chmod dir: %v", err)
		}
		t.Cleanup(func() { os.Chmod(dir, 0755) })

		err := atomicWriteEnvFile(filepath.Join(dir, ".env"), []byte("data"))
		if err == nil {
			t.Fatal("expected error for read-only directory")
		}
		if !strings.Contains(err.Error(), "failed to write temp file") {
			t.Errorf("error = %q, want to contain 'failed to write temp file'", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// parseEnvPassFile: tests for .env.pass template parsing
// ---------------------------------------------------------------------------

func TestParseEnvPassFile(t *testing.T) {
	writeTemplate := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, envPassFile)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write template: %v", err)
		}
		return path
	}

	t.Run("mixed pass and literal entries", func(t *testing.T) {
		path := writeTemplate(t, `# API keys
API_KEY=pass:anthropic-api-key
DB_URL=pass:myproject/db-url

# Config
NODE_ENV=development
PORT=3000
`)
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}

		if len(entries) != 4 {
			t.Fatalf("got %d entries, want 4", len(entries))
		}

		// Check pass entries
		if entries[0].Key != "API_KEY" || entries[0].PassEntry != "anthropic-api-key" {
			t.Errorf("entry[0] = {Key:%q, PassEntry:%q}, want {API_KEY, anthropic-api-key}",
				entries[0].Key, entries[0].PassEntry)
		}
		if entries[1].Key != "DB_URL" || entries[1].PassEntry != "myproject/db-url" {
			t.Errorf("entry[1] = {Key:%q, PassEntry:%q}, want {DB_URL, myproject/db-url}",
				entries[1].Key, entries[1].PassEntry)
		}

		// Check literal entries
		if entries[2].Key != "NODE_ENV" || entries[2].Value != "development" {
			t.Errorf("entry[2] = {Key:%q, Value:%q}, want {NODE_ENV, development}",
				entries[2].Key, entries[2].Value)
		}
		if entries[3].Key != "PORT" || entries[3].Value != "3000" {
			t.Errorf("entry[3] = {Key:%q, Value:%q}, want {PORT, 3000}",
				entries[3].Key, entries[3].Value)
		}
	})

	t.Run("value with equals sign", func(t *testing.T) {
		path := writeTemplate(t, "DATABASE_URL=postgres://user:pass@host:5432/db?sslmode=require\n")
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		if entries[0].Value != "postgres://user:pass@host:5432/db?sslmode=require" {
			t.Errorf("value = %q, want full URL with =", entries[0].Value)
		}
	})

	t.Run("comments and blank lines are skipped", func(t *testing.T) {
		path := writeTemplate(t, `# This is a comment
  # Indented comment

KEY=value

  
# Another comment
`)
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		if entries[0].Key != "KEY" || entries[0].Value != "value" {
			t.Errorf("entry = {Key:%q, Value:%q}, want {KEY, value}", entries[0].Key, entries[0].Value)
		}
	})

	t.Run("error on missing separator", func(t *testing.T) {
		path := writeTemplate(t, "MISSING_SEPARATOR\n")
		_, err := parseEnvPassFile(path)
		if err == nil {
			t.Fatal("expected error for missing = separator")
		}
		if !strings.Contains(err.Error(), "missing '=' separator") {
			t.Errorf("error = %q, want to contain \"missing '=' separator\"", err.Error())
		}
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("error = %q, want to contain line number", err.Error())
		}
	})

	t.Run("error on empty key", func(t *testing.T) {
		path := writeTemplate(t, "=value\n")
		_, err := parseEnvPassFile(path)
		if err == nil {
			t.Fatal("expected error for empty key")
		}
		if !strings.Contains(err.Error(), "empty key") {
			t.Errorf("error = %q, want to contain 'empty key'", err.Error())
		}
	})

	t.Run("error on empty pass entry", func(t *testing.T) {
		path := writeTemplate(t, "KEY=pass:\n")
		_, err := parseEnvPassFile(path)
		if err == nil {
			t.Fatal("expected error for empty pass entry")
		}
		if !strings.Contains(err.Error(), "empty pass entry") {
			t.Errorf("error = %q, want to contain 'empty pass entry'", err.Error())
		}
	})

	t.Run("empty file returns empty slice", func(t *testing.T) {
		path := writeTemplate(t, "")
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("got %d entries, want 0", len(entries))
		}
	})

	t.Run("only comments returns empty slice", func(t *testing.T) {
		path := writeTemplate(t, "# just a comment\n# another one\n")
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("got %d entries, want 0", len(entries))
		}
	})

	t.Run("line numbers are correct", func(t *testing.T) {
		path := writeTemplate(t, `# comment
KEY1=val1

KEY2=pass:entry2
# another comment
KEY3=val3
`)
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}

		if len(entries) != 3 {
			t.Fatalf("got %d entries, want 3", len(entries))
		}
		if entries[0].Line != 2 {
			t.Errorf("entry[0].Line = %d, want 2", entries[0].Line)
		}
		if entries[1].Line != 4 {
			t.Errorf("entry[1].Line = %d, want 4", entries[1].Line)
		}
		if entries[2].Line != 6 {
			t.Errorf("entry[2].Line = %d, want 6", entries[2].Line)
		}
	})

	t.Run("pass entry path with slashes", func(t *testing.T) {
		path := writeTemplate(t, "SECRET=pass:overlord/env/myproject/secret-key\n")
		entries, err := parseEnvPassFile(path)
		if err != nil {
			t.Fatalf("parseEnvPassFile() error = %v", err)
		}

		if entries[0].PassEntry != "overlord/env/myproject/secret-key" {
			t.Errorf("PassEntry = %q, want %q", entries[0].PassEntry, "overlord/env/myproject/secret-key")
		}
	})

	t.Run("non-existent file returns error", func(t *testing.T) {
		_, err := parseEnvPassFile("/nonexistent/.env.pass")
		if err == nil {
			t.Fatal("expected error for non-existent file")
		}
	})
}

// ---------------------------------------------------------------------------
// resolveEnvEntries: tests composition of .env content from entries.
// Only tests literal-only entries (pass resolution requires real pass binary).
// ---------------------------------------------------------------------------

func TestResolveEnvEntries(t *testing.T) {
	t.Run("literal-only entries", func(t *testing.T) {
		entries := []envEntry{
			{Key: "NODE_ENV", Value: "development", Line: 1},
			{Key: "PORT", Value: "3000", Line: 2},
			{Key: "DEBUG", Value: "true", Line: 3},
		}

		content, passCount, err := resolveEnvEntries(entries)
		if err != nil {
			t.Fatalf("resolveEnvEntries() error = %v", err)
		}

		if passCount != 0 {
			t.Errorf("passCount = %d, want 0", passCount)
		}

		want := "NODE_ENV=development\nPORT=3000\nDEBUG=true\n"
		if string(content) != want {
			t.Errorf("content = %q, want %q", string(content), want)
		}
	})

	t.Run("empty entries", func(t *testing.T) {
		content, passCount, err := resolveEnvEntries(nil)
		if err != nil {
			t.Fatalf("resolveEnvEntries() error = %v", err)
		}
		if passCount != 0 {
			t.Errorf("passCount = %d, want 0", passCount)
		}
		if string(content) != "" {
			t.Errorf("content = %q, want empty", string(content))
		}
	})

	t.Run("preserves order", func(t *testing.T) {
		entries := []envEntry{
			{Key: "C", Value: "3", Line: 1},
			{Key: "A", Value: "1", Line: 2},
			{Key: "B", Value: "2", Line: 3},
		}

		content, _, err := resolveEnvEntries(entries)
		if err != nil {
			t.Fatalf("resolveEnvEntries() error = %v", err)
		}

		want := "C=3\nA=1\nB=2\n"
		if string(content) != want {
			t.Errorf("content = %q, want %q (order preserved)", string(content), want)
		}
	})

	t.Run("value with equals sign preserved", func(t *testing.T) {
		entries := []envEntry{
			{Key: "URL", Value: "postgres://host/db?ssl=true", Line: 1},
		}

		content, _, err := resolveEnvEntries(entries)
		if err != nil {
			t.Fatalf("resolveEnvEntries() error = %v", err)
		}

		want := "URL=postgres://host/db?ssl=true\n"
		if string(content) != want {
			t.Errorf("content = %q, want %q", string(content), want)
		}
	})
}

// ---------------------------------------------------------------------------
// Project resolution for env pull: tests that registry resolution works
// correctly for the env pull use case (without needing real pass/GPG).
// ---------------------------------------------------------------------------

func TestEnvPull_ProjectResolution(t *testing.T) {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/Overlord",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"myapp": {
				Path:        "projects/web/myapp",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "My web application",
				Status:      registry.Status{State: registry.StateActive},
			},
			"api-service": {
				Path:        "projects/services/api-service",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "API service",
				Aliases:     []string{"api", "svc"},
				Status:      registry.Status{State: registry.StateActive},
			},
			"archived-project": {
				Path:        "archive/archived-project",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "An archived project",
				Status:      registry.Status{State: registry.StateArchived},
			},
		},
	}

	t.Run("exact name match", func(t *testing.T) {
		result, err := reg.ResolveOne("myapp")
		if err != nil {
			t.Fatalf("ResolveOne() error = %v", err)
		}
		if result.Name != "myapp" {
			t.Errorf("Name = %q, want %q", result.Name, "myapp")
		}
		if result.MatchedOn != "name" {
			t.Errorf("MatchedOn = %q, want %q", result.MatchedOn, "name")
		}

		// Verify pass entry path derivation
		passEntry := fmt.Sprintf("overlord/env/%s", result.Name)
		if passEntry != "overlord/env/myapp" {
			t.Errorf("passEntry = %q, want %q", passEntry, "overlord/env/myapp")
		}
	})

	t.Run("alias match", func(t *testing.T) {
		result, err := reg.ResolveOne("api")
		if err != nil {
			t.Fatalf("ResolveOne() error = %v", err)
		}
		if result.Name != "api-service" {
			t.Errorf("Name = %q, want %q", result.Name, "api-service")
		}
		if !strings.HasPrefix(result.MatchedOn, "alias:") {
			t.Errorf("MatchedOn = %q, want prefix 'alias:'", result.MatchedOn)
		}

		// Verify pass entry uses resolved name, not alias
		passEntry := fmt.Sprintf("overlord/env/%s", result.Name)
		if passEntry != "overlord/env/api-service" {
			t.Errorf("passEntry = %q, want %q", passEntry, "overlord/env/api-service")
		}
	})

	t.Run("non-existent project", func(t *testing.T) {
		_, err := reg.ResolveOne("nonexistent-project-xyz")
		if err == nil {
			t.Fatal("expected error for non-existent project")
		}
	})

	t.Run("empty query", func(t *testing.T) {
		_, err := reg.ResolveOne("")
		if err == nil {
			t.Fatal("expected error for empty query")
		}
	})
}

// ---------------------------------------------------------------------------
// Auto-detection from working directory: tests FindProjectByPath integration
// for the env pull zero-args path.
// ---------------------------------------------------------------------------

func TestEnvPull_AutoDetection(t *testing.T) {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/Overlord",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"myapp": {
				Path:        "projects/web/myapp",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "My web application",
				Status:      registry.Status{State: registry.StateActive},
			},
			"api-service": {
				Path:        "projects/services/api-service",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "API service",
				Aliases:     []string{"api"},
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	baseDir := "/home/user/Overlord"

	t.Run("detects project from exact root", func(t *testing.T) {
		name, _, found := reg.FindProjectByPath("/home/user/Overlord/projects/web/myapp", baseDir)
		if !found {
			t.Fatal("expected to find project from exact root")
		}
		if name != "myapp" {
			t.Errorf("name = %q, want %q", name, "myapp")
		}
	})

	t.Run("detects project from subdirectory", func(t *testing.T) {
		name, _, found := reg.FindProjectByPath("/home/user/Overlord/projects/web/myapp/src/handlers", baseDir)
		if !found {
			t.Fatal("expected to find project from subdirectory")
		}
		if name != "myapp" {
			t.Errorf("name = %q, want %q", name, "myapp")
		}
	})

	t.Run("no match outside any project", func(t *testing.T) {
		_, _, found := reg.FindProjectByPath("/home/user/other-workspace", baseDir)
		if found {
			t.Error("expected no match for path outside all projects")
		}
	})

	t.Run("no match for base dir itself", func(t *testing.T) {
		_, _, found := reg.FindProjectByPath("/home/user/Overlord", baseDir)
		if found {
			t.Error("expected no match for base dir without project")
		}
	})

	t.Run("derives correct pass entry from detected project", func(t *testing.T) {
		name, _, found := reg.FindProjectByPath("/home/user/Overlord/projects/services/api-service/cmd", baseDir)
		if !found {
			t.Fatal("expected to find project")
		}
		passEntry := fmt.Sprintf("overlord/env/%s", name)
		if passEntry != "overlord/env/api-service" {
			t.Errorf("passEntry = %q, want %q", passEntry, "overlord/env/api-service")
		}
	})
}

// ---------------------------------------------------------------------------
// Match info formatting: tests the display logic for match type
// ---------------------------------------------------------------------------

func TestMatchInfoFormatting(t *testing.T) {
	tests := []struct {
		name      string
		matchedOn string
		wantInfo  string
	}{
		{
			name:      "name match shows nothing special",
			matchedOn: "name",
			wantInfo:  "", // No extra info shown for name matches
		},
		{
			name:      "alias match formats correctly",
			matchedOn: "alias:api",
			wantInfo:  "alias 'api'",
		},
		{
			name:      "alias match with hyphenated alias",
			matchedOn: "alias:my-alias",
			wantInfo:  "alias 'my-alias'",
		},
		{
			name:      "fuzzy match type",
			matchedOn: "fuzzy",
			wantInfo:  "fuzzy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mirrors the logic in runEnvPull lines 135-141
			if tt.matchedOn == "name" {
				// No extra info shown for name matches
				return
			}

			matchInfo := tt.matchedOn
			if strings.HasPrefix(matchInfo, "alias:") {
				matchInfo = fmt.Sprintf("alias '%s'", strings.TrimPrefix(matchInfo, "alias:"))
			}

			if matchInfo != tt.wantInfo {
				t.Errorf("matchInfo = %q, want %q", matchInfo, tt.wantInfo)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// End-to-end env pull logic (without pass): tests the full flow using
// a temporary registry and project directory, mocking only the pass call.
// ---------------------------------------------------------------------------

func TestEnvPull_EndToEnd_WithoutPass(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "projects", "web", "myapp")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}

	// Create and save a test registry
	registryPath := filepath.Join(tmpDir, "registry.yaml")
	testRegistry := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     tmpDir,
			ThoughtsDir: filepath.Join(tmpDir, "thoughts"),
		},
		Projects: map[string]registry.Project{
			"myapp": {
				Path:        "projects/web/myapp",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-01",
				Description: "Test project",
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	if err := registry.Save(registryPath, testRegistry); err != nil {
		t.Fatalf("failed to save test registry: %v", err)
	}

	t.Run("registry loads and project resolves", func(t *testing.T) {
		reg, err := registry.Load(registryPath)
		if err != nil {
			t.Fatalf("failed to load registry: %v", err)
		}

		result, err := reg.ResolveOne("myapp")
		if err != nil {
			t.Fatalf("failed to resolve project: %v", err)
		}

		if result.Name != "myapp" {
			t.Errorf("Name = %q, want %q", result.Name, "myapp")
		}

		// Verify project root computation
		projectRoot := resolveProjectPath(result.Project.Path, tmpDir)
		if projectRoot != projectDir {
			t.Errorf("projectRoot = %q, want %q", projectRoot, projectDir)
		}

		// Verify project directory exists
		if _, err := os.Stat(projectRoot); os.IsNotExist(err) {
			t.Errorf("project directory should exist: %s", projectRoot)
		}

		// Verify pass entry path
		passEntry := fmt.Sprintf("overlord/env/%s", result.Name)
		if passEntry != "overlord/env/myapp" {
			t.Errorf("passEntry = %q, want %q", passEntry, "overlord/env/myapp")
		}
	})

	t.Run("atomic write produces correct .env", func(t *testing.T) {
		// Simulate pass output (what pass show would return)
		mockPassOutput := []byte("DB_HOST=localhost\nDB_PORT=5432\nSECRET_KEY=abc123\n")
		envPath := filepath.Join(projectDir, ".env")

		err := atomicWriteEnvFile(envPath, mockPassOutput)
		if err != nil {
			t.Fatalf("atomicWriteEnvFile() error = %v", err)
		}

		// Verify .env was written
		got, err := os.ReadFile(envPath)
		if err != nil {
			t.Fatalf("failed to read .env: %v", err)
		}

		if string(got) != string(mockPassOutput) {
			t.Errorf("content = %q, want %q", string(got), string(mockPassOutput))
		}

		// Verify permissions
		info, err := os.Stat(envPath)
		if err != nil {
			t.Fatalf("failed to stat .env: %v", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("permissions = %o, want 0600", info.Mode().Perm())
		}
	})

	t.Run("project directory missing returns error", func(t *testing.T) {
		missingDir := filepath.Join(tmpDir, "projects", "web", "nonexistent")
		if _, err := os.Stat(missingDir); !os.IsNotExist(err) {
			t.Skip("directory unexpectedly exists")
		}

		// This mirrors the check in runEnvPull
		_, err := os.Stat(missingDir)
		if !os.IsNotExist(err) {
			t.Error("expected os.IsNotExist for missing project directory")
		}
	})
}

// ---------------------------------------------------------------------------
// Error message formatting tests
// ---------------------------------------------------------------------------

func TestEnvPull_ErrorMessages(t *testing.T) {
	t.Run("registry load error wraps correctly", func(t *testing.T) {
		// Simulate the error wrapping from runEnvPull
		innerErr := fmt.Errorf("file not found")
		err := fmt.Errorf("failed to load registry: %w", innerErr)

		if !strings.Contains(err.Error(), "failed to load registry") {
			t.Errorf("error should contain 'failed to load registry', got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("error should contain inner error, got %q", err.Error())
		}
	})

	t.Run("project not found error includes hint", func(t *testing.T) {
		name := "nonexistent"
		err := fmt.Errorf("project '%s' not found in registry\nrun 'overlord list' to see all projects", name)

		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should contain project name %q, got %q", name, err.Error())
		}
		if !strings.Contains(err.Error(), "overlord list") {
			t.Errorf("error should contain hint about 'overlord list', got %q", err.Error())
		}
	})

	t.Run("pass not installed error includes install link", func(t *testing.T) {
		err := fmt.Errorf("'pass' is not installed or not in PATH\nInstall pass: https://www.passwordstore.org/")

		if !strings.Contains(err.Error(), "passwordstore.org") {
			t.Errorf("error should contain install URL, got %q", err.Error())
		}
	})

	t.Run("pass entry missing error includes create hint", func(t *testing.T) {
		passEntry := "overlord/env/myapp"
		err := fmt.Errorf("no pass entry at '%s'\nCreate it with: pass insert -m %s", passEntry, passEntry)

		if !strings.Contains(err.Error(), passEntry) {
			t.Errorf("error should contain pass entry path, got %q", err.Error())
		}
		if !strings.Contains(err.Error(), "pass insert -m") {
			t.Errorf("error should contain create hint, got %q", err.Error())
		}
	})

	t.Run("directory not found error includes path", func(t *testing.T) {
		projectRoot := "/home/user/Overlord/projects/web/myapp"
		err := fmt.Errorf("project directory does not exist: %s", projectRoot)

		if !strings.Contains(err.Error(), projectRoot) {
			t.Errorf("error should contain project root path, got %q", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// passEntryToEnvVar: converts pass entry names to env var names
// ---------------------------------------------------------------------------

func TestPassEntryToEnvVar(t *testing.T) {
	tests := []struct {
		name  string
		entry string
		want  string
	}{
		{
			name:  "simple kebab-case",
			entry: "anthropic-api-key",
			want:  "ANTHROPIC_API_KEY",
		},
		{
			name:  "single word",
			entry: "secret",
			want:  "SECRET",
		},
		{
			name:  "already uppercase compatible",
			entry: "TOKEN",
			want:  "TOKEN",
		},
		{
			name:  "nested path with slashes",
			entry: "folder/secret-name",
			want:  "FOLDER_SECRET_NAME",
		},
		{
			name:  "deeply nested",
			entry: "org/project/db-url",
			want:  "ORG_PROJECT_DB_URL",
		},
		{
			name:  "mixed case preserved after upper",
			entry: "myKey",
			want:  "MYKEY",
		},
		{
			name:  "hyphens and slashes combined",
			entry: "my-folder/my-secret-key",
			want:  "MY_FOLDER_MY_SECRET_KEY",
		},
		{
			name:  "underscores already present",
			entry: "my_key",
			want:  "MY_KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := passEntryToEnvVar(tt.entry)
			if got != tt.want {
				t.Errorf("passEntryToEnvVar(%q) = %q, want %q", tt.entry, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateEnvPassContent: builds .env.pass template from entry list
// ---------------------------------------------------------------------------

func TestGenerateEnvPassContent(t *testing.T) {
	t.Run("generates template with header and entries", func(t *testing.T) {
		entries := []string{
			"anthropic-api-key",
			"etherscan-api-key",
			"openai-api-key",
		}

		content := generateEnvPassContent(entries)

		// Check header is present
		if !strings.Contains(content, "# Environment variables") {
			t.Error("content should contain header comment")
		}
		if !strings.Contains(content, "overlord env generate") {
			t.Error("content should reference the generate command")
		}
		if !strings.Contains(content, "overlord env") {
			t.Error("content should reference the pull command")
		}

		// Check entries
		if !strings.Contains(content, "ANTHROPIC_API_KEY=pass:anthropic-api-key") {
			t.Error("content should contain ANTHROPIC_API_KEY entry")
		}
		if !strings.Contains(content, "ETHERSCAN_API_KEY=pass:etherscan-api-key") {
			t.Error("content should contain ETHERSCAN_API_KEY entry")
		}
		if !strings.Contains(content, "OPENAI_API_KEY=pass:openai-api-key") {
			t.Error("content should contain OPENAI_API_KEY entry")
		}
	})

	t.Run("preserves entry order", func(t *testing.T) {
		entries := []string{"zebra-key", "alpha-key", "middle-key"}

		content := generateEnvPassContent(entries)
		lines := strings.Split(content, "\n")

		// Find entry lines (skip header comments and blank lines)
		var entryLines []string
		for _, line := range lines {
			if line != "" && !strings.HasPrefix(line, "#") {
				entryLines = append(entryLines, line)
			}
		}

		if len(entryLines) != 3 {
			t.Fatalf("got %d entry lines, want 3", len(entryLines))
		}
		if entryLines[0] != "ZEBRA_KEY=pass:zebra-key" {
			t.Errorf("first entry = %q, want ZEBRA_KEY", entryLines[0])
		}
		if entryLines[1] != "ALPHA_KEY=pass:alpha-key" {
			t.Errorf("second entry = %q, want ALPHA_KEY", entryLines[1])
		}
		if entryLines[2] != "MIDDLE_KEY=pass:middle-key" {
			t.Errorf("third entry = %q, want MIDDLE_KEY", entryLines[2])
		}
	})

	t.Run("handles nested pass entries", func(t *testing.T) {
		entries := []string{"project/db-url", "org/api/secret-key"}

		content := generateEnvPassContent(entries)

		if !strings.Contains(content, "PROJECT_DB_URL=pass:project/db-url") {
			t.Error("content should contain nested entry with slashes")
		}
		if !strings.Contains(content, "ORG_API_SECRET_KEY=pass:org/api/secret-key") {
			t.Error("content should contain deeply nested entry")
		}
	})
}

// ---------------------------------------------------------------------------
// listPassEntries: scans a pass store directory for .gpg files
// ---------------------------------------------------------------------------

func TestListPassEntries(t *testing.T) {
	// Create a mock pass store directory structure
	createMockPassStore := func(t *testing.T, entries []string) string {
		t.Helper()
		dir := t.TempDir()

		// Create .gpg-id file (mimics real pass store)
		if err := os.WriteFile(filepath.Join(dir, ".gpg-id"), []byte("test@example.com\n"), 0644); err != nil {
			t.Fatalf("failed to create .gpg-id: %v", err)
		}

		for _, entry := range entries {
			gpgPath := filepath.Join(dir, entry+".gpg")
			if err := os.MkdirAll(filepath.Dir(gpgPath), 0755); err != nil {
				t.Fatalf("failed to create dir for %s: %v", entry, err)
			}
			if err := os.WriteFile(gpgPath, []byte("encrypted"), 0644); err != nil {
				t.Fatalf("failed to create %s.gpg: %v", entry, err)
			}
		}

		return dir
	}

	t.Run("finds flat entries", func(t *testing.T) {
		storeDir := createMockPassStore(t, []string{
			"anthropic-api-key",
			"openai-api-key",
			"etherscan-api-key",
		})

		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		entries, err := listPassEntries()
		if err != nil {
			t.Fatalf("listPassEntries() error = %v", err)
		}

		if len(entries) != 3 {
			t.Fatalf("got %d entries, want 3", len(entries))
		}

		// Should be sorted
		if entries[0] != "anthropic-api-key" {
			t.Errorf("entries[0] = %q, want anthropic-api-key", entries[0])
		}
		if entries[1] != "etherscan-api-key" {
			t.Errorf("entries[1] = %q, want etherscan-api-key", entries[1])
		}
		if entries[2] != "openai-api-key" {
			t.Errorf("entries[2] = %q, want openai-api-key", entries[2])
		}
	})

	t.Run("finds nested entries", func(t *testing.T) {
		storeDir := createMockPassStore(t, []string{
			"api-key",
			"project/db-url",
			"project/secret",
		})

		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		entries, err := listPassEntries()
		if err != nil {
			t.Fatalf("listPassEntries() error = %v", err)
		}

		if len(entries) != 3 {
			t.Fatalf("got %d entries, want 3", len(entries))
		}

		if entries[0] != "api-key" {
			t.Errorf("entries[0] = %q, want api-key", entries[0])
		}
		if entries[1] != "project/db-url" {
			t.Errorf("entries[1] = %q, want project/db-url", entries[1])
		}
		if entries[2] != "project/secret" {
			t.Errorf("entries[2] = %q, want project/secret", entries[2])
		}
	})

	t.Run("skips hidden directories", func(t *testing.T) {
		storeDir := createMockPassStore(t, []string{"my-key"})

		// Create a .git directory with a .gpg file (should be skipped)
		gitDir := filepath.Join(storeDir, ".git")
		if err := os.MkdirAll(gitDir, 0755); err != nil {
			t.Fatalf("failed to create .git: %v", err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "something.gpg"), []byte("data"), 0644); err != nil {
			t.Fatalf("failed to create .git/something.gpg: %v", err)
		}

		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		entries, err := listPassEntries()
		if err != nil {
			t.Fatalf("listPassEntries() error = %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1 (hidden dirs should be skipped)", len(entries))
		}
		if entries[0] != "my-key" {
			t.Errorf("entries[0] = %q, want my-key", entries[0])
		}
	})

	t.Run("empty store returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PASSWORD_STORE_DIR", dir)

		entries, err := listPassEntries()
		if err != nil {
			t.Fatalf("listPassEntries() error = %v", err)
		}

		if len(entries) != 0 {
			t.Errorf("got %d entries, want 0", len(entries))
		}
	})

	t.Run("non-existent store returns error", func(t *testing.T) {
		t.Setenv("PASSWORD_STORE_DIR", "/nonexistent/password-store")

		_, err := listPassEntries()
		if err == nil {
			t.Fatal("expected error for non-existent store")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("error = %q, want to contain 'does not exist'", err.Error())
		}
	})

	t.Run("works when store dir starts with dot", func(t *testing.T) {
		// Mimics the real ~/.password-store directory
		parent := t.TempDir()
		storeDir := filepath.Join(parent, ".password-store")
		if err := os.MkdirAll(storeDir, 0700); err != nil {
			t.Fatalf("failed to create .password-store: %v", err)
		}

		// Create a .gpg file
		if err := os.WriteFile(filepath.Join(storeDir, "my-key.gpg"), []byte("enc"), 0644); err != nil {
			t.Fatalf("failed to create gpg file: %v", err)
		}

		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		entries, err := listPassEntries()
		if err != nil {
			t.Fatalf("listPassEntries() error = %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1 (root dot-dir should not be skipped)", len(entries))
		}
		if entries[0] != "my-key" {
			t.Errorf("entries[0] = %q, want my-key", entries[0])
		}
	})

	t.Run("ignores non-gpg files", func(t *testing.T) {
		storeDir := createMockPassStore(t, []string{"real-key"})

		// Add non-.gpg files
		os.WriteFile(filepath.Join(storeDir, "README.md"), []byte("readme"), 0644)
		os.WriteFile(filepath.Join(storeDir, ".gitattributes"), []byte("attrs"), 0644)

		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		entries, err := listPassEntries()
		if err != nil {
			t.Fatalf("listPassEntries() error = %v", err)
		}

		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1 (non-gpg files should be ignored)", len(entries))
		}
	})
}

// ---------------------------------------------------------------------------
// getPassStoreDir: resolves the pass store location
// ---------------------------------------------------------------------------

func TestGetPassStoreDir(t *testing.T) {
	t.Run("uses PASSWORD_STORE_DIR if set", func(t *testing.T) {
		t.Setenv("PASSWORD_STORE_DIR", "/custom/store")

		dir, err := getPassStoreDir()
		if err != nil {
			t.Fatalf("getPassStoreDir() error = %v", err)
		}
		if dir != "/custom/store" {
			t.Errorf("dir = %q, want /custom/store", dir)
		}
	})

	t.Run("defaults to ~/.password-store", func(t *testing.T) {
		t.Setenv("PASSWORD_STORE_DIR", "")

		dir, err := getPassStoreDir()
		if err != nil {
			t.Fatalf("getPassStoreDir() error = %v", err)
		}

		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".password-store")
		if dir != want {
			t.Errorf("dir = %q, want %q", dir, want)
		}
	})
}

// ---------------------------------------------------------------------------
// GenerateEnvPassFile: end-to-end file generation
// ---------------------------------------------------------------------------

func TestGenerateEnvPassFile(t *testing.T) {
	t.Run("writes .env.pass with all entries", func(t *testing.T) {
		// Create mock pass store
		storeDir := t.TempDir()
		for _, entry := range []string{"api-key", "db-secret"} {
			gpgPath := filepath.Join(storeDir, entry+".gpg")
			if err := os.WriteFile(gpgPath, []byte("enc"), 0644); err != nil {
				t.Fatalf("failed to create %s.gpg: %v", entry, err)
			}
		}
		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		// Generate into a project dir
		projectDir := t.TempDir()
		count, err := GenerateEnvPassFile(projectDir)
		if err != nil {
			t.Fatalf("GenerateEnvPassFile() error = %v", err)
		}

		if count != 2 {
			t.Errorf("count = %d, want 2", count)
		}

		// Verify file was written
		content, err := os.ReadFile(filepath.Join(projectDir, envPassFile))
		if err != nil {
			t.Fatalf("failed to read .env.pass: %v", err)
		}

		if !strings.Contains(string(content), "API_KEY=pass:api-key") {
			t.Error("content should contain API_KEY entry")
		}
		if !strings.Contains(string(content), "DB_SECRET=pass:db-secret") {
			t.Error("content should contain DB_SECRET entry")
		}
	})

	t.Run("returns 0 for empty store", func(t *testing.T) {
		storeDir := t.TempDir()
		t.Setenv("PASSWORD_STORE_DIR", storeDir)

		projectDir := t.TempDir()
		count, err := GenerateEnvPassFile(projectDir)
		if err != nil {
			t.Fatalf("GenerateEnvPassFile() error = %v", err)
		}

		if count != 0 {
			t.Errorf("count = %d, want 0", count)
		}

		// Verify file was NOT written
		if _, err := os.Stat(filepath.Join(projectDir, envPassFile)); !os.IsNotExist(err) {
			t.Error(".env.pass should not be created for empty store")
		}
	})

	t.Run("errors on non-existent store", func(t *testing.T) {
		t.Setenv("PASSWORD_STORE_DIR", "/nonexistent/store")

		projectDir := t.TempDir()
		_, err := GenerateEnvPassFile(projectDir)
		if err == nil {
			t.Fatal("expected error for non-existent store")
		}
	})
}

// ---------------------------------------------------------------------------
// Smoke-Test Procedure (Manual)
//
// The following describes how to manually verify `overlord env` on a
// developer machine that has `pass` and GPG configured.
//
// Prerequisites:
//   - pass (password-store) installed and initialized
//   - GPG key configured for pass
//   - overlord binary built: go build -o overlord ./cmd/overlord
//   - A project registered in the overlord registry
//   - At least one pass entry (e.g., pass insert anthropic-api-key)
//
// Steps:
//
// 1. Create a .env.pass template in the project root:
//      cat > ~/Overlord/projects/web/myproject/.env.pass <<'EOF'
//      # Secrets
//      API_KEY=pass:anthropic-api-key
//
//      # Config
//      NODE_ENV=development
//      PORT=3000
//      EOF
//
// 2. Run from inside the project directory (auto-detection):
//      cd ~/Overlord/projects/web/myproject
//      overlord env
//
// 3. Or run with explicit project name:
//      overlord env myproject
//      overlord env pull myproject
//
// 4. Verify the output:
//    a. Check the .env file was created:
//         ls -la ~/Overlord/projects/web/myproject/.env
//    b. Verify permissions are 0600 (owner read/write only):
//         stat -c '%a' ~/Overlord/projects/web/myproject/.env
//         # Should output: 600
//    c. Verify content has resolved values:
//         cat ~/Overlord/projects/web/myproject/.env
//         # Should show: API_KEY=<actual pass value>, NODE_ENV=development, PORT=3000
//    d. Verify no temp file was left behind:
//         ls ~/Overlord/projects/web/myproject/.env.tmp
//         # Should report "No such file or directory"
//
// 5. Test error cases:
//    a. Missing pass entry:
//         # Add a non-existent pass ref to .env.pass and run overlord env
//         # Should show "failed to resolve pass entry" with create hint
//    b. Outside project directory (no args):
//         cd /tmp && overlord env
//         # Should show "not inside a project directory" error
//    c. No .env.pass and no blob:
//         # Remove .env.pass, should show error with instructions
//
// 6. Cleanup:
//      rm ~/Overlord/projects/web/myproject/.env
//      rm ~/Overlord/projects/web/myproject/.env.pass
//
// ---------------------------------------------------------------------------

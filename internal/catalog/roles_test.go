package catalog_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/catalog"
)

const explorerAndRestricted = `
main:
  description: The main agent
  models: {claude: opus, codex: gpt-6, opencode: openai/gpt-6}
restricted:
  description: Highest-intelligence agent for exceptional tasks.
  userInvokedOnly: true
  models: {claude: claude-fable-5-1, codex: gpt-6-astra, opencode: openai/gpt-6-astra#high}
  codexReasoningEffort: high
  prompt: roles/restricted.md
explorer:
  description: Read-only research and codebase exploration.
  readOnly: true
  models: {claude: haiku, codex: gpt-6-luna, opencode: openai/gpt-6-luna#low}
  codexReasoningEffort: low
  prompt: roles/explorer.md
`

func TestLoadRolesParsesACatalogWithExplorerAndRestricted(t *testing.T) {
	dir := agentsDir(t, explorerAndRestricted, map[string]string{
		"roles/explorer.md":   "Explore without editing.\n",
		"roles/restricted.md": "Run only when asked.\n",
	})

	roles, err := catalog.LoadRoles(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := []catalog.Role{
		{
			Name:                 "explorer",
			Description:          "Read-only research and codebase exploration.",
			ReadOnly:             true,
			Models:               catalog.Models{Claude: "haiku", Codex: "gpt-6-luna", OpenCode: "openai/gpt-6-luna#low"},
			CodexReasoningEffort: "low",
			Prompt:               "Explore without editing.\n",
		},
		{
			Name:        "main",
			Description: "The main agent",
			Models:      catalog.Models{Claude: "opus", Codex: "gpt-6", OpenCode: "openai/gpt-6"},
		},
		{
			Name:                 "restricted",
			Description:          "Highest-intelligence agent for exceptional tasks.",
			UserInvokedOnly:      true,
			Models:               catalog.Models{Claude: "claude-fable-5-1", Codex: "gpt-6-astra", OpenCode: "openai/gpt-6-astra#high"},
			CodexReasoningEffort: "high",
			Prompt:               "Run only when asked.\n",
		},
	}
	if !reflect.DeepEqual(roles, want) {
		t.Fatalf("roles:\n%+v\nwant:\n%+v", roles, want)
	}
}

func TestLoadRolesRejectsAnInvalidCatalog(t *testing.T) {
	tests := []struct {
		name      string
		rolesYAML string
		want      string
	}{
		{
			name: "missing prompt file",
			rolesYAML: `
explorer:
  description: d
  models: {claude: haiku, codex: gpt-6-luna, opencode: openai/gpt-6-luna}
  prompt: roles/explorer.md
`,
			want: `role explorer: prompt roles/explorer.md:`,
		},
		{
			name: "unknown Harness key",
			rolesYAML: `
main:
  models: {claude: opus, codex: gpt-6, opencode: openai/gpt-6, gemini: gemini-3}
`,
			want: `role main: unknown Harness "gemini" in models (want claude, codex, opencode)`,
		},
		{
			name: "Harness without a Model",
			rolesYAML: `
main:
  models: {claude: opus, codex: gpt-6}
`,
			want: `role main: no Model for Harness "opencode"`,
		},
		{
			name: "Role other than main without a prompt",
			rolesYAML: `
explorer:
  description: d
  models: {claude: haiku, codex: gpt-6-luna, opencode: openai/gpt-6-luna}
`,
			want: `role explorer: no prompt`,
		},
		{
			name: "unknown reasoning effort",
			rolesYAML: `
main:
  models: {claude: opus, codex: gpt-6, opencode: openai/gpt-6}
  codexReasoningEffort: extreme
`,
			want: `role main: codexReasoningEffort "extreme" is not low, medium or high`,
		},
		{
			name: "misspelled field",
			rolesYAML: `
main:
  readonly: true
  models: {claude: opus, codex: gpt-6, opencode: openai/gpt-6}
`,
			want: `field readonly not found`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := catalog.LoadRoles(agentsDir(t, tt.rolesYAML, map[string]string{}))

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestLoadRolesWithoutARolesFileIsNotExist(t *testing.T) {
	_, err := catalog.LoadRoles(t.TempDir())

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("error = %v, want fs.ErrNotExist", err)
	}
}

func agentsDir(t *testing.T, rolesYAML string, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	files["roles.yaml"] = rolesYAML
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

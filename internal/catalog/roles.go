// Package catalog reads the agents directory (by default ~/.agents): the
// single source of skills, Roles and global instructions.
package catalog

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"gopkg.in/yaml.v3"
)

// MainRole is the Role whose Models configure each Harness's main agent. It
// has no prompt and produces no agent file.
const MainRole = "main"

// Role is one entry of roles.yaml with its prompt body loaded.
type Role struct {
	Name            string
	Description     string
	UserInvokedOnly bool
	ReadOnly        bool
	Models          Models
	// CodexReasoningEffort is low, medium, high or empty when unset.
	CodexReasoningEffort string
	// Prompt is the content of the Role's prompt file; empty for main.
	Prompt string
}

// Models names the Model each Harness uses for a Role.
type Models struct {
	Claude   string
	Codex    string
	OpenCode string
}

// LoadRoles reads roles.yaml in dir and the prompt files it names, relative to
// dir. Roles are returned sorted by name. A missing roles.yaml is an error
// that satisfies errors.Is(err, fs.ErrNotExist).
func LoadRoles(dir string) ([]Role, error) {
	data, err := os.ReadFile(filepath.Join(dir, "roles.yaml"))
	if err != nil {
		return nil, err
	}
	var entries map[string]roleEntry
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("roles.yaml: %w", err)
	}
	var roles []Role
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		role, err := readRole(dir, name, entries[name])
		if err != nil {
			return nil, fmt.Errorf("roles.yaml: role %s: %w", name, err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

type roleEntry struct {
	Description          string            `yaml:"description"`
	UserInvokedOnly      bool              `yaml:"userInvokedOnly"`
	ReadOnly             bool              `yaml:"readOnly"`
	Models               map[string]string `yaml:"models"`
	CodexReasoningEffort string            `yaml:"codexReasoningEffort"`
	Prompt               string            `yaml:"prompt"`
}

// harnessKeys are the keys a Role's models map must have, one per Harness.
var harnessKeys = []string{"claude", "codex", "opencode"}

func readRole(dir, name string, entry roleEntry) (Role, error) {
	models, err := readModels(entry.Models)
	if err != nil {
		return Role{}, err
	}
	switch entry.CodexReasoningEffort {
	case "", "low", "medium", "high":
	default:
		return Role{}, fmt.Errorf("codexReasoningEffort %q is not low, medium or high", entry.CodexReasoningEffort)
	}
	role := Role{
		Name:                 name,
		Description:          entry.Description,
		UserInvokedOnly:      entry.UserInvokedOnly,
		ReadOnly:             entry.ReadOnly,
		Models:               models,
		CodexReasoningEffort: entry.CodexReasoningEffort,
	}
	if name == MainRole {
		return role, nil
	}
	if entry.Prompt == "" {
		return Role{}, fmt.Errorf("no prompt")
	}
	prompt, err := os.ReadFile(filepath.Join(dir, entry.Prompt))
	if err != nil {
		return Role{}, fmt.Errorf("prompt %s: %w", entry.Prompt, err)
	}
	role.Prompt = string(prompt)
	return role, nil
}

func readModels(models map[string]string) (Models, error) {
	for key := range models {
		if !slices.Contains(harnessKeys, key) {
			return Models{}, fmt.Errorf("unknown Harness %q in models (want claude, codex, opencode)", key)
		}
	}
	for _, key := range harnessKeys {
		if models[key] == "" {
			return Models{}, fmt.Errorf("no Model for Harness %q", key)
		}
	}
	return Models{Claude: models["claude"], Codex: models["codex"], OpenCode: models["opencode"]}, nil
}

// Package config reads Overlord's own config file,
// ~/.config/overlord/config.yaml.
package config

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/PoeAudits/overlord/internal/t3"
)

// Config is Overlord's config with every path absolute.
type Config struct {
	// AgentsDir is the catalog Sync applies (skills, Roles, instructions).
	AgentsDir string `yaml:"agentsDir"`
	// ProjectsRoot is where new-project clones new Projects.
	ProjectsRoot string `yaml:"projectsRoot"`
	// GitHubOwner is the GitHub account or organisation new-project creates
	// repositories under.
	GitHubOwner string    `yaml:"githubOwner"`
	Services    []Service `yaml:"services"`
}

// Service is one process start keeps running.
type Service struct {
	Name string `yaml:"name"`
	// Command is run with sh -c when the service is not healthy.
	Command string `yaml:"command"`
	// Health is the URL that answers when the service is up.
	Health string `yaml:"health"`
	// Timeout is how long start waits for health after launching.
	Timeout time.Duration `yaml:"timeout"`
	// Enabled is nil when the file leaves it out, which means enabled.
	Enabled *bool `yaml:"enabled"`
}

// IsEnabled reports whether start manages the service.
func (s Service) IsEnabled() bool {
	return s.Enabled == nil || *s.Enabled
}

const defaultTimeout = 30 * time.Second

// Path is the config file's location under home.
func Path(home string) string {
	return filepath.Join(home, ".config", "overlord", "config.yaml")
}

// Load reads the config under home. A missing file means the defaults; Load
// never writes. Keys the file leaves out take their default value.
func Load(home string) (Config, error) {
	data, err := os.ReadFile(Path(home))
	if errors.Is(err, os.ErrNotExist) {
		data, err = []byte(defaultFile), nil
	}
	if err != nil {
		return Config{}, err
	}
	// defaultFile is the one source of default values.
	var defaults, config Config
	if err := yaml.Unmarshal([]byte(defaultFile), &defaults); err != nil {
		return Config{}, fmt.Errorf("default config: %w", err)
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("%s: %w", Path(home), err)
	}
	return config.resolve(home, defaults)
}

// WriteDefault writes the default config when there is no config file. An
// existing file is never rewritten.
func WriteDefault(home string) error {
	path := Path(home)
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultFile), 0o644)
}

// resolve fills keys the file left out from defaults and makes paths
// absolute. Services are not defaulted: a file without them starts none.
func (c Config) resolve(home string, defaults Config) (Config, error) {
	c.AgentsDir = expandHome(home, cmp.Or(c.AgentsDir, defaults.AgentsDir))
	c.ProjectsRoot = expandHome(home, cmp.Or(c.ProjectsRoot, defaults.ProjectsRoot))
	c.GitHubOwner = cmp.Or(c.GitHubOwner, defaults.GitHubOwner)
	for i, service := range c.Services {
		if service.Name == "" || service.Command == "" || service.Health == "" {
			return Config{}, fmt.Errorf("%s: service %d needs a name, a command and a health URL", Path(home), i+1)
		}
		if service.Timeout <= 0 {
			c.Services[i].Timeout = defaultTimeout
		}
		if strings.Contains(service.Health, t3OriginPlaceholder) {
			c.Services[i].Health = strings.ReplaceAll(service.Health, t3OriginPlaceholder, t3.Origin(home))
		}
	}
	return c, nil
}

// expandHome turns a leading ~ into home, so the file stays portable.
func expandHome(home, path string) string {
	if path == "~" {
		return home
	}
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, rest)
	}
	return path
}

// t3OriginPlaceholder in a health URL stands for T3 Code's current origin, so
// the default t3 service follows T3 to another port without a config edit.
const t3OriginPlaceholder = "{t3Origin}"

// defaultFile is the config written on first run.
const defaultFile = `# Overlord's config. Written once with defaults; Overlord never rewrites it.
#
# agentsDir: the source of skills, Roles and global instructions for Sync.
# projectsRoot: where new-project clones new Projects.
# githubOwner: the GitHub account or organisation new-project creates
# repositories under.
# services: what start brings up, in order. A service is healthy when its
# health URL answers with any HTTP status below 500. When it is not, start
# runs command with sh -c, detached, output appended to
# ~/.local/state/overlord/<name>.log, and waits up to timeout for health.
# Set enabled: false to skip a service.
# {t3Origin} in a health URL is replaced, each time start runs, with the
# origin T3 Code's server last ran on (from
# ~/.t3/userdata/server-runtime.json; http://127.0.0.1:3773 when absent).
agentsDir: ~/.agents
projectsRoot: ~/projects
githubOwner: PoeAudits
services:
  - name: t3
    command: ~/.t3/bin/t3 service install
    health: "` + t3OriginPlaceholder + `/.well-known/t3/environment"
    timeout: 30s
  - name: sleev
    command: sleev gateway start
    health: http://127.0.0.1:17321/__sleeve_local/health
    timeout: 30s
  - name: opencode
    command: opencode serve --hostname 127.0.0.1 --port 4096
    # /api/info answers 401 without the server password, which proves the
    # API is up; other paths return the web page even when the API is broken.
    health: http://127.0.0.1:4096/api/info
    timeout: 30s
`

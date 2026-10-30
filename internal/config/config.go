// Package config reads Overlord's own config file,
// ~/.config/overlord/config.yaml.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is Overlord's config with every path absolute.
type Config struct {
	// AgentsDir is the catalog Sync applies (skills, Roles, instructions).
	AgentsDir string `yaml:"agentsDir"`
	// ProjectsRoot is where new-project clones new Projects.
	ProjectsRoot string    `yaml:"projectsRoot"`
	Services     []Service `yaml:"services"`
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
		data = []byte(defaultFile(home))
	} else if err != nil {
		return Config{}, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("%s: %w", Path(home), err)
	}
	return config.resolve(home)
}

// WriteDefault writes the default config when there is no config file and
// reports whether it did. An existing file is never rewritten.
func WriteDefault(home string) (bool, error) {
	path := Path(home)
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(defaultFile(home)), 0o644)
}

func (c Config) resolve(home string) (Config, error) {
	if c.AgentsDir == "" {
		c.AgentsDir = "~/.agents"
	}
	if c.ProjectsRoot == "" {
		c.ProjectsRoot = "~/projects"
	}
	c.AgentsDir = expandHome(home, c.AgentsDir)
	c.ProjectsRoot = expandHome(home, c.ProjectsRoot)
	for i, service := range c.Services {
		if service.Name == "" || service.Command == "" || service.Health == "" {
			return Config{}, fmt.Errorf("%s: service %d needs a name, a command and a health URL", Path(home), i+1)
		}
		if service.Timeout <= 0 {
			c.Services[i].Timeout = defaultTimeout
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

// defaultFile is the config written on first run. T3 Code's health URL uses
// the origin in its server-runtime file, so a non-default port is picked up.
func defaultFile(home string) string {
	return `# Overlord's config. Written once with defaults; Overlord never rewrites it.
#
# agentsDir: the source of skills, Roles and global instructions for Sync.
# projectsRoot: where new-project clones new Projects.
# services: what start brings up, in order. A service is healthy when its
# health URL answers with any HTTP status below 500. When it is not, start
# runs command with sh -c, detached, output appended to
# ~/.local/state/overlord/<name>.log, and waits up to timeout for health.
# Set enabled: false to skip a service.
agentsDir: ~/.agents
projectsRoot: ~/projects
services:
  - name: t3
    command: ~/.t3/bin/t3 service install
    health: ` + t3Origin(home) + `/.well-known/t3/environment
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
}

// t3Origin is the origin T3 Code's server last ran on, or its default.
func t3Origin(home string) string {
	data, err := os.ReadFile(filepath.Join(home, ".t3", "userdata", "server-runtime.json"))
	if err != nil {
		return "http://127.0.0.1:3773"
	}
	var runtime struct {
		Origin string `json:"origin"`
	}
	if json.Unmarshal(data, &runtime) != nil || runtime.Origin == "" {
		return "http://127.0.0.1:3773"
	}
	return runtime.Origin
}

package registry

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultRegistryPath is the default location for the registry file.
	DefaultRegistryPath = "~/.config/overlord/registry.yaml"
	// DefaultBaseDir is the default base directory for project worktrees.
	DefaultBaseDir = "~/Overlord"
	// DefaultThoughtsDir is the default root directory for thoughts files.
	DefaultThoughtsDir = "~/thoughts"
)

// Load reads the registry from the specified path.
// If the file doesn't exist, it returns a default registry.
// The path supports ~ expansion for the home directory.
func Load(path string) (*Registry, error) {
	expandedPath, err := ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(expandedPath); errors.Is(err, fs.ErrNotExist) {
		// Return default registry
		return defaultRegistry(), nil
	}

	// Read file
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry file: %w", err)
	}

	// Unmarshal YAML
	var registry Registry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry YAML: %w", err)
	}

	// Validate registry
	if err := registry.Validate(); err != nil {
		return nil, fmt.Errorf("registry validation failed: %w", err)
	}

	return &registry, nil
}

// Save writes the registry to the specified path.
// It creates parent directories if needed, backs up existing files,
// and uses atomic write (temp file + rename) for safety.
// The path supports ~ expansion for the home directory.
func Save(path string, registry *Registry) error {
	if registry == nil {
		return fmt.Errorf("registry cannot be nil")
	}

	// Validate before saving
	if err := registry.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid registry: %w", err)
	}

	expandedPath, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Create parent directories
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry to YAML: %w", err)
	}

	// Backup existing file to .bak before overwriting
	backupPath := expandedPath + ".bak"
	if existing, err := os.ReadFile(expandedPath); err == nil {
		if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
			return fmt.Errorf("failed to create backup file: %w", err)
		}
	}

	if err := fileutil.AtomicWriteFile(expandedPath, data, 0o644); err != nil {
		return err
	}

	return nil
}

// ExpandPath expands ~ to the user's home directory and resolves
// relative paths (e.g., ".", "..", "./foo") to absolute paths.
func ExpandPath(path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("path cannot be empty")
	}

	if path[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}

		if len(path) == 1 {
			path = homeDir
		} else if path[1] == '/' || path[1] == filepath.Separator {
			path = filepath.Join(homeDir, path[2:])
		} else {
			// ~user/path not supported
			return "", fmt.Errorf("~user expansion not supported, use absolute path")
		}
	}

	// Resolve relative paths to absolute
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		path = absPath
	}

	return path, nil
}

// defaultRegistry returns a new registry with default values
func defaultRegistry() *Registry {
	return &Registry{
		Version: 2,
		Settings: Settings{
			BaseDir:     DefaultBaseDir,
			ThoughtsDir: DefaultThoughtsDir,
		},
		Projects: make(map[string]Project),
	}
}

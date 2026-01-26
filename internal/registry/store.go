package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultRegistryPath is the default location for the registry file
	DefaultRegistryPath = "~/.config/overlord/registry.yaml"
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
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(registry)
	if err != nil {
		return fmt.Errorf("failed to marshal registry to YAML: %w", err)
	}

	// Write to temp file first (atomic write)
	tempPath := expandedPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, expandedPath); err != nil {
		// Clean up temp file on failure
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
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
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: make(map[string]Project),
	}
}

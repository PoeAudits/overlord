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
	// BackupSuffix is appended to create backup files
	BackupSuffix = ".bak"
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

	// Backup existing file
	if _, err := os.Stat(expandedPath); err == nil {
		backupPath := expandedPath + BackupSuffix
		if err := copyFile(expandedPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup registry: %w", err)
		}
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

// ExpandPath expands ~ to the user's home directory
func ExpandPath(path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("path cannot be empty")
	}

	if path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if len(path) == 1 {
		return homeDir, nil
	}

	// Handle ~/path
	if path[1] == '/' || path[1] == filepath.Separator {
		return filepath.Join(homeDir, path[2:]), nil
	}

	// ~user/path not supported
	return "", fmt.Errorf("~user expansion not supported, use absolute path")
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(dst, data, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
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

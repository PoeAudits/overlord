package machine

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigPath is the default location for the machine config file
	DefaultConfigPath = "~/.config/overlord/machine.yaml"
)

// Role represents the machine's role in the sync system
type Role string

// Role constants
const (
	RoleStorage    Role = "storage"
	RoleWorkingSet Role = "working-set"
)

// IsValid checks if the role is valid
func (r Role) IsValid() bool {
	switch r {
	case RoleStorage, RoleWorkingSet:
		return true
	}
	return false
}

// MachineConfig represents the machine-specific configuration
type MachineConfig struct {
	Name        string `yaml:"name"`
	Role        Role   `yaml:"role"`
	StorageHost string `yaml:"storage_host,omitempty"`
}

// Validate checks if the machine config is valid
func (m *MachineConfig) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("machine name cannot be empty")
	}

	if !m.Role.IsValid() {
		return fmt.Errorf("invalid role: %s (must be 'storage' or 'working-set')", m.Role)
	}

	// StorageHost is required for working-set machines
	if m.Role == RoleWorkingSet && m.StorageHost == "" {
		return fmt.Errorf("storage_host is required for working-set role")
	}

	// StorageHost should not be set for storage machines
	if m.Role == RoleStorage && m.StorageHost != "" {
		return fmt.Errorf("storage_host should not be set for storage role")
	}

	return nil
}

// Save writes the machine config to the specified path.
// It validates the config, creates parent directories if needed,
// and uses atomic write (temp file + rename) for safety.
// The path supports ~ expansion for the home directory.
func Save(path string, config *MachineConfig) error {
	if config == nil {
		return fmt.Errorf("machine config cannot be nil")
	}

	// Validate before saving
	if err := config.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid machine config: %w", err)
	}

	expandedPath, err := expandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	// Create parent directories
	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal machine config to YAML: %w", err)
	}

	// Write to temp file first (atomic write)
	tempPath := expandedPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, expandedPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Load reads the machine config from the specified path.
// If the file doesn't exist, it returns a default config.
// The path supports ~ expansion for the home directory.
func Load(path string) (*MachineConfig, error) {
	expandedPath, err := expandPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		// Return default config
		return defaultConfig()
	}

	// Read file
	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read machine config file: %w", err)
	}

	// Unmarshal YAML
	var config MachineConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse machine config YAML: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("machine config validation failed: %w", err)
	}

	return &config, nil
}

// defaultConfig returns a new machine config with default values
func defaultConfig() (*MachineConfig, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	return &MachineConfig{
		Name: hostname,
		Role: RoleStorage,
	}, nil
}

// expandPath expands ~ to the user's home directory and resolves
// relative paths (e.g., ".", "..", "./foo") to absolute paths.
func expandPath(path string) (string, error) {
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

package machine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/registry"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigPath is the default location for the machine config file
	DefaultConfigPath = "~/.config/overlord/machine.yaml"
	// DefaultPeersPath is the default location for shared peer configuration.
	DefaultPeersPath = "~/.config/overlord/peers.yaml"
)

// Role represents a machine role in a multi-machine setup.
type Role string

const (
	RoleMain   Role = "main"
	RoleVPS    Role = "vps"
	RoleRunner Role = "runner"
	RoleAux    Role = "aux"
)

// ValidRoles returns all supported machine roles.
func ValidRoles() []Role {
	return []Role{RoleMain, RoleVPS, RoleRunner, RoleAux}
}

// IsValid checks whether the role is one of the supported values.
func (r Role) IsValid() bool {
	for _, valid := range ValidRoles() {
		if r == valid {
			return true
		}
	}
	return false
}

// MachineConfig represents the machine-specific configuration
type MachineConfig struct {
	Name      string          `yaml:"name"`
	Role      Role            `yaml:"role,omitempty"`
	Syncthing SyncthingConfig `yaml:"syncthing,omitempty"`
}

// SyncthingConfig represents Syncthing API configuration for this machine.
type SyncthingConfig struct {
	DeviceID string `yaml:"device_id,omitempty"`
	APIURL   string `yaml:"api_url,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
	Address  string `yaml:"address,omitempty"`
}

// Validate checks if the machine config is valid
func (m *MachineConfig) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("machine name cannot be empty")
	}

	if err := m.Syncthing.Validate(); err != nil {
		return err
	}

	if m.Role != "" && !m.Role.IsValid() {
		return fmt.Errorf("invalid machine role %q", m.Role)
	}

	return nil
}

// Validate checks if the Syncthing configuration is valid.
// Empty config is permitted for backward compatibility.
func (c SyncthingConfig) Validate() error {
	if c.isZero() {
		return nil
	}

	if c.DeviceID == "" || c.APIURL == "" || c.APIKey == "" {
		return fmt.Errorf("syncthing config requires device_id, api_url, and api_key")
	}

	if c.Address == "" {
		return fmt.Errorf("syncthing config requires address")
	}

	return nil
}

func (c SyncthingConfig) isZero() bool {
	return c.DeviceID == "" && c.APIURL == "" && c.APIKey == "" && c.Address == ""
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

	expandedPath, err := registry.ExpandPath(path)
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
	expandedPath, err := registry.ExpandPath(path)
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
	}, nil
}

package machine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"gopkg.in/yaml.v3"

	"github.com/PoeAudits/overlord/internal/registry"
)

// PeersConfig represents shared peer metadata synced between machines.
type PeersConfig struct {
	Peers map[string]Peer `yaml:"peers"`
}

// MachineView combines local machine config with shared peer metadata.
type MachineView struct {
	Machine *MachineConfig
	Peers   *PeersConfig
}

// Peer represents one known machine peer.
type Peer struct {
	Role      Role          `yaml:"role"`
	Syncthing PeerSyncthing `yaml:"syncthing"`
	BaseDir   string        `yaml:"base_dir,omitempty"`
}

// PeerSyncthing contains non-sensitive Syncthing metadata for a peer.
type PeerSyncthing struct {
	DeviceID string `yaml:"device_id"`
	Address  string `yaml:"address"`
}

// Validate checks if the peer registry is valid.
func (p *PeersConfig) Validate() error {
	if p == nil {
		return fmt.Errorf("peers config cannot be nil")
	}

	for name, peer := range p.Peers {
		if name == "" {
			return fmt.Errorf("peer name cannot be empty")
		}
		if err := peer.Validate(); err != nil {
			return fmt.Errorf("peer %q: %w", name, err)
		}
	}

	return nil
}

// Validate checks if a single peer definition is valid.
func (p Peer) Validate() error {
	if p.Role == "" {
		return fmt.Errorf("role is required")
	}
	if !p.Role.IsValid() {
		return fmt.Errorf("invalid role %q", p.Role)
	}
	if err := p.Syncthing.Validate(); err != nil {
		return err
	}
	if p.BaseDir != "" && !filepath.IsAbs(p.BaseDir) {
		return fmt.Errorf("base_dir must be an absolute path")
	}
	return nil
}

// Validate checks if peer Syncthing metadata is valid.
func (p PeerSyncthing) Validate() error {
	if p.DeviceID == "" {
		return fmt.Errorf("syncthing.device_id is required")
	}
	if p.Address == "" {
		return fmt.Errorf("syncthing.address is required")
	}
	return nil
}

// LoadPeers reads peers config from path.
// Missing file returns an empty peers config.
func LoadPeers(path string) (*PeersConfig, error) {
	expandedPath, err := registry.ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	if _, err := os.Stat(expandedPath); errors.Is(err, fs.ErrNotExist) {
		return &PeersConfig{Peers: map[string]Peer{}}, nil
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read peers config file: %w", err)
	}

	var cfg PeersConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse peers config YAML: %w", err)
	}

	if cfg.Peers == nil {
		cfg.Peers = map[string]Peer{}
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("peers config validation failed: %w", err)
	}

	return &cfg, nil
}

// SavePeers writes peers config atomically to path.
func SavePeers(path string, config *PeersConfig) error {
	if config == nil {
		return fmt.Errorf("peers config cannot be nil")
	}
	if err := config.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid peers config: %w", err)
	}

	expandedPath, err := registry.ExpandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal peers config to YAML: %w", err)
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

// LoadWithPeers returns local machine config and shared peers in one value.
func LoadWithPeers(machinePath, peersPath string) (*MachineView, error) {
	machineConfig, err := Load(machinePath)
	if err != nil {
		return nil, err
	}

	peersConfig, err := LoadPeers(peersPath)
	if err != nil {
		return nil, err
	}

	return &MachineView{Machine: machineConfig, Peers: peersConfig}, nil
}

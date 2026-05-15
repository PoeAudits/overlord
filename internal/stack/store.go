package stack

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"gopkg.in/yaml.v3"
)

// Load reads the stack config from the specified path.
// If the file doesn't exist, it returns a default config.
// The path supports ~ expansion for the home directory.
func Load(path string) (*StackConfig, error) {
	expandedPath, err := registry.ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	if _, err := os.Stat(expandedPath); errors.Is(err, fs.ErrNotExist) {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack config file: %w", err)
	}

	var config StackConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse stack config YAML: %w", err)
	}

	return &config, nil
}

// Save writes the stack config to the specified path.
// It creates parent directories if needed, backs up existing files,
// and uses atomic write (temp file + rename) for safety.
// The path supports ~ expansion for the home directory.
func Save(path string, config *StackConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
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
		return fmt.Errorf("failed to marshal stack config to YAML: %w", err)
	}

	backupPath := expandedPath + ".bak"
	if _, err := os.Stat(expandedPath); err == nil {
		if err := os.Rename(expandedPath, backupPath); err != nil {
			return fmt.Errorf("failed to create backup file: %w", err)
		}
	}

	if err := fileutil.AtomicWriteFile(expandedPath, data, 0o644); err != nil {
		return err
	}

	return nil
}

// DefaultConfig returns a new stack config with default values.
// It is used as the initial configuration when no stack.yaml exists.
func DefaultConfig() *StackConfig {
	return &StackConfig{
		Version:  1,
		Projects: []string{},
		Services: map[string]ServiceConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]string{
					openCodeHostKey: defaultOpenCodeHost,
					openCodePortKey: strconv.Itoa(defaultOpenCodePort),
				},
			},
			t3CodeServiceName: {
				Enabled: true,
				Settings: map[string]string{
					t3CodeHostKey:    defaultT3CodeHost,
					t3CodePortKey:    strconv.Itoa(defaultT3CodePort),
					t3CodeWorkDirKey: defaultT3CodeDir,
					t3CodeBaseDirKey: defaultT3CodeBaseDir,
				},
			},
			"frontend": {
				Enabled: true,
				Settings: map[string]string{
					frontEndPortKey:    strconv.Itoa(defaultFrontEndPort),
					frontEndWorkDirKey: defaultFrontEndDir,
				},
			},
			"tailscale": {
				Enabled: true,
				Settings: map[string]string{
					tailscaleTargetKey: T3CodeURL(nil),
				},
			},
			"ollama": {
				Enabled:  true,
				Settings: map[string]string{},
			},
		},
	}
}

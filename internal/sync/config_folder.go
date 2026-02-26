package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

const ConfigFolderID = "overlord-config"

// folderEnsurer is the subset of *syncthing.Client used by EnsureConfigFolder.
// It is unexported so callers must pass *syncthing.Client; tests may use a fake.
type folderEnsurer interface {
	EnsureFolder(ctx context.Context, folder syncthing.Folder) (bool, error)
}

// EnsureConfigFolder registers the Overlord config directory as a Syncthing folder.
func EnsureConfigFolder(ctx context.Context, client folderEnsurer, machineConfig *machine.MachineConfig, peersConfig *machine.PeersConfig) error {
	if err := RequireConfig(machineConfig); err != nil {
		return err
	}

	configDir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := EnsureConfigIgnoreFile(configDir); err != nil {
		return err
	}

	deviceIDs := make([]string, 0, 1)
	deviceIDs = append(deviceIDs, machineConfig.Syncthing.DeviceID)
	if peersConfig != nil {
		for _, peer := range peersConfig.Peers {
			if peer.Syncthing.DeviceID != "" {
				deviceIDs = append(deviceIDs, peer.Syncthing.DeviceID)
			}
		}
	}

	seen := make(map[string]struct{}, len(deviceIDs))
	devices := make([]syncthing.FolderDevice, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		devices = append(devices, syncthing.FolderDevice{DeviceID: id})
	}

	folder := syncthing.Folder{
		ID:               ConfigFolderID,
		Label:            "overlord-config",
		Path:             configDir,
		Type:             "sendreceive",
		Devices:          devices,
		FSWatcherEnabled: true,
		Copiers:          1,
		Hashers:          1,
		CaseSensitiveFS:  true,
	}

	if _, err := client.EnsureFolder(ctx, folder); err != nil {
		return fmt.Errorf("failed to configure overlord config syncthing folder: %w", err)
	}

	return nil
}

// ConfigDir returns the Overlord config directory.
func ConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "overlord"), nil
}

// EnsureConfigIgnoreFile updates the config .stignore with required entries.
func EnsureConfigIgnoreFile(configDir string) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	ignorePath := filepath.Join(configDir, ".stignore")

	if err := ensureFileContainsLines(ignorePath, []string{"machine.yaml"}, nil); err != nil {
		return fmt.Errorf("failed to update %s: %w", ignorePath, err)
	}

	return nil
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

const overlordConfigFolderID = "overlord-config"

func ensureOverlordConfigSyncthingFolder(ctx context.Context, machineConfig *machine.MachineConfig, peersConfig *machine.PeersConfig) error {
	client, err := localSyncthingClient(machineConfig)
	if err != nil {
		return err
	}

	configDir, err := registryConfigDir()
	if err != nil {
		return err
	}

	if err := ensureOverlordConfigIgnoreFile(configDir); err != nil {
		return err
	}

	deviceIDs := make([]string, 0, 1+len(peersConfig.Peers))
	deviceIDs = append(deviceIDs, machineConfig.Syncthing.DeviceID)
	for _, peer := range peersConfig.Peers {
		if peer.Syncthing.DeviceID != "" {
			deviceIDs = append(deviceIDs, peer.Syncthing.DeviceID)
		}
	}

	devices := make([]syncthing.FolderDevice, 0, len(deviceIDs))
	seen := make(map[string]struct{}, len(deviceIDs))
	for _, id := range deviceIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		devices = append(devices, syncthing.FolderDevice{DeviceID: id})
	}

	folder := syncthing.Folder{
		ID:               overlordConfigFolderID,
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

func registryConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "overlord"), nil
}

func ensureOverlordConfigIgnoreFile(configDir string) error {
	ignorePath := filepath.Join(configDir, ".stignore")
	entry := "machine.yaml"

	content, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", ignorePath, err)
	}

	text := string(content)
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil
		}
	}

	var builder strings.Builder
	if len(text) > 0 {
		builder.WriteString(text)
		if !strings.HasSuffix(text, "\n") {
			builder.WriteString("\n")
		}
	}
	builder.WriteString(entry)
	builder.WriteString("\n")

	if err := os.WriteFile(ignorePath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", ignorePath, err)
	}

	return nil
}

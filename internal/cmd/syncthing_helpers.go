package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

type syncthingPeerSelection struct {
	name string
	peer machine.Peer
}

func resolveSyncthingPeer(peersConfig *machine.PeersConfig, requested string) (syncthingPeerSelection, error) {
	if peersConfig == nil {
		return syncthingPeerSelection{}, fmt.Errorf("peers config is not loaded")
	}

	peers := peersConfig.Peers
	if len(peers) == 0 {
		return syncthingPeerSelection{}, fmt.Errorf("no syncthing peers configured in peers.yaml")
	}

	if requested != "" {
		peer, ok := peers[requested]
		if !ok {
			return syncthingPeerSelection{}, fmt.Errorf("syncthing peer '%s' not found in peers.yaml", requested)
		}
		return syncthingPeerSelection{name: requested, peer: peer}, nil
	}

	if len(peers) == 1 {
		for name, peer := range peers {
			return syncthingPeerSelection{name: name, peer: peer}, nil
		}
	}

	names := make([]string, 0, len(peers))
	for name := range peers {
		names = append(names, name)
	}
	sort.Strings(names)
	return syncthingPeerSelection{}, fmt.Errorf("multiple syncthing peers configured; choose one with --peer (%s)", strings.Join(names, ", "))
}

func requireSyncthingConfig(config *machine.MachineConfig) error {
	if config.Syncthing.DeviceID == "" || config.Syncthing.APIURL == "" || config.Syncthing.APIKey == "" || config.Syncthing.Address == "" {
		return fmt.Errorf("syncthing config missing: set syncthing.device_id, syncthing.api_url, syncthing.api_key, syncthing.address in machine.yaml")
	}
	return nil
}

func localSyncthingClient(config *machine.MachineConfig) (*syncthing.Client, error) {
	if err := requireSyncthingConfig(config); err != nil {
		return nil, err
	}
	return syncthing.NewClient(config.Syncthing.APIURL, config.Syncthing.APIKey), nil
}

func buildSyncthingFolder(projectName, projectPath string, localDeviceID, peerDeviceID string) syncthing.Folder {
	devices := []syncthing.FolderDevice{
		{DeviceID: localDeviceID},
		{DeviceID: peerDeviceID},
	}

	return syncthing.Folder{
		ID:               projectName,
		Label:            projectName,
		Path:             projectPath,
		Type:             "sendreceive",
		Devices:          devices,
		FSWatcherEnabled: true,
		Copiers:          1,
		Hashers:          1,
		CaseSensitiveFS:  true,
	}
}

func ensureIgnoreFile(projectPath string) error {
	globalIgnorePath, err := syncthingGlobalIgnorePath()
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(projectPath, globalIgnorePath)
	if err != nil {
		return fmt.Errorf("failed to build ignore include path: %w", err)
	}

	includeLine := "#include " + filepath.ToSlash(rel)
	ignorePath := filepath.Join(projectPath, ".stignore")

	content, err := os.ReadFile(ignorePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read .stignore: %w", err)
	}

	if strings.Contains(string(content), includeLine) {
		return nil
	}

	var builder strings.Builder
	if len(content) > 0 {
		builder.WriteString(string(content))
		if !strings.HasSuffix(string(content), "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString(includeLine)
		builder.WriteString("\n")
	} else {
		builder.WriteString(includeLine)
		builder.WriteString("\n")
	}

	if err := os.WriteFile(ignorePath, []byte(builder.String()), 0644); err != nil {
		return fmt.Errorf("failed to write .stignore: %w", err)
	}

	return nil
}

func syncthingGlobalIgnorePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "syncthing", ".stglobalignore"), nil
}

func projectPathForPeer(project registry.Project, regBaseDir string, peerBaseDir string) (string, error) {
	baseDir := regBaseDir
	if peerBaseDir != "" {
		baseDir = peerBaseDir
	}
	if project.Path == "" {
		return "", fmt.Errorf("project path is empty")
	}
	if filepath.IsAbs(project.Path) {
		return project.Path, nil
	}
	if peerBaseDir == "" && regBaseDir != "" && !filepath.IsAbs(regBaseDir) {
		return "", fmt.Errorf("peer base_dir required for relative project paths")
	}
	return resolveProjectPath(project.Path, baseDir), nil
}

func ensureSyncthingDevice(ctx context.Context, client *syncthing.Client, deviceID, name string, address string) error {
	device := syncthing.Device{
		DeviceID:    deviceID,
		Name:        name,
		Addresses:   []string{address},
		Compression: "metadata",
	}
	_, err := client.EnsureDevice(ctx, device)
	return err
}

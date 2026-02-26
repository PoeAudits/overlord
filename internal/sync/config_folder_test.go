package sync

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

func TestConfigDirUsesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := ConfigDir()
	if err != nil {
		t.Fatalf("config dir error: %v", err)
	}
	expected := filepath.Join(home, ".config", "overlord")
	if dir != expected {
		t.Fatalf("expected %s, got %s", expected, dir)
	}
}

func TestEnsureConfigIgnoreFileAddsEntryOnce(t *testing.T) {
	configDir := t.TempDir()
	if err := EnsureConfigIgnoreFile(configDir); err != nil {
		t.Fatalf("ensure ignore failed: %v", err)
	}
	ignorePath := filepath.Join(configDir, ".stignore")
	contents, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read ignore: %v", err)
	}
	if !strings.Contains(string(contents), "machine.yaml") {
		t.Fatalf("machine.yaml entry missing: %q", contents)
	}
	if err := EnsureConfigIgnoreFile(configDir); err != nil {
		t.Fatalf("second ensure should succeed: %v", err)
	}
	contents, err = os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read ignore: %v", err)
	}
	if strings.Count(string(contents), "machine.yaml") != 1 {
		t.Fatalf("machine.yaml entry should not duplicate: %q", contents)
	}
}

func TestEnsureConfigFolderEnsuresSyncthingFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	machineConfig := &machine.MachineConfig{
		Syncthing: machine.SyncthingConfig{
			DeviceID: "local-id",
			APIURL:   "http://localhost:8384",
			APIKey:   "abc",
			Address:  "tcp://localhost",
		},
	}
	peers := &machine.PeersConfig{
		Peers: map[string]machine.Peer{
			"alpha": {Syncthing: machine.PeerSyncthing{DeviceID: "peer-a"}},
			"beta":  {Syncthing: machine.PeerSyncthing{DeviceID: "peer-b"}},
			"dup":   {Syncthing: machine.PeerSyncthing{DeviceID: "peer-a"}},
			"idle":  {Syncthing: machine.PeerSyncthing{}},
		},
	}
	mock := &mockFolderClient{}
	if err := EnsureConfigFolder(context.Background(), mock, machineConfig, peers); err != nil {
		t.Fatalf("ensure config folder returned error: %v", err)
	}
	if !mock.called {
		t.Fatalf("expected EnsureFolder to be invoked")
	}
	configDir, err := ConfigDir()
	if err != nil {
		t.Fatalf("config dir error: %v", err)
	}
	if mock.folder.ID != "overlord-config" {
		t.Fatalf("unexpected folder id: %s", mock.folder.ID)
	}
	if mock.folder.Path != configDir {
		t.Fatalf("folder should point to config dir: %s", mock.folder.Path)
	}
	if mock.folder.Type != "sendreceive" {
		t.Fatalf("expected sendreceive type, got %s", mock.folder.Type)
	}
	ids := make(map[string]struct{})
	for _, dev := range mock.folder.Devices {
		ids[dev.DeviceID] = struct{}{}
	}
	if len(ids) != 3 {
		t.Fatalf("expected three unique device IDs (local + peers), got %d", len(ids))
	}
	ignorePath := filepath.Join(configDir, ".stignore")
	contents, err := os.ReadFile(ignorePath)
	if err != nil {
		t.Fatalf("read config .stignore: %v", err)
	}
	if !strings.Contains(string(contents), "machine.yaml") {
		t.Fatalf("expected machine.yaml ignore entry in config dir")
	}

	mock.err = errors.New("boom")
	if err := EnsureConfigFolder(context.Background(), mock, machineConfig, peers); err == nil {
		t.Fatalf("expected error propagation when client fails")
	}
}

type mockFolderClient struct {
	folder syncthing.Folder
	called bool
	err    error
}

func (m *mockFolderClient) EnsureFolder(_ context.Context, folder syncthing.Folder) (bool, error) {
	m.folder = folder
	m.called = true
	return true, m.err
}

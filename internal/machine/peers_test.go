package machine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPeersConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *PeersConfig
		wantErr bool
	}{
		{
			name: "valid peers config",
			config: &PeersConfig{Peers: map[string]Peer{
				"main": {
					Role: RoleMain,
					Syncthing: PeerSyncthing{
						DeviceID: "AAAA-BBBB-CCCC-DDDD",
						Address:  "tcp://100.64.0.1:22000",
					},
					BaseDir: "/home/thomas/Overlord",
				},
			}},
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "invalid peer role",
			config: &PeersConfig{Peers: map[string]Peer{
				"main": {
					Role:      Role("invalid"),
					Syncthing: PeerSyncthing{DeviceID: "A", Address: "tcp://100.64.0.1:22000"},
				},
			}},
			wantErr: true,
		},
		{
			name: "missing device id",
			config: &PeersConfig{Peers: map[string]Peer{
				"main": {
					Role:      RoleMain,
					Syncthing: PeerSyncthing{Address: "tcp://100.64.0.1:22000"},
				},
			}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveLoadPeers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")

	original := &PeersConfig{Peers: map[string]Peer{
		"vps": {
			Role: RoleVPS,
			Syncthing: PeerSyncthing{
				DeviceID: "VVVV-YYYY-ZZZZ",
				Address:  "tcp://100.64.0.2:22000",
			},
			BaseDir: "/srv/overlord",
		},
	}}

	if err := SavePeers(path, original); err != nil {
		t.Fatalf("SavePeers() error = %v", err)
	}

	loaded, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers() error = %v", err)
	}

	peer, ok := loaded.Peers["vps"]
	if !ok {
		t.Fatalf("expected peer vps to exist")
	}
	if peer.Role != RoleVPS {
		t.Fatalf("role = %q, want %q", peer.Role, RoleVPS)
	}
	if peer.Syncthing.DeviceID != "VVVV-YYYY-ZZZZ" {
		t.Fatalf("device_id = %q", peer.Syncthing.DeviceID)
	}
	if peer.BaseDir != "/srv/overlord" {
		t.Fatalf("base_dir = %q", peer.BaseDir)
	}
}

func TestLoadPeersMissingFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")

	cfg, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers() error = %v", err)
	}
	if len(cfg.Peers) != 0 {
		t.Fatalf("expected empty peers map")
	}
}

func TestLoadPeersMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.yaml")

	if err := os.WriteFile(path, []byte("peers: [bad"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadPeers(path); err == nil {
		t.Fatalf("expected YAML parse error")
	}
}

package sync

import (
	"testing"

	"github.com/PoeAudits/overlord/internal/machine"
)

func TestResolvePeerRequestedPeer(t *testing.T) {
	peers := &machine.PeersConfig{
		Peers: map[string]machine.Peer{
			"vps": {
				Role: machine.RoleVPS,
				Syncthing: machine.PeerSyncthing{
					DeviceID: "vps-id",
					Address:  "tcp://vps",
				},
			},
		},
	}

	selection, err := ResolvePeer(peers, "vps")
	if err != nil {
		t.Fatalf("expected selection, got error: %v", err)
	}
	if selection.Name != "vps" {
		t.Fatalf("expected peer name 'vps', got %q", selection.Name)
	}
	if selection.Peer.Syncthing.DeviceID != "vps-id" {
		t.Fatalf("expected device id 'vps-id', got %q", selection.Peer.Syncthing.DeviceID)
	}
}

func TestResolvePeerSinglePeerWithoutRequest(t *testing.T) {
	peers := &machine.PeersConfig{
		Peers: map[string]machine.Peer{
			"primary": {
				Role: machine.RoleMain,
				Syncthing: machine.PeerSyncthing{
					DeviceID: "primary-id",
					Address:  "tcp://primary",
				},
			},
		},
	}

	selection, err := ResolvePeer(peers, "")
	if err != nil {
		t.Fatalf("expected automatic selection, got error: %v", err)
	}
	if selection.Name != "primary" {
		t.Fatalf("expected 'primary', got %q", selection.Name)
	}
}

func TestResolvePeerErrors(t *testing.T) {
	t.Run("no peers configured", func(t *testing.T) {
		peers := &machine.PeersConfig{Peers: map[string]machine.Peer{}}
		if _, err := ResolvePeer(peers, ""); err == nil {
			t.Fatalf("expected error when no peers are configured")
		}
	})

	t.Run("unknown peer requested", func(t *testing.T) {
		peers := &machine.PeersConfig{Peers: map[string]machine.Peer{"alpha": {}}}
		if _, err := ResolvePeer(peers, "beta"); err == nil {
			t.Fatalf("expected error for unknown peer")
		}
	})

	t.Run("multiple peers without explicit selection", func(t *testing.T) {
		peers := &machine.PeersConfig{
			Peers: map[string]machine.Peer{
				"alpha": {},
				"beta":  {},
			},
		}
		if _, err := ResolvePeer(peers, ""); err == nil {
			t.Fatalf("expected error when multiple peers exist without selection")
		}
	})
}

func TestRequireConfigValidation(t *testing.T) {
	valid := &machine.MachineConfig{
		Name: "local",
		Syncthing: machine.SyncthingConfig{
			DeviceID: "local-id",
			APIURL:   "http://localhost:8384",
			APIKey:   "abc",
			Address:  "tcp://local",
		},
	}

	if err := RequireConfig(valid); err != nil {
		t.Fatalf("expected config to be valid: %v", err)
	}

	missing := &machine.MachineConfig{Name: "local"}
	if err := RequireConfig(missing); err == nil {
		t.Fatalf("expected error for missing syncthing config")
	}
}

func TestLocalClientRequiresConfig(t *testing.T) {
	cfg := &machine.MachineConfig{Name: "local"}
	if _, err := LocalClient(cfg); err == nil {
		t.Fatalf("expected error when config is incomplete")
	}
}

func TestLocalClientCreatesClient(t *testing.T) {
	cfg := &machine.MachineConfig{
		Name: "local",
		Syncthing: machine.SyncthingConfig{
			DeviceID: "local-id",
			APIURL:   "http://localhost:8384",
			APIKey:   "abc",
			Address:  "tcp://local",
		},
	}

	client, err := LocalClient(cfg)
	if err != nil {
		t.Fatalf("expected client, got error: %v", err)
	}
	if client == nil {
		t.Fatalf("expected non-nil client")
	}
}

package sync

import (
	"fmt"
	"sort"
	"strings"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/syncthing"
)

// PeerSelection represents the resolved peer used for sync operations.
type PeerSelection struct {
	Name string
	Peer machine.Peer
}

// ResolvePeer selects a peer for sync operations.
func ResolvePeer(peers *machine.PeersConfig, requested string) (PeerSelection, error) {
	if peers == nil {
		return PeerSelection{}, fmt.Errorf("peers config is not loaded")
	}

	peerMap := peers.Peers
	if len(peerMap) == 0 {
		return PeerSelection{}, fmt.Errorf("no syncthing peers configured in peers.yaml")
	}

	if requested != "" {
		peer, ok := peerMap[requested]
		if !ok {
			return PeerSelection{}, fmt.Errorf("syncthing peer '%s' not found in peers.yaml", requested)
		}
		return PeerSelection{Name: requested, Peer: peer}, nil
	}

	if len(peerMap) == 1 {
		for name, peer := range peerMap {
			return PeerSelection{Name: name, Peer: peer}, nil
		}
	}

	names := make([]string, 0, len(peerMap))
	for name := range peerMap {
		names = append(names, name)
	}
	sort.Strings(names)
	return PeerSelection{}, fmt.Errorf("multiple syncthing peers configured; choose one with --peer (%s)", strings.Join(names, ", "))
}

// RequireConfig validates that the machine config contains Syncthing settings.
func RequireConfig(config *machine.MachineConfig) error {
	if config == nil {
		return fmt.Errorf("machine config is not loaded")
	}
	if config.Syncthing.DeviceID == "" || config.Syncthing.APIURL == "" || config.Syncthing.APIKey == "" || config.Syncthing.Address == "" {
		return fmt.Errorf("syncthing config missing: set syncthing.device_id, syncthing.api_url, syncthing.api_key, syncthing.address in machine.yaml")
	}
	return nil
}

// LocalClient creates a Syncthing REST client using the provided machine config.
func LocalClient(config *machine.MachineConfig) (*syncthing.Client, error) {
	if err := RequireConfig(config); err != nil {
		return nil, err
	}
	return syncthing.NewClient(config.Syncthing.APIURL, config.Syncthing.APIKey), nil
}

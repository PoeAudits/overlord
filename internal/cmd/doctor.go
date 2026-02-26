package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	syncpkg "github.com/PoeAudits/overlord/internal/sync"
	"github.com/PoeAudits/overlord/internal/syncthing"
	"github.com/spf13/cobra"
)

type doctorStatus string

const (
	doctorPass doctorStatus = "pass"
	doctorWarn doctorStatus = "warn"
	doctorFail doctorStatus = "fail"
)

type doctorCheck struct {
	name        string
	status      doctorStatus
	message     string
	remediation string
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Verify sync and machine configuration health",
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	ctx := context.Background()

	checks := make([]doctorCheck, 0, 10)

	machineCfg, machineCfgCheck := checkMachineConfig()
	if machineCfgCheck != nil {
		checks = append(checks, *machineCfgCheck)
		return printDoctorChecksAndExit(checks, s)
	}

	checks = append(checks, checkMachineIdentity(machineCfg))

	client, syncthingCfgCheck := checkSyncthingConfig(machineCfg)
	if syncthingCfgCheck != nil {
		checks = append(checks, *syncthingCfgCheck)
		return printDoctorChecksAndExit(checks, s)
	}

	checks = append(checks, checkSyncthingConnectivity(ctx, client))

	checks = append(checks, checkRegistryValidity())

	peersCfg, peerRegistryCheck, peersLoaded := checkPeerRegistry()
	checks = append(checks, peerRegistryCheck)

	checks = append(checks, checkRegistryLock(machineCfg.Name))

	if peersLoaded {
		checks = append(checks, checkPeerReachability(ctx, client, peersCfg))
	}

	configDir, configErr := syncpkg.ConfigDir()
	if configErr != nil {
		checks = append(checks, doctorCheck{
			name:    "Overlord config folder",
			status:  doctorFail,
			message: configErr.Error(),
		})
	} else {
		checks = append(checks, checkConfigIgnoreFile(configDir))
		checks = append(checks, checkConfigFolderConsistency(ctx, client))
	}

	reg, regErr := loadRegistry()
	if regErr == nil {
		checks = append(checks, checkFolderConsistency(ctx, client, reg))
	}

	return printDoctorChecksAndExit(checks, s)
}

func checkMachineConfig() (*machine.MachineConfig, *doctorCheck) {
	machineCfg, machineErr := machine.Load(machine.DefaultConfigPath)
	if machineErr != nil {
		check := doctorCheck{
			name:        "Machine config",
			status:      doctorFail,
			message:     machineErr.Error(),
			remediation: "Run `overlord setup --init` to generate machine.yaml",
		}
		return nil, &check
	}

	return machineCfg, nil
}

func checkMachineIdentity(machineCfg *machine.MachineConfig) doctorCheck {
	if machineCfg.Name == "" || machineCfg.Role == "" {
		return doctorCheck{
			name:        "Machine identity",
			status:      doctorFail,
			message:     "machine name or role is missing",
			remediation: "Set `name` and `role` in ~/.config/overlord/machine.yaml",
		}
	}

	return doctorCheck{
		name:    "Machine identity",
		status:  doctorPass,
		message: fmt.Sprintf("name=%s role=%s", machineCfg.Name, machineCfg.Role),
	}
}

func checkSyncthingConfig(machineCfg *machine.MachineConfig) (*syncthing.Client, *doctorCheck) {
	client, clientErr := syncpkg.LocalClient(machineCfg)
	if clientErr != nil {
		check := doctorCheck{
			name:        "Syncthing config",
			status:      doctorFail,
			message:     clientErr.Error(),
			remediation: "Set `syncthing.device_id/api_url/api_key/address` in machine.yaml",
		}
		return nil, &check
	}

	return client, nil
}

func checkSyncthingConnectivity(ctx context.Context, client *syncthing.Client) doctorCheck {
	if _, err := client.Folders(ctx); err != nil {
		return doctorCheck{
			name:        "Syncthing API",
			status:      doctorFail,
			message:     err.Error(),
			remediation: "Ensure local Syncthing is running and API URL/API key are correct",
		}
	}

	return doctorCheck{
		name:    "Syncthing API",
		status:  doctorPass,
		message: "reachable",
	}
}

func checkRegistryValidity() doctorCheck {
	registryPath, registryPathErr := registry.ExpandPath(registry.DefaultRegistryPath)
	if registryPathErr != nil {
		return doctorCheck{
			name:        "Registry file",
			status:      doctorFail,
			message:     fmt.Sprintf("failed to expand registry path: %s", registryPathErr.Error()),
			remediation: "Check HOME environment variable and file permissions",
		}
	}

	if _, err := os.Stat(registryPath); err != nil {
		return doctorCheck{
			name:        "Registry file",
			status:      doctorFail,
			message:     "registry.yaml not found",
			remediation: "Run `overlord setup` to create registry.yaml",
		}
	}

	if _, err := loadRegistry(); err != nil {
		return doctorCheck{
			name:        "Registry file",
			status:      doctorFail,
			message:     err.Error(),
			remediation: "Fix registry YAML format and rerun doctor",
		}
	}

	return doctorCheck{
		name:    "Registry file",
		status:  doctorPass,
		message: "present and readable",
	}
}

func checkPeerRegistry() (*machine.PeersConfig, doctorCheck, bool) {
	peersCfg, peersErr := machine.LoadPeers(machine.DefaultPeersPath)
	if peersErr != nil {
		return nil, doctorCheck{
			name:        "Peer registry",
			status:      doctorFail,
			message:     peersErr.Error(),
			remediation: "Fix peers.yaml and rerun doctor",
		}, false
	}

	return peersCfg, doctorCheck{
		name:    "Peer registry",
		status:  doctorPass,
		message: fmt.Sprintf("%d peer(s) loaded", len(peersCfg.Peers)),
	}, true
}

func checkRegistryLock(machineName string) doctorCheck {
	lock, lockErr := machine.LoadLock(machine.DefaultRegistryLockPath)
	if lockErr != nil {
		return doctorCheck{
			name:        "Registry lock",
			status:      doctorFail,
			message:     lockErr.Error(),
			remediation: "Fix registry.lock.yaml or regenerate with `overlord sync lock claim`",
		}
	}

	if lock == nil {
		return doctorCheck{
			name:        "Registry lock",
			status:      doctorFail,
			message:     "registry.lock.yaml is missing",
			remediation: "Run `overlord sync lock claim`",
		}
	}

	ownerMsg := fmt.Sprintf("owner=%s updated_at=%s", lock.Owner, lock.UpdatedAt)
	if lock.Owner != machineName {
		return doctorCheck{
			name:        "Registry lock",
			status:      doctorWarn,
			message:     ownerMsg,
			remediation: fmt.Sprintf("If needed, request transfer: `overlord sync lock transfer %s`", machineName),
		}
	}

	return doctorCheck{
		name:    "Registry lock",
		status:  doctorPass,
		message: ownerMsg,
	}
}

func checkPeerReachability(ctx context.Context, client *syncthing.Client, peersCfg *machine.PeersConfig) doctorCheck {
	connections, err := client.Connections(ctx)
	if err != nil {
		return doctorCheck{
			name:        "Peer connectivity",
			status:      doctorWarn,
			message:     err.Error(),
			remediation: "Check Syncthing connection status in UI",
		}
	}

	disconnected := make([]string, 0)
	for name, peer := range peersCfg.Peers {
		conn, ok := connections.Connections[peer.Syncthing.DeviceID]
		if !ok || !conn.Connected {
			disconnected = append(disconnected, name)
		}
	}

	if len(disconnected) > 0 {
		return doctorCheck{
			name:        "Peer connectivity",
			status:      doctorWarn,
			message:     fmt.Sprintf("disconnected: %s", strings.Join(disconnected, ", ")),
			remediation: "Ensure peers are online and reachable over Tailscale",
		}
	}

	return doctorCheck{
		name:    "Peer connectivity",
		status:  doctorPass,
		message: "all configured peers connected",
	}
}

func checkConfigIgnoreFile(configDir string) doctorCheck {
	ignorePath := filepath.Join(configDir, ".stignore")
	content, err := os.ReadFile(ignorePath)
	if err != nil {
		return doctorCheck{
			name:        "Config .stignore",
			status:      doctorFail,
			message:     ".stignore missing",
			remediation: "Run `overlord setup --init`",
		}
	}

	if !containsExactLine(string(content), "machine.yaml") {
		return doctorCheck{
			name:        "Config .stignore",
			status:      doctorFail,
			message:     "missing machine.yaml exclusion",
			remediation: "Add `machine.yaml` to ~/.config/overlord/.stignore",
		}
	}

	return doctorCheck{
		name:    "Config .stignore",
		status:  doctorPass,
		message: "machine.yaml excluded",
	}
}

func checkConfigFolderConsistency(ctx context.Context, client *syncthing.Client) doctorCheck {
	if _, err := client.Folder(ctx, syncpkg.ConfigFolderID); err != nil {
		return doctorCheck{
			name:        "Config Syncthing folder",
			status:      doctorFail,
			message:     err.Error(),
			remediation: "Run `overlord setup --init` to configure overlord-config folder",
		}
	}

	return doctorCheck{
		name:    "Config Syncthing folder",
		status:  doctorPass,
		message: fmt.Sprintf("folder '%s' configured", syncpkg.ConfigFolderID),
	}
}

func checkFolderConsistency(ctx context.Context, client *syncthing.Client, reg *registry.Registry) doctorCheck {
	active := syncActiveProjects(reg)
	baseDir, baseErr := expandBaseDir(reg)
	if baseErr != nil {
		return doctorCheck{
			name:        "Active project folders",
			status:      doctorFail,
			message:     baseErr.Error(),
			remediation: "Fix registry settings.base_dir",
		}
	}

	mismatches := make([]string, 0)
	for name := range active {
		project := reg.Projects[name]
		projectPath := fileutil.ResolveProjectPath(project.Path, baseDir)
		if _, err := os.Stat(projectPath); err != nil {
			mismatches = append(mismatches, fmt.Sprintf("%s (missing path)", name))
			continue
		}
		folder, err := client.Folder(ctx, name)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("%s (missing syncthing folder)", name))
			continue
		}
		if folder.Path != projectPath {
			mismatches = append(mismatches, fmt.Sprintf("%s (path mismatch)", name))
		}
	}

	if len(mismatches) > 0 {
		return doctorCheck{
			name:        "Active project folders",
			status:      doctorWarn,
			message:     strings.Join(mismatches, ", "),
			remediation: "Run `overlord setup --init` then `overlord activate <name>` for affected projects",
		}
	}

	return doctorCheck{
		name:    "Active project folders",
		status:  doctorPass,
		message: fmt.Sprintf("%d active project(s) verified", len(active)),
	}
}

func printDoctorChecksAndExit(checks []doctorCheck, s styles) error {
	fails := 0
	warns := 0

	for _, check := range checks {
		prefix := s.Success.Render("✓")
		switch check.status {
		case doctorFail:
			prefix = s.Error.Render("✗")
			fails++
		case doctorWarn:
			prefix = s.Warn.Render("!")
			warns++
		}

		fmt.Printf("%s %s: %s\n", prefix, check.name, check.message)
		if check.remediation != "" {
			fmt.Printf("  %s %s\n", s.Label.Render("Fix:"), check.remediation)
		}
	}

	if fails > 0 {
		return fmt.Errorf("doctor found %d failing check(s) and %d warning(s)", fails, warns)
	}
	if warns > 0 {
		fmt.Printf("%s doctor completed with %d warning(s)\n", s.Warn.Render("Warning:"), warns)
	}
	return nil
}

func containsExactLine(content, target string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

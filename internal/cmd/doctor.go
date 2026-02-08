package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
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

	machineCfg, machineErr := machine.Load(machine.DefaultConfigPath)
	if machineErr != nil {
		checks = append(checks, doctorCheck{
			name:        "Machine config",
			status:      doctorFail,
			message:     machineErr.Error(),
			remediation: "Run `overlord setup --init` to generate machine.yaml",
		})
		return printDoctorChecksAndExit(checks, s)
	}

	if machineCfg.Name == "" || machineCfg.Role == "" {
		checks = append(checks, doctorCheck{
			name:        "Machine identity",
			status:      doctorFail,
			message:     "machine name or role is missing",
			remediation: "Set `name` and `role` in ~/.config/overlord/machine.yaml",
		})
	} else {
		checks = append(checks, doctorCheck{
			name:    "Machine identity",
			status:  doctorPass,
			message: fmt.Sprintf("name=%s role=%s", machineCfg.Name, machineCfg.Role),
		})
	}

	client, clientErr := localSyncthingClient(machineCfg)
	if clientErr != nil {
		checks = append(checks, doctorCheck{
			name:        "Syncthing config",
			status:      doctorFail,
			message:     clientErr.Error(),
			remediation: "Set `syncthing.device_id/api_url/api_key/address` in machine.yaml",
		})
		return printDoctorChecksAndExit(checks, s)
	}

	if _, err := client.GetFolders(ctx); err != nil {
		checks = append(checks, doctorCheck{
			name:        "Syncthing API",
			status:      doctorFail,
			message:     err.Error(),
			remediation: "Ensure local Syncthing is running and API URL/API key are correct",
		})
	} else {
		checks = append(checks, doctorCheck{name: "Syncthing API", status: doctorPass, message: "reachable"})
	}

	registryPath, _ := registry.ExpandPath(registry.DefaultRegistryPath)
	if _, err := os.Stat(registryPath); err != nil {
		checks = append(checks, doctorCheck{
			name:        "Registry file",
			status:      doctorFail,
			message:     "registry.yaml not found",
			remediation: "Run `overlord setup` to create registry.yaml",
		})
	} else if _, err := loadRegistry(); err != nil {
		checks = append(checks, doctorCheck{name: "Registry file", status: doctorFail, message: err.Error(), remediation: "Fix registry YAML format and rerun doctor"})
	} else {
		checks = append(checks, doctorCheck{name: "Registry file", status: doctorPass, message: "present and readable"})
	}

	peersCfg, peersErr := machine.LoadPeers(machine.DefaultPeersPath)
	if peersErr != nil {
		checks = append(checks, doctorCheck{name: "Peer registry", status: doctorFail, message: peersErr.Error(), remediation: "Fix peers.yaml and rerun doctor"})
	} else {
		checks = append(checks, doctorCheck{name: "Peer registry", status: doctorPass, message: fmt.Sprintf("%d peer(s) loaded", len(peersCfg.Peers))})
	}

	lock, lockErr := machine.LoadLock(machine.DefaultRegistryLockPath)
	if lockErr != nil {
		checks = append(checks, doctorCheck{name: "Registry lock", status: doctorFail, message: lockErr.Error(), remediation: "Fix registry.lock.yaml or regenerate with `overlord sync lock claim`"})
	} else if lock == nil {
		checks = append(checks, doctorCheck{name: "Registry lock", status: doctorFail, message: "registry.lock.yaml is missing", remediation: "Run `overlord sync lock claim`"})
	} else {
		ownerMsg := fmt.Sprintf("owner=%s updated_at=%s", lock.Owner, lock.UpdatedAt)
		if lock.Owner != machineCfg.Name {
			checks = append(checks, doctorCheck{name: "Registry lock", status: doctorWarn, message: ownerMsg, remediation: fmt.Sprintf("If needed, request transfer: `overlord sync lock transfer %s`", machineCfg.Name)})
		} else {
			checks = append(checks, doctorCheck{name: "Registry lock", status: doctorPass, message: ownerMsg})
		}
	}

	if peersErr == nil {
		connections, err := client.GetConnections(ctx)
		if err != nil {
			checks = append(checks, doctorCheck{name: "Peer connectivity", status: doctorWarn, message: err.Error(), remediation: "Check Syncthing connection status in UI"})
		} else {
			disconnected := make([]string, 0)
			for name, peer := range peersCfg.Peers {
				conn, ok := connections.Connections[peer.Syncthing.DeviceID]
				if !ok || !conn.Connected {
					disconnected = append(disconnected, name)
				}
			}
			if len(disconnected) > 0 {
				checks = append(checks, doctorCheck{name: "Peer connectivity", status: doctorWarn, message: fmt.Sprintf("disconnected: %s", strings.Join(disconnected, ", ")), remediation: "Ensure peers are online and reachable over Tailscale"})
			} else {
				checks = append(checks, doctorCheck{name: "Peer connectivity", status: doctorPass, message: "all configured peers connected"})
			}
		}
	}

	configDir, configErr := registryConfigDir()
	if configErr != nil {
		checks = append(checks, doctorCheck{name: "Overlord config folder", status: doctorFail, message: configErr.Error()})
	} else {
		ignorePath := filepath.Join(configDir, ".stignore")
		content, err := os.ReadFile(ignorePath)
		if err != nil {
			checks = append(checks, doctorCheck{name: "Config .stignore", status: doctorFail, message: ".stignore missing", remediation: "Run `overlord setup --init`"})
		} else if !containsExactLine(string(content), "machine.yaml") {
			checks = append(checks, doctorCheck{name: "Config .stignore", status: doctorFail, message: "missing machine.yaml exclusion", remediation: "Add `machine.yaml` to ~/.config/overlord/.stignore"})
		} else {
			checks = append(checks, doctorCheck{name: "Config .stignore", status: doctorPass, message: "machine.yaml excluded"})
		}

		if _, err := client.GetFolder(ctx, overlordConfigFolderID); err != nil {
			checks = append(checks, doctorCheck{name: "Config Syncthing folder", status: doctorFail, message: err.Error(), remediation: "Run `overlord setup --init` to configure overlord-config folder"})
		} else {
			checks = append(checks, doctorCheck{name: "Config Syncthing folder", status: doctorPass, message: fmt.Sprintf("folder '%s' configured", overlordConfigFolderID)})
		}
	}

	reg, regErr := loadRegistry()
	if regErr == nil {
		active := getSyncActiveProjects(reg)
		mismatches := make([]string, 0)
		baseDir, baseErr := expandBaseDir(reg)
		if baseErr != nil {
			checks = append(checks, doctorCheck{name: "Active project folders", status: doctorFail, message: baseErr.Error(), remediation: "Fix registry settings.base_dir"})
		} else {
			for name := range active {
				project := reg.Projects[name]
				projectPath := resolveProjectPath(project.Path, baseDir)
				if _, err := os.Stat(projectPath); err != nil {
					mismatches = append(mismatches, fmt.Sprintf("%s (missing path)", name))
					continue
				}
				folder, err := client.GetFolder(ctx, name)
				if err != nil {
					mismatches = append(mismatches, fmt.Sprintf("%s (missing syncthing folder)", name))
					continue
				}
				if folder.Path != projectPath {
					mismatches = append(mismatches, fmt.Sprintf("%s (path mismatch)", name))
				}
			}

			if len(mismatches) > 0 {
				checks = append(checks, doctorCheck{name: "Active project folders", status: doctorWarn, message: strings.Join(mismatches, ", "), remediation: "Run `overlord setup --init` then `overlord activate <name>` for affected projects"})
			} else {
				checks = append(checks, doctorCheck{name: "Active project folders", status: doctorPass, message: fmt.Sprintf("%d active project(s) verified", len(active))})
			}
		}
	}

	return printDoctorChecksAndExit(checks, s)
}

func printDoctorChecksAndExit(checks []doctorCheck, s Styles) error {
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

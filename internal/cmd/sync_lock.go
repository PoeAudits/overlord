package cmd

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/spf13/cobra"
)

var syncLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Manage registry write lock ownership",
}

var syncLockShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current registry lock owner",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := NewStyles()

		lock, err := machine.LoadLock(machine.DefaultRegistryLockPath)
		if err != nil {
			return err
		}
		if lock == nil {
			fmt.Printf("%s No registry lock exists yet\n", s.Info.Render("Info:"))
			return nil
		}

		machineConfig, err := machine.Load(machine.DefaultConfigPath)
		if err != nil {
			return err
		}

		fmt.Printf("%s %s\n", s.Label.Render("Owner:"), lock.Owner)
		fmt.Printf("%s %s\n", s.Label.Render("Updated:"), lock.UpdatedAt)
		if lock.Note != "" {
			fmt.Printf("%s %s\n", s.Label.Render("Note:"), lock.Note)
		}

		if lock.Owner == machineConfig.Name {
			fmt.Printf("%s this machine owns the lock\n", s.Success.Render("✓"))
		} else {
			fmt.Printf("%s this machine does not own the lock\n", s.Warn.Render("Warning:"))
		}

		return nil
	},
}

var syncLockTransferCmd = &cobra.Command{
	Use:   "transfer <machine>",
	Short: "Transfer registry lock to another machine",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := args[0]
		s := NewStyles()

		machineConfig, err := machine.Load(machine.DefaultConfigPath)
		if err != nil {
			return err
		}

		peers, err := machine.LoadPeers(machine.DefaultPeersPath)
		if err != nil {
			return err
		}

		if target != machineConfig.Name {
			if _, ok := peers.Peers[target]; !ok {
				return fmt.Errorf("target machine '%s' not found in peers.yaml", target)
			}
		}

		lock, err := machine.LoadLock(machine.DefaultRegistryLockPath)
		if err != nil {
			return err
		}

		if lock != nil && lock.Owner != machineConfig.Name {
			return fmt.Errorf("lock is currently owned by '%s'; local machine '%s' cannot transfer it\nIf owner is offline, run: overlord sync lock claim", lock.Owner, machineConfig.Name)
		}

		next := machine.NewRegistryLock(target, "lock transfer")
		if err := machine.SaveLock(machine.DefaultRegistryLockPath, next); err != nil {
			return err
		}

		fmt.Printf("%s lock transferred to '%s'\n", s.Success.Render("✓"), target)
		fmt.Printf("%s run 'overlord sync lock show' to verify on each machine\n", s.Info.Render("Next:"))
		return nil
	},
}

var syncLockClaimCmd = &cobra.Command{
	Use:   "claim",
	Short: "Force-claim registry lock for this machine",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		s := NewStyles()

		machineConfig, err := machine.Load(machine.DefaultConfigPath)
		if err != nil {
			return err
		}

		current, err := machine.LoadLock(machine.DefaultRegistryLockPath)
		if err != nil {
			return err
		}

		next := machine.NewRegistryLock(machineConfig.Name, "force claim")
		if err := machine.SaveLock(machine.DefaultRegistryLockPath, next); err != nil {
			return err
		}

		if current != nil && current.Owner != machineConfig.Name {
			fmt.Printf("%s lock force-claimed from '%s'\n", s.Warn.Render("Warning:"), current.Owner)
		}
		fmt.Printf("%s lock owner is now '%s'\n", s.Success.Render("✓"), machineConfig.Name)
		return nil
	},
}

func init() {
	syncCmd.AddCommand(syncLockCmd)
	syncLockCmd.AddCommand(syncLockShowCmd)
	syncLockCmd.AddCommand(syncLockTransferCmd)
	syncLockCmd.AddCommand(syncLockClaimCmd)
}

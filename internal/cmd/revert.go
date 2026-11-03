package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/revert"
	"github.com/PoeAudits/overlord/internal/system"
)

// Temporary, with internal/revert: see its package comment for removal.

func newRevertCommand(sys system.System) *cobra.Command {
	return &cobra.Command{
		Use:   "revert",
		Short: "Temporary: put the machine's Harness setup back as it was before Overlord v2 first changed it",
		Long: `Temporary safety net for the move to Overlord v2.

The first time "sync" or "start" is about to change anything, Overlord saves
a copy of every path they can change (skills, instruction files, agent files,
hooks, Harness and T3 Code settings, the agents directory and Overlord's own
config) in ~/.local/state/overlord/revert. "revert" puts all of them back,
including removing what Sync created. Whatever revert replaces is kept in
~/.local/state/overlord/revert-replaced-*, and the saved copy is removed so
the next Sync saves a fresh one.

Revert does not stop services Start launched, undo Setup in a Project (use
git there) or switch the overlord program itself back to the old version.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			replaced, err := revert.Restore(sys)
			if err != nil {
				return err
			}
			fmt.Fprintln(sys.Stdout, "Reverted to the saved setup")
			if replaced != "" {
				fmt.Fprintln(sys.Stdout, "  what it replaced is kept in", machine.DisplayPath(sys.Home, replaced))
			}
			return nil
		},
	}
}

func newSnapshotCommand(sys system.System) *cobra.Command {
	return &cobra.Command{
		Use:   "snapshot",
		Short: "Temporary: save the setup now so \"overlord revert\" can restore it",
		Long: `Save the paths "revert" restores, unless a saved copy already exists.
"sync" and "start" do this themselves before they first change anything; run
this before changing those paths by hand.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			saved, err := revert.Save(sys)
			if err != nil {
				return err
			}
			if !saved {
				fmt.Fprintln(sys.Stdout, "A saved copy already exists:", machine.DisplayPath(sys.Home, revert.Dir(sys.Home)))
				return nil
			}
			fmt.Fprintln(sys.Stdout, "Saved to", machine.DisplayPath(sys.Home, revert.Dir(sys.Home)))
			return nil
		},
	}
}

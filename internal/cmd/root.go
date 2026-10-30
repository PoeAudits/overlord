// Package cmd wires Overlord's Cobra commands to the packages that do the work.
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/system"
)

// Execute runs the overlord command line against sys and returns the process
// exit code.
func Execute(sys system.System, args []string) int {
	root := &cobra.Command{
		Use:           "overlord",
		Short:         "Set up Projects and keep every Harness configured from one source",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetArgs(args)
	root.SetOut(sys.Stdout)
	root.SetErr(sys.Stderr)
	root.AddCommand(newSetupCommand(sys))
	root.AddCommand(newSyncCommand(sys))
	if err := root.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(sys.Stderr, "overlord:", err)
		return 1
	}
	return 0
}

// Package cmd wires Overlord's Cobra commands to the packages that do the work.
package cmd

import (
	"context"
	"errors"
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
	root.AddCommand(newDoctorCommand(sys))
	root.AddCommand(newStartCommand(sys))
	root.AddCommand(newNewProjectCommand(sys))
	err := root.ExecuteContext(context.Background())
	if err == nil {
		return 0
	}
	if !errors.Is(err, errReported) {
		fmt.Fprintln(sys.Stderr, "overlord:", err)
	}
	return 1
}

// errReported fails a command whose output already says why.
var errReported = errors.New("failure already reported")

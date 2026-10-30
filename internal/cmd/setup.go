package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/project"
	"github.com/PoeAudits/overlord/internal/system"
)

func newSetupCommand(sys system.System) *cobra.Command {
	var check, asJSON bool
	command := &cobra.Command{
		Use:   "setup [path]",
		Short: "Check a Project for the files Overlord guarantees",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !check {
				return errors.New("setup without --check is not implemented yet")
			}
			plan, err := project.Check(projectRoot(sys, args))
			if err != nil {
				return err
			}
			if asJSON {
				encoder := json.NewEncoder(sys.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(plan)
			}
			fmt.Fprintf(sys.Stdout, "Setup plan for %s\n", plan.Root)
			for _, item := range plan.Items {
				fmt.Fprintf(sys.Stdout, "  %-7s  %s\n", item.Status, item.Path)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&check, "check", false, "print the plan without changing anything")
	command.Flags().BoolVar(&asJSON, "json", false, "print the plan as JSON")
	return command
}

// projectRoot picks the Project root: T3 Code's T3CODE_PROJECT_ROOT when set,
// else the path argument, else the working directory.
func projectRoot(sys system.System, args []string) string {
	if root := sys.Getenv("T3CODE_PROJECT_ROOT"); root != "" {
		return root
	}
	if len(args) == 0 {
		return sys.Workdir
	}
	if filepath.IsAbs(args[0]) {
		return filepath.Clean(args[0])
	}
	return filepath.Join(sys.Workdir, args[0])
}

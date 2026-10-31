package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/system"
)

func newStartCommand(sys system.System) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Sync, then bring up the services T3 Code needs",
		Long: `Sync, then bring up the services in Overlord's config
(~/.config/overlord/config.yaml, written with defaults on first run) and
finally the T3 Code desktop app if it is not running.

A service whose health URL answers is left alone. Otherwise its command is
launched detached, with output appended to ~/.local/state/overlord/<name>.log,
and start waits up to the service's timeout for health. A service that does
not become healthy is reported and the rest still start; start then exits
non-zero. A Sync failure is reported first and does not stop the services
from starting; start then exits non-zero too.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if _, err := config.WriteDefault(sys.Home); err != nil {
				return err
			}
			cfg, err := config.Load(sys.Home)
			if err != nil {
				return err
			}
			// A Sync failure must not keep the services down: report it now
			// and fail once everything has been started.
			report, syncErr := machine.Sync(sys, cfg.AgentsDir, false)
			if syncErr != nil {
				fmt.Fprintln(sys.Stderr, "overlord:", syncErr)
			}
			// Start runs at login: show only what needs the user.
			for _, result := range report.Results {
				if result.Action == machine.Conflict || result.Action == machine.Shadowed {
					fmt.Fprintln(sys.Stdout, resultLine(sys.Home, result, false))
				}
			}
			fmt.Fprintln(sys.Stdout, "Start")
			failed := false
			for _, result := range machine.Start(command.Context(), sys, cfg.Services) {
				line := fmt.Sprintf("  %-8s  %s: %s", result.Status, result.Name, result.Detail)
				if result.Log != "" {
					line += "; see " + displayPath(sys.Home, result.Log)
				}
				fmt.Fprintln(sys.Stdout, line)
				failed = failed || result.Status == machine.Failed
			}
			if failed {
				return errors.New("start: some services did not start")
			}
			if syncErr != nil {
				return errors.New("start: sync failed")
			}
			return nil
		},
	}
}

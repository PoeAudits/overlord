package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

var (
	startRestartLoadStackConfig = loadStackConfig
	startRestartServices        = stack.RestartServices
)

var startRestartCmd = &cobra.Command{
	Use:   "restart <service...>",
	Short: "Restart stack service windows",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStartRestart,
}

func init() {
	startCmd.AddCommand(startRestartCmd)
}

func runStartRestart(cmd *cobra.Command, args []string) error {
	cfg, err := startRestartLoadStackConfig()
	if err != nil {
		return cmdError("start restart", "failed to load stack config", err.Error())
	}

	result, err := startRestartServices(cfg, args)
	if err != nil {
		return cmdError("start restart", "failed to restart services", err.Error())
	}

	s := NewStyles()
	w := cmd.ErrOrStderr()
	for _, sr := range result.ServiceResults {
		fmt.Fprintf(w, "%s restarted %s\n", s.Success.Render("Restarted:"), s.Bold.Render(sr.DisplayName))
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(w, "%s %s\n", s.Warn.Render("Warning:"), warning)
	}
	return nil
}

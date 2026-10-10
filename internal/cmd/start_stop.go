package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

var startStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop development stack",
	Args:  cobra.NoArgs,
	RunE:  runStartStop,
}

func init() {
	startCmd.AddCommand(startStopCmd)
}

func runStartStop(cmd *cobra.Command, args []string) error {
	cfg, err := loadStackConfig()
	configWarning := ""
	if err != nil {
		configWarning = "failed to load stack config; continuing with default stop behavior: " + err.Error()
		cfg = nil
	}

	result, err := stack.StopStack(cfg)
	if err != nil {
		return cmdError("start stop", "failed to stop stack", err.Error())
	}

	s := NewStyles()
	if result.SessionStopped {
		fmt.Fprintf(os.Stderr, "%s %s session stopped\n", s.Success.Render("Stopped:"), s.Bold.Render(stack.SessionName))
	} else {
		fmt.Fprintf(os.Stderr, "%s %s session not running\n", s.Warn.Render("Warning:"), s.Bold.Render(stack.SessionName))
	}

	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "%s %s\n", s.Warn.Render("Warning:"), w)
	}
	if configWarning != "" {
		fmt.Fprintf(os.Stderr, "%s %s\n", s.Warn.Render("Warning:"), configWarning)
	}

	if len(result.Warnings) > 0 {
		return cmdError("start stop", "stack stopped with warnings", strings.Join(result.Warnings, "; "))
	}

	return nil
}

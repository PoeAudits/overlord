package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/checkpoint"
	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// startCmd represents the start command.
// Package-level so other files in cmd can register subcommands via startCmd.AddCommand().
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start development stack and restore workspace checkpoint",
	Args:  cobra.NoArgs,
	RunE:  runStart,
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start development stack without opening project sessions",
	Args:  cobra.NoArgs,
	RunE:  runStartUp,
}

// topLevelUpCmd provides a convenience alias for `overlord start up`.
var topLevelUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Alias for 'overlord start up'",
	Args:  cobra.NoArgs,
	RunE:  runStartUp,
}

var (
	startLoadStackConfig   = loadStackConfig
	startStack             = stack.StartStack
	startLoadCheckpoints   = func() (*checkpoint.Store, error) { return checkpoint.Load(checkpoint.DefaultPath) }
	startRestoreCheckpoint = tmux.RestoreCheckpoint
	startAttach            = func(name string) error { return tmux.NewSession(name, "").AttachOrSwitch() }
)

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(topLevelUpCmd)
	startCmd.AddCommand(upCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	return doStart(false)
}

func runStartUp(cmd *cobra.Command, args []string) error {
	return doStart(true)
}

// doStart contains the shared logic for both start and start up.
// When silent is true, only stack services are ensured; workspace sessions are
// not opened and no tmux client switch/attach occurs.
//
// StartServer is intentionally independent from workspace restoration.
func doStart(silent bool) error {
	cfg, err := startLoadStackConfig()
	if err != nil {
		return cmdError("start", "failed to load stack config", err.Error())
	}

	s := NewStyles()

	result, err := startStack(cfg)
	if err != nil {
		return cmdError("start", "failed to start stack", err.Error())
	}

	if silent {
		return nil
	}

	store, err := startLoadCheckpoints()
	if err != nil {
		return cmdError("start", "failed to load workspace checkpoints", err.Error())
	}

	// Print status table to stderr (design decision D4: all output before tmux attach).
	if !silent {
		printStackStatus(result, s)
	}

	// Print any warnings from StartStack.
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "%s %s\n", s.Warn.Render("Warning:"), w)
	}

	if store.Current == nil {
		fmt.Fprintf(os.Stderr, "%s no workspace checkpoint saved; attaching %s\n", s.Warn.Render("Warning:"), stack.SessionName)
		return startAttach(stack.SessionName)
	}
	available := startRestoreCheckpoint(store.Current, nil, func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "%s %s\n", s.Warn.Render("Warning:"), fmt.Sprintf(format, args...))
	})
	target := ""
	for _, name := range available {
		if name == store.Current.AttachSession {
			target = name
			break
		}
	}
	if target == "" && len(available) > 0 {
		target = available[0]
	}
	if target == "" {
		target = stack.SessionName
	}
	return startAttach(target)
}

// printStackStatus writes a styled status table to stderr.
func printStackStatus(result *stack.StartResult, s styles) {
	fmt.Fprintf(os.Stderr, "  %s\n", s.Label.Render(stack.SessionName+" stack"))
	for _, sr := range result.ServiceResults {
		var statusStr string
		switch sr.Status {
		case stack.StatusStarted:
			statusStr = s.Success.Render("started")
		case stack.StatusHealthy:
			statusStr = s.Success.Render("healthy")
		case stack.StatusUnhealthy:
			detail := ""
			if sr.Error != nil {
				detail = "  (" + sr.Error.Error() + ")"
			}
			statusStr = s.Warn.Render("unhealthy") + detail
		case stack.StatusSkipped:
			statusStr = s.Warn.Render("skipped")
		default:
			statusStr = string(sr.Status)
		}
		fmt.Fprintf(os.Stderr, "  %-14s %s\n", s.Label.Render(sr.DisplayName), statusStr)
	}
	fmt.Fprintln(os.Stderr)
}

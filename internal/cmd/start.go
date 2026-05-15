package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// startCmd represents the start command.
// Package-level so other files in cmd can register subcommands via startCmd.AddCommand().
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start development stack and open projects",
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
	startLoadStackConfig    = loadStackConfig
	startLoadRegistry       = loadRegistry
	startStack              = stack.StartStack
	startEnsureProject      = ensureProjectSession
	startOpenProjectSession = openProject
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
// When silent is true, only stack services are ensured; project sessions are
// not opened and no tmux client switch/attach occurs.
//
// Order of operations: start the stack before opening project sessions so the
// StartServer session is created first among the sessions this command starts.
func doStart(silent bool) error {
	cfg, err := startLoadStackConfig()
	if err != nil {
		return cmdError("start", "failed to load stack config", err.Error())
	}

	s := NewStyles()

	// Start the stack before opening project sessions so its tmux session keeps a
	// stable first-created position relative to the sessions spawned here.
	result, err := startStack(cfg)
	if err != nil {
		return cmdError("start", "failed to start stack", err.Error())
	}

	if silent {
		return nil
	}

	reg, err := startLoadRegistry()
	if err != nil {
		return cmdError("start", "failed to load registry", err.Error())
	}

	// Resolve configured projects. Collect successful resolutions; warn on failures.
	resolved := make([]openTarget, 0, len(cfg.Projects))
	for _, projectName := range cfg.Projects {
		res, err := reg.ResolveOne(projectName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s could not resolve project %q: %v\n",
				s.Warn.Render("Warning:"), projectName, err)
			continue
		}
		if res.Project.Status.State == registry.StateArchived {
			fmt.Fprintf(os.Stderr, "%s skipping archived project %q (restore with 'overlord unarchive %s')\n",
				s.Warn.Render("Warning:"), res.Name, res.Name)
			continue
		}
		resolved = append(resolved, openTarget{name: res.Name, project: res.Project})
	}

	// Create background sessions for all but the last project after starting the
	// stack. The final project is still attached last below.
	if len(resolved) > 1 {
		for _, rp := range resolved[:len(resolved)-1] {
			recordLastOpened(rp.name, rp.project, reg)
			if err := startEnsureProject(rp.name, rp.project, reg); err != nil {
				fmt.Fprintf(os.Stderr, "%s failed to create session for %q: %v\n",
					s.Warn.Render("Warning:"), rp.name, err)
			}
		}
	}

	// Print status table to stderr (design decision D4: all output before tmux attach).
	if !silent {
		printStackStatus(result, s)
	}

	// Print any warnings from StartStack.
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "%s %s\n", s.Warn.Render("Warning:"), w)
	}

	// No projects resolved -- attach to the stack session.
	if len(resolved) == 0 {
		session := tmux.NewSession(stack.SessionName, "")
		return session.AttachOrSwitch()
	}

	// Attach to the last project (design decision D4: attach is the final action).
	lastRP := resolved[len(resolved)-1]
	recordLastOpened(lastRP.name, lastRP.project, reg)
	return startOpenProjectSession(lastRP.name, lastRP.project, reg)
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

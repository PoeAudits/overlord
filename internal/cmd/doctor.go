package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/system"
)

func newDoctorCommand(sys system.System) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report drift between the agents directory, every Harness and T3 Code without changing anything",
		Long: `Report drift without changing anything. Doctor reports what "overlord
sync --check" would change (missing or dangling skill links, global
instruction files that are not symlinks, missing session hooks, a missing T3
overlord-setup script), plus:

  - global instruction links and ~/.claude/skills links whose target is gone
  - ~/.config/opencode/skills, or an agents directory that is not a git
    checkout: stale skill copies
  - global agent files without the generated header, Role or not
  - T3 projects whose own scripts override lacks overlord-setup, which
    replaces the global list so Setup does not run there
  - UPSTREAM.md sources whose remote HEAD differs from the recorded commit

UPSTREAM.md in the agents directory is Markdown kept by hand. Doctor reads only
list items of three fields; every other line is prose:

  - <skill name> <git URL> <commit SHA, full or abbreviated>

A remote that cannot be reached (offline) is listed under "Not checked".
Doctor exits 1 when it finds drift and 0 otherwise.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			cfg, err := config.Load(sys.Home)
			if err != nil {
				return err
			}
			diagnosis, err := machine.Doctor(command.Context(), sys, cfg.AgentsDir)
			if err != nil {
				return err
			}
			if len(diagnosis.Findings) == 0 {
				fmt.Fprintln(sys.Stdout, "Doctor: no drift found")
			}
			printFindings(sys, "Doctor found drift", diagnosis.Findings)
			printFindings(sys, "Not checked", diagnosis.Notes)
			if len(diagnosis.Findings) > 0 {
				return errors.New("doctor found drift")
			}
			return nil
		},
	}
}

// printFindings prints header and one line per finding, or nothing when
// there are none.
func printFindings(sys system.System, header string, findings []machine.Finding) {
	if len(findings) == 0 {
		return
	}
	fmt.Fprintln(sys.Stdout, header)
	for _, finding := range findings {
		fmt.Fprintf(sys.Stdout, "  %s: %s\n", machine.DisplayPath(sys.Home, finding.Path), finding.Problem)
	}
}

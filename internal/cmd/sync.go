package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/system"
)

func newSyncCommand(sys system.System) *cobra.Command {
	var check, quiet bool
	command := &cobra.Command{
		Use:   "sync",
		Short: "Apply skills and global instructions from the agents directory to every Harness",
		Long: `Apply skills and global instructions from the agents directory to every
Harness, and register "overlord setup" with T3 Code.

"sync --check" changes nothing and exits 1 when Sync would create, remove,
replace or update anything, or finds a conflict. A shadowed skill (a skill
directory of your own in the way of a catalog skill) is reported but does not
fail the check.

When the agents directory has no AGENTS.md, Sync links no global instruction
file and reports the missing file as a conflict, so "sync --check" fails.

The T3 step ensures ~/.t3/userdata/settings.json has a defaultProjectScripts
entry with id overlord-setup that runs "overlord setup" on every new
worktree. Every other setting is kept as it is.

T3 runs only the first script marked to run on worktree creation, so chain
Project-specific post-clone commands behind "overlord setup" instead of adding
a second script. If a Project has its own scripts override in T3, that
override replaces the global list entirely, so Setup does not run for that
Project unless the override includes the overlord-setup entry.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := config.Load(sys.Home)
			if err != nil {
				return err
			}
			report, err := machine.Sync(sys, cfg.AgentsDir, check)
			if err != nil {
				return err
			}
			if !quiet {
				fmt.Fprintln(sys.Stdout, syncHeader(check))
			}
			for _, result := range report.Results {
				if quiet && !result.NeedsUser() {
					continue
				}
				fmt.Fprintln(sys.Stdout, resultLine(sys.Home, result, check))
			}
			if check && report.NeedsWork() {
				return errors.New("sync --check found paths that need work")
			}
			return nil
		},
	}
	command.Flags().BoolVar(&check, "check", false, "report what Sync would change without changing anything; exit 1 if anything would change or conflicts")
	command.Flags().BoolVar(&quiet, "quiet", false, "print only conflicts and shadowed skills")
	return command
}

func syncHeader(check bool) string {
	if check {
		return "Sync check (nothing changed)"
	}
	return "Sync"
}

// resultLine formats one result: the action (past tense once applied), the
// path and any detail.
func resultLine(home string, result machine.Result, check bool) string {
	label := string(result.Action)
	if !check && pastTense[result.Action] != "" {
		label = pastTense[result.Action]
	}
	line := fmt.Sprintf("  %-8s  %s", label, machine.DisplayPath(home, result.Path))
	if result.Detail == "" {
		return line
	}
	return line + ": " + result.Detail
}

var pastTense = map[machine.Action]string{
	machine.Create:  "created",
	machine.Remove:  "removed",
	machine.Replace: "replaced",
	machine.Update:  "updated",
}

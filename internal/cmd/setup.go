package cmd

import (
	"cmp"
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
		Short: "Create or refresh the files Overlord guarantees in a Project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			root := setupTarget(sys, args)
			// T3 points T3CODE_PROJECT_ROOT at the main checkout, where
			// languages are detected; without it the target is the Project.
			projectRoot := cmp.Or(sys.Getenv("T3CODE_PROJECT_ROOT"), root)
			if check {
				plan, err := project.Check(command.Context(), sys.Runner, root, projectRoot)
				if err != nil {
					return err
				}
				if err := printPlan(sys, plan, asJSON, "Setup plan for", nil); err != nil {
					return err
				}
				if plan.NeedsWork() {
					return errors.New("setup --check found items that need work")
				}
				return nil
			}
			plan, err := project.Apply(command.Context(), sys.Runner, root, projectRoot)
			if err != nil {
				return err
			}
			if err := printPlan(sys, plan, asJSON, "Setup applied to", applyWords); err != nil {
				return err
			}
			if plan.HasFailures() {
				return errors.New("setup could not set up every item; see the failed lines above")
			}
			return nil
		},
	}
	command.Flags().BoolVar(&check, "check", false, "print the plan without changing anything; exit 1 if any item is missing, stale or conflict")
	command.Flags().BoolVar(&asJSON, "json", false, "print the plan as JSON")
	return command
}

// applyWords says what a status Setup acted on became; every other status is
// printed as it is.
var applyWords = map[project.Status]string{project.Missing: "created", project.Stale: "refreshed"}

func printPlan(sys system.System, plan project.Plan, asJSON bool, heading string, words map[project.Status]string) error {
	if asJSON {
		encoder := json.NewEncoder(sys.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(plan)
	}
	fmt.Fprintf(sys.Stdout, "%s %s\n", heading, plan.Root)
	for _, item := range plan.Items {
		path := item.Path
		if item.Block != "" {
			path += "#" + item.Block
		}
		line := fmt.Sprintf("  %-9s  %s", cmp.Or(words[item.Status], string(item.Status)), path)
		if item.Detail != "" {
			line += " (" + item.Detail + ")"
		}
		fmt.Fprintln(sys.Stdout, line)
	}
	return nil
}

// setupTarget picks the checkout Setup works on: the path argument, else T3
// Code's T3CODE_WORKTREE_PATH, else the working directory. T3CODE_PROJECT_ROOT
// is never the target: T3 sets it to the main checkout, so writing there would
// leave the new worktree unchanged.
func setupTarget(sys system.System, args []string) string {
	if len(args) > 0 && filepath.IsAbs(args[0]) {
		return filepath.Clean(args[0])
	}
	if len(args) > 0 {
		return filepath.Join(sys.Workdir, args[0])
	}
	if worktree := sys.Getenv("T3CODE_WORKTREE_PATH"); worktree != "" {
		return worktree
	}
	return sys.Workdir
}

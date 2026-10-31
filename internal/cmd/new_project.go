package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/project"
	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/t3"
)

func newNewProjectCommand(sys system.System) *cobra.Command {
	var description string
	var public bool
	command := &cobra.Command{
		Use:   "new-project <name>",
		Short: "Create a GitHub Project, clone it, set it up and register it with T3 Code",
		Long: `Create the GitHub repository <githubOwner>/<name> (private unless --public),
clone it into <projectsRoot>/<name>, run Setup in the clone, commit
"Initial project setup", push it, and register the clone with T3 Code
(t3 project add). The command stops at the first step that fails.
githubOwner and projectsRoot come from ~/.config/overlord/config.yaml
(defaults PoeAudits and ~/Overlord/projects).`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if err := config.WriteDefault(sys.Home); err != nil {
				return err
			}
			cfg, err := config.Load(sys.Home)
			if err != nil {
				return err
			}
			plan, err := project.Create(command.Context(), sys.Runner, project.NewProject{
				Name:        args[0],
				Owner:       cfg.GitHubOwner,
				Root:        cfg.ProjectsRoot,
				Description: description,
				Public:      public,
				Launcher:    t3.LauncherPath(sys.Home),
			})
			// Text output cannot fail; only JSON encoding can.
			if plan.Root != "" {
				_ = printPlan(sys, plan, false, "Setup applied to", applyWords)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(sys.Stdout, "Created %s/%s at %s and registered it with T3 Code\n", cfg.GitHubOwner, args[0], plan.Root)
			return nil
		},
	}
	command.Flags().StringVar(&description, "description", "", "GitHub repository description")
	command.Flags().BoolVar(&public, "public", false, "create a public repository (default private)")
	return command
}

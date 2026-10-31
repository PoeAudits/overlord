package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/project"
	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/t3"
)

// githubOwner is the account every new Project is created under.
const githubOwner = "PoeAudits"

func newNewProjectCommand(sys system.System) *cobra.Command {
	var description string
	var public bool
	command := &cobra.Command{
		Use:   "new-project <name>",
		Short: "Create a GitHub Project, clone it, set it up and register it with T3 Code",
		Long: `Create the GitHub repository PoeAudits/<name> (private unless --public),
clone it into <projectsRoot>/<name>, run Setup in the clone, commit
"Initial project setup", push it, and register the clone with T3 Code
(t3 project add). The command stops at the first step that fails.`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			name := args[0]
			if name != filepath.Base(name) || name == "." || name == ".." {
				return fmt.Errorf("project name %q must be a single directory name", name)
			}
			cfg, err := config.Load(sys.Home)
			if err != nil {
				return err
			}
			root := cfg.ProjectsRoot
			path := filepath.Join(root, name)
			if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("%s already exists; choose another name or remove it", path)
			}
			if err := os.MkdirAll(root, 0o755); err != nil {
				return err
			}
			ctx := command.Context()
			repo := githubOwner + "/" + name
			create := []string{"repo", "create", repo, visibility(public)}
			if description != "" {
				create = append(create, "--description", description)
			}
			if err := step(ctx, sys, root, "gh", create...); err != nil {
				return err
			}
			if err := step(ctx, sys, root, "gh", "repo", "clone", repo, path); err != nil {
				return err
			}
			plan, err := project.Apply(ctx, sys.Runner, path, path)
			if err != nil {
				return fmt.Errorf("setup in %s: %w", path, err)
			}
			if err := printPlan(sys, plan, false, "Setup applied to", applyWords); err != nil {
				return err
			}
			for _, args := range [][]string{
				{"add", "-A"},
				{"commit", "-m", "Initial project setup"},
				{"push", "-u", "origin", "HEAD"},
			} {
				if err := step(ctx, sys, path, "git", args...); err != nil {
					return err
				}
			}
			if err := step(ctx, sys, path, t3.LauncherPath(sys.Home), "project", "add", "--title", name, path); err != nil {
				return err
			}
			fmt.Fprintf(sys.Stdout, "Created %s at %s and registered it with T3 Code\n", repo, path)
			return nil
		},
	}
	command.Flags().StringVar(&description, "description", "", "GitHub repository description")
	command.Flags().BoolVar(&public, "public", false, "create a public repository (default private)")
	return command
}

func visibility(public bool) string {
	if public {
		return "--public"
	}
	return "--private"
}

// step runs one external command and turns a failure into an error naming
// the command and its first line of output.
func step(ctx context.Context, sys system.System, dir, name string, args ...string) error {
	output, err := sys.Runner.Run(ctx, dir, name, args...)
	if err == nil {
		return nil
	}
	line, _, _ := bytes.Cut(bytes.TrimSpace(output), []byte("\n"))
	command := strings.Join(append([]string{filepath.Base(name)}, args...), " ")
	if len(line) == 0 {
		return fmt.Errorf("%s failed: %w", command, err)
	}
	return fmt.Errorf("%s failed: %s (%w)", command, line, err)
}

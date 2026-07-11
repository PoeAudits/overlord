package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
)

var (
	pathLoadRegistry = loadRegistry
	pathShowPicker   = func(reg *registry.Registry, initialQuery string) (string, registry.Project, error) {
		return showPickerForCommand("path", reg, initialQuery)
	}
)

var pathCmd = &cobra.Command{
	Use:   "path [name...]",
	Short: "Print project paths",
	Long: `Print the full filesystem path for one or more projects in the registry.

Name arguments can be project names, aliases, or fuzzy matches. If no name is
provided, an interactive fuzzy finder is shown.

Examples:
  overlord path overlord
  overlord path oc
  overlord path overlord oc`,
	Args: cobra.ArbitraryArgs,
	RunE: runPath,
}

func init() {
	rootCmd.AddCommand(pathCmd)
}

func runPath(cmd *cobra.Command, args []string) error {
	reg, err := pathLoadRegistry()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		selectedName, project, err := pathShowPicker(reg, "")
		if err != nil || selectedName == "" {
			return err
		}
		return printProjectPath(cmd, reg, project)
	}

	projects := make([]registry.Project, 0, len(args))
	for _, query := range args {
		selectedName, project, err := resolvePathMatch(reg, query)
		if err != nil {
			return err
		}
		if selectedName == "" {
			return nil
		}
		projects = append(projects, project)
	}

	for _, project := range projects {
		if err := printProjectPath(cmd, reg, project); err != nil {
			return err
		}
	}

	return nil
}

func printProjectPath(cmd *cobra.Command, reg *registry.Registry, project registry.Project) error {
	projectPath, err := fileutil.ExpandProjectPath(project.Path, reg.Settings.BaseDir)
	if err != nil {
		return cmdError("path", fmt.Sprintf("failed to expand path for '%s'", project.Path), err.Error())
	}

	fmt.Fprintln(cmd.OutOrStdout(), projectPath)
	return nil
}

func resolvePathMatch(reg *registry.Registry, query string) (string, registry.Project, error) {
	return resolveProjectMatch(
		reg,
		query,
		func(query string) error {
			return cmdError("path", fmt.Sprintf("no project found matching '%s'", query), "Run 'overlord list' to see all projects")
		},
		func(results []registry.ResolveResult) (string, registry.Project, error) {
			return showPickerWithResultsForCommand("path", results)
		},
	)
}

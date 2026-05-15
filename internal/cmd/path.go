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
	Use:   "path [name]",
	Short: "Print a project path",
	Long: `Print the full filesystem path for a project in the registry.

The name argument can be a project name, alias, or fuzzy match. If no name is
provided, an interactive fuzzy finder is shown.

Examples:
  overlord path overlord
  overlord path oc`,
	Args: cobra.MaximumNArgs(1),
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

	var selectedName string
	var project registry.Project
	if len(args) == 0 {
		selectedName, project, err = pathShowPicker(reg, "")
	} else {
		selectedName, project, err = resolvePathMatch(reg, args[0])
	}
	if err != nil {
		return err
	}
	if selectedName == "" {
		return nil
	}

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

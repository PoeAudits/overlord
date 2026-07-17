package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/tmux"
)

var (
	switchLoadRegistry = loadRegistry
	switchOpenWindow   = func(name, dir string) error {
		return tmux.OpenWindowInCurrentSession(name, dir, nil)
	}
)

var switchCmd = &cobra.Command{
	Use:   "switch <name>",
	Short: "Switch to a project in the current tmux session",
	Long: `Select or create a project window in the current tmux session.

The window starts in the Project directory. Name arguments can be project
names, aliases, or fuzzy matches.

Examples:
	  overlord switch agentbench`,
	Args: cobra.ExactArgs(1),
	RunE: runSwitch,
}

func init() {
	rootCmd.AddCommand(switchCmd)
}

func runSwitch(cmd *cobra.Command, args []string) error {
	reg, err := switchLoadRegistry()
	if err != nil {
		return err
	}

	projectName, project, err := resolveProjectMatch(
		reg,
		args[0],
		func(query string) error {
			return cmdError("switch", fmt.Sprintf("no project found matching '%s'", query), "Run 'overlord list' to see all projects")
		},
		func(results []registry.ResolveResult) (string, registry.Project, error) {
			return showPickerWithResultsForCommand("switch", results)
		},
	)
	if err != nil {
		return err
	}

	projectPath, err := fileutil.ExpandProjectPath(project.Path, reg.Settings.BaseDir)
	if err != nil {
		return cmdError("switch", fmt.Sprintf("failed to expand path for '%s'", project.Path), err.Error())
	}
	info, err := os.Stat(projectPath)
	if errors.Is(err, fs.ErrNotExist) {
		return cmdError("switch", fmt.Sprintf("project directory not found at %s", projectPath), "Check the path in 'overlord info <name>' or re-register with 'overlord add'")
	}
	if err != nil {
		return cmdError("switch", fmt.Sprintf("failed to inspect project directory at %s", projectPath), err.Error())
	}
	if !info.IsDir() {
		return cmdError("switch", fmt.Sprintf("project path is not a directory: %s", projectPath), "Check the path in 'overlord info <name>'")
	}

	if err := switchOpenWindow(projectName, projectPath); err != nil {
		if errors.Is(err, tmux.ErrNotInSession) {
			return cmdError("switch", "not inside a tmux session", "Use 'overlord open <name>' to open a Project Workspace")
		}
		return cmdError("switch", fmt.Sprintf("failed to open tmux window for '%s'", projectName), err.Error())
	}
	return nil
}

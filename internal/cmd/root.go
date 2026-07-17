package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var version = "0.1.0" // Will be set by build flags later

var (
	rootRunList = runList
	rootRunOpen = runOpen
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "overlord [project-name]",
	Short: "Project management CLI with tmux workspace integration",
	Long: `Overlord - Project management CLI with tmux workspace integration

Manage development projects with category-based organization, tmux workspaces,
and an AI-first registry.

Usage:
  overlord [command] [flags]
  overlord <name>            Open project workspace (shorthand for 'overlord open')
  overlord                   List active projects (shorthand for 'overlord list')

Project Lifecycle:
  new          Create a new project with full setup
  add          Register an existing project directory
  open         Open a project workspace in tmux
  path         Print a project path
  switch       Open a project window in the current tmux session
  info         Show detailed information about a project
  rm           Remove projects from the registry (keeps files)
  clear        Permanently remove projects and their files

Organization:
  list         List projects in the registry
  backup       Create a backup zip for tagged projects
  archive      Move projects to the archive directory
  unarchive    Restore projects from archive
  detect       Find projects on disk not in the registry
  init         Initialize a directory with project templates

Config:
  setup        Set up overlord configuration or bootstrap a project
  config       Open the registry file in your editor
  code         Attach current directory to local OpenCode

Workflow Examples:
  # Create a new Go service and open it
  overlord new my-api --category=services --go

  # Register an existing project and start working
  overlord add ~/projects/my-app --ts --category=web
  overlord my-app

  # List recent projects, then open one by partial name
  overlord list --recent
  overlord my-a

  # Archive old projects
  overlord archive stale-project

Output Format:
  All commands append a metadata footer to stderr:
    [exit:<code> | <duration>]
  Exit codes: 0 = success, 1 = error, 2 = usage error

Error Conventions:
  Errors are printed to stderr in the format:
    [error] <command>: <problem>. <corrective action>
  Every error message suggests what to do next.

Configuration is stored at ~/.config/overlord/`,
	Version: version,
	// Allow unknown commands to be treated as project names
	SilenceErrors:      true,
	SilenceUsage:       true,
	DisableFlagParsing: false,
	Args:               cobra.ArbitraryArgs,
	RunE:               runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
// Returns the exit code (0 for success, 1 for error, 2 for usage error).
func Execute() int {
	start := time.Now()
	err := rootCmd.Execute()

	code := 0
	if err != nil {
		code = 1
		fmt.Fprintln(os.Stderr, err)
	}

	printExitFooter(code, time.Since(start))
	return code
}

func init() {
	// Local flags
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")

	// Set custom flag error handler for better help messages
	rootCmd.SetFlagErrorFunc(flagErrorFunc)
}

// flagErrorFunc inspects Cobra's string errors to attach targeted usage/help.
// Cobra does not expose structured error types for these parser failures, so
// this function matches known error patterns for required and unknown flags.
func flagErrorFunc(cmd *cobra.Command, err error) error {
	errStr := err.Error()

	if isRequiredFlagError(errStr) && cmd.Name() == "new" {
		return fmt.Errorf("%s\n\n%s", errStr, formatNewUsage())
	}

	if !strings.Contains(errStr, "unknown flag") {
		return err
	}

	usage, hasUsage := usageForCommand(cmd.Name())
	if !hasUsage {
		return err
	}

	suggestion := unknownFlagSuggestion(errStr, cmd.Name())
	if suggestion == "" {
		return fmt.Errorf("%s\n\n%s", errStr, usage)
	}

	return fmt.Errorf("%s\n\n%s\n\n%s", errStr, suggestion, usage)
}

func isRequiredFlagError(errStr string) bool {
	return strings.Contains(errStr, "required flag") && strings.Contains(errStr, "not set")
}

func usageForCommand(commandName string) (string, bool) {
	switch commandName {
	case "new":
		return formatNewUsage(), true
	case "add":
		return formatAddUsage(), true
	default:
		return "", false
	}
}

func unknownFlagSuggestion(errStr, commandName string) string {
	suggestions := map[string]string{
		"--python":     "Use --py for Python",
		"--typescript": "Use --ts for TypeScript",
		"--golang":     "Use --go for Go",
		"--solidity":   "Use --sol for Solidity",
		"--lang":       "Language flags are: --py, --ts, --go, --sol, --base",
		"--language":   "Language flags are: --py, --ts, --go, --sol, --base",
		"--cat":        "Use --category=<category>",
		"--type":       "Use --category=<category>",
		"--template":   "Language flags are: --py, --ts, --go, --sol, --base",
	}

	if commandName == "new" {
		suggestions["--base"] = "Language flags are: --py, --ts, --go, --sol (or omit for base)"
	}

	for flag, suggestion := range suggestions {
		if strings.Contains(errStr, flag) {
			return suggestion
		}
	}

	return ""
}

// runRoot handles the root command - routes to list or open based on args
func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// No args - run list command
		return rootRunList(cmd, args)
	}

	// Has args - treat as project name and run open
	return rootRunOpen(cmd, args)
}

package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var version = "0.1.0" // Will be set by build flags later

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "overlord [project-name]",
	Short: "Project management CLI with tmux workspace integration",
	Long: `Overlord - Project Management CLI

A tool for managing development projects with category-based organization,
tmux workspace integration, and AI-first registry.

Usage:
  overlord [command]
  overlord <name>       Opens project workspace (shorthand for 'overlord open')

When called with a project name (or partial name), opens the project workspace.
When called without arguments, lists active projects.

Configuration is stored at ~/.config/overlord/`,
	Version: version,
	// Allow unknown commands to be treated as project names
	SilenceUsage:       true,
	DisableFlagParsing: false,
	Args:               cobra.ArbitraryArgs,
	RunE:               runRoot,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Local flags
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")

	// Set custom flag error handler for better help messages
	rootCmd.SetFlagErrorFunc(flagErrorFunc)
}

// flagErrorFunc provides better error messages for flag errors
func flagErrorFunc(cmd *cobra.Command, err error) error {
	errStr := err.Error()

	// Handle "required flag not set" errors
	if strings.Contains(errStr, "required flag") && strings.Contains(errStr, "not set") {
		if cmd.Name() == "new" {
			return fmt.Errorf("%s\n\n%s", errStr, formatNewUsage())
		}
	}

	// Handle unknown flag errors with suggestions
	if strings.Contains(errStr, "unknown flag") {
		// Common flag suggestions shared between new and add commands
		flagSuggestions := map[string]string{
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

		if cmd.Name() == "new" {
			flagSuggestions["--base"] = "Language flags are: --py, --ts, --go, --sol (or omit for base)"
			for flag, suggestion := range flagSuggestions {
				if strings.Contains(errStr, flag) {
					return fmt.Errorf("%s\n\n%s\n\n%s", errStr, suggestion, formatNewUsage())
				}
			}
			return fmt.Errorf("%s\n\n%s", errStr, formatNewUsage())
		}

		if cmd.Name() == "add" {
			for flag, suggestion := range flagSuggestions {
				if strings.Contains(errStr, flag) {
					return fmt.Errorf("%s\n\n%s\n\n%s", errStr, suggestion, formatAddUsage())
				}
			}
			return fmt.Errorf("%s\n\n%s", errStr, formatAddUsage())
		}
	}

	return err
}

// runRoot handles the root command - routes to list or open based on args
func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// No args - run list command
		return runList(cmd, args)
	}

	// Has args - treat as project name and run open
	return runOpen(cmd, args)
}

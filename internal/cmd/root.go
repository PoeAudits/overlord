package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version = "0.1.0" // Will be set by build flags later
)

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
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/overlord/config.yaml)")

	// Local flags
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
}

// runRoot handles the root command - routes to list or open based on args
func runRoot(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		// No args - run list command
		return RunList(cmd, args)
	}

	// Has args - treat as project name and run open
	return RunOpen(cmd, args)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Search config in ~/.config/overlord directory with name "config" (without extension)
		configDir := home + "/.config/overlord"
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	// Read in environment variables that match
	viper.SetEnvPrefix("OVERLORD")
	viper.AutomaticEnv()

	// If a config file is found, read it in
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

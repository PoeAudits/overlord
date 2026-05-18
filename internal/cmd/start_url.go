package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

var startURLRemote bool

var (
	startURLLoadStackConfig   = loadStackConfig
	startURLRemoteOpenCodeURL = stack.RemoteOpenCodeURL
)

var startURLCmd = &cobra.Command{
	Use:   "url",
	Short: "Print OpenCode service URL",
	Args:  cobra.NoArgs,
	RunE:  runStartURL,
}

func init() {
	startURLCmd.Flags().BoolVar(&startURLRemote, "remote", false, "print remote Tailscale URL")
	startCmd.AddCommand(startURLCmd)
}

func runStartURL(cmd *cobra.Command, args []string) error {
	cfg, err := startURLLoadStackConfig()
	if err != nil {
		return cmdError("start url", "failed to load stack config", err.Error())
	}

	if !startURLRemote {
		fmt.Fprintln(cmd.OutOrStdout(), stack.OpenCodeURL(cfg))
		return nil
	}

	remote, err := startURLRemoteOpenCodeURL(cfg)
	if err != nil {
		return cmdError("start url", "failed to resolve remote URL", err.Error())
	}
	fmt.Fprintln(cmd.OutOrStdout(), remote)
	return nil
}

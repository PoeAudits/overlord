package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

var (
	startPairLoadStackConfig = loadStackConfig
	startPairRemoteT3CodeURL = stack.RemoteT3CodeURL
	startPairRunT3CLI        = runT3CLI
)

var startPairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Print phone-ready T3 pairing URL and QR code",
	Args:  cobra.NoArgs,
	RunE:  runStartPair,
}

func init() {
	startCmd.AddCommand(startPairCmd)
}

func runStartPair(cmd *cobra.Command, args []string) error {
	cfg, err := startPairLoadStackConfig()
	if err != nil {
		return cmdError("start pair", "failed to load stack config", err.Error())
	}

	baseURL, err := startPairRemoteT3CodeURL(cfg)
	if err != nil {
		baseURL = stack.T3CodeURL(cfg)
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to resolve remote Tailscale URL, using local URL %s: %v\n", baseURL, err)
	}

	cliArgs := startPairCLIArgs(stack.T3CodeBaseDir(cfg), baseURL)
	output, err := startPairRunT3CLI(stack.T3CodeWorkDir(cfg), cliArgs)
	if err != nil {
		return cmdError("start pair", "failed to create T3 pairing credential", err.Error())
	}
	_, _ = cmd.OutOrStdout().Write(output)
	return nil
}

func startPairCLIArgs(baseDir string, baseURL string) []string {
	return []string{
		filepath.Join("apps", "server", "dist", "bin.mjs"),
		"auth",
		"pairing",
		"create",
		"--base-dir",
		baseDir,
		"--base-url",
		baseURL,
		"--headless-output",
	}
}

func runT3CLI(workDir string, args []string) ([]byte, error) {
	cmd := exec.Command("node", args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "NODE_NO_WARNINGS=1")
	return cmd.CombinedOutput()
}

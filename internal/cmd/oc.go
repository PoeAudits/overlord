package cmd

import (
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

var codeCmd = &cobra.Command{
	Use:   "code",
	Short: "Attach current directory to local OpenCode",
	Args:  cobra.NoArgs,
	RunE:  runCode,
}

var (
	ocLoadStackConfig       = loadStackConfig
	ocStartStack            = stack.StartStack
	ocResolveOpenCodeBinary = stack.ResolveOpenCodeBinary
	ocGetwd                 = os.Getwd
	ocAttachDirectory       = attachOpenCodeDirectory
)

func init() {
	rootCmd.AddCommand(codeCmd)
}

func runCode(cmd *cobra.Command, args []string) error {
	cfg, err := ocLoadStackConfig()
	if err != nil {
		return cmdError("code", "failed to load stack config", err.Error())
	}

	if _, err := ocStartStack(cfg); err != nil {
		return cmdError("code", "failed to ensure stack", err.Error())
	}

	cwd, err := ocGetwd()
	if err != nil {
		return cmdError("code", "failed to resolve current directory", err.Error())
	}

	if err := ocAttachDirectory(
		ocResolveOpenCodeBinary(),
		stack.OpenCodeURL(cfg),
		cwd,
		cmd.InOrStdin(),
		cmd.OutOrStdout(),
		cmd.ErrOrStderr(),
	); err != nil {
		return cmdError("code", "attach failed", err.Error())
	}

	return nil
}

func attachOpenCodeDirectory(binary, url, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
	cmd := exec.Command(binary, "attach", url, "--dir", dir)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Open the registry file in your editor",
	Long: `Open the overlord registry file in your default editor.

Uses $EDITOR environment variable, falling back to nvim, vim, then vi.

Examples:
  overlord config                    # Open registry in $EDITOR
  EDITOR=code overlord config        # Open registry in VS Code`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	// Expand registry path
	registryPath, err := expandPath(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to expand registry path: %w", err)
	}

	// Check if registry file exists
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		return fmt.Errorf("registry file not found: %s\nRun 'overlord setup' to initialize", registryPath)
	}

	// Determine editor
	editor := resolveEditor()

	// Open editor
	editorCmd := exec.Command(editor, registryPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor '%s': %w", editor, err)
	}

	return nil
}

// resolveEditor returns the editor to use, checking $EDITOR then fallbacks.
func resolveEditor() string {
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Fallback chain: nvim -> vim -> vi
	for _, editor := range []string{"nvim", "vim", "vi"} {
		if _, err := exec.LookPath(editor); err == nil {
			return editor
		}
	}

	return "vi"
}

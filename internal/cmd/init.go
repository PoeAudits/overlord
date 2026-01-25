package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize overlord directories and configuration",
	Long: `Initialize all required directories and configuration files for overlord.

Creates:
  - Config directory (~/.config/overlord/)
  - Registry file (~/.config/overlord/registry.yaml)
  - Base directory (~/Overlord by default)
  - Category directories (projects/*, core/*, sandbox/)
  - Thoughts directory (~/thoughts by default)

If directories or files already exist, they are left unchanged.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Initializing overlord...")

	// Load or create registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Expand paths from settings
	baseDir, err := registry.ExpandPath(reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base_dir: %w", err)
	}

	thoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts_dir: %w", err)
	}

	// Create config directory (handled by Save, but be explicit)
	configDir, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand config path: %w", err)
	}

	if err := createDir(configDir, "config"); err != nil {
		return err
	}

	// Create base directory
	if err := createDir(baseDir, "base"); err != nil {
		return err
	}

	// Create category directories
	for _, category := range registry.ValidCategories() {
		categoryPath := filepath.Join(baseDir, category.Path())
		if err := createDir(categoryPath, string(category)); err != nil {
			return err
		}
	}

	// Create thoughts directory
	if err := createDir(thoughtsDir, "thoughts"); err != nil {
		return err
	}

	// Save registry (creates it if it doesn't exist)
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	fmt.Printf("  Registry: %s\n", registry.DefaultRegistryPath)

	fmt.Println("\nInitialization complete!")
	return nil
}

// createDir creates a directory if it doesn't exist
func createDir(path, name string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s directory: %w", name, err)
		}
		fmt.Printf("  Created: %s\n", path)
	} else if err != nil {
		return fmt.Errorf("failed to check %s directory: %w", name, err)
	} else {
		fmt.Printf("  Exists:  %s\n", path)
	}
	return nil
}

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/PoeAudits/overlord/internal/gitops"
	"github.com/PoeAudits/overlord/internal/registry"
	"golang.org/x/term"
)

// loadRegistry loads the registry from the default path.
func loadRegistry() (*registry.Registry, error) {
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}
	return reg, nil
}

// saveRegistry saves the registry to the default path.
func saveRegistry(reg *registry.Registry) error {
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	return nil
}

// commitAndPushRegistry stages, commits, and pushes registry.yaml changes.
// The action parameter is used as the commit message (e.g. "activate project-name").
// Returns an error only for stage/commit failures; push failures are warned but not fatal.
func commitAndPushRegistry(action string) error {
	gitRepoPath, err := registry.ExpandPath("~/.config/overlord")
	if err != nil {
		return fmt.Errorf("failed to expand git repo path: %w", err)
	}

	git := gitops.New(gitRepoPath)

	if err := git.Add(context.Background(), "registry.yaml"); err != nil {
		return fmt.Errorf("failed to stage registry: %w", err)
	}

	if err := git.Commit(context.Background(), action); err != nil {
		return fmt.Errorf("failed to commit registry: %w", err)
	}

	// Push - warn but don't fail if push fails
	if err := git.Push(context.Background()); err != nil {
		s := NewStyles()
		fmt.Printf("  %s Failed to push registry: %v\n", s.Warn.Render("Warning:"), err)
	}

	return nil
}

// confirmAction prompts the user with the given prompt followed by " [y/N]: "
// and returns true if the user responds with "y" or "yes".
// Returns (false, nil) if not in a terminal.
func confirmAction(prompt string) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, nil
	}

	fmt.Printf("%s [y/N]: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// expandBaseDir expands the base directory path from registry settings.
func expandBaseDir(reg *registry.Registry) (string, error) {
	baseDir, err := registry.ExpandPath(reg.Settings.BaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base directory: %w", err)
	}
	return baseDir, nil
}

// countTrue returns the number of true boolean values in the provided flags.
func countTrue(flags ...bool) int {
	count := 0
	for _, flag := range flags {
		if flag {
			count++
		}
	}
	return count
}

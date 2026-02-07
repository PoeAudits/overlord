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

	hasRemote, err := git.HasRemote(context.Background())
	if err != nil {
		return fmt.Errorf("failed to verify registry remote: %w", err)
	}
	if !hasRemote {
		s := NewStyles()
		fmt.Printf("  %s Registry committed locally, but no git remote is configured for ~/.config/overlord\n", s.Warn.Render("Warning:"))
		return nil
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
	return confirm(prompt, "[y/N]: ", func(r string) bool {
		return r == "y" || r == "yes"
	})
}

// confirmStrict prompts the user and requires typing "yes" (not just "y").
// Use for destructive operations that cannot be undone.
// Returns (false, nil) if not in a terminal.
func confirmStrict(prompt string) (bool, error) {
	return confirm(prompt, "Type 'yes' to confirm: ", func(r string) bool {
		return r == "yes"
	})
}

// confirm is the shared implementation for confirmation prompts.
func confirm(prompt, suffix string, accept func(string) bool) (bool, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, nil
	}

	fmt.Printf("%s %s", prompt, suffix)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return accept(response), nil
}

// expandBaseDir expands the base directory path from registry settings.
func expandBaseDir(reg *registry.Registry) (string, error) {
	baseDir, err := registry.ExpandPath(reg.Settings.BaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base directory: %w", err)
	}
	return baseDir, nil
}

// resolveProject resolves a project name using fuzzy matching.
// Returns the resolved name and project, or an error with a helpful message.
func resolveProject(reg *registry.Registry, name string) (string, registry.Project, error) {
	result, err := reg.ResolveOne(name)
	if err != nil {
		return "", registry.Project{}, fmt.Errorf("project '%s' not found\nRun 'overlord list' to see all projects", name)
	}
	return result.Name, result.Project, nil
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

// mergeExcludes combines global and per-project exclusion patterns.
func mergeExcludes(global, project []string) []string {
	excludes := make([]string, 0, len(global)+len(project))
	excludes = append(excludes, global...)
	excludes = append(excludes, project...)
	return excludes
}

// languageFromFlags determines the language from boolean flags.
// Returns the language and whether any flag was explicitly set.
func languageFromFlags(py, ts, gol, sol, base bool) (registry.Language, bool) {
	switch {
	case py:
		return registry.LanguagePython, true
	case ts:
		return registry.LanguageTypeScript, true
	case gol:
		return registry.LanguageGo, true
	case sol:
		return registry.LanguageSolidity, true
	case base:
		return registry.LanguageBase, true
	default:
		return "", false
	}
}

// validateCategory validates a category string and returns the typed Category.
// Returns an error with a helpful message listing valid categories if invalid.
func validateCategory(name string) (registry.Category, error) {
	category := registry.Category(name)
	if !category.IsValid() {
		validCats := make([]string, 0, len(registry.ValidCategories()))
		for _, c := range registry.ValidCategories() {
			validCats = append(validCats, string(c))
		}
		return "", fmt.Errorf("invalid category '%s'\nValid categories: %s", name, strings.Join(validCats, ", "))
	}
	return category, nil
}

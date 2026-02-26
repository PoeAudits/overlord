package cmd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/machine"
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
	if err := enforceRegistryLockOwnership(); err != nil {
		return err
	}

	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	return nil
}

func enforceRegistryLockOwnership() error {
	lock, err := machine.LoadLock(machine.DefaultRegistryLockPath)
	if err != nil {
		return fmt.Errorf("failed to load registry lock: %w", err)
	}
	if lock == nil {
		return nil
	}

	machineConfig, err := machine.Load(machine.DefaultConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load machine config for lock check: %w", err)
	}

	localMachine := machineConfig.Name
	if lock.Owner == localMachine {
		return nil
	}

	return fmt.Errorf(
		"registry lock is owned by '%s' and this machine is '%s'\n"+
			"Run on owner machine: overlord sync lock transfer %s\n"+
			"If owner is offline, run: overlord sync lock claim",
		lock.Owner,
		localMachine,
		localMachine,
	)
}

// confirmAction allows a quick "y" acknowledgement for reversible operations,
// accepting "y" or "yes" to lower friction for common confirmations.
// Returns (false, nil) if not in a terminal.
func confirmAction(prompt string) (bool, error) {
	return confirm(prompt, "[y/N]: ", func(r string) bool {
		return r == "y" || r == "yes"
	})
}

// confirmStrict requires typing the full word "yes" to prevent accidental
// destructive operations — a single keystroke is not sufficient to confirm.
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
		return "", registry.Project{}, fmt.Errorf("%w\nhint: run 'overlord list' to see all projects", err)
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

// languageFromFlags determines the language from boolean flags.
// Returns the language and whether any flag was explicitly set.
func languageFromFlags(py, ts, goLang, sol, base bool) (registry.Language, bool) {
	switch {
	case py:
		return registry.LanguagePython, true
	case ts:
		return registry.LanguageTypeScript, true
	case goLang:
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

// confirmDelete prompts the user for confirmation before deleting.
func confirmDelete(projectName, projectPath string) (bool, error) {
	return confirmAction(fmt.Sprintf("Delete project '%s' and all files at %s?", projectName, projectPath))
}

// confirmArchiveMultiple prompts the user for confirmation before archiving multiple projects.
func confirmArchiveMultiple(projects []resolvedProject) (bool, error) {
	if len(projects) == 1 {
		p := projects[0]
		fmt.Printf("Archive project '%s'?\n", p.name)
		fmt.Printf("  From: %s\n", fileutil.FormatPathWithTilde(p.sourcePath))
		fmt.Printf("  To:   %s\n", fileutil.FormatPathWithTilde(p.destPath))
	} else {
		fmt.Printf("Archive %d projects?\n", len(projects))
		for _, p := range projects {
			fmt.Printf("  %s\n", p.name)
			fmt.Printf("    From: %s\n", fileutil.FormatPathWithTilde(p.sourcePath))
			fmt.Printf("    To:   %s\n", fileutil.FormatPathWithTilde(p.destPath))
		}
	}
	return confirmAction("Continue?")
}

// confirmClear displays everything that will be removed and prompts for confirmation.
func confirmClear(targets []clearTarget) (bool, error) {
	// Styles for the warning
	s := NewStyles()
	warnStyle := s.Error
	nameStyle := s.Label
	pathStyle := s.Path
	dimStyle := s.Dim

	fmt.Println()
	fmt.Printf("%s This will permanently delete the following:\n\n", warnStyle.Render("WARNING:"))

	for _, t := range targets {
		fmt.Printf("  %s\n", nameStyle.Render(t.name))
		fmt.Printf("    %s\n", dimStyle.Render("Registry entry"))

		if t.projectExists {
			fmt.Printf("    %s\n", pathStyle.Render(fileutil.FormatPathWithTilde(t.projectPath)))
		} else {
			fmt.Printf("    %s %s\n", pathStyle.Render(fileutil.FormatPathWithTilde(t.projectPath)), dimStyle.Render("(not found)"))
		}

		if t.thoughtsExists {
			fmt.Printf("    %s\n", pathStyle.Render(fileutil.FormatPathWithTilde(t.thoughtsDir)))
		}

		if t.archiveExists {
			fmt.Printf("    %s\n", pathStyle.Render(fileutil.FormatPathWithTilde(t.archiveDir)))
		}

		if !t.thoughtsExists && !t.archiveExists {
			fmt.Printf("    %s\n", dimStyle.Render("No thoughts directories found"))
		}

		fmt.Println()
	}

	fmt.Printf("%s This cannot be undone.\n", warnStyle.Render("CAUTION:"))
	return confirmStrict("")
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	base := float64(value)
	exp := int(math.Min(float64(len(units)-1), math.Floor(math.Log(base)/math.Log(1024))))
	formatted := base / math.Pow(1024, float64(exp))
	return fmt.Sprintf("%.1f %s", formatted, units[exp])
}

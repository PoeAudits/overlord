package cmd

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/lifecycle"
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

	return cmdError("registry",
		fmt.Sprintf("lock is owned by '%s' and this machine is '%s'", lock.Owner, localMachine),
		fmt.Sprintf("Run on owner machine: overlord sync lock transfer %s. If owner is offline: overlord sync lock claim", localMachine))
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

func buildThoughtsSettings(reg *registry.Registry, s styles) map[string]string {
	settings := map[string]string{}
	if reg == nil {
		return settings
	}

	thoughtsDir := reg.Settings.ThoughtsDir
	if thoughtsDir == "" {
		return settings
	}

	expandedThoughtsDir, err := registry.ExpandPath(thoughtsDir)
	if err != nil {
		fmt.Printf("%s Failed to expand thoughts directory: %v\n", s.Warn.Render("Warning:"), err)
		settings["thoughts_dir"] = thoughtsDir
		return settings
	}

	settings["thoughts_dir"] = expandedThoughtsDir
	return settings
}

func fireProjectEvent(eventType lifecycle.EventType, name, path, thoughtsDir string) error {
	settings := map[string]string{}
	if thoughtsDir != "" {
		settings["thoughts_dir"] = thoughtsDir
	}

	result := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        eventType,
		ProjectName: name,
		ProjectPath: path,
		Settings:    settings,
	})
	if result.Err != nil {
		return fmt.Errorf("failed to fire %s lifecycle event for project '%s': %w", eventType, name, result.Err)
	}

	return nil
}

func withHintOnNoProjectMatch(err error, hint string) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "no project found matching") {
		return fmt.Errorf("%w\nhint: %s", err, hint)
	}
	return err
}

func resolveProjectTargets(reg *registry.Registry, args []string, all bool) ([]string, error) {
	if all {
		names := make([]string, 0, len(reg.Projects))
		for name := range reg.Projects {
			names = append(names, name)
		}
		sort.Strings(names)
		return names, nil
	}

	seen := make(map[string]struct{}, len(args))
	targets := make([]string, 0, len(args))
	for _, arg := range args {
		result, err := reg.ResolveOne(arg)
		if err != nil {
			return nil, err
		}
		resolved := result.Name
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		targets = append(targets, resolved)
	}

	return targets, nil
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
		return "", cmdError("validate", fmt.Sprintf("invalid category '%s'", name), fmt.Sprintf("Valid categories: %s", strings.Join(validCats, ", ")))
	}
	return category, nil
}

// confirmDelete prompts the user for confirmation before deleting.
func confirmDelete(projectName, projectPath string) (bool, error) {
	return confirmStrict(fmt.Sprintf("Delete project '%s' and all files at %s?", projectName, projectPath))
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
		printClearTarget(t, nameStyle, pathStyle, dimStyle)
	}

	fmt.Printf("%s This cannot be undone.\n", warnStyle.Render("CAUTION:"))
	return confirmStrict("")
}

func printClearTarget(t clearTarget, nameStyle, pathStyle, dimStyle interface{ Render(...string) string }) {
	fmt.Printf("  %s\n", nameStyle.Render(t.name))
	fmt.Printf("    %s\n", dimStyle.Render("Registry entry"))

	projectPath := pathStyle.Render(fileutil.FormatPathWithTilde(t.projectPath))
	if !t.projectExists {
		fmt.Printf("    %s %s\n", projectPath, dimStyle.Render("(not found)"))
	} else {
		fmt.Printf("    %s\n", projectPath)
	}

	printedThoughts := false
	if t.thoughtsExists {
		fmt.Printf("    %s\n", pathStyle.Render(fileutil.FormatPathWithTilde(t.thoughtsDir)))
		printedThoughts = true
	}
	if t.archiveExists {
		fmt.Printf("    %s\n", pathStyle.Render(fileutil.FormatPathWithTilde(t.archiveDir)))
		printedThoughts = true
	}
	if !printedThoughts {
		fmt.Printf("    %s\n", dimStyle.Render("No thoughts directories found"))
	}

	fmt.Println()
}

// formatDuration returns a human-readable duration string for the metadata footer.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dus", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
}

// printExitFooter writes the [exit:<code> | <duration>] metadata footer to stderr.
func printExitFooter(code int, d time.Duration) {
	fmt.Fprintf(os.Stderr, "[exit:%d | %s]\n", code, formatDuration(d))
}

// cmdError formats a navigational error message following the pattern:
//
//	[error] <command>: <problem>. <suggestion>
//
// The suggestion tells the caller what to do instead. Use empty suggestion
// when the problem is self-explanatory or the corrective action is obvious.
func cmdError(command, problem, suggestion string) error {
	if suggestion == "" {
		return fmt.Errorf("[error] %s: %s", command, problem)
	}
	return fmt.Errorf("[error] %s: %s. %s", command, problem, suggestion)
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

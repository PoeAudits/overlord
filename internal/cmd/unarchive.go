package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"overlord-v2/internal/registry"
)

// categoryPickerModel is the Bubbletea model for category selection
type categoryPickerModel struct {
	categories []registry.Category
	cursor     int
	selected   *registry.Category
	cancelled  bool
}

func newCategoryPickerModel(categories []registry.Category) categoryPickerModel {
	return categoryPickerModel{
		categories: categories,
		cursor:     0,
	}
}

func (m categoryPickerModel) Init() tea.Cmd {
	return nil
}

func (m categoryPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if len(m.categories) > 0 && m.cursor < len(m.categories) {
				m.selected = &m.categories[m.cursor]
			}
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.cursor < len(m.categories)-1 {
				m.cursor++
			}
			return m, nil
		}
	}

	return m, nil
}

func (m categoryPickerModel) View() string {
	var b strings.Builder

	// Styles
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	gray := lipgloss.Color("245")
	white := lipgloss.Color("255")

	promptStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(white)
	helpStyle := lipgloss.NewStyle().Foreground(gray)

	// Header
	b.WriteString(promptStyle.Render("Select category:"))
	b.WriteString("\n\n")

	// Show categories
	for i, cat := range m.categories {
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + string(cat)))
		} else {
			b.WriteString(normalStyle.Render("  " + string(cat)))
		}
		b.WriteString("\n")
	}

	// Footer with help
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ navigate • enter select • esc cancel"))

	return b.String()
}

// unarchiveCmd represents the unarchive command
var unarchiveCmd = &cobra.Command{
	Use:   "unarchive <name>",
	Short: "Restore a project from archive to an active category",
	Long: `Restore a project from archive to an active category.

The project will be moved from ~/Overlord/archive/{name}/ to the appropriate
category directory (e.g., ~/Overlord/projects/web/{name}/), and its registry
status will be updated to 'active'.

If --category is not provided, an interactive picker will be shown.

Examples:
  overlord unarchive my-project --category=web
  overlord unarchive my-project              # Interactive category picker`,
	Args: cobra.ExactArgs(1),
	RunE: runUnarchive,
}

var unarchiveCategoryFlag string

func init() {
	rootCmd.AddCommand(unarchiveCmd)
	unarchiveCmd.Flags().StringVar(&unarchiveCategoryFlag, "category", "", "Target category for the project")
}

func runUnarchive(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Resolve project name (support aliases)
	resolvedName := projectName
	project, ok := reg.GetProject(projectName)
	if !ok {
		// Try alias
		name, proj, found := reg.GetProjectByAlias(projectName)
		if !found {
			return fmt.Errorf("Error: project '%s' not found\nRun 'overlord list --all' to see all projects", projectName)
		}
		resolvedName = name
		project = proj
	}

	// Check if project is archived
	if project.Status.State != registry.StateArchived {
		return fmt.Errorf("Error: project '%s' is not archived\nCurrent state: %s", resolvedName, project.Status.State)
	}

	// Get or prompt for category
	var targetCategory registry.Category
	if unarchiveCategoryFlag != "" {
		targetCategory = registry.Category(unarchiveCategoryFlag)
		if !targetCategory.IsValid() {
			validCats := registry.ValidCategories()
			catNames := make([]string, len(validCats))
			for i, c := range validCats {
				catNames[i] = string(c)
			}
			return fmt.Errorf("invalid category '%s'\nValid categories: %s",
				unarchiveCategoryFlag, strings.Join(catNames, ", "))
		}
	} else {
		// Show interactive picker
		cat, err := showCategoryPicker()
		if err != nil {
			return err
		}
		if cat == nil {
			// User cancelled
			return nil
		}
		targetCategory = *cat
	}

	// Calculate paths
	sourcePath, err := expandProjectPath(filepath.Join("archive", resolvedName), reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}

	targetPath, err := expandProjectPath(filepath.Join(targetCategory.Path(), resolvedName), reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand target path: %w", err)
	}

	// Check if source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("Error: project directory not found\nExpected location: %s", sourcePath)
	}

	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("Error: target directory already exists\nLocation: %s\nRemove it first or choose a different category", targetPath)
	}

	// Create target parent directory if needed
	targetParent := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetParent, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Move directory (try rename first, fallback to copy+delete for cross-filesystem)
	if err := os.Rename(sourcePath, targetPath); err != nil {
		// Check if it's a cross-device link error
		if strings.Contains(err.Error(), "cross-device") || strings.Contains(err.Error(), "invalid cross-device link") {
			fmt.Println("Cross-filesystem move detected, copying files...")
			if err := copyDirectory(sourcePath, targetPath); err != nil {
				return fmt.Errorf("failed to copy directory: %w", err)
			}

			// Remove source after successful copy
			if err := os.RemoveAll(sourcePath); err != nil {
				return fmt.Errorf("failed to remove source directory after copy: %w", err)
			}
		} else {
			return fmt.Errorf("failed to move directory: %w", err)
		}
	}

	// Update registry
	project.Path = filepath.Join(targetCategory.Path(), resolvedName)
	project.Category = targetCategory
	project.Status.State = registry.StateActive
	reg.Projects[resolvedName] = project

	// Save registry
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}

	// Update thoughts symlinks
	if err := updateThoughtsSymlinks(resolvedName, targetPath, reg.Settings); err != nil {
		// Non-fatal - warn but continue
		fmt.Fprintf(os.Stderr, "Warning: failed to update thoughts symlinks: %v\n", err)
	}

	// Print success message
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	pathStyle := lipgloss.NewStyle().Foreground(gray)

	fmt.Printf("%s '%s' to category '%s'\n",
		successStyle.Render("Unarchived"),
		resolvedName,
		targetCategory)
	fmt.Printf("  %s %s\n", labelStyle.Render("From:"), pathStyle.Render(formatPathWithTilde(sourcePath)))
	fmt.Printf("  %s %s\n", labelStyle.Render("To:"), pathStyle.Render(formatPathWithTilde(targetPath)))

	return nil
}

// showCategoryPicker shows the interactive category picker
func showCategoryPicker() (*registry.Category, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, fmt.Errorf("interactive picker requires a terminal\nProvide a category: overlord unarchive <name> --category=<cat>")
	}

	categories := registry.ValidCategories()
	m := newCategoryPickerModel(categories)

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("picker error: %w", err)
	}

	result := finalModel.(categoryPickerModel)
	if result.cancelled {
		return nil, nil
	}

	return result.selected, nil
}

// updateThoughtsSymlinks updates symlinks in thoughts directory
func updateThoughtsSymlinks(projectName, projectPath string, settings registry.Settings) error {
	thoughtsDir, err := expandPath(settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	projectThoughtsDir := filepath.Join(thoughtsDir, "projects", projectName)

	// Check if thoughts directory exists
	if _, err := os.Stat(projectThoughtsDir); os.IsNotExist(err) {
		// No thoughts directory - nothing to update
		return nil
	}

	// Remove old symlinks
	entries, err := os.ReadDir(projectThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to read thoughts directory: %w", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(projectThoughtsDir, entry.Name())
		info, err := os.Lstat(entryPath)
		if err != nil {
			continue
		}

		// Remove if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(entryPath); err != nil {
				return fmt.Errorf("failed to remove old symlink %s: %w", entryPath, err)
			}
		}
	}

	// Create new symlinks
	// Calculate relative path from thoughts project dir to project dir
	relPath, err := filepath.Rel(projectThoughtsDir, projectPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Common symlink targets
	symlinkTargets := []string{"README.md", "AGENTS.md", "docs", ".git"}

	for _, target := range symlinkTargets {
		targetPath := filepath.Join(projectPath, target)
		if _, err := os.Stat(targetPath); err == nil {
			// Target exists - create symlink
			linkPath := filepath.Join(projectThoughtsDir, target)
			linkTarget := filepath.Join(relPath, target)

			if err := os.Symlink(linkTarget, linkPath); err != nil {
				// Non-fatal - continue with other symlinks
				fmt.Fprintf(os.Stderr, "Warning: failed to create symlink %s: %v\n", linkPath, err)
			}
		}
	}

	return nil
}

package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
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
	s := NewStyles()
	promptStyle := s.Prompt
	selectedStyle := s.Selected
	normalStyle := s.Name
	helpStyle := s.Desc

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

Thoughts directories are restored alongside the project:
  From: ~/thoughts/archive/{name}/{briefs,research,plans,logs,docs}/
  To:   ~/thoughts/projects/{name}/{briefs,research,plans,logs,docs}/

Symlinks in docs/ and project-root thoughts symlink are updated.

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
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve project name (support aliases)
	resolvedName := projectName
	project, ok := reg.ProjectByName(projectName)
	if !ok {
		// Try alias
		name, proj, found := reg.ProjectByAlias(projectName)
		if !found {
			return fmt.Errorf("project '%s' not found\nRun 'overlord list --all' to see all projects", projectName)
		}
		resolvedName = name
		project = proj
	}

	// Check if project is archived
	if project.Status.State != registry.StateArchived {
		return fmt.Errorf("project '%s' is not archived\nCurrent state: %s", resolvedName, project.Status.State)
	}

	// Get or prompt for category
	var targetCategory registry.Category
	if unarchiveCategoryFlag != "" {
		var err error
		targetCategory, err = validateCategory(unarchiveCategoryFlag)
		if err != nil {
			return err
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
	sourcePath, err := fileutil.ExpandProjectPath(filepath.Join("archive", resolvedName), reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand source path: %w", err)
	}

	targetPath, err := fileutil.ExpandProjectPath(filepath.Join(targetCategory.Path(), resolvedName), reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand target path: %w", err)
	}

	// Check if source exists
	if _, err := os.Stat(sourcePath); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("project directory not found\nExpected location: %s", sourcePath)
	}

	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("target directory already exists\nLocation: %s\nRemove it first or choose a different category", targetPath)
	}

	// Create target parent directory if needed
	targetParent := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Define styles for output
	s := NewStyles()

	thoughtsDir := reg.Settings.ThoughtsDir
	settings := map[string]string{}
	thoughtsRestored := false
	if expandedThoughtsDir, err := registry.ExpandPath(thoughtsDir); err != nil {
		fmt.Printf("%s Failed to expand thoughts directory: %v\n", s.StrongWarn.Render("Warning:"), err)
	} else {
		settings["thoughts_dir"] = expandedThoughtsDir
		archiveThoughtsBase := filepath.Join(expandedThoughtsDir, "archive", resolvedName)
		if _, err := os.Stat(archiveThoughtsBase); err == nil {
			thoughtsRestored = true
		}
	}

	preUnarchiveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectUnarchiving,
		ProjectName: resolvedName,
		ProjectPath: sourcePath,
		DestPath:    targetPath,
		Settings:    settings,
	})
	if preUnarchiveResult.Err != nil {
		return fmt.Errorf("pre-unarchive hook failed: %w", preUnarchiveResult.Err)
	}

	// Move directory
	if err := fileutil.MoveDirectory(sourcePath, targetPath); err != nil {
		return fmt.Errorf("failed to move directory: %w", err)
	}
	var thoughtsWarnings []string

	if thoughtsSetting, ok := settings["thoughts_dir"]; ok {
		if warning, err := EnsureProjectThoughtsSymlink(thoughtsSetting, resolvedName, targetPath, false); err != nil {
			thoughtsWarnings = append(thoughtsWarnings, fmt.Sprintf("failed to update project thoughts symlink: %v", err))
		} else if warning != "" {
			thoughtsWarnings = append(thoughtsWarnings, warning)
		}
	}

	// Update registry
	project.Path = filepath.Join(targetCategory.Path(), resolvedName)
	project.Category = targetCategory
	project.Status.State = registry.StateActive
	reg.Projects[resolvedName] = project

	// Save registry
	if err := saveRegistry(reg); err != nil {
		fmt.Printf("%s Registry save failed after directory move; filesystem and registry may be out of sync: %v\n",
			s.StrongWarn.Render("Warning:"), err)
		return err
	}

	postUnarchiveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectUnarchived,
		ProjectName: resolvedName,
		ProjectPath: targetPath,
		DestPath:    targetPath,
		Settings:    settings,
	})
	thoughtsWarnings = append(thoughtsWarnings, postUnarchiveResult.Warnings...)

	labelStyle := s.Label
	successStyle := s.Success
	pathStyle := s.Path

	// Print success message
	fmt.Printf("%s '%s' to category '%s'\n",
		successStyle.Render("Unarchived"),
		resolvedName,
		targetCategory)
	fmt.Printf("  %s %s\n", labelStyle.Render("From:"), pathStyle.Render(fileutil.FormatPathWithTilde(sourcePath)))
	fmt.Printf("  %s %s\n", labelStyle.Render("To:"), pathStyle.Render(fileutil.FormatPathWithTilde(targetPath)))

	// Print thoughts status
	if thoughtsRestored {
		fmt.Printf("  %s restored from %s/archive/%s/\n", labelStyle.Render("Thoughts:"), thoughtsDir, resolvedName)
	}
	fmt.Println()

	// Print any warnings from thoughts restore
	if len(thoughtsWarnings) > 0 {
		for _, warning := range thoughtsWarnings {
			fmt.Printf("%s %s\n", s.StrongWarn.Render("Warning:"), warning)
		}
		fmt.Println()
	}

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

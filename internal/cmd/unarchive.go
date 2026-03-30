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
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	resolvedName, project, err := resolveArchivedProject(reg, projectName)
	if err != nil {
		return err
	}

	targetCategory, cancelled, err := resolveUnarchiveCategory()
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}

	sourcePath, targetPath, err := validateUnarchiveTarget(reg, resolvedName, targetCategory)
	if err != nil {
		return err
	}

	s := NewStyles()
	settings := buildThoughtsSettings(reg, s)
	thoughtsRestored := thoughtsArchiveExists(settings, resolvedName)

	if err := firePreUnarchiveEvent(resolvedName, sourcePath, targetPath, settings); err != nil {
		return err
	}

	thoughtsWarnings, err := performUnarchive(resolvedName, sourcePath, targetPath, settings)
	if err != nil {
		return err
	}

	if err := updateRegistryForUnarchive(reg, resolvedName, project, targetCategory, s); err != nil {
		return err
	}

	thoughtsWarnings = append(thoughtsWarnings, firePostUnarchiveEvent(resolvedName, targetPath, settings)...)
	printUnarchiveSuccess(s, resolvedName, targetCategory, sourcePath, targetPath, reg.Settings.ThoughtsDir, thoughtsRestored, thoughtsWarnings)

	return nil
}

func resolveArchivedProject(reg *registry.Registry, projectName string) (string, registry.Project, error) {
	result, err := reg.ResolveOne(projectName)
	if err != nil {
		return "", registry.Project{}, withHintOnNoProjectMatch(err, "run 'overlord list --all' to see all projects")
	}

	if result.Project.Status.State != registry.StateArchived {
		return "", registry.Project{}, cmdError("unarchive", fmt.Sprintf("project '%s' is not archived (current state: %s)", result.Name, result.Project.Status.State), "Use 'overlord archive <name>' to archive it first")
	}

	return result.Name, result.Project, nil
}

func resolveUnarchiveCategory() (registry.Category, bool, error) {
	if unarchiveCategoryFlag != "" {
		category, err := validateCategory(unarchiveCategoryFlag)
		if err != nil {
			return "", false, err
		}
		return category, false, nil
	}

	category, err := showCategoryPicker()
	if err != nil {
		return "", false, err
	}
	if category == nil {
		return "", true, nil
	}

	return *category, false, nil
}

func validateUnarchiveTarget(reg *registry.Registry, resolvedName string, targetCategory registry.Category) (string, string, error) {
	sourcePath, err := fileutil.ExpandProjectPath(filepath.Join("archive", resolvedName), reg.Settings.BaseDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to expand source path: %w", err)
	}

	targetPath, err := fileutil.ExpandProjectPath(filepath.Join(targetCategory.Path(), resolvedName), reg.Settings.BaseDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to expand target path: %w", err)
	}

	if _, err := os.Stat(sourcePath); errors.Is(err, fs.ErrNotExist) {
		return "", "", fmt.Errorf("project directory not found\nExpected location: %s", sourcePath)
	}

	if _, err := os.Stat(targetPath); err == nil {
		return "", "", cmdError("unarchive", fmt.Sprintf("target directory already exists: %s", targetPath), "Remove it first or choose a different category")
	}

	targetParent := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetParent, 0o755); err != nil {
		return "", "", fmt.Errorf("failed to create target directory: %w", err)
	}

	return sourcePath, targetPath, nil
}

func thoughtsArchiveExists(settings map[string]string, projectName string) bool {
	thoughtsSetting, ok := settings["thoughts_dir"]
	if !ok {
		return false
	}

	archiveThoughtsBase := filepath.Join(thoughtsSetting, "archive", projectName)
	_, err := os.Stat(archiveThoughtsBase)
	return err == nil
}

func firePreUnarchiveEvent(resolvedName, sourcePath, targetPath string, settings map[string]string) error {
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

	return nil
}

func performUnarchive(projectName, sourcePath, targetPath string, settings map[string]string) ([]string, error) {
	if err := fileutil.MoveDirectory(sourcePath, targetPath); err != nil {
		return nil, fmt.Errorf("failed to move directory: %w", err)
	}

	if thoughtsSetting, ok := settings["thoughts_dir"]; ok {
		warning, err := EnsureProjectThoughtsSymlink(thoughtsSetting, projectName, targetPath, false)
		if err != nil {
			return []string{fmt.Sprintf("failed to update project thoughts symlink: %v", err)}, nil
		}
		if warning != "" {
			return []string{warning}, nil
		}
	}

	return nil, nil
}

func updateRegistryForUnarchive(reg *registry.Registry, resolvedName string, project registry.Project, targetCategory registry.Category, s styles) error {
	project.Path = filepath.Join(targetCategory.Path(), resolvedName)
	project.Category = targetCategory
	project.Status.State = registry.StateActive
	reg.Projects[resolvedName] = project

	if err := saveRegistry(reg); err != nil {
		fmt.Printf("%s Registry save failed after directory move; filesystem and registry may be out of sync: %v\n",
			s.StrongWarn.Render("Warning:"), err)
		return err
	}

	return nil
}

func firePostUnarchiveEvent(resolvedName, targetPath string, settings map[string]string) []string {
	postUnarchiveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectUnarchived,
		ProjectName: resolvedName,
		ProjectPath: targetPath,
		DestPath:    targetPath,
		Settings:    settings,
	})

	return postUnarchiveResult.Warnings
}

func printUnarchiveSuccess(
	s styles,
	resolvedName string,
	targetCategory registry.Category,
	sourcePath string,
	targetPath string,
	thoughtsDir string,
	thoughtsRestored bool,
	thoughtsWarnings []string,
) {
	labelStyle := s.Label
	successStyle := s.Success
	pathStyle := s.Path

	fmt.Printf("%s '%s' to category '%s'\n",
		successStyle.Render("Unarchived"),
		resolvedName,
		targetCategory)
	fmt.Printf("  %s %s\n", labelStyle.Render("From:"), pathStyle.Render(fileutil.FormatPathWithTilde(sourcePath)))
	fmt.Printf("  %s %s\n", labelStyle.Render("To:"), pathStyle.Render(fileutil.FormatPathWithTilde(targetPath)))

	if thoughtsRestored {
		fmt.Printf("  %s restored from %s/archive/%s/\n", labelStyle.Render("Thoughts:"), thoughtsDir, resolvedName)
	}
	fmt.Println()

	if len(thoughtsWarnings) > 0 {
		for _, warning := range thoughtsWarnings {
			fmt.Printf("%s %s\n", s.StrongWarn.Render("Warning:"), warning)
		}
		fmt.Println()
	}
}

// showCategoryPicker shows the interactive category picker
func showCategoryPicker() (*registry.Category, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, cmdError("unarchive", "interactive picker requires a terminal", "Provide a category: overlord unarchive <name> --category=<cat>")
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

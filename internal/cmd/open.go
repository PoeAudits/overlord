package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// projectItem represents a project for the picker
type projectItem struct {
	name        string
	category    string
	description string
	path        string
	status      registry.State
	aliases     []string
}

// pickerModel is the Bubbletea model for the interactive picker
type pickerModel struct {
	textInput  textinput.Model
	projects   []projectItem
	filtered   []projectItem
	cursor     int
	selected   *projectItem
	cancelled  bool
	width      int
	height     int
	maxNameLen int
	maxCatLen  int
	lastQuery  string
}

func newPickerModel(projects []projectItem) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = maxProjectNameLen
	ti.Width = pickerInputWidth

	// Calculate max lengths for alignment
	maxNameLen := 0
	maxCatLen := 0
	for _, p := range projects {
		if len(p.name) > maxNameLen {
			maxNameLen = len(p.name)
		}
		if len(p.category) > maxCatLen {
			maxCatLen = len(p.category)
		}
	}

	return pickerModel{
		textInput:  ti,
		projects:   projects,
		filtered:   projects,
		cursor:     0,
		maxNameLen: maxNameLen,
		maxCatLen:  maxCatLen,
	}
}

func (m pickerModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit

		case tea.KeyEnter:
			if len(m.filtered) > 0 && m.cursor < len(m.filtered) {
				m.selected = &m.filtered[m.cursor]
			}
			return m, tea.Quit

		case tea.KeyUp:
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case tea.KeyDown:
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	// Update text input
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter projects based on input (only if query changed)
	if m.textInput.Value() != m.lastQuery {
		m.filterProjects()
	}

	return m, cmd
}

func (m *pickerModel) filterProjects() {
	query := m.textInput.Value()
	m.lastQuery = query

	if query == "" {
		m.filtered = m.projects
		m.cursor = 0
		return
	}

	// Build list of searchable strings
	targets := make([]string, len(m.projects))
	for i, p := range m.projects {
		if len(p.aliases) > 0 {
			targets[i] = p.name + " " + strings.Join(p.aliases, " ")
		} else {
			targets[i] = p.name
		}
	}

	// Perform fuzzy match
	matches := fuzzy.Find(query, targets)

	// Build filtered list
	m.filtered = make([]projectItem, 0, len(matches))
	for _, match := range matches {
		m.filtered = append(m.filtered, m.projects[match.Index])
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = 0
	}
}

func (m pickerModel) View() string {
	var b strings.Builder

	// Styles
	s := NewStyles()
	promptStyle := s.Prompt
	selectedStyle := s.Selected
	nameStyle := s.Name
	categoryStyle := s.Category
	descStyle := s.Desc
	cursorStyle := s.Cursor

	// Header
	b.WriteString(promptStyle.Render("Find project: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	// Calculate visible items (limit to terminal height minus header/footer)
	maxVisible := maxPickerVisibleItems
	if m.height > 0 {
		maxVisible = m.height - pickerHeaderFooterLines
		if maxVisible < minPickerVisibleItems {
			maxVisible = minPickerVisibleItems
		}
	}

	// Calculate scroll offset
	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	// Show filtered projects
	if len(m.filtered) == 0 {
		b.WriteString(descStyle.Render("  No matching projects"))
		b.WriteString("\n")
	} else {
		for i := startIdx; i < endIdx; i++ {
			p := m.filtered[i]

			// Cursor indicator
			if i == m.cursor {
				b.WriteString(cursorStyle.Render("> "))
			} else {
				b.WriteString("  ")
			}

			// Format: name (padded) | category (padded) | description (truncated)
			name := p.name
			aliasStr := ""
			if len(p.aliases) > 0 {
				aliasStr = " (" + strings.Join(p.aliases, ", ") + ")"
			}
			if i == m.cursor {
				name = selectedStyle.Render(padRight(name, m.maxNameLen)) + descStyle.Render(aliasStr)
			} else {
				name = nameStyle.Render(padRight(name, m.maxNameLen)) + descStyle.Render(aliasStr)
			}

			category := categoryStyle.Render(padRight(p.category, m.maxCatLen))

			// Truncate description
			desc := p.description
			if len(desc) > maxDescDisplayLen {
				desc = desc[:maxDescDisplayLen-3] + "..."
			}
			desc = descStyle.Render(desc)

			b.WriteString(fmt.Sprintf("%s  %s  %s\n", name, category, desc))
		}
	}

	// Footer with help
	b.WriteString("\n")
	b.WriteString(descStyle.Render("↑/↓ navigate • enter select • esc cancel"))

	return b.String()
}

// padRight pads a string to the specified width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// openCmd represents the open command
var openCmd = &cobra.Command{
	Use:   "open [name]",
	Short: "Open a project workspace in tmux",
	Long: `Open a project workspace in tmux. If the session exists, attach to it.
If not, create a new session and optionally source .tmux.local.

If no name is provided, or multiple projects match, an interactive
fuzzy finder is shown.

Examples:
  overlord open dispatch       # Open project by name
  overlord open                # Interactive picker
  overlord dispatch            # Shorthand (same as 'overlord open dispatch')`,
	Args: cobra.MaximumNArgs(1),
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	var projectName string
	var project registry.Project

	if len(args) == 0 {
		// No name provided - show picker with all active projects
		projectName, project, err = showPicker(reg, "")
		if err != nil {
			return err
		}
		if projectName == "" {
			// User cancelled
			return nil
		}
	} else {
		// Name provided - try to resolve
		query := args[0]
		// Use Resolve (multi-result) instead of ResolveOne so ambiguous matches can
		// flow into the interactive picker instead of returning an ambiguity error.
		results := reg.Resolve(query)

		if len(results) == 0 {
			// Check if query looks like a command that doesn't exist
			commonCommands := map[string]string{
				"create":   "Use 'overlord new <name>' to create a project",
				"delete":   "Use 'overlord rm <name>' to remove from registry",
				"remove":   "Use 'overlord rm <name>' to remove from registry",
				"status":   "Use 'overlord list' to see projects or 'overlord info <name>' for details",
				"show":     "Use 'overlord info <name>' to see project details",
				"get":      "Use 'overlord info <name>' to see project details",
				"start":    "Use 'overlord open <name>' to open a project workspace",
				"run":      "Use 'overlord open <name>' to open a project workspace",
				"enter":    "Use 'overlord open <name>' to open a project workspace",
				"ls":       "Use 'overlord list' to see projects",
				"projects": "Use 'overlord list' to see projects",
				"register": "Use 'overlord add <path>' to register an existing project",
				"restore":  "Use 'overlord unarchive <name>' to restore a project",
			}
			if suggestion, ok := commonCommands[strings.ToLower(query)]; ok {
				return fmt.Errorf("'%s' is not a command.\n%s\n\nRun 'overlord help' for available commands", query, suggestion)
			}
			return fmt.Errorf("no project found matching '%s'\n\nRun 'overlord list' to see all projects\nRun 'overlord help' for available commands", query)
		}

		if len(results) == 1 {
			// Single match - use it
			projectName = results[0].Name
			project = results[0].Project
		} else {
			// Multiple matches - show picker with filtered results
			projectName, project, err = showPickerWithResults(results)
			if err != nil {
				return err
			}
			if projectName == "" {
				// User cancelled
				return nil
			}
		}
	}

	// Check if project is archived
	if project.Status.State == registry.StateArchived {
		return fmt.Errorf("'%s' is archived\nRestore it first: overlord unarchive %s --category=%s",
			projectName, projectName, project.Category)
	}

	// Record last opened timestamp (save only, no commit+push)
	project.LastOpened = time.Now().UTC().Format(time.RFC3339)
	reg.Projects[projectName] = project
	if err := saveRegistry(reg); err != nil {
		// Warn but don't fail - opening the project is more important
		s := NewStyles()
		fmt.Printf("%s failed to record open time: %v\n", s.Warn.Render("Warning:"), err)
	}

	// Open the project
	return openProject(projectName, project, reg)
}

// showPicker shows the interactive picker with all active projects
func showPicker(reg *registry.Registry, initialQuery string) (string, registry.Project, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", registry.Project{}, fmt.Errorf("interactive picker requires a terminal\nProvide a project name: overlord open <name>")
	}

	// Get active projects
	activeProjects := reg.ActiveProjects()
	if len(activeProjects) == 0 {
		return "", registry.Project{}, fmt.Errorf("no active projects found\nRun 'overlord list --all' to see all projects")
	}

	// Convert to picker items
	items := make([]projectItem, 0, len(activeProjects))
	for name, proj := range activeProjects {
		items = append(items, projectItem{
			name:        name,
			category:    string(proj.Category),
			description: proj.Description,
			path:        proj.Path,
			status:      proj.Status.State,
			aliases:     proj.Aliases,
		})
	}

	// Sort by name
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})

	// Run picker
	selected, err := runPicker(items, initialQuery)
	if err != nil {
		return "", registry.Project{}, err
	}
	if selected == nil {
		return "", registry.Project{}, nil
	}

	// Get the full project from registry
	proj, ok := reg.ProjectByName(selected.name)
	if !ok {
		return "", registry.Project{}, fmt.Errorf("selected project '%s' not found in registry", selected.name)
	}
	return selected.name, proj, nil
}

// showPickerWithResults shows the picker with pre-filtered results
func showPickerWithResults(results []registry.ResolveResult) (string, registry.Project, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Not in terminal - show ambiguous error
		names := make([]string, len(results))
		for i, result := range results {
			names[i] = result.Name
		}
		return "", registry.Project{}, fmt.Errorf("multiple projects match: %s\nProvide a more specific name", strings.Join(names, ", "))
	}

	// Convert to picker items
	items := make([]projectItem, 0, len(results))
	for _, result := range results {
		items = append(items, projectItem{
			name:        result.Name,
			category:    string(result.Project.Category),
			description: result.Project.Description,
			path:        result.Project.Path,
			status:      result.Project.Status.State,
			aliases:     result.Project.Aliases,
		})
	}

	// Run picker (already sorted by score from Resolve)
	selected, err := runPicker(items, "")
	if err != nil {
		return "", registry.Project{}, err
	}
	if selected == nil {
		return "", registry.Project{}, nil
	}

	// Find the project in results
	for _, result := range results {
		if result.Name == selected.name {
			return result.Name, result.Project, nil
		}
	}

	return "", registry.Project{}, fmt.Errorf("selected project not found")
}

// runPicker runs the Bubbletea picker and returns the selected item
func runPicker(items []projectItem, initialQuery string) (*projectItem, error) {
	m := newPickerModel(items)
	if initialQuery != "" {
		m.textInput.SetValue(initialQuery)
		m.filterProjects()
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("picker error: %w", err)
	}

	result := finalModel.(pickerModel)
	if result.cancelled {
		return nil, nil
	}

	return result.selected, nil
}

// openProject opens a project in tmux
func openProject(name string, project registry.Project, reg *registry.Registry) error {
	// Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}
	settings := reg.Settings

	// Build full project path (handles both relative and absolute paths)
	projectPath := fileutil.ResolveProjectPath(project.Path, baseDir)

	// Check if directory exists
	if _, err := os.Stat(projectPath); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("project directory not found\nExpected location: %s", projectPath)
	}

	openResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectOpening,
		ProjectName: name,
		ProjectPath: projectPath,
		Settings: map[string]string{
			"thoughts_dir": settings.ThoughtsDir,
		},
	})
	if openResult.Err != nil {
		return fmt.Errorf("pre-open hook failed: %w", openResult.Err)
	}

	// Define styles for output
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Success
	warnStyle := s.Warn
	pathStyle := s.Path

	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH\nInstall tmux to use workspace features")
	}

	// Create tmux session helper
	session := tmux.NewSession(name, projectPath)

	// Check if session already exists
	exists, err := session.Exists()
	if err != nil {
		return err
	}
	if exists {
		fmt.Printf("%s %s\n", labelStyle.Render("Attaching to session:"), name)
	} else {
		fmt.Printf("%s %s\n", labelStyle.Render("Creating session:"), name)
		fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(projectPath))

		// Create new session
		if err := session.Create(); err != nil {
			return err
		}

		// Source .tmux.local if it exists
		if err := session.SourceLocal(); err != nil {
			fmt.Printf("%s %v\n", warnStyle.Render("Warning:"), err)
		}

		fmt.Printf("%s\n", successStyle.Render("Session created!"))
	}

	// Attach to session (or switch if already in tmux)
	if err := session.AttachOrSwitch(); err != nil {
		fmt.Printf("%s %v\n", warnStyle.Render("Warning:"), err)
		fmt.Printf("Attach manually: tmux attach -t %s\n", name)
	}

	return nil
}

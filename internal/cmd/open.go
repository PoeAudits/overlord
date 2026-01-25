package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/PoeAudits/overlord/internal/registry"
)

// projectItem represents a project for the picker
type projectItem struct {
	name        string
	category    string
	description string
	path        string
	status      registry.State
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
}

func newPickerModel(projects []projectItem) pickerModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Focus()
	ti.CharLimit = 64
	ti.Width = 40

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

		// Handle j/k for vim-style navigation (only when not typing)
		if msg.Type == tea.KeyRunes {
			switch string(msg.Runes) {
			case "j":
				// Only navigate if input is empty or ends with j
				// Actually, let's just use arrow keys for navigation
				// and let j/k be typed as filter characters
			case "k":
				// Same as above
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	// Update text input
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Filter projects based on input
	m.filterProjects()

	return m, cmd
}

func (m *pickerModel) filterProjects() {
	query := m.textInput.Value()

	if query == "" {
		m.filtered = m.projects
		m.cursor = 0
		return
	}

	// Build list of searchable strings
	targets := make([]string, len(m.projects))
	for i, p := range m.projects {
		targets[i] = p.name
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
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	gray := lipgloss.Color("245")
	purple := lipgloss.Color("99")
	white := lipgloss.Color("255")

	promptStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(white)
	categoryStyle := lipgloss.NewStyle().Foreground(purple)
	descStyle := lipgloss.NewStyle().Foreground(gray)
	cursorStyle := lipgloss.NewStyle().Foreground(green).Bold(true)

	// Header
	b.WriteString(promptStyle.Render("Find project: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")

	// Calculate visible items (limit to terminal height - 5 for header/footer)
	maxVisible := 15
	if m.height > 0 {
		maxVisible = m.height - 5
		if maxVisible < 5 {
			maxVisible = 5
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
			if i == m.cursor {
				name = selectedStyle.Render(padRight(name, m.maxNameLen))
			} else {
				name = nameStyle.Render(padRight(name, m.maxNameLen))
			}

			category := categoryStyle.Render(padRight(p.category, m.maxCatLen))

			// Truncate description
			desc := p.description
			maxDescLen := 40
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen-3] + "..."
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
	RunE: RunOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

// RunOpen executes the open command (exported for root command handler)
func RunOpen(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
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
		results := reg.Resolve(query)

		if len(results) == 0 {
			return fmt.Errorf("Error: no project found matching '%s'\nRun 'overlord list' to see all projects", query)
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
		return fmt.Errorf("Error: '%s' is archived\nRestore it first: overlord unarchive %s --category=%s",
			projectName, projectName, project.Category)
	}

	// Open the project
	return openProject(projectName, project, reg.Settings)
}

// showPicker shows the interactive picker with all active projects
func showPicker(reg *registry.Registry, initialQuery string) (string, registry.Project, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", registry.Project{}, fmt.Errorf("interactive picker requires a terminal\nProvide a project name: overlord open <name>")
	}

	// Get active projects
	activeProjects := reg.GetActiveProjects()
	if len(activeProjects) == 0 {
		return "", registry.Project{}, fmt.Errorf("Error: no active projects found\nRun 'overlord list --all' to see all projects")
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
	proj, _ := reg.GetProject(selected.name)
	return selected.name, proj, nil
}

// showPickerWithResults shows the picker with pre-filtered results
func showPickerWithResults(results []registry.ResolveResult) (string, registry.Project, error) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		// Not in terminal - show ambiguous error
		names := make([]string, len(results))
		for i, r := range results {
			names[i] = r.Name
		}
		return "", registry.Project{}, fmt.Errorf("Error: multiple projects match: %s\nProvide a more specific name", strings.Join(names, ", "))
	}

	// Convert to picker items
	items := make([]projectItem, 0, len(results))
	for _, r := range results {
		items = append(items, projectItem{
			name:        r.Name,
			category:    string(r.Project.Category),
			description: r.Project.Description,
			path:        r.Project.Path,
			status:      r.Project.Status.State,
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
	for _, r := range results {
		if r.Name == selected.name {
			return r.Name, r.Project, nil
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
func openProject(name string, project registry.Project, settings registry.Settings) error {
	// Expand base directory
	baseDir, err := expandPath(settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base directory: %w", err)
	}

	// Build full project path
	projectPath := filepath.Join(baseDir, project.Path)

	// Check if directory exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("Error: project directory not found\nExpected location: %s", projectPath)
	}

	// Define styles for output
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	yellow := lipgloss.Color("220")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(yellow)
	pathStyle := lipgloss.NewStyle().Foreground(gray)

	// Check if tmux is available
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("Error: tmux not found in PATH\nInstall tmux to use workspace features")
	}

	// Check if session already exists
	checkSession := exec.Command("tmux", "has-session", "-t", name)
	sessionExists := checkSession.Run() == nil

	if sessionExists {
		fmt.Printf("%s %s\n", labelStyle.Render("Attaching to session:"), name)
	} else {
		fmt.Printf("%s %s\n", labelStyle.Render("Creating session:"), name)
		fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(projectPath))

		// Create new session (detached)
		createSession := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", projectPath)
		if output, err := createSession.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to create tmux session: %s", strings.TrimSpace(string(output)))
		}

		// Source .tmux.local if it exists
		tmuxLocalPath := filepath.Join(projectPath, ".tmux.local")
		if _, err := os.Stat(tmuxLocalPath); err == nil {
			// Run .tmux.local as a bash script with environment variables
			tmuxSource := exec.Command("bash", tmuxLocalPath)
			tmuxSource.Env = append(os.Environ(),
				fmt.Sprintf("TMUX_SESSION=%s", name),
				fmt.Sprintf("TMUX_PROJECT_DIR=%s", projectPath),
			)
			if output, err := tmuxSource.CombinedOutput(); err != nil {
				fmt.Printf("%s failed to source .tmux.local: %s\n", warnStyle.Render("Warning:"), strings.TrimSpace(string(output)))
			}
		}

		fmt.Printf("%s\n", successStyle.Render("Session created!"))
	}

	// Attach to session (or switch if already in tmux)
	if os.Getenv("TMUX") != "" {
		// Already in tmux, switch client
		switchCmd := exec.Command("tmux", "switch-client", "-t", name)
		switchCmd.Stdin = os.Stdin
		switchCmd.Stdout = os.Stdout
		switchCmd.Stderr = os.Stderr
		if err := switchCmd.Run(); err != nil {
			fmt.Printf("%s failed to switch to session: %v\n", warnStyle.Render("Warning:"), err)
			fmt.Printf("Attach manually: tmux attach -t %s\n", name)
		}
	} else {
		// Not in tmux, attach
		attachCmd := exec.Command("tmux", "attach", "-t", name)
		attachCmd.Stdin = os.Stdin
		attachCmd.Stdout = os.Stdout
		attachCmd.Stderr = os.Stderr
		if err := attachCmd.Run(); err != nil {
			fmt.Printf("%s failed to attach to session: %v\n", warnStyle.Render("Warning:"), err)
			fmt.Printf("Attach manually: tmux attach -t %s\n", name)
		}
	}

	return nil
}

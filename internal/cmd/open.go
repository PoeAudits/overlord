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

var (
	openLoadRegistry      = loadRegistry
	openShowPicker        = showPicker
	openShowPickerResults = showPickerWithResults
	openEnsureProject     = ensureProjectSession
	openOpenProject       = openProject
	openSaveRegistry      = saveRegistry
	openNow               = time.Now
	openIsTerminal        = term.IsTerminal
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
	s := NewStyles()
	var b strings.Builder

	m.renderPickerHeader(&b, s)
	m.renderPickerItems(&b, s)
	m.renderPickerFooter(&b, s)

	return b.String()
}

func (m pickerModel) renderPickerHeader(b *strings.Builder, s styles) {
	b.WriteString(s.Prompt.Render("Find project: "))
	b.WriteString(m.textInput.View())
	b.WriteString("\n\n")
}

func (m pickerModel) renderPickerItems(b *strings.Builder, s styles) {
	if len(m.filtered) == 0 {
		b.WriteString(s.Desc.Render("  No matching projects"))
		b.WriteString("\n")
		return
	}

	startIdx, endIdx := m.visibleRange()
	for i := startIdx; i < endIdx; i++ {
		b.WriteString(m.renderPickerItem(i, s))
	}
}

func (m pickerModel) visibleRange() (int, int) {
	maxVisible := maxPickerVisibleItems
	if m.height > 0 {
		maxVisible = m.height - pickerHeaderFooterLines
		if maxVisible < minPickerVisibleItems {
			maxVisible = minPickerVisibleItems
		}
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}

	endIdx := startIdx + maxVisible
	if endIdx > len(m.filtered) {
		endIdx = len(m.filtered)
	}

	return startIdx, endIdx
}

func (m pickerModel) renderPickerItem(idx int, s styles) string {
	p := m.filtered[idx]
	cursor := "  "
	if idx == m.cursor {
		cursor = s.Cursor.Render("> ")
	}

	name := s.Name.Render(padRight(p.name, m.maxNameLen))
	if idx == m.cursor {
		name = s.Selected.Render(padRight(p.name, m.maxNameLen))
	}

	aliasStr := ""
	if len(p.aliases) > 0 {
		aliasStr = " (" + strings.Join(p.aliases, ", ") + ")"
	}

	category := s.Category.Render(padRight(p.category, m.maxCatLen))
	desc := p.description
	if len(desc) > maxDescDisplayLen {
		desc = desc[:maxDescDisplayLen-3] + "..."
	}

	return fmt.Sprintf("%s%s%s  %s  %s\n", cursor, name, s.Desc.Render(aliasStr), category, s.Desc.Render(desc))
}

func (m pickerModel) renderPickerFooter(b *strings.Builder, s styles) {
	b.WriteString("\n")
	b.WriteString(s.Desc.Render("↑/↓ navigate • enter select • esc cancel"))
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
	Use:   "open [name...]",
	Short: "Open a project workspace in tmux",
	Long: `Open one or more project workspaces in tmux. If the session exists, attach to it.
If not, create a new session and optionally source .tmux.local.

If no name is provided, or multiple projects match, an interactive
fuzzy finder is shown.

Examples:
	  overlord open dispatch       # Open project by name
	  overlord open agents prompt-lab opencode
	  overlord open                # Interactive picker
	  overlord dispatch            # Shorthand (same as 'overlord open dispatch')`,
	Args: cobra.ArbitraryArgs,
	RunE: runOpen,
}

func init() {
	rootCmd.AddCommand(openCmd)
}

func runOpen(cmd *cobra.Command, args []string) error {
	reg, err := openLoadRegistry()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		projectName, project, err := openShowPicker(reg, "")
		if err != nil || projectName == "" {
			return err
		}
		return openResolvedProject(projectName, project, reg)
	}

	if len(args) > 1 {
		return openMultipleProjects(cmd, reg, args)
	}

	projectName, project, err := resolveOpenMatch(cmd, reg, args[0])
	if err != nil || projectName == "" {
		return err
	}

	return openResolvedProject(projectName, project, reg)
}

type openTarget struct {
	name    string
	project registry.Project
}

func openMultipleProjects(cmd *cobra.Command, reg *registry.Registry, queries []string) error {
	resolvedProjects := make([]openTarget, 0, len(queries))
	for _, query := range queries {
		projectName, project, err := resolveOpenMatch(cmd, reg, query)
		if err != nil {
			return err
		}
		if projectName == "" {
			return nil
		}

		resolvedProjects = append(resolvedProjects, openTarget{
			name:    projectName,
			project: project,
		})
	}

	archivedProjectNames := archivedProjectNames(resolvedProjects)
	if len(archivedProjectNames) > 0 {
		return cmdError("open", fmt.Sprintf("archived projects cannot be opened: %s", strings.Join(archivedProjectNames, ", ")), "Restore them first with 'overlord unarchive <name>'")
	}

	lastProjectIdx := len(resolvedProjects) - 1
	for idx, resolved := range resolvedProjects {
		recordLastOpened(resolved.name, resolved.project, reg)

		if idx == lastProjectIdx {
			if err := openOpenProject(resolved.name, resolved.project, reg); err != nil {
				return err
			}
			continue
		}

		if err := openEnsureProject(resolved.name, resolved.project, reg); err != nil {
			return err
		}
	}

	return nil
}

func archivedProjectNames(resolvedProjects []openTarget) []string {
	archivedProjectNames := make([]string, 0)
	for _, resolved := range resolvedProjects {
		if resolved.project.Status.State != registry.StateArchived {
			continue
		}
		archivedProjectNames = append(archivedProjectNames, resolved.name)
	}

	return archivedProjectNames
}

func resolveOpenMatch(cmd *cobra.Command, reg *registry.Registry, query string) (string, registry.Project, error) {
	return resolveProjectMatch(
		reg,
		query,
		func(query string) error { return openNoMatchError(cmd, query) },
		openShowPickerResults,
	)
}

func resolveProjectMatch(reg *registry.Registry, query string, noMatch func(string) error, ambiguous func([]registry.ResolveResult) (string, registry.Project, error)) (string, registry.Project, error) {
	results := reg.Resolve(query)
	switch len(results) {
	case 0:
		return "", registry.Project{}, noMatch(query)
	case 1:
		return results[0].Name, results[0].Project, nil
	default:
		return ambiguous(results)
	}
}

func openNoMatchError(cmd *cobra.Command, query string) error {
	commonCommands := map[string]string{
		"create":   "Use 'overlord new <name>' to create a project",
		"delete":   "Use 'overlord rm <name>' to remove from registry",
		"remove":   "Use 'overlord rm <name>' to remove from registry",
		"status":   "Use 'overlord list' to see projects or 'overlord info <name>' for details",
		"show":     "Use 'overlord info <name>' to see project details",
		"get":      "Use 'overlord info <name>' to see project details",
		"run":      "Use 'overlord open <name>' to open a project workspace",
		"enter":    "Use 'overlord open <name>' to open a project workspace",
		"ls":       "Use 'overlord list' to see projects",
		"projects": "Use 'overlord list' to see projects",
		"register": "Use 'overlord add <path>' to register an existing project",
		"restore":  "Use 'overlord unarchive <name>' to restore a project",
	}

	if suggestion, ok := commonCommands[strings.ToLower(query)]; ok {
		return cmdError("open", fmt.Sprintf("'%s' is not a project or command", query), suggestion)
	}

	return cmdError("open", fmt.Sprintf("no project found matching '%s'", query), "Run 'overlord list' to see all projects")
}

func openResolvedProject(projectName string, project registry.Project, reg *registry.Registry) error {
	// Check if project is archived
	if project.Status.State == registry.StateArchived {
		return cmdError("open", fmt.Sprintf("'%s' is archived", projectName), fmt.Sprintf("Restore it first: overlord unarchive %s --category=%s", projectName, project.Category))
	}

	recordLastOpened(projectName, project, reg)

	return openOpenProject(projectName, project, reg)
}

func recordLastOpened(projectName string, project registry.Project, reg *registry.Registry) {
	project.LastOpened = openNow().UTC().Format(time.RFC3339)
	reg.Projects[projectName] = project
	if err := openSaveRegistry(reg); err != nil {
		s := NewStyles()
		fmt.Printf("%s failed to record open time: %v\n", s.Warn.Render("Warning:"), err)
	}
}

// showPicker shows the interactive picker with all active projects
func showPicker(reg *registry.Registry, initialQuery string) (string, registry.Project, error) {
	return showPickerForCommand("open", reg, initialQuery)
}

func showPickerForCommand(command string, reg *registry.Registry, initialQuery string) (string, registry.Project, error) {
	// Check if we're in a terminal
	if !openIsTerminal(int(os.Stdin.Fd())) {
		return "", registry.Project{}, cmdError(command, "interactive picker requires a terminal", fmt.Sprintf("Provide a project name: overlord %s <name>", command))
	}

	// Get active projects
	activeProjects := reg.ActiveProjects()
	if len(activeProjects) == 0 {
		return "", registry.Project{}, cmdError(command, "no active projects found", "Run 'overlord list --all' to see all projects")
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
	return showPickerWithResultsForCommand("open", results)
}

func showPickerWithResultsForCommand(command string, results []registry.ResolveResult) (string, registry.Project, error) {
	// Check if we're in a terminal
	if !openIsTerminal(int(os.Stdin.Fd())) {
		// Not in terminal - show ambiguous error
		names := make([]string, len(results))
		for i, result := range results {
			names[i] = result.Name
		}
		return "", registry.Project{}, cmdError(command, fmt.Sprintf("multiple projects match: %s", strings.Join(names, ", ")), "Provide a more specific name")
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
	if err := ensureProjectSession(name, project, reg); err != nil {
		return err
	}

	// Attach to session (or switch if already in tmux)
	session := tmux.NewSession(name, "")
	if err := session.AttachOrSwitch(); err != nil {
		s := NewStyles()
		fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
		fmt.Printf("Attach manually: tmux attach -t %s\n", name)
	}

	return nil
}

func ensureProjectSession(name string, project registry.Project, reg *registry.Registry) error {
	// Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Build full project path (handles both relative and absolute paths)
	projectPath := fileutil.ResolveProjectPath(project.Path, baseDir)

	// Check if directory exists
	if _, err := os.Stat(projectPath); errors.Is(err, fs.ErrNotExist) {
		return cmdError("open", fmt.Sprintf("project directory not found at %s", projectPath), "Check the path in 'overlord info <name>' or re-register with 'overlord add'")
	}

	openResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectOpening,
		ProjectName: name,
		ProjectPath: projectPath,
		Settings:    map[string]string{},
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
		return cmdError("open", "tmux not found in PATH", "Install tmux to use workspace features")
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

	return nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

// listFlags holds the flags for the list command
type listFlags struct {
	jsonOutput bool
	category   string
	tags       []string
	archived   bool
	all        bool
	recent     string
}

var listOpts listFlags

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects in the registry",
	Long: `List projects in the registry with optional filtering.

By default, only active projects are shown. Use --archived to show only
archived projects, or --all to show everything.

Use --recent to show only recently opened projects, sorted by most recent first.
The default window is 7 days. Specify a custom window with --recent=30d, --recent=2w,
or --recent=24h.

Examples:
  overlord list                    # List active projects
  overlord list --category=web     # Filter by category
  overlord list --tag=api          # Filter by tag
  overlord list --archived         # Show only archived projects
  overlord list --all              # Show all projects (active + archived)
  overlord list --recent           # Recently opened (last 7 days)
  overlord list --recent=30d       # Recently opened (last 30 days)
  overlord list --recent=2w        # Recently opened (last 2 weeks)
  overlord list --json             # Output as JSON`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Flags for list command
	listCmd.Flags().BoolVar(&listOpts.jsonOutput, "json", false, "Output as JSON")
	listCmd.Flags().StringVar(&listOpts.category, "category", "", "Filter by category")
	listCmd.Flags().StringSliceVar(&listOpts.tags, "tag", nil, "Filter by tag (can be repeated)")
	listCmd.Flags().BoolVar(&listOpts.archived, "archived", false, "Include archived projects")
	listCmd.Flags().BoolVar(&listOpts.all, "all", false, "Include all projects (active and archived)")
	listCmd.Flags().StringVar(&listOpts.recent, "recent", "", "Show recently opened projects (default: 7d, e.g. 30d, 2w, 24h)")
	listCmd.Flags().Lookup("recent").NoOptDefVal = "7d"
}

// projectOutput represents a project for JSON output
type projectOutput struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Category    string   `json:"category"`
	Lang        string   `json:"lang"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Tags        []string `json:"tags,omitempty"`
	LastOpened  string   `json:"last_opened,omitempty"`
}

func runList(cmd *cobra.Command, args []string) error {
	// Validate --recent flag value early
	if listOpts.recent != "" {
		if _, err := recentCutoff(listOpts.recent); err != nil {
			return fmt.Errorf("invalid --recent value: %w", err)
		}
	}

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Get filtered projects
	projects := filterProjects(reg)

	// No output if no projects match
	if len(projects) == 0 {
		return nil
	}

	// Output based on format
	if listOpts.jsonOutput {
		return outputListJSON(projects)
	}
	return outputListTable(projects)
}

// filterProjects applies all filters and returns matching projects as a sorted slice.
func filterProjects(reg *registry.Registry) []projectOutput {
	var candidates map[string]registry.Project

	switch {
	case listOpts.all:
		candidates = reg.Projects
	case listOpts.archived:
		candidates = reg.ArchivedProjects()
	default:
		candidates = reg.ActiveProjects()
	}

	candidates = filterProjectsByCategory(candidates, listOpts.category)
	candidates = filterProjectsByTags(candidates, listOpts.tags)
	candidates = filterProjectsByRecent(candidates, listOpts.recent)

	result := filterProjectsToOutput(candidates, reg)
	sortProjectOutput(result, listOpts.recent)

	return result
}

// filterProjectsByCategory returns only projects matching the given category.
// Returns candidates unchanged if category is empty.
func filterProjectsByCategory(candidates map[string]registry.Project, category string) map[string]registry.Project {
	if category == "" {
		return candidates
	}
	filtered := make(map[string]registry.Project)
	cat := registry.Category(category)
	for name, proj := range candidates {
		if proj.Category == cat {
			filtered[name] = proj
		}
	}
	return filtered
}

// filterProjectsByTags returns only projects that have all required tags.
// Returns candidates unchanged if tags is empty.
func filterProjectsByTags(candidates map[string]registry.Project, tags []string) map[string]registry.Project {
	if len(tags) == 0 {
		return candidates
	}
	filtered := make(map[string]registry.Project)
	for name, proj := range candidates {
		if hasAllTags(proj.Tags, tags) {
			filtered[name] = proj
		}
	}
	return filtered
}

// filterProjectsByRecent returns only projects opened after the cutoff derived from recentStr.
// Returns candidates unchanged if recentStr is empty or unparseable.
func filterProjectsByRecent(candidates map[string]registry.Project, recentStr string) map[string]registry.Project {
	if recentStr == "" {
		return candidates
	}
	cutoff, err := recentCutoff(recentStr)
	if err != nil {
		return candidates
	}
	filtered := make(map[string]registry.Project)
	for name, proj := range candidates {
		if proj.LastOpened == "" {
			continue
		}
		opened, parseErr := time.Parse(time.RFC3339, proj.LastOpened)
		if parseErr != nil {
			continue
		}
		if opened.After(cutoff) {
			filtered[name] = proj
		}
	}
	return filtered
}

// filterProjectsToOutput converts filtered project candidates to output format.
func filterProjectsToOutput(candidates map[string]registry.Project, reg *registry.Registry) []projectOutput {
	result := make([]projectOutput, 0, len(candidates))
	for name, proj := range candidates {
		status := string(proj.Status.State)

		result = append(result, projectOutput{
			Name:        name,
			Path:        proj.Path,
			Category:    string(proj.Category),
			Lang:        string(proj.Lang),
			Description: proj.Description,
			Status:      status,
			Tags:        proj.Tags,
			LastOpened:  proj.LastOpened,
		})
	}
	return result
}

// sortProjectOutput sorts projects in place: by last_opened descending when
// recent is set, otherwise by status rank, category, language, then name.
func sortProjectOutput(result []projectOutput, recent string) {
	if recent != "" {
		sort.Slice(result, func(i, j int) bool {
			return result[i].LastOpened > result[j].LastOpened
		})
		return
	}
	sort.Slice(result, func(i, j int) bool {
		ri, rj := statusRank(result[i].Status), statusRank(result[j].Status)
		if ri != rj {
			return ri < rj
		}
		if result[i].Category != result[j].Category {
			return result[i].Category < result[j].Category
		}
		if result[i].Lang != result[j].Lang {
			return result[i].Lang < result[j].Lang
		}
		return result[i].Name < result[j].Name
	})
}

// statusRank returns a sort rank for status (lower = sorted first)
func statusRank(status string) int {
	switch status {
	case "active":
		return 0
	case "archived":
		return 1
	default:
		return 2
	}
}

// hasAllTags checks if the project has all required tags
func hasAllTags(projectTags, requiredTags []string) bool {
	tagSet := make(map[string]struct{}, len(projectTags))
	for _, t := range projectTags {
		tagSet[t] = struct{}{}
	}

	for _, required := range requiredTags {
		if _, ok := tagSet[required]; !ok {
			return false
		}
	}
	return true
}

// outputListJSON outputs projects as JSON array
func outputListJSON(projects []projectOutput) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(projects)
}

// Category colors - grouped by type with distinct colors per category
var categoryColors = map[string]lipgloss.Color{
	// Core group - blue shades
	"core-agents": colorBlue,
	"core-tools":  colorLightBlue,
	"core-evals":  colorPurple,
	// Blockchain - pink/magenta
	"contracts": colorMagenta,
	// Web/Services group - green shades
	"web":      colorGreen,
	"services": colorLime,
	// Standalone categories
	"ml":      colorAmber,
	"libs":    colorCyan,
	"cli":     colorYellow,
	"sandbox": colorGray,
}

// Language colors - distinct colors per language
var languageColors = map[string]lipgloss.Color{
	"go":         colorTeal,
	"python":     colorLavender,
	"typescript": colorYellow,
	"solidity":   colorPink,
	"base":       colorGray,
}

// outputListTable outputs projects as a formatted table.
func outputListTable(projects []projectOutput) error {
	showRecent := listOpts.recent != ""

	rows := outputListTableRows(projects, showRecent)

	lastCol := 4

	styles := NewStyles()
	styleFn := listTableStyleFunc(rows, lastCol, showRecent, styles)

	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(styleFn)

	t = t.Headers(listTableHeaders(showRecent)...)
	t = t.Rows(rows...)
	fmt.Println(t)
	return nil
}

// outputListTableRows builds the string rows for the list table.
func outputListTableRows(projects []projectOutput, showRecent bool) [][]string {
	rows := make([][]string, 0, len(projects))
	for _, proj := range projects {
		desc := truncate(proj.Description, maxDescDisplayLen)

		row := []string{
			proj.Name,
			proj.Category,
			proj.Lang,
			desc,
		}

		if showRecent {
			row = append(row, relativeTime(proj.LastOpened))
		} else {
			row = append(row, proj.Status)
		}

		rows = append(rows, row)
	}
	return rows
}

// listTableStyleFunc returns the style function for the list table.
func listTableStyleFunc(rows [][]string, lastCol int, showRecent bool, styles styles) func(row, col int) lipgloss.Style {
	return func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return styles.TableHeader
		}
		if row < 0 || row >= len(rows) {
			return styles.TableCell
		}

		if col == 0 {
			return styles.TableName
		}
		if col == 3 {
			return styles.TableMuted
		}
		if col == 1 {
			return listColorStyle(styles.TableCell, rows[row][1], categoryColors, styles.TableMuted)
		}
		if col == 2 {
			return listColorStyle(styles.TableCell, rows[row][2], languageColors, styles.TableMuted)
		}
		if col != lastCol {
			return styles.TableCell
		}

		return listStatusStyle(rows[row][lastCol], showRecent, styles)
	}
}

func listColorStyle(base lipgloss.Style, key string, palette map[string]lipgloss.Color, fallback lipgloss.Style) lipgloss.Style {
	if color, ok := palette[key]; ok {
		return base.Foreground(color)
	}
	return fallback
}

func listStatusStyle(status string, showRecent bool, styles styles) lipgloss.Style {
	if showRecent {
		return styles.TableRecent
	}

	if status == "active" {
		return styles.TableStatusActive
	}
	return styles.TableStatusDefault
}

// listTableHeaders returns the header strings for the list table.
func listTableHeaders(showRecent bool) []string {
	headers := []string{"NAME", "CATEGORY", "LANG", "DESCRIPTION"}
	if showRecent {
		headers = append(headers, "OPENED")
	} else {
		headers = append(headers, "STATUS")
	}
	return headers
}

// recentCutoff parses a recent duration string and returns the cutoff time.
// Supported formats: Nd (days), Nw (weeks), Nh (hours). Plain number defaults to days.
func recentCutoff(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty duration string")
	}

	// Determine unit suffix and numeric part
	unit := s[len(s)-1]
	numStr := s

	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
		numStr = s[:len(s)-1]
	case 'w':
		multiplier = 7 * 24 * time.Hour
		numStr = s[:len(s)-1]
	case 'h':
		multiplier = time.Hour
		numStr = s[:len(s)-1]
	default:
		// No unit suffix — default to days
		multiplier = 24 * time.Hour
	}

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid duration: %s", s)
	}
	if n <= 0 {
		return time.Time{}, fmt.Errorf("duration must be positive: %s", s)
	}

	return time.Now().UTC().Add(-time.Duration(n) * multiplier), nil
}

// relativeTime returns a human-readable relative time string (e.g. "2h ago", "3d ago")
func relativeTime(rfcTime string) string {
	t, err := time.Parse(time.RFC3339, rfcTime)
	if err != nil {
		return ""
	}

	d := time.Since(t)
	switch {
	case d < time.Hour:
		m := int(d.Minutes())
		if m <= 1 {
			return "just now"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		w := int(d.Hours() / 24 / 7)
		if w == 1 {
			return "1w ago"
		}
		return fmt.Sprintf("%dw ago", w)
	}
}

// truncate truncates a string to maxLen characters, adding "..." if truncated
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Account for "..." suffix
	if maxLen <= 3 {
		return strings.Repeat(".", maxLen)
	}
	return s[:maxLen-3] + "..."
}


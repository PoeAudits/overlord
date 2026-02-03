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

	"github.com/PoeAudits/overlord/internal/machine"
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

// RunList is exported so it can be called from root command handler
func RunList(cmd *cobra.Command, args []string) error {
	return runList(cmd, args)
}

// projectOutput represents a project for JSON output
type projectOutput struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Category    string   `json:"category"`
	Lang        string   `json:"lang"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	SyncStatus  string   `json:"sync_status,omitempty"`
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

// filterProjects applies all filters and returns matching projects as a sorted slice
func filterProjects(reg *registry.Registry) []projectOutput {
	// Start with all projects or filtered by state
	var candidates map[string]registry.Project

	switch {
	case listOpts.all:
		candidates = reg.Projects
	case listOpts.archived:
		// Only show archived projects when --archived is set
		candidates = reg.GetArchivedProjects()
	default:
		// Default: only active projects
		// On working-set machines, show only synced projects
		machineConfig, err := machine.Load(machine.DefaultConfigPath)
		if err == nil && machineConfig.Role == machine.RoleWorkingSet {
			candidates = getSyncedProjects(reg)
		} else {
			candidates = reg.GetActiveProjects()
		}
	}

	// Apply category filter
	if listOpts.category != "" {
		filtered := make(map[string]registry.Project)
		cat := registry.Category(listOpts.category)
		for name, proj := range candidates {
			if proj.Category == cat {
				filtered[name] = proj
			}
		}
		candidates = filtered
	}

	// Apply tag filters (all tags must match)
	if len(listOpts.tags) > 0 {
		filtered := make(map[string]registry.Project)
		for name, proj := range candidates {
			if hasAllTags(proj.Tags, listOpts.tags) {
				filtered[name] = proj
			}
		}
		candidates = filtered
	}

	// Apply recent filter if set
	if listOpts.recent != "" {
		cutoff, err := recentCutoff(listOpts.recent)
		if err == nil {
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
			candidates = filtered
		}
	}

	// Convert to output format and sort
	result := make([]projectOutput, 0, len(candidates))
	for name, proj := range candidates {
		status := string(proj.Status.State)
		syncStatus := string(proj.Sync.Status)

		// Display "synced" for active projects with sync enabled
		if proj.Status.State == registry.StateActive && proj.Sync.Status == registry.SyncActive {
			status = "synced"
		}

		result = append(result, projectOutput{
			Name:        name,
			Path:        proj.Path,
			Category:    string(proj.Category),
			Lang:        string(proj.Lang),
			Description: proj.Description,
			Status:      status,
			SyncStatus:  syncStatus,
			Tags:        proj.Tags,
			LastOpened:  proj.LastOpened,
		})
	}

	// Sort: when --recent is set, sort by last_opened descending; otherwise default sort
	if listOpts.recent != "" {
		sort.Slice(result, func(i, j int) bool {
			// Most recently opened first (reverse chronological)
			return result[i].LastOpened > result[j].LastOpened
		})
	} else {
		// Sort by: status (synced first, then active, then archived), category, language, name
		sort.Slice(result, func(i, j int) bool {
			// 1. Status priority: synced > active > archived
			ri, rj := statusRank(result[i].Status), statusRank(result[j].Status)
			if ri != rj {
				return ri < rj
			}
			// 2. Category
			if result[i].Category != result[j].Category {
				return result[i].Category < result[j].Category
			}
			// 3. Language
			if result[i].Lang != result[j].Lang {
				return result[i].Lang < result[j].Lang
			}
			// 4. Name
			return result[i].Name < result[j].Name
		})
	}

	return result
}

// statusRank returns a sort rank for status (lower = sorted first)
func statusRank(status string) int {
	switch status {
	case "synced":
		return 0
	case "active":
		return 1
	case "archived":
		return 2
	default:
		return 3
	}
}

// getSyncedProjects returns active projects that have sync status 'active'.
// Used on working-set machines to show only locally synced projects.
func getSyncedProjects(reg *registry.Registry) map[string]registry.Project {
	result := make(map[string]registry.Project)
	for name, project := range reg.Projects {
		if project.Status.State == registry.StateActive && project.Sync.Status == registry.SyncActive {
			result[name] = project
		}
	}
	return result
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
	"core-agents": lipgloss.Color("75"), // Blue
	"core-tools":  lipgloss.Color("39"), // Light Blue
	// Blockchain - pink/magenta
	"contracts": lipgloss.Color("201"), // Magenta
	// Web/Services group - green shades
	"web":      lipgloss.Color("82"),  // Green
	"services": lipgloss.Color("156"), // Light Green
	// Standalone categories
	"ml":      lipgloss.Color("208"), // Orange
	"libs":    lipgloss.Color("87"),  // Cyan
	"cli":     lipgloss.Color("220"), // Yellow
	"sandbox": lipgloss.Color("245"), // Gray
}

// Language colors - distinct colors per language
var languageColors = map[string]lipgloss.Color{
	"go":         lipgloss.Color("37"),  // Cyan/Teal
	"python":     lipgloss.Color("141"), // Purple/Lavender
	"typescript": lipgloss.Color("220"), // Yellow
	"solidity":   lipgloss.Color("213"), // Pink
	"base":       lipgloss.Color("245"), // Gray
}

// outputListTable outputs projects as a formatted table
func outputListTable(projects []projectOutput) error {
	// Define colors
	syncedColor := lipgloss.Color("213") // Pink/Magenta for synced

	// Define styles (headerStyle has padding - list-specific)
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorCyan).
		Padding(0, 1)

	showRecent := listOpts.recent != ""

	// Build rows
	rows := make([][]string, 0, len(projects))
	for _, proj := range projects {
		// Truncate description
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

	// Determine last column index for styling
	lastCol := 4

	// Create table
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}

			style := lipgloss.NewStyle().Padding(0, 1)

			// Bounds check for row access
			if row < 0 || row >= len(rows) {
				return style
			}

			switch col {
			case 0: // Name
				return style.Foreground(lipgloss.Color("255")).Bold(true)
			case 1: // Category
				category := rows[row][1]
				if color, ok := categoryColors[category]; ok {
					return style.Foreground(color)
				}
				return style.Foreground(ColorGray)
			case 2: // Lang
				lang := rows[row][2]
				if color, ok := languageColors[lang]; ok {
					return style.Foreground(color)
				}
				return style.Foreground(ColorGray)
			case 3: // Description
				return style.Foreground(ColorGray)
			}
			if col == lastCol {
				if showRecent {
					return style.Foreground(ColorCyan)
				}
				status := rows[row][lastCol]
				switch status {
				case "synced":
					return style.Foreground(syncedColor)
				case "active":
					return style.Foreground(ColorGreen)
				default:
					return style.Foreground(ColorGray)
				}
			}
			return style
		})

	if showRecent {
		t = t.Headers("NAME", "CATEGORY", "LANG", "DESCRIPTION", "OPENED")
	} else {
		t = t.Headers("NAME", "CATEGORY", "LANG", "DESCRIPTION", "STATUS")
	}

	t = t.Rows(rows...)
	fmt.Println(t)
	return nil
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

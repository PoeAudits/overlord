package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

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
}

var listOpts listFlags

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects in the registry",
	Long: `List projects in the registry with optional filtering.

By default, only active projects are shown. Use --archived to show only
archived projects, or --all to show everything.

Examples:
  overlord list                    # List active projects
  overlord list --category=web     # Filter by category
  overlord list --tag=api          # Filter by tag
  overlord list --archived         # Show only archived projects
  overlord list --all              # Show all projects (active + archived)
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
}

func runList(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
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
		candidates = reg.GetActiveProjects()
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
		})
	}

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
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	syncedColor := lipgloss.Color("213") // Pink/Magenta for synced
	gray := lipgloss.Color("245")

	// Define styles
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(cyan).
		Padding(0, 1)

	// Build rows
	rows := make([][]string, 0, len(projects))
	for _, proj := range projects {
		// Truncate description to 40 chars
		desc := truncate(proj.Description, 40)

		rows = append(rows, []string{
			proj.Name,
			proj.Category,
			proj.Lang,
			desc,
			proj.Status,
		})
	}

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
				return style.Foreground(gray)
			case 2: // Lang
				lang := rows[row][2]
				if color, ok := languageColors[lang]; ok {
					return style.Foreground(color)
				}
				return style.Foreground(gray)
			case 3: // Description
				return style.Foreground(gray)
			case 4: // Status
				status := rows[row][4]
				switch status {
				case "synced":
					return style.Foreground(syncedColor)
				case "active":
					return style.Foreground(green)
				default:
					return style.Foreground(gray)
				}
			}
			return style
		}).
		Headers("NAME", "CATEGORY", "LANG", "DESCRIPTION", "STATUS").
		Rows(rows...)

	fmt.Println(t)
	return nil
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

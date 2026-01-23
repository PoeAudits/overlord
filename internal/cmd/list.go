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

	"overlord-v2/internal/registry"
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
		result = append(result, projectOutput{
			Name:        name,
			Path:        proj.Path,
			Category:    string(proj.Category),
			Lang:        string(proj.Lang),
			Description: proj.Description,
			Status:      string(proj.Status.State),
			Tags:        proj.Tags,
		})
	}

	// Sort alphabetically by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

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

// outputListTable outputs projects as a formatted table
func outputListTable(projects []projectOutput) error {
	// Define colors
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	yellow := lipgloss.Color("220")
	gray := lipgloss.Color("245")
	purple := lipgloss.Color("99")

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

			switch col {
			case 0: // Name
				return style.Foreground(lipgloss.Color("255")).Bold(true)
			case 1: // Category
				return style.Foreground(purple)
			case 2: // Lang
				return style.Foreground(yellow)
			case 3: // Description
				return style.Foreground(gray)
			case 4: // Status
				// Get the actual row data to check status
				if row >= 0 && row < len(rows) {
					status := rows[row][4]
					if status == "active" {
						return style.Foreground(green)
					}
					return style.Foreground(gray)
				}
				return style
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

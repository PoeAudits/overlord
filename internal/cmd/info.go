package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	infoJSON bool
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed information about a project",
	Long: `Show detailed information about a single project, including all metadata
and thoughts directory paths.

The name argument can be either a project name or alias.

Examples:
  overlord info dispatch
  overlord info dispatch --json`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolVar(&infoJSON, "json", false, "Output in JSON format")
}

func runInfo(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Resolve project using fuzzy matching
	result, err := reg.ResolveOne(name)
	if err != nil {
		return fmt.Errorf("Error: project '%s' not found\nRun 'overlord list' to see all projects", name)
	}
	projectName := result.Name
	project := result.Project

	// Output based on format
	if infoJSON {
		return outputInfoJSON(projectName, project, reg.Settings)
	}

	return outputInfoHuman(projectName, project, reg.Settings)
}

func outputInfoHuman(name string, project registry.Project, settings registry.Settings) error {
	// Define colors (same as list command)
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	yellow := lipgloss.Color("220")
	gray := lipgloss.Color("245")
	purple := lipgloss.Color("99")
	white := lipgloss.Color("255")

	// Define styles
	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(white)
	categoryStyle := lipgloss.NewStyle().Foreground(purple)
	langStyle := lipgloss.NewStyle().Foreground(yellow)
	statusActiveStyle := lipgloss.NewStyle().Foreground(green)
	statusArchivedStyle := lipgloss.NewStyle().Foreground(gray)
	pathStyle := lipgloss.NewStyle().Foreground(gray)
	sectionStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)

	// Format aliases
	aliasesStr := "-"
	if len(project.Aliases) > 0 {
		aliasesStr = strings.Join(project.Aliases, ", ")
	}

	// Format tags
	tagsStr := "-"
	if len(project.Tags) > 0 {
		tagsStr = strings.Join(project.Tags, ", ")
	}

	// Get thoughts paths
	thoughtsPaths := getThoughtsPaths(name, settings.ThoughtsDir)

	// Determine status style
	statusStyle := statusActiveStyle
	if project.Status.State == registry.StateArchived {
		statusStyle = statusArchivedStyle
	}

	// Print formatted output
	fmt.Printf("%s %s\n", labelStyle.Render("Project:"), valueStyle.Render(name))
	fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(project.Path))
	fmt.Printf("%s %s\n", labelStyle.Render("Category:"), categoryStyle.Render(string(project.Category)))
	fmt.Printf("%s %s\n", labelStyle.Render("Language:"), langStyle.Render(string(project.Lang)))
	fmt.Printf("%s %s\n", labelStyle.Render("Description:"), valueStyle.Render(project.Description))
	fmt.Printf("%s %s\n", labelStyle.Render("Tags:"), valueStyle.Render(tagsStr))
	fmt.Printf("%s %s\n", labelStyle.Render("Status:"), statusStyle.Render(string(project.Status.State)))
	fmt.Printf("%s %s\n", labelStyle.Render("Created:"), valueStyle.Render(project.Created))
	fmt.Printf("%s %s\n", labelStyle.Render("Aliases:"), valueStyle.Render(aliasesStr))
	fmt.Println()
	fmt.Printf("%s\n", sectionStyle.Render("Thoughts:"))
	fmt.Printf("  %s %s\n", labelStyle.Render("Plans:"), pathStyle.Render(thoughtsPaths.Plans))
	fmt.Printf("  %s %s\n", labelStyle.Render("Logs:"), pathStyle.Render(thoughtsPaths.Logs))
	fmt.Printf("  %s %s\n", labelStyle.Render("Sessions:"), pathStyle.Render(thoughtsPaths.Sessions))

	return nil
}

func outputInfoJSON(name string, project registry.Project, settings registry.Settings) error {
	thoughtsPaths := getThoughtsPaths(name, settings.ThoughtsDir)

	output := map[string]interface{}{
		"name":        name,
		"path":        project.Path,
		"category":    project.Category,
		"lang":        project.Lang,
		"description": project.Description,
		"tags":        project.Tags,
		"aliases":     project.Aliases,
		"status":      project.Status.State,
		"created":     project.Created,
		"thoughts": map[string]string{
			"plans":    thoughtsPaths.Plans,
			"logs":     thoughtsPaths.Logs,
			"sessions": thoughtsPaths.Sessions,
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

type thoughtsPaths struct {
	Plans    string
	Logs     string
	Sessions string
}

func getThoughtsPaths(projectName, thoughtsDir string) thoughtsPaths {
	// Convert to tilde notation for display
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	formatPath := func(path string) string {
		if homeDir != "" && strings.HasPrefix(path, homeDir) {
			return "~" + strings.TrimPrefix(path, homeDir)
		}
		return path
	}

	// Expand thoughts dir if it starts with ~
	expandedThoughtsDir := thoughtsDir
	if strings.HasPrefix(thoughtsDir, "~") {
		if homeDir != "" {
			expandedThoughtsDir = filepath.Join(homeDir, strings.TrimPrefix(thoughtsDir, "~"))
		}
	}

	plansPath := filepath.Join(expandedThoughtsDir, "plans", projectName) + "/"
	logsPath := filepath.Join(expandedThoughtsDir, "logs", projectName) + "/"
	sessionsPath := filepath.Join(expandedThoughtsDir, "sessions", projectName) + "/"

	return thoughtsPaths{
		Plans:    formatPath(plansPath),
		Logs:     formatPath(logsPath),
		Sessions: formatPath(sessionsPath),
	}
}

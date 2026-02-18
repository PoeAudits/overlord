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
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve project using fuzzy matching
	projectName, project, err := resolveProject(reg, name)
	if err != nil {
		return err
	}

	// Output based on format
	if infoJSON {
		return outputInfoJSON(projectName, project, reg.Settings)
	}

	return outputInfoHuman(projectName, project, reg.Settings)
}

func outputInfoHuman(name string, project registry.Project, settings registry.Settings) error {
	// Define styles
	s := NewStyles()
	labelStyle := s.Label
	valueStyle := s.Value
	categoryStyle := s.Category
	langStyle := lipgloss.NewStyle().Foreground(ColorYellow)
	statusActiveStyle := s.Success
	statusArchivedStyle := s.Info
	pathStyle := s.Path
	sectionStyle := s.Section

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
	thoughtsPaths := getThoughtsPaths(name, project.Status.State, project.Path, settings.ThoughtsDir)

	projectPath, projectPathErr := expandProjectPath(project.Path, settings.BaseDir)
	projectThoughtsSymlink := "-"
	if projectPathErr == nil {
		projectThoughtsSymlink = formatPathWithTilde(filepath.Join(projectPath, "thoughts"))
	}

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
	fmt.Printf("  %s %s\n", labelStyle.Render("Briefs:"), pathStyle.Render(thoughtsPaths.Briefs))
	fmt.Printf("  %s %s\n", labelStyle.Render("Research:"), pathStyle.Render(thoughtsPaths.Research))
	fmt.Printf("  %s %s\n", labelStyle.Render("Plans:"), pathStyle.Render(thoughtsPaths.Plans))
	fmt.Printf("  %s %s\n", labelStyle.Render("Logs:"), pathStyle.Render(thoughtsPaths.Logs))
	fmt.Printf("  %s %s\n", labelStyle.Render("Docs:"), pathStyle.Render(thoughtsPaths.Docs))
	fmt.Printf("  %s %s\n", labelStyle.Render("Project link:"), pathStyle.Render(projectThoughtsSymlink))

	return nil
}

func outputInfoJSON(name string, project registry.Project, settings registry.Settings) error {
	thoughtsPaths := getThoughtsPaths(name, project.Status.State, project.Path, settings.ThoughtsDir)

	projectPath, projectPathErr := expandProjectPath(project.Path, settings.BaseDir)
	projectThoughtsSymlink := ""
	if projectPathErr == nil {
		projectThoughtsSymlink = formatPathWithTilde(filepath.Join(projectPath, "thoughts"))
	}

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
			"briefs":   thoughtsPaths.Briefs,
			"research": thoughtsPaths.Research,
			"plans":    thoughtsPaths.Plans,
			"logs":     thoughtsPaths.Logs,
			"docs":     thoughtsPaths.Docs,
			"symlink":  projectThoughtsSymlink,
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
	Briefs   string
	Research string
	Plans    string
	Logs     string
	Docs     string
}

func getThoughtsPaths(projectName string, state registry.State, projectPath, thoughtsDir string) thoughtsPaths {
	// Expand thoughts dir if it starts with ~
	expandedThoughtsDir := thoughtsDir
	homeDir, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(thoughtsDir, "~") {
		expandedThoughtsDir = filepath.Join(homeDir, strings.TrimPrefix(thoughtsDir, "~"))
	}

	base := "projects"
	if state == registry.StateArchived || strings.HasPrefix(projectPath, "archive/") {
		base = "archive"
	}

	briefsPath := filepath.Join(expandedThoughtsDir, base, projectName, "briefs") + "/"
	researchPath := filepath.Join(expandedThoughtsDir, base, projectName, "research") + "/"
	plansPath := filepath.Join(expandedThoughtsDir, base, projectName, "plans") + "/"
	logsPath := filepath.Join(expandedThoughtsDir, base, projectName, "logs") + "/"
	docsPath := filepath.Join(expandedThoughtsDir, base, projectName, "docs") + "/"

	return thoughtsPaths{
		Briefs:   formatPathWithTilde(briefsPath),
		Research: formatPathWithTilde(researchPath),
		Plans:    formatPathWithTilde(plansPath),
		Logs:     formatPathWithTilde(logsPath),
		Docs:     formatPathWithTilde(docsPath),
	}
}

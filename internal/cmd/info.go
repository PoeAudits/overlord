package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
)

type infoFlags struct {
	json bool
}

var infoOpts infoFlags

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed information about a project",
	Long: `Show detailed information about a single project, including all metadata.

The name argument can be either a project name or alias.

Examples:
  overlord info dispatch
  overlord info dispatch --json`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.Flags().BoolVar(&infoOpts.json, "json", false, "Output in JSON format")
}

func runInfo(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Resolve project using fuzzy matching
	result, err := reg.ResolveOne(name)
	if err != nil {
		return withHintOnNoProjectMatch(err, "run 'overlord list' to see available projects")
	}
	projectName := result.Name
	project := result.Project

	// Output based on format
	if infoOpts.json {
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
	langStyle := s.Lang
	statusActiveStyle := s.Success
	statusArchivedStyle := s.Info
	pathStyle := s.Path

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
	return nil
}

// infoOutput is the typed shape for --json output from the info command.
type infoOutput struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Category    registry.Category `json:"category"`
	Lang        registry.Language `json:"lang"`
	Description string            `json:"description"`
	Tags        []string          `json:"tags"`
	Aliases     []string          `json:"aliases"`
	Status      registry.State    `json:"status"`
	Created     string            `json:"created"`
}

func outputInfoJSON(name string, project registry.Project, settings registry.Settings) error {
	output := infoOutput{
		Name:        name,
		Path:        project.Path,
		Category:    project.Category,
		Lang:        project.Lang,
		Description: project.Description,
		Tags:        project.Tags,
		Aliases:     project.Aliases,
		Status:      project.Status.State,
		Created:     project.Created,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

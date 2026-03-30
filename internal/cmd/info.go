package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/thoughts"
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

	projectPath, projectPathErr := fileutil.ExpandProjectPath(project.Path, settings.BaseDir)
	projectThoughtsSymlink := "-"
	if projectPathErr == nil {
		projectThoughtsSymlink = fileutil.FormatPathWithTilde(filepath.Join(projectPath, "thoughts"))
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

// infoThoughts holds the thoughts directory paths for JSON output.
type infoThoughts struct {
	Briefs   string `json:"briefs"`
	Research string `json:"research"`
	Plans    string `json:"plans"`
	Logs     string `json:"logs"`
	Docs     string `json:"docs"`
	Symlink  string `json:"symlink"`
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
	Thoughts    infoThoughts      `json:"thoughts"`
}

func outputInfoJSON(name string, project registry.Project, settings registry.Settings) error {
	thoughtsPaths := getThoughtsPaths(name, project.Status.State, project.Path, settings.ThoughtsDir)

	projectPath, projectPathErr := fileutil.ExpandProjectPath(project.Path, settings.BaseDir)
	projectThoughtsSymlink := ""
	if projectPathErr == nil {
		projectThoughtsSymlink = fileutil.FormatPathWithTilde(filepath.Join(projectPath, "thoughts"))
	}

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
		Thoughts: infoThoughts{
			Briefs:   thoughtsPaths.Briefs,
			Research: thoughtsPaths.Research,
			Plans:    thoughtsPaths.Plans,
			Logs:     thoughtsPaths.Logs,
			Docs:     thoughtsPaths.Docs,
			Symlink:  projectThoughtsSymlink,
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
}

func getThoughtsPaths(projectName string, state registry.State, projectPath, thoughtsDir string) thoughts.Paths {
	expandedThoughtsDir, err := registry.ExpandPath(thoughtsDir)
	if err != nil {
		expandedThoughtsDir = thoughtsDir
	}

	paths := thoughts.ProjectPaths(expandedThoughtsDir, projectName)
	if state == registry.StateArchived || strings.HasPrefix(projectPath, "archive/") {
		paths = thoughts.ArchivePaths(expandedThoughtsDir, projectName)
	}

	paths.Briefs = fileutil.FormatPathWithTilde(paths.Briefs) + "/"
	paths.Research = fileutil.FormatPathWithTilde(paths.Research) + "/"
	paths.Plans = fileutil.FormatPathWithTilde(paths.Plans) + "/"
	paths.Logs = fileutil.FormatPathWithTilde(paths.Logs) + "/"
	paths.Docs = fileutil.FormatPathWithTilde(paths.Docs) + "/"

	return paths
}

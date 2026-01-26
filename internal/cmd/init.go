package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/templates"
)

// initFlags holds the flags for the init command
type initFlags struct {
	name        string
	description string
	// Language shorthand flags
	langPy  bool
	langTs  bool
	langGo  bool
	langSol bool
}

var initOpts initFlags

var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize a directory with project template files",
	Long: `Initialize a directory with common project template files.

Writes the following files (skips any that already exist):
  - .tmux.local (tmux session layout)
  - Makefile (language-specific build commands)
  - README.md (project documentation)
  - AGENTS.md (AI agent instructions)
  - .opencode/opencode.jsonc (editor configuration)

Language is auto-detected from project files if not specified.
Project name defaults to the directory name.

Examples:
  overlord init                      # Initialize current directory
  overlord init .                    # Same as above
  overlord init ~/projects/my-app    # Initialize specific directory
  overlord init --ts                 # Initialize with TypeScript templates
  overlord init --go --name=my-svc   # Go templates with custom name`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInitCmd,
}

func init() {
	rootCmd.AddCommand(initCmd)

	// Name override
	initCmd.Flags().StringVar(&initOpts.name, "name", "", "Project name (defaults to directory name)")

	// Description
	initCmd.Flags().StringVar(&initOpts.description, "description", "", "Project description")

	// Language shorthand flags
	initCmd.Flags().BoolVar(&initOpts.langPy, "py", false, "Use Python templates")
	initCmd.Flags().BoolVar(&initOpts.langTs, "ts", false, "Use TypeScript templates")
	initCmd.Flags().BoolVar(&initOpts.langGo, "go", false, "Use Go templates")
	initCmd.Flags().BoolVar(&initOpts.langSol, "sol", false, "Use Solidity templates")
}

// getInitLanguage determines the language from init flags
func getInitLanguage() (registry.Language, bool) {
	switch {
	case initOpts.langPy:
		return registry.LanguagePython, true
	case initOpts.langTs:
		return registry.LanguageTypeScript, true
	case initOpts.langGo:
		return registry.LanguageGo, true
	case initOpts.langSol:
		return registry.LanguageSolidity, true
	default:
		return "", false
	}
}

func runInitCmd(cmd *cobra.Command, args []string) error {
	// Determine target path
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	// Resolve to absolute path
	absPath, err := expandPath(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check directory exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("directory does not exist: %s", absPath)
	}
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absPath)
	}

	// Determine project name
	name := initOpts.name
	if name == "" {
		name = filepath.Base(absPath)
	}

	// Determine language
	var lang registry.Language
	if flagLang, hasFlag := getInitLanguage(); hasFlag {
		lang = flagLang
	} else {
		lang = detectLanguage(absPath)
	}

	// Determine description
	description := initOpts.description
	if description == "" {
		description = fmt.Sprintf("%s project", name)
	}

	// Styles
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	yellow := lipgloss.Color("220")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(yellow)
	pathStyle := lipgloss.NewStyle().Foreground(gray)

	fmt.Printf("%s %s\n", labelStyle.Render("Initializing:"), pathStyle.Render(absPath))
	fmt.Printf("%s %s\n", labelStyle.Render("Language:"), string(lang))
	fmt.Println()

	// Create template data
	templateData := templates.NewTemplateData(
		name,
		description,
		"",
		string(lang),
	)

	// Map registry language to template language
	templateLang := templates.Language(lang)

	// Get all templates
	allTemplates, err := templates.GetAllTemplates(templateLang, templateData)
	if err != nil {
		return fmt.Errorf("failed to render templates: %w", err)
	}

	// Write each template, skipping existing files
	written := 0
	skipped := 0

	for templateType, content := range allTemplates {
		outputPath := filepath.Join(absPath, templateType.OutputPath())
		relPath := templateType.OutputPath()

		// Check if file already exists
		if _, err := os.Stat(outputPath); err == nil {
			fmt.Printf("  %s %s (already exists)\n", warnStyle.Render("Skipped"), relPath)
			skipped++
			continue
		}

		// Ensure parent directory exists (for .opencode/opencode.jsonc)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", relPath, err)
		}

		// Write file
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", relPath, err)
		}

		fmt.Printf("  %s %s\n", successStyle.Render("Written"), relPath)
		written++
	}

	fmt.Println()
	fmt.Printf("%s (%d written, %d skipped)\n",
		successStyle.Render("Initialization complete!"), written, skipped)

	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"overlord-v2/internal/registry"
)

// addFlags holds the flags for the add command
type addFlags struct {
	category    string
	description string
	aliases     []string
	// Language shorthand flags
	langPy  bool
	langTs  bool
	langGo  bool
	langSol bool
}

var addOpts addFlags

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add <name> <path>",
	Short: "Register an existing project directory in the registry",
	Long: `Register an existing project directory in the registry.

The command will:
  - Validate the project path exists
  - Auto-detect language from project files (or use --py/--ts/--go/--sol)
  - Auto-detect category from path (or use --category)
  - Create thoughts directories with symlinks
  - Register project in registry

Language auto-detection checks for:
  - go.mod → Go
  - pyproject.toml or requirements.txt → Python
  - package.json → TypeScript
  - foundry.toml or hardhat.config.* → Solidity
  - Default: base

Category auto-detection (if path is under ~/Overlord/):
  - ~/Overlord/projects/contracts/ → contracts
  - ~/Overlord/projects/web/ → web
  - ~/Overlord/projects/services/ → services
  - etc.

Examples:
  overlord add my-api ~/Overlord/projects/services/my-api
  overlord add my-app ./my-app --category=web --ts
  overlord add my-lib ../my-lib --py --description="Python library"
  overlord add my-contract ~/contracts/my-contract --sol --alias=mc`,
	Args: cobra.ExactArgs(2),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Category flag
	addCmd.Flags().StringVar(&addOpts.category, "category", "", "Project category (auto-detect if not specified)")

	// Description flag
	addCmd.Flags().StringVar(&addOpts.description, "description", "", "Project description")

	// Alias flag (can be repeated)
	addCmd.Flags().StringArrayVar(&addOpts.aliases, "alias", []string{}, "Add alias (can be repeated)")

	// Language shorthand flags
	addCmd.Flags().BoolVar(&addOpts.langPy, "py", false, "Use Python language")
	addCmd.Flags().BoolVar(&addOpts.langTs, "ts", false, "Use TypeScript language")
	addCmd.Flags().BoolVar(&addOpts.langGo, "go", false, "Use Go language")
	addCmd.Flags().BoolVar(&addOpts.langSol, "sol", false, "Use Solidity language")
}

// detectLanguage detects the language from project files
func detectLanguage(projectPath string) registry.Language {
	// Check for language-specific files
	checks := []struct {
		files []string
		lang  registry.Language
	}{
		{[]string{"go.mod"}, registry.LanguageGo},
		{[]string{"pyproject.toml", "requirements.txt"}, registry.LanguagePython},
		{[]string{"package.json"}, registry.LanguageTypeScript},
		{[]string{"foundry.toml", "hardhat.config.js", "hardhat.config.ts"}, registry.LanguageSolidity},
	}

	for _, check := range checks {
		for _, file := range check.files {
			if _, err := os.Stat(filepath.Join(projectPath, file)); err == nil {
				return check.lang
			}
		}
	}

	return registry.LanguageBase
}

// detectCategory detects the category from the project path
func detectCategory(projectPath, baseDir string) (registry.Category, error) {
	// Expand both paths for comparison
	expandedPath, err := expandPath(projectPath)
	if err != nil {
		return "", fmt.Errorf("failed to expand project path: %w", err)
	}

	expandedBase, err := expandPath(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to expand base directory: %w", err)
	}

	// Check if path is under base directory
	relPath, err := filepath.Rel(expandedBase, expandedPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// Path is not under base directory
		return "", fmt.Errorf("cannot auto-detect category: path is not under base directory (%s)", baseDir)
	}

	// Try to match category from path
	for _, cat := range registry.ValidCategories() {
		catPath := cat.Path()
		if strings.HasPrefix(relPath, catPath) {
			return cat, nil
		}
	}

	return "", fmt.Errorf("cannot auto-detect category from path, please specify --category")
}

// getLanguageFromFlags determines the language from flags
func getLanguageFromFlags() (registry.Language, bool) {
	switch {
	case addOpts.langPy:
		return registry.LanguagePython, true
	case addOpts.langTs:
		return registry.LanguageTypeScript, true
	case addOpts.langGo:
		return registry.LanguageGo, true
	case addOpts.langSol:
		return registry.LanguageSolidity, true
	default:
		return "", false
	}
}

// createThoughtsStructure creates thoughts directories and symlinks
func createThoughtsStructure(name, projectPath, thoughtsDir string) error {
	// Expand project path to absolute path
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create thoughts directories
	thoughtsProjectDir := filepath.Join(thoughtsDir, "projects", name)
	thoughtsPlansDir := filepath.Join(thoughtsDir, "plans", name)
	thoughtsLogsDir := filepath.Join(thoughtsDir, "logs", name)
	thoughtsSessionsDir := filepath.Join(thoughtsDir, "sessions", name)

	for _, dir := range []string{thoughtsProjectDir, thoughtsPlansDir, thoughtsLogsDir, thoughtsSessionsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create thoughts directory %s: %w", dir, err)
		}
	}

	// Create symlinks in projects directory
	readmeLink := filepath.Join(thoughtsProjectDir, "README.md")
	agentsLink := filepath.Join(thoughtsProjectDir, "AGENTS.md")

	// Calculate relative path from thoughts/projects/{name}/ to project directory
	relPath, err := filepath.Rel(thoughtsProjectDir, absProjectPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Create symlinks using relative paths
	readmeRelTarget := filepath.Join(relPath, "README.md")
	agentsRelTarget := filepath.Join(relPath, "AGENTS.md")

	// Remove existing symlinks if they exist
	os.Remove(readmeLink)
	os.Remove(agentsLink)

	if err := os.Symlink(readmeRelTarget, readmeLink); err != nil {
		return fmt.Errorf("failed to create README.md symlink: %w", err)
	}

	if err := os.Symlink(agentsRelTarget, agentsLink); err != nil {
		return fmt.Errorf("failed to create AGENTS.md symlink: %w", err)
	}

	return nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectPath := args[1]

	// Define styles for output
	cyan := lipgloss.Color("86")
	green := lipgloss.Color("82")
	yellow := lipgloss.Color("220")
	gray := lipgloss.Color("245")

	labelStyle := lipgloss.NewStyle().Foreground(cyan).Bold(true)
	successStyle := lipgloss.NewStyle().Foreground(green).Bold(true)
	warnStyle := lipgloss.NewStyle().Foreground(yellow)
	pathStyle := lipgloss.NewStyle().Foreground(gray)

	// Validate name
	if err := validateName(name); err != nil {
		return err
	}

	// Expand and validate project path
	expandedPath, err := expandPath(projectPath)
	if err != nil {
		return fmt.Errorf("failed to expand project path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("Error: project directory does not exist\nPath: %s\nUse 'overlord new' to create a new project", expandedPath)
	}

	// Load registry
	reg, err := registry.Load(registry.DefaultRegistryPath)
	if err != nil {
		return fmt.Errorf("failed to load registry: %w", err)
	}

	// Check if project already exists
	if _, exists := reg.GetProject(name); exists {
		return fmt.Errorf("Error: project '%s' already exists\nUse 'overlord info %s' to see details", name, name)
	}

	// Expand base directory
	baseDir, err := expandPath(reg.Settings.BaseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base directory: %w", err)
	}

	// Expand thoughts directory
	thoughtsDir, err := expandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	// Determine language (flag takes precedence over auto-detection)
	var lang registry.Language
	if flagLang, hasFlag := getLanguageFromFlags(); hasFlag {
		lang = flagLang
	} else {
		lang = detectLanguage(expandedPath)
		fmt.Printf("%s %s\n", warnStyle.Render("Auto-detected language:"), string(lang))
	}

	// Determine category (flag takes precedence over auto-detection)
	var category registry.Category
	if addOpts.category != "" {
		category = registry.Category(addOpts.category)
		if !category.IsValid() {
			validCats := make([]string, 0, len(registry.ValidCategories()))
			for _, c := range registry.ValidCategories() {
				validCats = append(validCats, string(c))
			}
			return fmt.Errorf("invalid category '%s'\nValid categories: %s", addOpts.category, strings.Join(validCats, ", "))
		}
	} else {
		detectedCat, err := detectCategory(expandedPath, baseDir)
		if err != nil {
			return err
		}
		category = detectedCat
		fmt.Printf("%s %s\n", warnStyle.Render("Auto-detected category:"), string(category))
	}

	// Set description default if not provided
	description := addOpts.description
	if description == "" {
		description = fmt.Sprintf("%s project", name)
	}

	fmt.Println()
	fmt.Printf("%s %s\n", labelStyle.Render("Adding project:"), name)
	fmt.Printf("%s %s\n", labelStyle.Render("Category:"), string(category))
	fmt.Printf("%s %s\n", labelStyle.Render("Language:"), string(lang))
	fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(expandedPath))
	fmt.Println()

	// Create thoughts structure
	fmt.Printf("  Creating thoughts structure...")
	if err := createThoughtsStructure(name, expandedPath, thoughtsDir); err != nil {
		return err
	}
	fmt.Println(" done")

	// Calculate relative path from base_dir (or use absolute if not under base_dir)
	var registryPath string
	relativePath, err := filepath.Rel(baseDir, expandedPath)
	if err != nil || strings.HasPrefix(relativePath, "..") {
		// Path is not under base directory, use absolute path
		registryPath = expandedPath
		fmt.Printf("  %s Project is not under base directory, storing absolute path\n", warnStyle.Render("Note:"))
	} else {
		registryPath = relativePath
	}

	// Register project in registry
	fmt.Printf("  Registering project...")

	project := registry.Project{
		Path:        registryPath,
		Category:    category,
		Lang:        lang,
		Created:     time.Now().Format("2006-01-02"),
		Description: description,
		Aliases:     addOpts.aliases,
		Tags:        []string{},
		Status:      registry.Status{State: registry.StateActive},
	}

	// Add to registry
	if reg.Projects == nil {
		reg.Projects = make(map[string]registry.Project)
	}
	reg.Projects[name] = project

	// Save registry
	if err := registry.Save(registry.DefaultRegistryPath, reg); err != nil {
		return fmt.Errorf("failed to save registry: %w", err)
	}
	fmt.Println(" done")

	fmt.Println()
	fmt.Printf("%s\n", successStyle.Render("Project added successfully!"))
	fmt.Println()
	fmt.Printf("%s %s\n", labelStyle.Render("Project path:"), pathStyle.Render(expandedPath))
	fmt.Printf("%s %s\n", labelStyle.Render("Thoughts:"), pathStyle.Render(filepath.Join(thoughtsDir, "projects", name)))
	if len(addOpts.aliases) > 0 {
		fmt.Printf("%s %s\n", labelStyle.Render("Aliases:"), strings.Join(addOpts.aliases, ", "))
	}
	fmt.Println()

	return nil
}

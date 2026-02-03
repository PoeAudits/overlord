package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

// addFlags holds the flags for the add command
type addFlags struct {
	name        string
	category    string
	description string
	aliases     []string
	// Language shorthand flags
	langPy   bool
	langTs   bool
	langGo   bool
	langSol  bool
	langBase bool
}

var addOpts addFlags

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Register an existing project directory in the registry",
	Long: `Register an existing project directory in the registry.

The project name is derived from the directory name by default.
Use --name to override.

The command will:
  - Validate the project path exists
  - Auto-detect language from project files (or use --py/--ts/--go/--sol/--base)
  - Auto-detect category from path (or use --category)
  - Create thoughts directories with symlinks
  - Register project in registry

Language auto-detection checks for:
  - go.mod → Go
  - pyproject.toml or requirements.txt → Python
  - package.json → TypeScript
  - foundry.toml or hardhat.config.* → Solidity
  - Errors if no markers found (use --base for language-agnostic projects)

Category auto-detection requires the project to be under the base directory
(~/Overlord/ by default). The category is inferred from the subdirectory.

Examples:
  overlord add                            # Add current directory
  overlord add .                          # Same as above
  overlord add ../my-project              # Add relative path
  overlord add ~/Overlord/projects/web/x  # Add absolute path
  overlord add --name=custom-name         # Override derived name
  overlord add --py                       # Specify language explicitly
  overlord add . --alias=mp               # Add with alias
  overlord add my-api ~/path/to/api       # Legacy: explicit name + path`,
	Args:    cobra.RangeArgs(0, 2),
	PreRunE: validateAddFlags,
	RunE:    runAdd,
}

// formatAddUsage returns a helpful usage string for the add command
func formatAddUsage() string {
	validCats := make([]string, 0, len(registry.ValidCategories()))
	for _, c := range registry.ValidCategories() {
		validCats = append(validCats, string(c))
	}

	return fmt.Sprintf(`Usage: overlord add [path] [options]
       overlord add <name> <path>   (legacy two-argument form)

The project name is derived from the directory name by default.

Categories (auto-detected from path, or use --category):
  %s

Language flags (auto-detected, or specify one):
  --py    Python
  --ts    TypeScript
  --go    Go
  --sol   Solidity
  --base  Language-agnostic

Options:
  --name="..."          Override auto-detected project name
  --description="..."   Project description
  --alias=<name>        Add alias (can be repeated)
  --category=<cat>      Override auto-detected category

Examples:
  overlord add                          # Add current directory
  overlord add .                        # Same as above
  overlord add --name=custom --py       # Custom name, Python
  overlord add ../my-lib --sol          # Relative path, Solidity`, strings.Join(validCats, ", "))
}

// validateAddFlags validates flags and provides helpful error messages
func validateAddFlags(cmd *cobra.Command, args []string) error {
	// Check that only one language flag is set
	if countTrue(addOpts.langPy, addOpts.langTs, addOpts.langGo, addOpts.langSol, addOpts.langBase) > 1 {
		return fmt.Errorf("only one language flag can be set (--py, --ts, --go, --sol, or --base)")
	}

	// Warn if --name is provided with 2 args (name comes from first arg)
	if addOpts.name != "" && len(args) == 2 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: --name is ignored when using two-argument form (name comes from first argument)\n")
	}

	return nil
}

func init() {
	rootCmd.AddCommand(addCmd)

	// Name override flag
	addCmd.Flags().StringVar(&addOpts.name, "name", "", "Override auto-detected project name")

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
	addCmd.Flags().BoolVar(&addOpts.langBase, "base", false, "Use base (language-agnostic)")
}

// detectLanguage detects the language from project files.
// Returns an error if no recognized language marker files are found.
func detectLanguage(projectPath string) (registry.Language, error) {
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
				return check.lang, nil
			}
		}
	}

	return "", fmt.Errorf("could not auto-detect language in %s\nNo recognized files found (go.mod, pyproject.toml, package.json, foundry.toml)\nSpecify: --py, --ts, --go, --sol, or --base", projectPath)
}

// detectCategory detects the category from the project path.
// Both projectPath and baseDir should already be expanded/absolute paths.
func detectCategory(projectPath, baseDir string) (registry.Category, error) {
	// Check if path is under base directory
	relPath, err := filepath.Rel(baseDir, projectPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		// Build helpful error listing all category paths
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("cannot auto-detect category: path is not under base directory\n"))
		sb.WriteString(fmt.Sprintf("  Path:     %s\n", projectPath))
		sb.WriteString(fmt.Sprintf("  Base dir: %s\n\n", baseDir))
		sb.WriteString("Projects must be under one of:\n")
		for _, cat := range registry.ValidCategories() {
			sb.WriteString(fmt.Sprintf("  %s%-30s → %s\n", baseDir+"/", cat.Path(), string(cat)))
		}
		sb.WriteString(fmt.Sprintf("\nOr specify manually: --category=<category>"))
		return "", fmt.Errorf("%s", sb.String())
	}

	// Try to match category from path
	for _, cat := range registry.ValidCategories() {
		catPath := cat.Path()
		if strings.HasPrefix(relPath, catPath) {
			return cat, nil
		}
	}

	// Path is under base dir but doesn't match any known category subdirectory
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("cannot auto-detect category from path\n"))
	sb.WriteString(fmt.Sprintf("  Path (relative): %s\n\n", relPath))
	sb.WriteString("Expected subdirectory structure:\n")
	for _, cat := range registry.ValidCategories() {
		sb.WriteString(fmt.Sprintf("  %-30s → %s\n", cat.Path(), string(cat)))
	}
	sb.WriteString(fmt.Sprintf("\nOr specify manually: --category=<category>"))
	return "", fmt.Errorf("%s", sb.String())
}

// getLanguageFromFlags determines the language from flags
func getLanguageFromFlags() (registry.Language, bool) {
	return languageFromFlags(addOpts.langPy, addOpts.langTs, addOpts.langGo, addOpts.langSol, addOpts.langBase)
}

// createThoughtsStructure creates thoughts directories and symlinks.
// Creates all 8 subdirectories under ~/thoughts/projects/{name}/:
// plans, logs, docs, research, sessions, handoffs, reviews, briefs
// Symlinks to README.md and AGENTS.md are created in the docs/ subdirectory.
func createThoughtsStructure(name, projectPath, thoughtsDir string) error {
	// Expand project path to absolute path
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get all 8 thoughts subdirectory paths
	paths := GetThoughtsPaths(thoughtsDir, name)

	// Create all 8 subdirectories
	for _, dir := range paths.toSlice() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create thoughts directory %s: %w", dir, err)
		}
	}

	// Create symlinks in docs/ subdirectory (not project root)
	docsDir := paths.Docs
	readmeLink := filepath.Join(docsDir, "README.md")
	agentsLink := filepath.Join(docsDir, "AGENTS.md")

	// Calculate relative path from thoughts/projects/{name}/docs/ to project directory
	relPath, err := filepath.Rel(docsDir, absProjectPath)
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

// resolveAddArgs resolves the project name and path from command arguments and flags.
//
//	0 args: path = ".", name = basename(cwd)
//	1 arg:  path = args[0], name = basename(resolved path)
//	2 args: name = args[0], path = args[1]  (legacy backward-compatible form)
//
// The --name flag overrides the auto-derived name in the 0/1-arg modes.
func resolveAddArgs(args []string) (name, projectPath string, err error) {
	switch len(args) {
	case 0:
		projectPath = "."
	case 1:
		projectPath = args[0]
	case 2:
		// Legacy two-argument form: overlord add <name> <path>
		return args[0], args[1], nil
	}

	// Resolve to absolute to derive a stable directory name
	expanded, err := registry.ExpandPath(projectPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Derive name from directory basename
	name = filepath.Base(expanded)

	// --name flag overrides
	if addOpts.name != "" {
		name = addOpts.name
	}

	return name, projectPath, nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	// Resolve name and path from args
	name, projectPath, err := resolveAddArgs(args)
	if err != nil {
		return err
	}

	// Define styles for output
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Success
	warnStyle := s.Warn
	pathStyle := s.Path

	// Validate name
	if err := validateName(name); err != nil {
		return err
	}

	// Expand and validate project path
	expandedPath, err := registry.ExpandPath(projectPath)
	if err != nil {
		return fmt.Errorf("failed to expand project path: %w", err)
	}

	// Check if directory exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s\nUse 'overlord new' to create a new project", expandedPath)
	}

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Check if project name already exists
	if _, exists := reg.GetProject(name); exists {
		return fmt.Errorf("project '%s' already exists in registry\nUse 'overlord info %s' to see details", name, name)
	}

	// Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Check if this path is already registered under a different name
	if existingName, _, found := reg.FindProjectByPath(expandedPath, baseDir); found {
		return fmt.Errorf("this directory is already registered as project '%s'\nUse 'overlord info %s' to see details", existingName, existingName)
	}

	// Expand thoughts directory
	thoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	// Determine language (flag takes precedence over auto-detection)
	var lang registry.Language
	if flagLang, hasFlag := getLanguageFromFlags(); hasFlag {
		lang = flagLang
	} else {
		detected, langErr := detectLanguage(expandedPath)
		if langErr != nil {
			return langErr
		}
		lang = detected
		fmt.Printf("%s %s\n", warnStyle.Render("Auto-detected language:"), string(lang))
	}

	// Determine category (flag takes precedence over auto-detection)
	var category registry.Category
	if addOpts.category != "" {
		var err error
		category, err = validateCategory(addOpts.category)
		if err != nil {
			return err
		}
	} else {
		detectedCat, catErr := detectCategory(expandedPath, baseDir)
		if catErr != nil {
			return catErr
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
	if err := saveRegistry(reg); err != nil {
		return err
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

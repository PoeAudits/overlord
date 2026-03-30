package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/lifecycle"
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
		return cmdError("add", "only one language flag can be set", "Choose one of: --py, --ts, --go, --sol, --base")
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

	return "", cmdError("add", fmt.Sprintf("could not auto-detect language in %s", projectPath), "No recognized files found (go.mod, pyproject.toml, package.json, foundry.toml). Specify: --py, --ts, --go, --sol, or --base")
}

// detectCategory detects the category from the project path.
// Both projectPath and baseDir should already be expanded/absolute paths.
func detectCategory(projectPath, baseDir string) (registry.Category, error) {
	// Check if path is under base directory
	relPath, err := filepath.Rel(baseDir, projectPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", cmdError("add",
			fmt.Sprintf("cannot auto-detect category: path is not under base directory (%s)", baseDir),
			"Specify manually: --category=<category>")
	}

	// Try to match category from path
	for _, cat := range registry.ValidCategories() {
		catPath := cat.Path()
		if strings.HasPrefix(relPath, catPath) {
			return cat, nil
		}
	}

	// Path is under base dir but doesn't match any known category subdirectory
	catNames := make([]string, 0, len(registry.ValidCategories()))
	for _, cat := range registry.ValidCategories() {
		catNames = append(catNames, string(cat))
	}
	return "", cmdError("add",
		fmt.Sprintf("cannot auto-detect category from path '%s'", relPath),
		fmt.Sprintf("Specify manually: --category=<category>. Valid categories: %s", strings.Join(catNames, ", ")))
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

// addProjectContext bundles resolved and validated inputs for the add workflow.
type addProjectContext struct {
	name               string
	expandedPath       string
	thoughtsDir        string
	lang               registry.Language
	languageAutoDetect bool
	category           registry.Category
	categoryAutoDetect bool
	description        string
	aliases            []string
	registryPath       string
	registryPathIsAbs  bool
	reg                *registry.Registry
}

// buildAddContext validates inputs and resolves all paths needed by the add workflow.
func buildAddContext(_ *cobra.Command, args []string) (*addProjectContext, error) {
	name, projectPath, err := resolveAddArgs(args)
	if err != nil {
		return nil, err
	}

	if err := validateName(name); err != nil {
		return nil, err
	}

	expandedPath, err := registry.ExpandPath(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand project path: %w", err)
	}

	if _, err := os.Stat(expandedPath); errors.Is(err, fs.ErrNotExist) {
		return nil, cmdError("add", fmt.Sprintf("project directory does not exist: %s", expandedPath), "Use 'overlord new' to create a new project")
	}

	reg, err := loadRegistry()
	if err != nil {
		return nil, err
	}

	if _, exists := reg.ProjectByName(name); exists {
		return nil, cmdError("add", fmt.Sprintf("project '%s' already exists in registry", name), fmt.Sprintf("Use 'overlord info %s' to see details", name))
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return nil, err
	}

	if existingName, _, found := reg.FindProjectByPath(expandedPath, baseDir); found {
		return nil, cmdError("add", fmt.Sprintf("this directory is already registered as project '%s'", existingName), fmt.Sprintf("Use 'overlord info %s' to see details", existingName))
	}

	thoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	var (
		lang               registry.Language
		languageAutoDetect bool
	)
	if flagLang, hasFlag := languageFromFlags(addOpts.langPy, addOpts.langTs, addOpts.langGo, addOpts.langSol, addOpts.langBase); hasFlag {
		lang = flagLang
	} else {
		detected, langErr := detectLanguage(expandedPath)
		if langErr != nil {
			return nil, langErr
		}
		lang = detected
		languageAutoDetect = true
	}

	var (
		category           registry.Category
		categoryAutoDetect bool
	)
	if addOpts.category != "" {
		category, err = validateCategory(addOpts.category)
		if err != nil {
			return nil, err
		}
	} else {
		detectedCat, catErr := detectCategory(expandedPath, baseDir)
		if catErr != nil {
			return nil, catErr
		}
		category = detectedCat
		categoryAutoDetect = true
	}

	description := addOpts.description
	if description == "" {
		description = fmt.Sprintf("%s project", name)
	}

	registryPath := expandedPath
	registryPathIsAbs := true
	if relativePath, relErr := filepath.Rel(baseDir, expandedPath); relErr == nil && !strings.HasPrefix(relativePath, "..") {
		registryPath = relativePath
		registryPathIsAbs = false
	}

	return &addProjectContext{
		name:               name,
		expandedPath:       expandedPath,
		thoughtsDir:        thoughtsDir,
		lang:               lang,
		languageAutoDetect: languageAutoDetect,
		category:           category,
		categoryAutoDetect: categoryAutoDetect,
		description:        description,
		aliases:            addOpts.aliases,
		registryPath:       registryPath,
		registryPathIsAbs:  registryPathIsAbs,
		reg:                reg,
	}, nil
}

// runAddRegister adds the project to the registry and fires lifecycle hooks.
func runAddRegister(ctx *addProjectContext, s styles) error {
	fmt.Printf("  Registering project...")
	if ctx.registryPathIsAbs {
		fmt.Printf("  %s Project is not under base directory, storing absolute path\n", s.Warn.Render("Note:"))
	}

	project := registry.Project{
		Path:        ctx.registryPath,
		Category:    ctx.category,
		Lang:        ctx.lang,
		Created:     time.Now().Format("2006-01-02"),
		Description: ctx.description,
		Aliases:     ctx.aliases,
		Tags:        []string{},
		Status:      registry.Status{State: registry.StateActive},
	}

	if ctx.reg.Projects == nil {
		ctx.reg.Projects = make(map[string]registry.Project)
	}
	ctx.reg.Projects[ctx.name] = project

	if err := saveRegistry(ctx.reg); err != nil {
		return err
	}

	postCreateResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectCreated,
		ProjectName: ctx.name,
		ProjectPath: ctx.expandedPath,
		Settings:    buildThoughtsSettings(ctx.reg, s),
	})
	for _, warning := range postCreateResult.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	fmt.Println(" done")
	return nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	ctx, err := buildAddContext(cmd, args)
	if err != nil {
		return err
	}

	// Define styles for output
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Success
	warnStyle := s.Warn
	pathStyle := s.Path

	if ctx.languageAutoDetect {
		fmt.Printf("%s %s\n", warnStyle.Render("Auto-detected language:"), string(ctx.lang))
	}

	if ctx.categoryAutoDetect {
		fmt.Printf("%s %s\n", warnStyle.Render("Auto-detected category:"), string(ctx.category))
	}

	fmt.Println()
	fmt.Printf("%s %s\n", labelStyle.Render("Adding project:"), ctx.name)
	fmt.Printf("%s %s\n", labelStyle.Render("Category:"), string(ctx.category))
	fmt.Printf("%s %s\n", labelStyle.Render("Language:"), string(ctx.lang))
	fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(ctx.expandedPath))
	fmt.Println()

	fmt.Printf("  Creating thoughts structure...")
	if err := fireProjectEvent(lifecycle.ProjectCreating, ctx.name, ctx.expandedPath, ctx.thoughtsDir); err != nil {
		return fmt.Errorf("pre-create hook failed: %w", err)
	}
	if warning, err := EnsureProjectThoughtsSymlink(ctx.thoughtsDir, ctx.name, ctx.expandedPath, false); err != nil {
		return err
	} else if warning != "" {
		fmt.Printf("\n  %s %s", warnStyle.Render("Warning:"), warning)
	}
	fmt.Println(" done")

	if err := runAddRegister(ctx, s); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("%s\n", successStyle.Render("Project added successfully!"))
	fmt.Println()
	fmt.Printf("%s %s\n", labelStyle.Render("Project path:"), pathStyle.Render(ctx.expandedPath))
	fmt.Printf("%s %s\n", labelStyle.Render("Thoughts:"), pathStyle.Render(filepath.Join(ctx.thoughtsDir, "projects", ctx.name)))
	if len(ctx.aliases) > 0 {
		fmt.Printf("%s %s\n", labelStyle.Render("Aliases:"), strings.Join(ctx.aliases, ", "))
	}
	fmt.Println()

	return nil
}

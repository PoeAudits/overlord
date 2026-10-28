package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/templates"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// newFlags holds the flags for the new command
type newFlags struct {
	category    string
	description string
	aliases     []string
	noGit       bool
	noOpen      bool
	// Language shorthand flags
	langPy  bool
	langTs  bool
	langGo  bool
	langSol bool
}

var newOpts newFlags

var (
	newLoadRegistry = loadRegistry
	newSaveRegistry = saveRegistry
)

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new project with full setup",
	Long: `Create a new project with directory structure, git initialization,
templates, and registry entry.

The project will be created at ~/Overlord/{category-path}/{name}/ with:
  - .tmux.local (tmux session configuration)
  - Makefile (language-specific build commands)
  - README.md (project documentation)
  - AGENTS.md (AI agent instructions)
  - opencode.jsonc (editor configuration)

Examples:
  overlord new my-api --category=services --go
  overlord new my-app --category=web --ts --description="Web application"
  overlord new my-lib --category=libs --py --no-git
  overlord new sandbox-test --category=sandbox --no-open
  overlord new my-project --category=cli --go --alias=mp`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

// formatNewUsage returns a helpful usage string for the new command
func formatNewUsage() string {
	return fmt.Sprintf(`Usage: overlord new <name> --category=<category> [--lang-flag] [options]

Categories (required):
  %s

Language flags (optional, defaults to base):
  --py    Python
  --ts    TypeScript
  --go    Go
  --sol   Solidity

Options:
  --description="..."   Project description
  --alias=<name>        Add alias (can be repeated)
  --no-git              Skip git initialization
  --no-open             Don't open workspace after creation

Examples:
  overlord new my-api --category=services --go
  overlord new my-app --category=web --ts --description="Web app"
  overlord new my-project --category=cli --go --alias=mp
  overlord new sandbox-test --category=sandbox`, strings.Join(validCategoryNames(), ", "))
}

func init() {
	rootCmd.AddCommand(newCmd)

	// Category flag - validation done in PreRunE for better error messages
	newCmd.Flags().StringVar(&newOpts.category, "category", "", "Project category (required)")

	// Description flag
	newCmd.Flags().StringVar(&newOpts.description, "description", "", "Project description")

	// Alias flag (can be repeated)
	newCmd.Flags().StringArrayVar(&newOpts.aliases, "alias", []string{}, "Add alias (can be repeated)")

	// Language shorthand flags
	newCmd.Flags().BoolVar(&newOpts.langPy, "py", false, "Use Python language")
	newCmd.Flags().BoolVar(&newOpts.langTs, "ts", false, "Use TypeScript language")
	newCmd.Flags().BoolVar(&newOpts.langGo, "go", false, "Use Go language")
	newCmd.Flags().BoolVar(&newOpts.langSol, "sol", false, "Use Solidity language")

	// Behavior flags
	newCmd.Flags().BoolVar(&newOpts.noGit, "no-git", false, "Skip git initialization")
	newCmd.Flags().BoolVar(&newOpts.noOpen, "no-open", false, "Don't open workspace after creation")

	// Add PreRunE for category validation with helpful error
	newCmd.PreRunE = validateNewFlags
}

// nameRegex validates project name characters (length checked separately via maxProjectNameLen)
var nameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// validateNewFlags validates required flags and provides helpful error messages
func validateNewFlags(cmd *cobra.Command, args []string) error {
	if newOpts.category == "" {
		return cmdError("new", "--category is required", formatNewUsage())
	}

	// Validate category
	if _, err := validateCategory(newOpts.category); err != nil {
		return err
	}

	// Check that only one language flag is set
	if countTrue(newOpts.langPy, newOpts.langTs, newOpts.langGo, newOpts.langSol) > 1 {
		return cmdError("new", "only one language flag can be set", "Choose one of: --py, --ts, --go, --sol")
	}

	return nil
}

// validateName checks if the project name is valid
func validateName(name string) error {
	if name == "" {
		return cmdError("new", "project name cannot be empty", "Usage: overlord new <name> --category=<category>")
	}

	if len(name) > maxProjectNameLen {
		return cmdError("new", fmt.Sprintf("project name too long (max %d characters)", maxProjectNameLen), "")
	}

	if !nameRegex.MatchString(name) {
		return cmdError("new", fmt.Sprintf("invalid project name '%s'", name), "Must start with letter or number, contain only alphanumeric, dash, or underscore")
	}

	return nil
}

// cleanup removes created directories on failure
type cleanup struct {
	paths   []string
	success bool
}

func (c *cleanup) add(path string) {
	c.paths = append(c.paths, path)
}

func (c *cleanup) run() {
	if c.success {
		return
	}

	// Remove in reverse order (deepest first)
	for i := len(c.paths) - 1; i >= 0; i-- {
		if err := os.RemoveAll(c.paths[i]); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to clean up %s: %v\n", c.paths[i], err)
		}
	}
}

// newProjectContext bundles resolved parameters for the new project workflow.
type newProjectContext struct {
	name         string
	category     registry.Category
	categoryPath string
	lang         registry.Language
	description  string
	projectPath  string
	reg          *registry.Registry
}

func runNew(cmd *cobra.Command, args []string) error {
	s := NewStyles()

	// Validate inputs and resolve paths
	ctx, err := runNewValidate(args[0])
	if err != nil {
		return err
	}

	// Setup cleanup on failure
	c := &cleanup{}
	defer c.run()

	fmt.Printf("%s %s\n", s.Label.Render("Creating project:"), ctx.name)
	fmt.Printf("%s %s\n", s.Label.Render("Category:"), string(ctx.category))
	fmt.Printf("%s %s\n", s.Label.Render("Language:"), string(ctx.lang))
	fmt.Printf("%s %s\n", s.Label.Render("Path:"), s.Path.Render(ctx.projectPath))
	fmt.Println()

	// Create project directory structure
	if err := runNewDirectories(ctx.projectPath, c); err != nil {
		return err
	}

	// Render and write templates
	if err := runNewTemplates(ctx, s); err != nil {
		return err
	}

	// Initialize git repository
	runNewGitInit(ctx.projectPath, s)

	// Register project in registry and fire lifecycle hooks
	if err := runNewRegister(ctx); err != nil {
		return err
	}

	// Mark success (prevents cleanup)
	c.success = true

	// Print summary
	fmt.Println()
	fmt.Printf("%s\n", s.Success.Render("Project created successfully!"))
	fmt.Println()
	fmt.Printf("%s %s\n", s.Label.Render("Project path:"), s.Path.Render(ctx.projectPath))
	if len(newOpts.aliases) > 0 {
		fmt.Printf("%s %s\n", s.Label.Render("Aliases:"), strings.Join(newOpts.aliases, ", "))
	}
	fmt.Println()

	// Open workspace (tmux session)
	runNewWorkspace(ctx.name, ctx.projectPath, s)

	return nil
}

// runNewValidate validates inputs and resolves all paths needed by the new project workflow.
func runNewValidate(name string) (*newProjectContext, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}

	category := registry.Category(newOpts.category)
	lang, set := languageFromFlags(newOpts.langPy, newOpts.langTs, newOpts.langGo, newOpts.langSol, false)
	if !set {
		lang = registry.LanguageBase
	}

	reg, err := newLoadRegistry()
	if err != nil {
		return nil, err
	}

	if _, exists := reg.ProjectByName(name); exists {
		return nil, fmt.Errorf("project '%s' already exists\nUse 'overlord info %s' to see details", name, name)
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return nil, err
	}

	categoryPath := category.Path()
	projectPath := filepath.Join(baseDir, categoryPath, name)

	if _, err := os.Stat(projectPath); err == nil {
		return nil, fmt.Errorf("directory already exists\nLocation: %s\nUse 'overlord add' to register an existing project", projectPath)
	}

	description := projectDescription(name, newOpts.description)

	return &newProjectContext{
		name:         name,
		category:     category,
		categoryPath: categoryPath,
		lang:         lang,
		description:  description,
		projectPath:  projectPath,
		reg:          reg,
	}, nil
}

// runNewDirectories creates the project directory structure.
func runNewDirectories(projectPath string, c *cleanup) error {
	fmt.Printf("  Creating directory structure...")

	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	c.add(projectPath)

	fmt.Println(" done")
	return nil
}

// runNewTemplates renders and writes project templates.
func runNewTemplates(ctx *newProjectContext, s styles) error {
	fmt.Printf("  Writing templates...")

	templateData := templates.NewTemplateData(
		ctx.name,
		ctx.description,
		string(ctx.category),
		string(ctx.lang),
	)

	allTemplates, err := templates.GetAllTemplates(templates.Language(ctx.lang), templateData)
	if err != nil {
		return fmt.Errorf("failed to render templates: %w", err)
	}

	for templateType, content := range allTemplates {
		outputPath := filepath.Join(ctx.projectPath, templateType.OutputPath())

		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", templateType, err)
		}

		if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", templateType, err)
		}
	}
	fmt.Println(" done")

	return nil
}

// runNewGitInit initializes a git repository in the project directory.
// Git failures are treated as warnings rather than fatal errors.
func runNewGitInit(projectPath string, s styles) {
	if newOpts.noGit {
		fmt.Println("  Skipping git initialization (--no-git)")
		return
	}

	fmt.Printf("  Initializing git repository...")

	if _, err := exec.LookPath("git"); err != nil {
		fmt.Println()
		fmt.Printf("  %s git not found, skipping initialization\n", s.Warn.Render("Warning:"))
		return
	}

	gitCommands := []struct {
		args []string
		name string
	}{
		{[]string{"init"}, "git init"},
		{[]string{"add", "."}, "git add"},
		{[]string{"commit", "-m", "Initial commit"}, "git commit"},
	}

	for _, gitCmd := range gitCommands {
		cmd := exec.Command("git", gitCmd.args...)
		cmd.Dir = projectPath
		if output, err := cmd.CombinedOutput(); err != nil {
			fmt.Println()
			fmt.Printf("  %s %s failed: %s\n", s.Warn.Render("Warning:"), gitCmd.name, strings.TrimSpace(string(output)))
			return
		}
	}

	fmt.Println(" done")
}

// runNewRegister adds the project to the registry and fires lifecycle hooks.
func runNewRegister(ctx *newProjectContext) error {
	fmt.Printf("  Registering project...")

	relativePath := filepath.Join(ctx.categoryPath, ctx.name)

	project := registry.Project{
		Path:        relativePath,
		Category:    ctx.category,
		Lang:        ctx.lang,
		Description: ctx.description,
		Aliases:     newOpts.aliases,
	}

	registerProject(ctx.reg, ctx.name, project)

	if err := newSaveRegistry(ctx.reg); err != nil {
		return err
	}

	postCreateResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectCreated,
		ProjectName: ctx.name,
		ProjectPath: ctx.projectPath,
		Settings:    map[string]string{},
	})
	printLifecycleWarnings(postCreateResult.Warnings)

	fmt.Println(" done")
	return nil
}

// runNewWorkspace opens the project in a tmux session.
// Tmux failures are treated as warnings rather than fatal errors.
func runNewWorkspace(name, projectPath string, s styles) {
	if newOpts.noOpen {
		fmt.Printf("To open workspace: overlord open %s\n", name)
		return
	}

	fmt.Printf("Opening workspace...\n")

	if _, err := exec.LookPath("tmux"); err != nil {
		fmt.Printf("%s tmux not found, cannot open workspace\n", s.Warn.Render("Warning:"))
		fmt.Printf("To open manually: cd %s\n", projectPath)
		return
	}

	session := tmux.NewSession(name, projectPath)

	exists, err := session.Exists()
	if err != nil {
		fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
		return
	}

	if !exists {
		if err := session.Create(); err != nil {
			fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
			return
		}

		if err := session.SourceLocal(); err != nil {
			fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
		}
	}

	if err := session.AttachOrSwitch(); err != nil {
		fmt.Printf("%s %v\n", s.Warn.Render("Warning:"), err)
		fmt.Printf("Attach manually: tmux attach -t %s\n", name)
	}
}

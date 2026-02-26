package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

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

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new project with full setup",
	Long: `Create a new project with directory structure, git initialization,
templates, thoughts integration, and registry entry.

The project will be created at ~/Overlord/{category-path}/{name}/ with:
  - .tmux.local (tmux session configuration)
  - Makefile (language-specific build commands)
  - README.md (project documentation)
  - AGENTS.md (AI agent instructions)
  - opencode.jsonc (editor configuration)

Thoughts directories will be created at:
  - ~/thoughts/projects/{name}/{briefs,research,audits,plans,logs,reviews,docs}/
  - Symlinks to README.md and AGENTS.md placed in docs/ subdirectory
  - Project root thoughts symlink -> ~/thoughts/projects/{name}

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
	validCats := make([]string, 0, len(registry.ValidCategories()))
	for _, c := range registry.ValidCategories() {
		validCats = append(validCats, string(c))
	}

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
  overlord new sandbox-test --category=sandbox`, strings.Join(validCats, ", "))
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
		return fmt.Errorf("--category is required\n\n%s", formatNewUsage())
	}

	// Validate category
	if _, err := validateCategory(newOpts.category); err != nil {
		return err
	}

	// Check that only one language flag is set
	if countTrue(newOpts.langPy, newOpts.langTs, newOpts.langGo, newOpts.langSol) > 1 {
		return fmt.Errorf("only one language flag can be set (--py, --ts, --go, or --sol)")
	}

	return nil
}

// validateName checks if the project name is valid
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if len(name) > maxProjectNameLen {
		return fmt.Errorf("project name too long (max %d characters)", maxProjectNameLen)
	}

	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid project name: must start with letter or number, contain only alphanumeric, dash, or underscore")
	}

	return nil
}

// selectedLanguage determines the language from flags.
func selectedLanguage() registry.Language {
	lang, set := languageFromFlags(newOpts.langPy, newOpts.langTs, newOpts.langGo, newOpts.langSol, false)
	if !set {
		return registry.LanguageBase
	}
	return lang
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
	thoughtsDir  string
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

	// Render and write templates + .env.pass
	if err := runNewTemplates(ctx, s); err != nil {
		return err
	}

	// Create thoughts directories and symlink
	if err := runNewThoughts(ctx, c, s); err != nil {
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
	thoughtsProjectBase := filepath.Join(ctx.thoughtsDir, "projects", ctx.name)
	fmt.Println()
	fmt.Printf("%s\n", s.Success.Render("Project created successfully!"))
	fmt.Println()
	fmt.Printf("%s %s\n", s.Label.Render("Project path:"), s.Path.Render(ctx.projectPath))
	fmt.Printf("%s %s\n", s.Label.Render("Thoughts:"), s.Path.Render(thoughtsProjectBase))
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
	lang := selectedLanguage()

	reg, err := loadRegistry()
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

	thoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	categoryPath := category.Path()
	projectPath := filepath.Join(baseDir, categoryPath, name)

	if _, err := os.Stat(projectPath); err == nil {
		return nil, fmt.Errorf("directory already exists\nLocation: %s\nUse 'overlord add' to register an existing project", projectPath)
	}

	description := newOpts.description
	if description == "" {
		description = fmt.Sprintf("%s project", name)
	}

	return &newProjectContext{
		name:         name,
		category:     category,
		categoryPath: categoryPath,
		lang:         lang,
		description:  description,
		projectPath:  projectPath,
		thoughtsDir:  thoughtsDir,
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

// runNewTemplates renders and writes project templates, then generates .env.pass.
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

	// Generate .env.pass template from pass store (best-effort)
	fmt.Printf("  Generating .env.pass template...")
	envCount, envErr := GenerateEnvPassFile(ctx.projectPath)
	if envErr != nil {
		fmt.Println()
		fmt.Printf("  %s %s\n", s.Warn.Render("Warning:"), envErr)
	} else if envCount == 0 {
		fmt.Println(" skipped (no pass entries)")
	} else {
		fmt.Printf(" done (%d entries)\n", envCount)
	}

	return nil
}

// runNewThoughts creates thoughts directories and the project-root thoughts symlink.
func runNewThoughts(ctx *newProjectContext, c *cleanup, s styles) error {
	fmt.Printf("  Creating thoughts structure...")

	thoughtsProjectBase := filepath.Join(ctx.thoughtsDir, "projects", ctx.name)
	c.add(thoughtsProjectBase)

	preCreateResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectCreating,
		ProjectName: ctx.name,
		ProjectPath: ctx.projectPath,
		Settings: map[string]string{
			"thoughts_dir": ctx.thoughtsDir,
		},
	})
	if preCreateResult.Err != nil {
		return fmt.Errorf("pre-create hook failed: %w", preCreateResult.Err)
	}

	if warning, err := EnsureProjectThoughtsSymlink(ctx.thoughtsDir, ctx.name, ctx.projectPath, false); err != nil {
		return err
	} else if warning != "" {
		fmt.Printf("\n  %s %s", s.Warn.Render("Warning:"), warning)
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
		Created:     time.Now().Format("2006-01-02"),
		Description: ctx.description,
		Aliases:     newOpts.aliases,
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
		ProjectPath: ctx.projectPath,
		Settings: map[string]string{
			"thoughts_dir": ctx.thoughtsDir,
		},
	})
	for _, warning := range postCreateResult.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

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

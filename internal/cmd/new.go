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

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/templates"
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
  - .opencode/opencode.jsonc (editor configuration)

Thoughts directories will be created at:
  - ~/thoughts/projects/{name}/{plans,logs,docs,research,sessions,handoffs,reviews,briefs}/
  - Symlinks to README.md and AGENTS.md placed in docs/ subdirectory

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
	category := registry.Category(newOpts.category)
	if !category.IsValid() {
		validCats := make([]string, 0, len(registry.ValidCategories()))
		for _, c := range registry.ValidCategories() {
			validCats = append(validCats, string(c))
		}
		return fmt.Errorf("invalid category '%s'\n\nValid categories: %s", newOpts.category, strings.Join(validCats, ", "))
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

// getLanguage determines the language from flags
func getLanguage() registry.Language {
	switch {
	case newOpts.langPy:
		return registry.LanguagePython
	case newOpts.langTs:
		return registry.LanguageTypeScript
	case newOpts.langGo:
		return registry.LanguageGo
	case newOpts.langSol:
		return registry.LanguageSolidity
	default:
		return registry.LanguageBase
	}
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
		os.RemoveAll(c.paths[i])
	}
}

func runNew(cmd *cobra.Command, args []string) error {
	name := args[0]

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

	// Category already validated in PreRunE
	category := registry.Category(newOpts.category)

	// Get language
	lang := getLanguage()

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Check if project already exists
	if _, exists := reg.GetProject(name); exists {
		return fmt.Errorf("Error: project '%s' already exists\nUse 'overlord info %s' to see details", name, name)
	}

	// Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Expand thoughts directory
	thoughtsDir, err := registry.ExpandPath(reg.Settings.ThoughtsDir)
	if err != nil {
		return fmt.Errorf("failed to expand thoughts directory: %w", err)
	}

	// Build project path
	categoryPath := category.Path()
	projectPath := filepath.Join(baseDir, categoryPath, name)

	// Check if directory already exists
	if _, err := os.Stat(projectPath); err == nil {
		return fmt.Errorf("Error: directory already exists\nLocation: %s\nUse 'overlord add' to register an existing project", projectPath)
	}

	// Setup cleanup on failure
	c := &cleanup{}
	defer c.run()

	fmt.Printf("%s %s\n", labelStyle.Render("Creating project:"), name)
	fmt.Printf("%s %s\n", labelStyle.Render("Category:"), string(category))
	fmt.Printf("%s %s\n", labelStyle.Render("Language:"), string(lang))
	fmt.Printf("%s %s\n", labelStyle.Render("Path:"), pathStyle.Render(projectPath))
	fmt.Println()

	// 1. Create project directory
	fmt.Printf("  Creating directory structure...")
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}
	c.add(projectPath)

	// Create .opencode directory
	opencodePath := filepath.Join(projectPath, ".opencode")
	if err := os.MkdirAll(opencodePath, 0755); err != nil {
		return fmt.Errorf("failed to create .opencode directory: %w", err)
	}
	fmt.Println(" done")

	// 2. Render and write templates
	fmt.Printf("  Writing templates...")

	// Set description default if not provided
	description := newOpts.description
	if description == "" {
		description = fmt.Sprintf("%s project", name)
	}

	// Create template data
	templateData := templates.NewTemplateData(
		name,
		description,
		string(category),
		string(lang),
	)

	// Map registry language to template language
	templateLang := templates.Language(lang)

	// Get all templates
	allTemplates, err := templates.GetAllTemplates(templateLang, templateData)
	if err != nil {
		return fmt.Errorf("failed to render templates: %w", err)
	}

	// Write each template
	for templateType, content := range allTemplates {
		outputPath := filepath.Join(projectPath, templateType.OutputPath())

		// Ensure parent directory exists (for .opencode/opencode.jsonc)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", templateType, err)
		}

		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", templateType, err)
		}
	}
	fmt.Println(" done")

	// 2b. Generate .env.pass template from pass store (best-effort)
	fmt.Printf("  Generating .env.pass template...")
	envCount, envErr := GenerateEnvPassFile(projectPath)
	if envErr != nil {
		fmt.Println()
		fmt.Printf("  %s %s\n", warnStyle.Render("Warning:"), envErr)
	} else if envCount == 0 {
		fmt.Println(" skipped (no pass entries)")
	} else {
		fmt.Printf(" done (%d entries)\n", envCount)
	}

	// 3. Create thoughts directories (all 8 subdirectories under ~/thoughts/projects/{name}/)
	fmt.Printf("  Creating thoughts structure...")

	// Get paths for all 8 thoughts subdirectories using the helper function
	thoughtsPaths := GetThoughtsPaths(thoughtsDir, name)
	thoughtsProjectBase := filepath.Join(thoughtsDir, "projects", name)

	// Create all 8 thoughts subdirectories
	for _, dir := range thoughtsPaths.toSlice() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create thoughts directory %s: %w", dir, err)
		}
		c.add(dir)
	}
	// Also track the project base directory for cleanup
	c.add(thoughtsProjectBase)

	// Create relative symlinks in docs/ subdirectory
	// Calculate relative path from thoughts/projects/{name}/docs/ to project directory
	readmeSrc := filepath.Join(projectPath, "README.md")
	agentsSrc := filepath.Join(projectPath, "AGENTS.md")

	readmeLink := filepath.Join(thoughtsPaths.Docs, "README.md")
	agentsLink := filepath.Join(thoughtsPaths.Docs, "AGENTS.md")

	// Calculate relative path from docs/ subdirectory to project
	relPath, err := filepath.Rel(thoughtsPaths.Docs, projectPath)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Create symlinks using relative paths
	readmeRelTarget := filepath.Join(relPath, "README.md")
	agentsRelTarget := filepath.Join(relPath, "AGENTS.md")

	if err := os.Symlink(readmeRelTarget, readmeLink); err != nil {
		return fmt.Errorf("failed to create README.md symlink: %w", err)
	}

	if err := os.Symlink(agentsRelTarget, agentsLink); err != nil {
		return fmt.Errorf("failed to create AGENTS.md symlink: %w", err)
	}

	// Verify symlinks point to existing files
	if _, err := os.Stat(readmeSrc); err != nil {
		return fmt.Errorf("README.md not found at %s", readmeSrc)
	}
	if _, err := os.Stat(agentsSrc); err != nil {
		return fmt.Errorf("AGENTS.md not found at %s", agentsSrc)
	}

	fmt.Println(" done")

	// 4. Initialize git repository
	if !newOpts.noGit {
		fmt.Printf("  Initializing git repository...")

		// Check if git is available
		if _, err := exec.LookPath("git"); err != nil {
			fmt.Println()
			fmt.Printf("  %s git not found, skipping initialization\n", warnStyle.Render("Warning:"))
		} else {
			// Run git commands sequentially, break on error
			gitCommands := []struct {
				args []string
				name string
			}{
				{[]string{"init"}, "git init"},
				{[]string{"add", "."}, "git add"},
				{[]string{"commit", "-m", "Initial commit"}, "git commit"},
			}

			success := true
			for _, gitCmd := range gitCommands {
				cmd := exec.Command("git", gitCmd.args...)
				cmd.Dir = projectPath
				if output, err := cmd.CombinedOutput(); err != nil {
					fmt.Println()
					fmt.Printf("  %s %s failed: %s\n", warnStyle.Render("Warning:"), gitCmd.name, strings.TrimSpace(string(output)))
					success = false
					break
				}
			}

			if success {
				fmt.Println(" done")
			}
		}
	} else {
		fmt.Println("  Skipping git initialization (--no-git)")
	}

	// 5. Register project in registry
	fmt.Printf("  Registering project...")

	// Calculate relative path from base_dir
	relativePath := filepath.Join(categoryPath, name)

	project := registry.Project{
		Path:        relativePath,
		Category:    category,
		Lang:        lang,
		Created:     time.Now().Format("2006-01-02"),
		Description: description,
		Aliases:     newOpts.aliases,
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

	// Mark success (prevents cleanup)
	c.success = true

	fmt.Println()
	fmt.Printf("%s\n", successStyle.Render("Project created successfully!"))
	fmt.Println()
	fmt.Printf("%s %s\n", labelStyle.Render("Project path:"), pathStyle.Render(projectPath))
	fmt.Printf("%s %s\n", labelStyle.Render("Thoughts:"), pathStyle.Render(thoughtsProjectBase))
	if len(newOpts.aliases) > 0 {
		fmt.Printf("%s %s\n", labelStyle.Render("Aliases:"), strings.Join(newOpts.aliases, ", "))
	}
	fmt.Println()

	// 6. Open workspace (unless --no-open)
	if !newOpts.noOpen {
		fmt.Printf("Opening workspace...\n")

		// Check if tmux is available
		if _, err := exec.LookPath("tmux"); err != nil {
			fmt.Printf("%s tmux not found, cannot open workspace\n", warnStyle.Render("Warning:"))
			fmt.Printf("To open manually: cd %s\n", projectPath)
			return nil
		}

		// Create and configure tmux session
		session := &tmuxSession{
			Name: name,
			Dir:  projectPath,
		}

		// Create session (skip if already exists)
		if !session.exists() {
			if err := session.create(); err != nil {
				fmt.Printf("%s %v\n", warnStyle.Render("Warning:"), err)
				return nil
			}

			// Source .tmux.local
			if err := session.sourceLocal(); err != nil {
				fmt.Printf("%s %v\n", warnStyle.Render("Warning:"), err)
			}
		}

		// Attach or switch to session
		if err := session.attachOrSwitch(); err != nil {
			fmt.Printf("%s %v\n", warnStyle.Render("Warning:"), err)
			fmt.Printf("Attach manually: tmux attach -t %s\n", name)
		}
	} else {
		fmt.Printf("To open workspace: overlord open %s\n", name)
	}

	return nil
}

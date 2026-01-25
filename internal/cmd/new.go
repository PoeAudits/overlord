package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/templates"
)

// newFlags holds the flags for the new command
type newFlags struct {
	category    string
	description string
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
  - ~/thoughts/projects/{name}/ (with symlinks to README.md and AGENTS.md)
  - ~/thoughts/plans/{name}/
  - ~/thoughts/logs/{name}/
  - ~/thoughts/sessions/{name}/

Examples:
  overlord new my-api --category=services --go
  overlord new my-app --category=web --ts --description="Web application"
  overlord new my-lib --category=libs --py --no-git
  overlord new sandbox-test --category=sandbox --no-open`,
	Args: cobra.ExactArgs(1),
	RunE: runNew,
}

func init() {
	rootCmd.AddCommand(newCmd)

	// Category flag
	newCmd.Flags().StringVar(&newOpts.category, "category", "", "Project category (required)")
	newCmd.MarkFlagRequired("category")

	// Description flag
	newCmd.Flags().StringVar(&newOpts.description, "description", "", "Project description")

	// Language shorthand flags
	newCmd.Flags().BoolVar(&newOpts.langPy, "py", false, "Use Python language")
	newCmd.Flags().BoolVar(&newOpts.langTs, "ts", false, "Use TypeScript language")
	newCmd.Flags().BoolVar(&newOpts.langGo, "go", false, "Use Go language")
	newCmd.Flags().BoolVar(&newOpts.langSol, "sol", false, "Use Solidity language")

	// Behavior flags
	newCmd.Flags().BoolVar(&newOpts.noGit, "no-git", false, "Skip git initialization")
	newCmd.Flags().BoolVar(&newOpts.noOpen, "no-open", false, "Don't open workspace after creation")
}

// nameRegex validates project names
var nameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// validateName checks if the project name is valid
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}

	if len(name) > 64 {
		return fmt.Errorf("project name too long (max 64 characters)")
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

// expandPath expands ~ to the user's home directory
func expandPath(path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("path cannot be empty")
	}

	if path[0] != '~' {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	if len(path) == 1 {
		return homeDir, nil
	}

	if path[1] == '/' || path[1] == filepath.Separator {
		return filepath.Join(homeDir, path[2:]), nil
	}

	return "", fmt.Errorf("~user expansion not supported, use absolute path")
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

	// Validate category
	category := registry.Category(newOpts.category)
	if !category.IsValid() {
		validCats := make([]string, 0, len(registry.ValidCategories()))
		for _, c := range registry.ValidCategories() {
			validCats = append(validCats, string(c))
		}
		return fmt.Errorf("invalid category '%s'\nValid categories: %s", newOpts.category, strings.Join(validCats, ", "))
	}

	// Get language
	lang := getLanguage()

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

	// 3. Create thoughts directories
	fmt.Printf("  Creating thoughts structure...")

	thoughtsProjectDir := filepath.Join(thoughtsDir, "projects", name)
	thoughtsPlansDir := filepath.Join(thoughtsDir, "plans", name)
	thoughtsLogsDir := filepath.Join(thoughtsDir, "logs", name)
	thoughtsSessionsDir := filepath.Join(thoughtsDir, "sessions", name)

	// Create all thoughts directories
	for _, dir := range []string{thoughtsProjectDir, thoughtsPlansDir, thoughtsLogsDir, thoughtsSessionsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create thoughts directory %s: %w", dir, err)
		}
		c.add(dir)
	}

	// Create relative symlinks in projects directory
	// Calculate relative path from thoughts/projects/{name}/ to project directory
	readmeSrc := filepath.Join(projectPath, "README.md")
	agentsSrc := filepath.Join(projectPath, "AGENTS.md")

	readmeLink := filepath.Join(thoughtsProjectDir, "README.md")
	agentsLink := filepath.Join(thoughtsProjectDir, "AGENTS.md")

	// Calculate relative path
	relPath, err := filepath.Rel(thoughtsProjectDir, projectPath)
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
			// git init
			gitInit := exec.Command("git", "init")
			gitInit.Dir = projectPath
			if output, err := gitInit.CombinedOutput(); err != nil {
				fmt.Println()
				fmt.Printf("  %s git init failed: %s\n", warnStyle.Render("Warning:"), strings.TrimSpace(string(output)))
			} else {
				// git add .
				gitAdd := exec.Command("git", "add", ".")
				gitAdd.Dir = projectPath
				if output, err := gitAdd.CombinedOutput(); err != nil {
					fmt.Println()
					fmt.Printf("  %s git add failed: %s\n", warnStyle.Render("Warning:"), strings.TrimSpace(string(output)))
				} else {
					// git commit
					gitCommit := exec.Command("git", "commit", "-m", "Initial commit")
					gitCommit.Dir = projectPath
					if output, err := gitCommit.CombinedOutput(); err != nil {
						fmt.Println()
						fmt.Printf("  %s git commit failed: %s\n", warnStyle.Render("Warning:"), strings.TrimSpace(string(output)))
					} else {
						fmt.Println(" done")
						_ = output // suppress unused warning
					}
				}
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
		Aliases:     []string{},
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

	// Mark success (prevents cleanup)
	c.success = true

	fmt.Println()
	fmt.Printf("%s\n", successStyle.Render("Project created successfully!"))
	fmt.Println()
	fmt.Printf("%s %s\n", labelStyle.Render("Project path:"), pathStyle.Render(projectPath))
	fmt.Printf("%s %s\n", labelStyle.Render("Thoughts:"), pathStyle.Render(thoughtsProjectDir))
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

		// Create tmux session
		tmuxNew := exec.Command("tmux", "new-session", "-d", "-s", name, "-c", projectPath)
		if output, err := tmuxNew.CombinedOutput(); err != nil {
			// Session might already exist
			if !strings.Contains(string(output), "duplicate session") {
				fmt.Printf("%s failed to create tmux session: %s\n", warnStyle.Render("Warning:"), strings.TrimSpace(string(output)))
				return nil
			}
		}

		// Source .tmux.local
		tmuxLocalPath := filepath.Join(projectPath, ".tmux.local")
		if _, err := os.Stat(tmuxLocalPath); err == nil {
			// Run .tmux.local as a bash script with environment variables
			tmuxSource := exec.Command("bash", tmuxLocalPath)
			tmuxSource.Env = append(os.Environ(),
				fmt.Sprintf("TMUX_SESSION=%s", name),
				fmt.Sprintf("TMUX_PROJECT_DIR=%s", projectPath),
			)
			if output, err := tmuxSource.CombinedOutput(); err != nil {
				fmt.Printf("%s failed to source .tmux.local: %s\n", warnStyle.Render("Warning:"), strings.TrimSpace(string(output)))
			}
		}

		// Attach to session (or switch if already in tmux)
		if os.Getenv("TMUX") != "" {
			// Already in tmux, switch client
			tmuxSwitch := exec.Command("tmux", "switch-client", "-t", name)
			tmuxSwitch.Stdin = os.Stdin
			tmuxSwitch.Stdout = os.Stdout
			tmuxSwitch.Stderr = os.Stderr
			if err := tmuxSwitch.Run(); err != nil {
				fmt.Printf("%s failed to switch to session: %v\n", warnStyle.Render("Warning:"), err)
				fmt.Printf("Attach manually: tmux attach -t %s\n", name)
			}
		} else {
			// Not in tmux, attach
			tmuxAttach := exec.Command("tmux", "attach", "-t", name)
			tmuxAttach.Stdin = os.Stdin
			tmuxAttach.Stdout = os.Stdout
			tmuxAttach.Stderr = os.Stderr
			if err := tmuxAttach.Run(); err != nil {
				fmt.Printf("%s failed to attach to session: %v\n", warnStyle.Render("Warning:"), err)
				fmt.Printf("Attach manually: tmux attach -t %s\n", name)
			}
		}
	} else {
		fmt.Printf("To open workspace: overlord open %s\n", name)
	}

	return nil
}

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
)

const envPassFile = ".env.pass"

// envProjectInfo holds resolved project information for env commands.
type envProjectInfo struct {
	Name        string
	Project     registry.Project
	ProjectRoot string
	MatchInfo   string // how the match was made (for display)
}

// resolveEnvProject resolves a project from args or current working directory.
// If args is non-empty, uses fuzzy matching. Otherwise, detects from cwd.
func resolveEnvProject(args []string, reg *registry.Registry, baseDir, cwdUsage string) (*envProjectInfo, error) {
	info := &envProjectInfo{}

	if len(args) > 0 {
		// Explicit project name — resolve via fuzzy matching
		result, err := reg.ResolveOne(args[0])
		if err != nil {
			return nil, cmdError("env", fmt.Sprintf("project '%s' not found in registry", args[0]), "Run 'overlord list' to see all projects")
		}
		info.Name = result.Name
		info.Project = result.Project

		if result.MatchedOn != "name" {
			info.MatchInfo = result.MatchedOn
			if strings.HasPrefix(info.MatchInfo, "alias:") {
				info.MatchInfo = fmt.Sprintf("alias '%s'", strings.TrimPrefix(info.MatchInfo, "alias:"))
			}
		}
	} else {
		// No args — auto-detect project from current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}

		name, proj, found := reg.FindProjectByPath(cwd, baseDir)
		if !found {
			return nil, fmt.Errorf("not inside a project directory\n%s", cwdUsage)
		}
		info.Name = name
		info.Project = proj
	}

	// Derive and verify project root
	info.ProjectRoot = fileutil.ResolveProjectPath(info.Project.Path, baseDir)
	if _, err := os.Stat(info.ProjectRoot); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("project directory does not exist: %s", info.ProjectRoot)
	}

	return info, nil
}

// envEntry represents a single entry in an .env.pass template file.
type envEntry struct {
	Key       string // environment variable name
	PassEntry string // pass store entry path (non-empty if pass reference)
	Value     string // literal value (non-empty if literal)
	Line      int    // line number in template (for error reporting)
}

// envCmd represents the env command group
var envCmd = &cobra.Command{
	Use:   "env [project]",
	Short: "Manage project environment files",
	Long: `Manage project environment files using pass (password-store).

Reads a .env.pass template from the project root and composes the .env file
by resolving pass references and copying literal values.

Template format (.env.pass):
  # Secrets — fetched from pass store
  API_KEY=pass:ai/anthropic-api-key
  DB_URL=pass:myproject/db-url

  # Config — written as-is
  NODE_ENV=development
  PORT=3000

When run without arguments inside a project directory, the project is detected
automatically. You can also specify a project name explicitly.

Examples:
  overlord env                  # Auto-detect project, pull from .env.pass
  overlord env myproject        # Pull .env for a specific project
  overlord env pull myproject   # Same as above (explicit subcommand)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEnvPull,
}

// envPullCmd represents the env pull subcommand
var envPullCmd = &cobra.Command{
	Use:   "pull [project]",
	Short: "Pull .env file from pass into a project",
	Long: `Pull environment secrets from pass and write them to the project's .env file.

Reads a .env.pass template from the project root and composes the .env file
by resolving pass references and copying literal values.

Template format (.env.pass):
  # Secrets — fetched from pass store
  API_KEY=pass:ai/anthropic-api-key
  DB_URL=pass:myproject/db-url

  # Config — written as-is
  NODE_ENV=development
  PORT=3000

When run without arguments inside a project directory, the project is detected
automatically. You can also specify a project name explicitly.

The project name can be an exact name, alias, or partial match (fuzzy).
Secret content is never printed to the terminal.

The write is atomic: content is written to a temporary file, permissions are set
to 0600, then the file is renamed to .env. This prevents partial writes.

Examples:
  overlord env pull                # Auto-detect project from current directory
  overlord env pull myproject      # Pull .env for myproject
  overlord env pull mp             # Pull .env using alias or fuzzy match`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEnvPull,
}

// envGenerateCmd represents the env generate subcommand
var envGenerateCmd = &cobra.Command{
	Use:   "generate [project]",
	Short: "Generate a .env.pass template with all pass store entries",
	Long: `Generate a .env.pass template file in the project root containing all entries
from the pass store.

Each pass entry is converted to an environment variable name:
  ai/anthropic-api-key  →  AI_ANTHROPIC_API_KEY=pass:ai/anthropic-api-key
  hugging-face-token    →  HUGGING_FACE_TOKEN=pass:hugging-face-token

When run without arguments inside a project directory, the project is detected
automatically. You can also specify a project name explicitly.

If .env.pass already exists, the command will refuse to overwrite unless --force
is specified.

Examples:
  overlord env generate              # Auto-detect project, generate .env.pass
  overlord env generate myproject    # Generate for a specific project
  overlord env generate --force      # Overwrite existing .env.pass`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEnvGenerate,
}

var envGenerateForce bool

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envPullCmd)
	envCmd.AddCommand(envGenerateCmd)

	envGenerateCmd.Flags().BoolVar(&envGenerateForce, "force", false, "Overwrite existing .env.pass file")
}

// runEnvPull handles the env pull command logic.
// When called with an argument, resolves the project by name/alias/fuzzy match.
// When called without arguments, auto-detects the project from the current directory.
func runEnvPull(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	// Expand base directory
	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Resolve project from args or cwd
	info, err := resolveEnvProject(args, reg, baseDir, "specify a project name: overlord env pull <project>")
	if err != nil {
		return err
	}

	envPath := filepath.Join(info.ProjectRoot, ".env")
	envPassPath := filepath.Join(info.ProjectRoot, envPassFile)

	// Try .env.pass template first
	if _, err := os.Stat(envPassPath); err == nil {
		return pullFromTemplate(envPassPath, envPath, info.Name, info.MatchInfo)
	}

	// Fall back to blob approach (overlord/env/<project-name> in pass)
	return pullFromBlob(envPath, info.Name, info.MatchInfo)
}

// pullFromTemplate reads a .env.pass template, resolves pass references, and writes .env.
func pullFromTemplate(envPassPath, envPath, projectName, matchInfo string) error {
	entries, err := parseEnvPassFile(envPassPath)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return cmdError("env", fmt.Sprintf("%s is empty", envPassFile), "Add entries in the format: KEY=pass:pass-entry-name or KEY=literal-value")
	}

	content, passCount, err := resolveEnvEntries(entries)
	if err != nil {
		return err
	}

	if err := fileutil.AtomicWriteFile(envPath, content, 0o600); err != nil {
		return err
	}

	printEnvPullSuccess(envPath, projectName, matchInfo, "Template:", fmt.Sprintf("%s (%d vars, %d from pass)", envPassPath, len(entries), passCount))

	return nil
}

// pullFromBlob falls back to the old approach: fetch entire .env from a single pass entry.
func pullFromBlob(envPath, projectName, matchInfo string) error {
	passEntry := fmt.Sprintf("overlord/env/%s", projectName)

	passCmd := exec.Command("pass", "show", passEntry)
	output, err := passCmd.Output()
	if err != nil {
		// Check if pass is installed
		if _, lookErr := exec.LookPath("pass"); lookErr != nil {
			return cmdError("env", "'pass' is not installed or not in PATH", "Install pass: https://www.passwordstore.org/")
		}
		return cmdError("env", fmt.Sprintf("no %s file found and no pass entry at '%s'", envPassFile, passEntry),
			fmt.Sprintf("Create a %s file in the project root with: KEY=pass:pass-entry-name or KEY=literal-value", envPassFile))
	}

	if err := fileutil.AtomicWriteFile(envPath, output, 0o600); err != nil {
		return err
	}

	printEnvPullSuccess(envPath, projectName, matchInfo, "Source:", passEntry)

	return nil
}

// printEnvPullSuccess prints consistent success output for env pull operations.
func printEnvPullSuccess(envPath, projectName, matchInfo, sourceLabel, sourceValue string) {
	s := NewStyles()

	fmt.Printf("%s Pulled .env for '%s'\n", s.Success.Render("✓"), projectName)
	fmt.Printf("  %s %s\n", s.Label.Render(sourceLabel), sourceValue)
	fmt.Printf("  %s %s\n", s.Label.Render("Written to:"), envPath)

	if matchInfo != "" {
		fmt.Printf("  %s %s (matched via %s)\n", s.Label.Render("Project:"), projectName, matchInfo)
	}
}

// parseEnvPassFile parses a .env.pass template file.
//
// Format:
//   - Lines starting with # are comments (leading whitespace is trimmed)
//   - Blank lines are ignored
//   - KEY=pass:entry-name  → value is fetched from pass store
//   - KEY=literal-value    → value is written as-is
//   - Only the first = is used as separator (values may contain =)
func parseEnvPassFile(path string) ([]envEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	var entries []envEntry
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip blank lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first = only
		idx := strings.Index(line, "=")
		if idx < 0 {
			return nil, fmt.Errorf("invalid line %d in %s: missing '=' separator: %s",
				lineNum, filepath.Base(path), line)
		}

		key := strings.TrimSpace(line[:idx])
		value := line[idx+1:]

		if key == "" {
			return nil, fmt.Errorf("invalid line %d in %s: empty key", lineNum, filepath.Base(path))
		}

		entry := envEntry{
			Key:  key,
			Line: lineNum,
		}

		if strings.HasPrefix(value, "pass:") {
			entry.PassEntry = strings.TrimSpace(strings.TrimPrefix(value, "pass:"))
			if entry.PassEntry == "" {
				return nil, fmt.Errorf("invalid line %d in %s: empty pass entry for key %s",
					lineNum, filepath.Base(path), key)
			}
		} else {
			entry.Value = value
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filepath.Base(path), err)
	}

	return entries, nil
}

// resolveEnvEntries resolves pass references in entries and returns the composed
// .env file content. Returns the content, count of pass entries resolved, and any error.
func resolveEnvEntries(entries []envEntry) ([]byte, int, error) {
	// Check if any entries need pass
	hasPassEntries := false
	for _, e := range entries {
		if e.PassEntry != "" {
			hasPassEntries = true
			break
		}
	}

	// Only check for pass binary if we actually need it
	if hasPassEntries {
		if _, err := exec.LookPath("pass"); err != nil {
			return nil, 0, fmt.Errorf("'pass' is not installed or not in PATH\nInstall pass: https://www.passwordstore.org/")
		}
	}

	var buf strings.Builder
	passCount := 0

	for _, entry := range entries {
		if entry.PassEntry != "" {
			// Fetch from pass store
			cmd := exec.Command("pass", "show", entry.PassEntry)
			output, err := cmd.Output()
			if err != nil {
				return nil, 0, cmdError("env",
					fmt.Sprintf("failed to resolve pass entry '%s' (line %d of %s)", entry.PassEntry, entry.Line, envPassFile),
					fmt.Sprintf("Create it with: pass insert %s", entry.PassEntry))
			}
			// pass output may have trailing newline — trim it
			value := strings.TrimRight(string(output), "\n")
			fmt.Fprintf(&buf, "%s=%s\n", entry.Key, value)
			passCount++
		} else {
			// Literal value
			fmt.Fprintf(&buf, "%s=%s\n", entry.Key, entry.Value)
		}
	}

	return []byte(buf.String()), passCount, nil
}

// runEnvGenerate handles the env generate command logic.
// Scans the pass store for all entries, converts them to env var names,
// and writes a .env.pass template into the project root.
func runEnvGenerate(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	successStyle := s.Success
	labelStyle := s.Label

	// Load registry
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	baseDir, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Resolve project from args or cwd
	info, err := resolveEnvProject(args, reg, baseDir, "specify a project name: overlord env generate <project>")
	if err != nil {
		return err
	}

	envPassPath := filepath.Join(info.ProjectRoot, envPassFile)

	// Check if .env.pass already exists
	if _, err := os.Stat(envPassPath); err == nil && !envGenerateForce {
		return cmdError("env generate", fmt.Sprintf(".env.pass already exists at %s", envPassPath), "Use --force to overwrite")
	}

	// Scan pass store
	entries, err := listPassEntries()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return cmdError("env generate", "no entries found in pass store", "Add entries with: pass insert <entry-name>")
	}

	// Generate and write the template
	content := generateEnvPassContent(entries)

	if err := os.WriteFile(envPassPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", envPassFile, err)
	}

	fmt.Printf("%s Generated %s for '%s'\n", successStyle.Render("✓"), envPassFile, info.Name)
	fmt.Printf("  %s %s (%d entries from pass)\n",
		labelStyle.Render("Template:"), envPassPath, len(entries))
	fmt.Printf("  %s overlord env\n", labelStyle.Render("Pull with:"))

	return nil
}

// passStoreDir returns the password store directory.
// Uses $PASSWORD_STORE_DIR if set, otherwise defaults to ~/.password-store.
func passStoreDir() (string, error) {
	dir := os.Getenv("PASSWORD_STORE_DIR")
	if dir != "" {
		return dir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".password-store"), nil
}

// listPassEntries walks the pass store directory and returns all entry names.
// Entry names are the relative paths of .gpg files without the .gpg extension.
// Results are sorted alphabetically.
func listPassEntries() ([]string, error) {
	storeDir, err := passStoreDir()
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(storeDir); errors.Is(err, fs.ErrNotExist) {
		return nil, cmdError("env", fmt.Sprintf("pass store directory does not exist: %s", storeDir), "Initialize with: pass init <gpg-id>")
	}

	var entries []string

	err = filepath.WalkDir(storeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden subdirectories (e.g., .git) but not the root store dir itself
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != storeDir {
			return filepath.SkipDir
		}

		// Only process .gpg files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".gpg") {
			return nil
		}

		// Get relative path from store dir and strip .gpg suffix
		rel, err := filepath.Rel(storeDir, path)
		if err != nil {
			return err
		}
		entry := strings.TrimSuffix(rel, ".gpg")
		entries = append(entries, entry)

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan pass store: %w", err)
	}

	sort.Strings(entries)
	return entries, nil
}

// passEntryToEnvVar converts a pass entry name to an environment variable name.
// Replaces hyphens and slashes with underscores and uppercases.
//
//	ai/anthropic-api-key → AI_ANTHROPIC_API_KEY
//	folder/secret-name   → FOLDER_SECRET_NAME
func passEntryToEnvVar(entry string) string {
	s := strings.ReplaceAll(entry, "-", "_")
	s = strings.ReplaceAll(s, "/", "_")
	return strings.ToUpper(s)
}

// generateEnvPassContent builds the .env.pass template content from pass entries.
// Each entry becomes KEY=pass:entry-name with a header comment.
func generateEnvPassContent(entries []string) string {
	var buf strings.Builder

	buf.WriteString("# Environment variables — generated by overlord env generate\n")
	buf.WriteString("# Format: ENV_VAR=pass:pass-entry-name\n")
	buf.WriteString("#\n")
	buf.WriteString("# To pull secrets into .env: overlord env\n")
	buf.WriteString("# To add literal values:     KEY=value\n")
	buf.WriteString("\n")

	for _, entry := range entries {
		envVar := passEntryToEnvVar(entry)
		fmt.Fprintf(&buf, "%s=pass:%s\n", envVar, entry)
	}

	return buf.String()
}

// GenerateEnvPassFile generates a .env.pass template in the given directory.
// This is exported for use by the `new` command.
// Returns the number of entries written and any error.
func GenerateEnvPassFile(projectRoot string) (int, error) {
	entries, err := listPassEntries()
	if err != nil {
		return 0, err
	}

	if len(entries) == 0 {
		return 0, nil
	}

	content := generateEnvPassContent(entries)
	envPassPath := filepath.Join(projectRoot, envPassFile)

	if err := os.WriteFile(envPassPath, []byte(content), 0o644); err != nil {
		return 0, fmt.Errorf("failed to write %s: %w", envPassFile, err)
	}

	return len(entries), nil
}

# overlord-v2 - Project Management System

A Go-based CLI tool for managing Git worktrees, tmux sessions, and project registries.

## Architecture Overview

Overlord v2 is a layered CLI application built with Cobra, featuring fuzzy matching, interactive TUI, and embedded templates:

```
┌─────────────────────────────────────────────────────────┐
│      CLI Commands (Cobra)                               │  ← User interface
│      - Root (routing), List, Info, New, Add, Rm         │
│      - Open (with TUI picker), Archive, Unarchive       │
├─────────────────────────────────────────────────────────┤
│      Registry Operations                                │  ← Business logic
│      - Resolve (fuzzy matching)                         │
│      - Filter (category, tags, status)                  │
│      - Validate (all types)                             │
├─────────────────────────────────────────────────────────┤
│      Templates (Embedded)                               │  ← Project initialization
│      - Language-specific templates                      │
│      - Variable substitution                            │
├─────────────────────────────────────────────────────────┤
│      YAML Store (Load/Save)                             │  ← Data persistence
│      - Atomic writes (temp + rename)                    │
│      - Automatic backups                                │
├─────────────────────────────────────────────────────────┤
│      Filesystem (~/.config/overlord)                    │  ← Storage
└─────────────────────────────────────────────────────────┘
```

**Core Principles:**
- Registry is the single source of truth for project metadata
- All operations validate before persisting
- Category-based organization (not language-based)
- Fuzzy matching for flexible project resolution
- Interactive TUI when ambiguous or no args provided
- Embedded templates for portability

## Directory Structure

```
overlord-v2/
├── cmd/overlord/              # Application entry point
│   └── main.go                # Calls cmd.Execute()
├── internal/                  # Private application code
│   ├── cmd/                   # Cobra command definitions
│   │   ├── root.go            # Root command + routing logic
│   │   ├── list.go            # List projects with filtering
│   │   ├── info.go            # Show project details
│   │   ├── new.go             # Create new project
│   │   ├── add.go             # Register existing project
│   │   ├── rm.go              # Remove from registry
│   │   ├── open.go            # Open workspace (with TUI picker)
│   │   ├── archive.go         # Archive project
│   │   ├── unarchive.go       # Restore archived project
│   │   ├── fileutil.go        # File operations (expandPath, copyFile)
│   │   ├── *_test.go          # Command tests
│   │   └── open_test.go       # TUI picker tests
│   ├── registry/              # Registry types and operations
│   │   ├── types.go           # Registry, Project, Category, Language types
│   │   ├── types_test.go      # Type validation tests
│   │   ├── store.go           # Load/Save with atomic writes
│   │   ├── store_test.go      # Store operation tests
│   │   ├── resolve.go         # Fuzzy matching and resolution
│   │   ├── resolve_test.go    # Resolution tests
│   │   └── example_test.go    # Usage examples
│   └── templates/             # Embedded templates
│       ├── templates.go       # Template loading and rendering
│       ├── templates_test.go  # Template tests
│       ├── example_test.go    # Template usage examples
│       └── templates/         # Template files (embedded via go:embed)
│           ├── base/          # Base templates (all languages)
│           │   ├── Makefile
│           │   ├── README.md.tmpl
│           │   └── AGENTS.md.tmpl
│           ├── go/            # Go-specific templates
│           ├── python/        # Python-specific templates
│           ├── typescript/    # TypeScript-specific templates
│           ├── solidity/      # Solidity-specific templates
│           └── shared/        # Shared files
│               └── .tmux.local
├── Makefile                   # Build and worktree commands
├── go.mod                     # Go module definition
└── AGENTS.md                  # This file
```

### Key Conventions
- `cmd/` contains only the main entry point
- `internal/cmd/` contains Cobra command definitions
- `internal/registry/` contains all registry-related logic
- `internal/templates/` contains template system
- Tests are co-located with source files (`*_test.go`)
- Example tests demonstrate usage patterns
- Templates are embedded at compile time (go:embed)

## Key Patterns

### Registry Pattern

The registry is a versioned YAML structure with validation at every level.

**Structure:**
```go
type Registry struct {
    Version  int                `yaml:"version"`
    Settings Settings           `yaml:"settings"`
    Projects map[string]Project `yaml:"projects"`
}
```

**Location:** `internal/registry/types.go`

**Rules:**
- Version must be 2 (for future compatibility)
- All types implement `Validate() error`
- Validation cascades: Registry → Settings + Projects → Project fields
- Enums (Category, Language, State) have `IsValid()` methods

### Atomic Write Pattern

Registry saves use atomic writes to prevent corruption.

**Flow:**
1. Validate registry before writing
2. Create parent directories if needed
3. Backup existing file (`.bak` suffix)
4. Write to temporary file (`.tmp` suffix)
5. Atomic rename temp → final
6. Clean up temp file on failure

**Location:** `internal/registry/store.go` (`Save` function)

**Rules:**
- Never write invalid data
- Always backup before overwriting
- Use temp file + rename for atomicity
- Clean up temp files on error

### Enum Validation Pattern

Categories and Languages are type-safe enums with validation.

**Structure:**
```go
type Category string

const (
    CategoryWeb Category = "web"
    // ... other categories
)

func (c Category) IsValid() bool {
    switch c {
    case CategoryWeb, CategoryServices, ...:
        return true
    }
    return false
}
```

**Location:** `internal/registry/types.go`

**Rules:**
- Define as typed string constants
- Provide `IsValid()` method
- Provide `ValidCategories()` / `ValidLanguages()` for listing
- Categories have `Path()` method for filesystem mapping

### Cobra Command Pattern

Commands are defined using Cobra with Viper for configuration.

**Structure:**
```go
var rootCmd = &cobra.Command{
    Use:   "overlord",
    Short: "Brief description",
    Long:  `Detailed description`,
}

func init() {
    cobra.OnInitialize(initConfig)
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
}
```

**Location:** `internal/cmd/root.go`

**Rules:**
- Root command in `internal/cmd/root.go`
- Subcommands in separate files (list.go, info.go, etc.)
- Use Viper for config management
- Config location: `~/.config/overlord/config.yaml`

### Root Command Routing Pattern

The root command routes to different subcommands based on arguments.

**Flow:**
```go
func runRoot(cmd *cobra.Command, args []string) error {
    if len(args) == 0 {
        // No args - run list command
        return RunList(cmd, args)
    }
    // Has args - treat as project name and run open
    return RunOpen(cmd, args)
}
```

**Location:** `internal/cmd/root.go`

**Rules:**
- `overlord` with no args → list active projects
- `overlord <name>` → open project (shorthand for `overlord open <name>`)
- Subcommands exported as `RunXxx` for reuse

### Fuzzy Matching Pattern

Project resolution uses a three-tier matching strategy.

**Flow:**
1. Check for exact name match (score: 1000)
2. Check for exact alias match (score: 900)
3. Fuzzy match on names and aliases (score: variable)

**Location:** `internal/registry/resolve.go`

**Rules:**
- Exact matches return immediately (no fuzzy search)
- Fuzzy matches require minimum score (50)
- Results sorted by score (highest first)
- Ambiguous matches (equal scores) trigger interactive picker

### Interactive TUI Pattern

The open command uses Bubbletea for interactive project selection.

**Structure:**
```go
type pickerModel struct {
    textInput  textinput.Model
    projects   []projectItem
    filtered   []projectItem
    cursor     int
    selected   *projectItem
    cancelled  bool
}
```

**Location:** `internal/cmd/open.go`

**Rules:**
- Picker shown when: no args, multiple matches, or ambiguous
- Real-time fuzzy filtering on user input
- Arrow keys for navigation
- Enter to select, Esc to cancel
- Terminal detection (fallback to error if not TTY)

### Template Embedding Pattern

Templates are embedded at compile time using go:embed.

**Structure:**
```go
//go:embed all:templates
var templateFS embed.FS

func GetTemplate(lang Language, templateType TemplateType) (string, error) {
    // Fallback logic: language-specific -> base -> shared
}
```

**Location:** `internal/templates/templates.go`

**Rules:**
- Templates embedded via `go:embed all:templates`
- Fallback order: `templates/{lang}/` → `templates/base/` → `templates/shared/`
- `.tmpl` files processed with text/template
- Non-`.tmpl` files returned as-is
- Variable substitution: `{{.ProjectName}}`, `{{.Description}}`, etc.

## Coding Conventions

### Naming Conventions

| Thing | Convention | Example |
|-------|------------|---------|
| Files | snake_case | `types.go`, `store.go` |
| Test files | `*_test.go` | `types_test.go` |
| Types | PascalCase | `Registry`, `Project` |
| Functions | PascalCase (exported) | `Load`, `Save` |
| Functions | camelCase (private) | `expandPath`, `copyFile` |
| Constants | PascalCase with prefix | `CategoryWeb`, `StateActive` |
| YAML tags | snake_case | `yaml:"base_dir"` |

### File Organization

- One primary type per file (e.g., `types.go` has all registry types)
- Related functions in same file (e.g., `store.go` has Load/Save)
- Tests co-located: `types.go` → `types_test.go`
- Example tests in `example_test.go`

### Error Handling

- Return errors, don't panic
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Validate before operations (fail fast)
- Use descriptive error messages

### Testing

- Table-driven tests for validation
- Test both valid and invalid cases
- Use `t.Run()` for subtests
- Example tests demonstrate usage

## Key Abstractions

### Registry

**Purpose:** Central data structure for project metadata  
**Location:** `internal/registry/types.go`  
**Key Methods:**
- `Validate() error` - Validates entire registry
- `GetProject(name) (Project, bool)` - Retrieve by name
- `GetProjectByAlias(alias) (string, Project, bool)` - Retrieve by alias
- `GetProjectsByCategory(Category) map[string]Project` - Filter by category
- `GetProjectsByTag(tag) map[string]Project` - Filter by tag
- `GetActiveProjects() map[string]Project` - Filter by state

### Project

**Purpose:** Represents a single project entry  
**Location:** `internal/registry/types.go`  
**Fields:**
- `Path` - Relative path from base_dir
- `Category` - Project category (enum)
- `Lang` - Programming language (enum)
- `Created` - Creation date (YYYY-MM-DD)
- `Description` - Human-readable description
- `Aliases` - Alternative names for lookup
- `Tags` - Freeform tags for filtering
- `Status` - Current state (active/archived)

### Category

**Purpose:** Type-safe project categorization  
**Location:** `internal/registry/types.go`  
**Values:** contracts, web, services, ml, libs, cli, core-agents, core-tools, sandbox  
**Methods:**
- `IsValid() bool` - Validates category
- `Path() string` - Returns filesystem path for category

### Language

**Purpose:** Type-safe language identification  
**Location:** `internal/registry/types.go`  
**Values:** python, typescript, go, solidity, base  
**Methods:**
- `IsValid() bool` - Validates language

## Dependencies

### External Dependencies

| Package | Purpose | Usage |
|---------|---------|-------|
| `github.com/spf13/cobra` | CLI framework | Command definitions |
| `github.com/spf13/viper` | Configuration | Config file loading |
| `gopkg.in/yaml.v3` | YAML parsing | Registry serialization |
| `github.com/sahilm/fuzzy` | Fuzzy matching | Project resolution |
| `github.com/charmbracelet/bubbletea` | TUI framework | Interactive picker |
| `github.com/charmbracelet/bubbles` | TUI components | Text input widget |
| `github.com/charmbracelet/lipgloss` | Terminal styling | Colors and tables |
| `golang.org/x/term` | Terminal detection | TTY checking |

### Internal Dependencies

```
cmd/overlord/main.go
    └── internal/cmd (Execute)
            └── github.com/spf13/cobra
            └── github.com/spf13/viper

internal/cmd/open.go
    └── internal/registry (Load, Resolve)
    └── github.com/charmbracelet/bubbletea
    └── github.com/sahilm/fuzzy

internal/cmd/new.go
    └── internal/registry (Load, Save)
    └── internal/templates (RenderTemplate)

internal/registry/store.go
    └── internal/registry/types.go (Registry, Project)
    └── gopkg.in/yaml.v3

internal/registry/resolve.go
    └── internal/registry/types.go (Registry, Project)
    └── github.com/sahilm/fuzzy

internal/templates/templates.go
    └── embed (go:embed)
    └── text/template
```

## Common Tasks

### Adding a New Category

1. Add constant to `internal/registry/types.go`:
   ```go
   const CategoryNewType Category = "new-type"
   ```
2. Update `ValidCategories()` function
3. Update `IsValid()` switch statement
4. Add path mapping in `Path()` method
5. Update tests in `types_test.go`

### Adding a New Language

1. Add constant to `internal/registry/types.go`:
   ```go
   const LanguageNewLang Language = "new-lang"
   ```
2. Update `ValidLanguages()` function
3. Update `IsValid()` switch statement
4. Update tests in `types_test.go`

### Adding a New Registry Field

1. Add field to struct in `internal/registry/types.go`
2. Add YAML tag: `` `yaml:"field_name"` ``
3. Update `Validate()` method if validation needed
4. Update tests in `types_test.go`
5. Update example in `example_test.go`

### Adding a New Cobra Command

1. Create new file in `internal/cmd/` (e.g., `list.go`)
2. Define command with `cobra.Command`
3. Add command to root in `init()`: `rootCmd.AddCommand(listCmd)`
4. Implement command logic
5. Update Makefile if command needs a shortcut

## Do NOT

### Architecture
- Do NOT put business logic in `cmd/overlord/main.go` (only call Execute)
- Do NOT skip validation before saving registry
- Do NOT modify registry without going through Load/Save
- Do NOT use global variables for registry state

### Code Style
- Do NOT use `panic()` for expected errors (return errors)
- Do NOT ignore errors (always check and handle)
- Do NOT use `interface{}` without good reason (use concrete types)
- Do NOT mutate function parameters (return new values)

### Registry Operations
- Do NOT write registry without validation
- Do NOT skip atomic write pattern (temp file + rename)
- Do NOT skip backup before overwriting
- Do NOT hardcode paths (use `expandPath` for ~ expansion)

### Testing
- Do NOT skip table-driven tests for validation
- Do NOT test only happy paths (test error cases)
- Do NOT use `t.Error` when `t.Fatal` is appropriate
- Do NOT skip example tests for public APIs

## Makefile Commands

**Worktree Management:**
```bash
make worktree-new [BRANCH=name]     # Create worktree + tmux session
make worktree-list                  # List worktrees
make worktree-attach BRANCH=name    # Attach to session
make worktree-remove BRANCH=name    # Remove worktree + kill session
make worktree-archive BRANCH=name   # Archive logs from worktree
make worktree-archive-remove BRANCH=name  # Archive logs then remove worktree
make worktree-setup                 # Run .worktree-setup.sh in current directory
```

**Tmux Session Management:**
```bash
make worktree-sessions               # List tmux sessions for all worktrees
```

**Cross-Session Communication:**
```bash
make worktree-send BRANCH=x WINDOW=y CMD="z"   # Send command to worktree session window
make worktree-read BRANCH=x WINDOW=y           # Read visible pane content from worktree
```

**Current Session Utilities (run from within tmux):**
```bash
make tmux-send WINDOW=x CMD="y"     # Send command to current session window
make tmux-read WINDOW=x             # Read visible pane content from window
make tmux-list                      # List all windows in current session
```

**Go Commands:**
```bash
make install                        # Install dependencies (go mod download)
make build                          # Build the application
make run                            # Run the application (go run .)
make test                           # Run tests (go test -v ./...)
make bench                          # Run benchmarks
make fmt                            # Format code (go fmt ./...)
make vet                            # Vet code (go vet ./...)
make clean                          # Clean build artifacts
```

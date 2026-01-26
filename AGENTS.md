# overlord-v2 - Project Management System

A Go-based CLI tool for managing Git worktrees, tmux sessions, and project registries.

## Architecture Overview

Overlord v2 is a layered CLI application built with Cobra, featuring fuzzy matching, interactive TUI, and embedded templates:

```
┌─────────────────────────────────────────────────────────┐
│      CLI Commands (Cobra)                               │  ← User interface
│      - Root (routing), List, Info, New, Add, Rm         │
│      - Open (with TUI picker), Archive, Unarchive       │
│      - Init (scaffolding), Setup, Config                 │
│      - Activate, Sync, Deactivate (multi-machine sync)  │
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
- Automatic thoughts directory management during archive/unarchive

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
│   │   ├── archive.go         # Archive project + thoughts
│   │   ├── unarchive.go       # Restore project + thoughts
│   │   ├── thoughts.go        # Thoughts directory management helpers
│   │   ├── init.go            # Initialize directory with templates
│   │   ├── setup.go           # Set up overlord directories/config
│   │   ├── config.go          # Open registry in editor
│   │   ├── activate.go        # Activate sync for project
│   │   ├── sync.go            # Sync files between machines
│   │   ├── deactivate.go      # Deactivate sync and optionally remove
│   │   ├── fileutil.go        # File operations (resolveProjectPath, copyFile)
│   │   ├── *_test.go          # Command tests
│   │   ├── open_test.go       # TUI picker tests
│   │   ├── thoughts_test.go   # Thoughts helpers tests
│   │   ├── archive_test.go    # Archive command tests
│   │   └── unarchive_test.go  # Unarchive command tests
│   ├── registry/              # Registry types and operations
│   │   ├── types.go           # Registry, Project, Category, Language types
│   │   ├── types_test.go      # Type validation tests
│   │   ├── store.go           # Load/Save with atomic writes
│   │   ├── store_test.go      # Store operation tests
│   │   ├── resolve.go         # Fuzzy matching and resolution
│   │   ├── resolve_test.go    # Resolution tests
│   │   └── example_test.go    # Usage examples
│   ├── machine/               # Machine configuration for sync
│   │   ├── config.go          # MachineConfig, Role types, Load function
│   │   ├── config_test.go     # Machine config tests
│   │   └── example_test.go    # Usage examples
│   ├── sync/                  # Sync infrastructure
│   │   ├── rsync.go           # Rsync wrapper for file sync
│   │   ├── rsync_test.go      # Rsync tests
│   │   ├── conflict.go        # Conflict detection
│   │   ├── conflict_test.go   # Conflict detection tests
│   │   ├── example_test.go    # Rsync usage examples
│   │   └── example_conflict_test.go  # Conflict detection examples
│   ├── gitops/                # Git operations for registry sync
│   │   ├── git.go             # GitOps struct with Add, Commit, Push, Pull
│   │   ├── git_test.go        # Git operations tests
│   │   └── example_test.go    # Git operations examples
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
- `internal/machine/` contains machine-specific configuration for sync
- `internal/sync/` contains rsync wrapper and conflict detection
- `internal/gitops/` contains git operations for registry sync
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

### Machine Config Pattern

Machine configuration defines the role and sync behavior for each machine.

**Structure:**
```go
type MachineConfig struct {
    Name        string `yaml:"name"`
    Role        Role   `yaml:"role"`
    StorageHost string `yaml:"storage_host,omitempty"`
}
```

**Location:** `internal/machine/config.go`

**Roles:**
- `storage` - Machine that holds the full project set (no storage_host)
- `working-set` - Machine that syncs a subset from storage (requires storage_host)

**Rules:**
- Config file: `~/.config/overlord/machine.yaml`
- If file doesn't exist, defaults to storage role with hostname
- `storage_host` required for working-set, forbidden for storage
- All configs validated before use
- Path expansion supports `~` for home directory

### Rsync Wrapper Pattern

The rsync wrapper provides a Go interface to rsync for bidirectional file sync.

**Structure:**
```go
type Rsync struct {
    executor CommandExecutor
    lookPath LookPathFunc
}

type RsyncOptions struct {
    Source   string   // Source path
    Dest     string   // Destination path
    Excludes []string // Exclusion patterns
    DryRun   bool     // Trial run with no changes
    Delete   bool     // Delete extraneous files from dest
}
```

**Location:** `internal/sync/rsync.go`

**Key Methods:**
- `Push(opts) (*RsyncResult, error)` - Sync local → remote
- `Pull(opts) (*RsyncResult, error)` - Sync remote → local
- `DryRun(opts) (string, error)` - Preview changes without syncing

**Rules:**
- Source paths automatically get trailing `/` for rsync
- Remote paths formatted as `host:path` or `user@host:path`
- Uses `-avz --progress` flags by default
- Validates options before execution
- Returns exit code and output in RsyncResult
- Supports custom CommandExecutor for testing

**Helper Functions:**
- `FormatRemotePath(host, path)` - Format as `host:path`
- `FormatRemotePathWithUser(user, host, path)` - Format as `user@host:path`
- `ParseRemotePath(remotePath)` - Parse into user, host, path components
- `IsRemotePath(path)` - Check if path is remote format

### Git Operations Pattern

GitOps provides git operations for registry sync with automatic commit prefixing.

**Structure:**
```go
type GitOps struct {
    RepoPath string
}
```

**Location:** `internal/gitops/git.go`

**Key Methods:**
- `HasChanges() (bool, error)` - Check for uncommitted changes
- `Add(files...)` - Stage files (defaults to all if none specified)
- `Commit(message)` - Commit with "overlord: " prefix
- `Push()` - Push to remote (no-op if no remote)
- `Pull()` - Pull with rebase (no-op if no remote)

**Rules:**
- All commits automatically prefixed with "overlord: "
- Push/Pull gracefully handle repos without remotes
- Uses `git -C <path>` for operations in specific directory
- Pull uses `--rebase` to avoid merge commits
- Errors include git output for debugging

**Usage Pattern:**
```go
git := gitops.New("~/.config/overlord")
if hasChanges, _ := git.HasChanges(); hasChanges {
    git.Add()
    git.Commit("sync completed")
    git.Push()
}
```

### Conflict Detection Pattern

Conflict detection uses bidirectional rsync dry-runs to identify file conflicts.

**Algorithm:**
1. Run rsync dry-run local→remote (what would be pushed)
2. Run rsync dry-run remote→local (what would be pulled)
3. Parse outputs to extract file lists
4. Compare lists to categorize conflicts:
   - Files in both lists = `ConflictBothModified`
   - Files only in push list = `ConflictLocalOnly`
   - Files only in pull list = `ConflictRemoteOnly`

**Structure:**
```go
type Conflict struct {
    Path string        // Relative path of conflicting file
    Type ConflictType  // Type of conflict
}

type ConflictType string
const (
    ConflictLocalOnly      ConflictType = "local-only"
    ConflictRemoteOnly     ConflictType = "remote-only"
    ConflictBothModified   ConflictType = "both-modified"
)
```

**Location:** `internal/sync/conflict.go`

**Key Functions:**
- `DetectConflicts(localPath, remoteHost, remotePath, excludes)` - Detect conflicts
- `parseRsyncOutput(output)` - Extract file list from rsync output
- `detectConflictsFromFileLists(pushFiles, pullFiles)` - Categorize conflicts

**Rules:**
- Uses rsync dry-run to avoid actual file transfers
- Parses rsync verbose output to extract file paths
- Skips directories (only tracks files)
- Returns all conflicts, not just first one found
- Supports custom Rsync instance for testing

### Sync Command Patterns

The sync commands (activate, sync, deactivate) manage multi-machine project synchronization.

#### Activate Command Pattern

**Purpose:** Mark a project for sync by setting `sync.status = active`

**Location:** `internal/cmd/activate.go`

**Flow:**
1. Load registry
2. Resolve project (fuzzy matching)
3. Check if project is archived (error if true)
4. Check if already active (info message if true)
5. Update `project.Sync.Status = registry.SyncActive`
6. Save registry
7. Git add, commit, push registry changes

**Key Features:**
- Uses fuzzy matching for project resolution
- Validates project state (no archived projects)
- Auto-commits with message: "overlord: activate <project>"
- Warns but doesn't fail if git push fails

**Example:**
```go
// Activate sync for a project
overlord activate my-project
```

#### Sync Command Pattern

**Purpose:** Bidirectionally sync files between working-set and storage machines

**Location:** `internal/cmd/sync.go`

**Flow:**
1. Load machine config
2. Check role (storage machines show info message and exit)
3. Pull registry from git
4. Load registry
5. Determine projects to sync (all active or specific project)
6. For each project:
   - Detect conflicts (bidirectional rsync dry-run)
   - Prompt user if conflicts found (unless --force)
   - Pull from storage (remote → local)
   - Push to storage (local → remote)
7. Print summary

**Flags:**
- `--dry-run` - Preview changes without syncing
- `--force` - Skip conflict confirmation

**Key Features:**
- Respects global + per-project exclusion patterns
- Conflict detection before sync
- Interactive confirmation for conflicts
- Detailed progress output with styled messages
- Summary report (synced, partial, failed, skipped)

**Sync Order:**
- Always pull first, then push (prevents overwriting remote changes)

**Example:**
```go
// Sync all active projects
overlord sync

// Sync specific project
overlord sync my-project

// Preview what would sync
overlord sync --dry-run
```

#### Deactivate Command Pattern

**Purpose:** Deactivate sync and optionally remove local directory

**Location:** `internal/cmd/deactivate.go`

**Flow:**
1. Load machine config
2. Load registry
3. Resolve project (fuzzy matching)
4. Check if project is archived (error if true)
5. Check if already inactive (info message if true)
6. **On working-set machines:**
   - Sync final changes to storage (push only)
   - If sync fails and not --force-remove, abort
7. Update `project.Sync.Status = registry.SyncInactive`
8. Save registry
9. Git add, commit, push registry changes
10. **On working-set machines (if not --keep-local):**
    - Prompt for directory removal (unless --force)
    - Remove directory if confirmed

**Flags:**
- `--force` - Skip confirmation prompts
- `--keep-local` - Don't remove local directory
- `--force-remove` - Remove directory even if sync failed

**Key Features:**
- Syncs to storage before deactivating (data safety)
- Only removes directories on working-set machines
- Requires confirmation before removal (unless --force)
- Aborts if sync fails (unless --force-remove)
- Auto-commits with message: "overlord: deactivate <project>"

**Example:**
```go
// Deactivate and prompt for removal
overlord deactivate my-project

// Deactivate but keep local files
overlord deactivate my-project --keep-local

// Deactivate and remove without prompt
overlord deactivate my-project --force
```

**Safety Rules:**
- Always sync to storage before deactivating (on working-set)
- Never remove directories on storage machines
- Abort if sync fails (unless --force-remove)
- Require confirmation for directory removal (unless --force)

### Thoughts Management Pattern

**Purpose:** Automatically manage thoughts directories when archiving/unarchiving projects

**Location:** `internal/cmd/thoughts.go`

**Thoughts Directory Structure:**

Main location (active projects) - **Project-centric structure**:
```
~/thoughts/projects/{project-name}/
├── plans/
├── logs/
├── docs/          # Symlinks to project files
├── research/
├── sessions/
├── handoffs/
├── reviews/
└── briefs/
```

Archive location (archived projects):
```
~/thoughts/archive/{project-name}/
├── plans/
├── logs/
├── docs/          # Symlinks to archived project files
├── research/
├── sessions/
├── handoffs/
├── reviews/
└── briefs/
```

**Key Types:**
```go
type ThoughtsPaths struct {
    Plans    string
    Logs     string
    Docs     string
    Research string
    Sessions string
    Handoffs string
    Reviews  string
    Briefs   string
}
```

**Key Functions:**
- `GetThoughtsPaths(thoughtsDir, projectName)` - Get paths for main location (8 subdirectories)
- `GetArchiveThoughtsPaths(thoughtsDir, projectName)` - Get paths for archive location (8 subdirectories)
- `ThoughtsExist(paths)` - Check which directories exist
- `MoveThoughtsToArchive(thoughtsDir, projectName)` - Move thoughts to archive
- `MoveThoughtsFromArchive(thoughtsDir, projectName)` - Restore thoughts from archive
- `UpdateProjectSymlinks(thoughtsDocsDir, actualProjectPath)` - Update symlinks in docs/ to project files

**Archive Behavior:**

When archiving a project:
1. Move project directory to `~/Overlord/archive/{name}/`
2. Move thoughts from `~/thoughts/projects/{name}/` to `~/thoughts/archive/{name}/`
3. Update symlinks in archived thoughts to point to archived project location
4. Warn if thoughts directories don't exist (non-fatal)

**Unarchive Behavior:**

When unarchiving a project:
1. Move project directory from archive to category directory
2. Restore thoughts from `~/thoughts/archive/{name}/` to `~/thoughts/projects/{name}/`
3. Update symlinks to point to restored project location
4. Handle legacy archives (projects archived before this feature) gracefully

**Rules:**
- Thoughts operations are secondary (warn on failure, don't fail archive/unarchive)
- Always update symlinks after moving thoughts
- Use `moveDirectory` for cross-filesystem compatibility
- Clean up empty archive directories after restoration
- Skip creating broken symlinks (only symlink if target exists)

**Example:**
```go
// Archive thoughts
warnings, err := MoveThoughtsToArchive("~/thoughts", "my-project")
if err != nil {
    // Warn but continue - project archive succeeded
    fmt.Printf("Warning: Failed to move thoughts: %v\n", err)
}

// Restore thoughts
warnings, err := MoveThoughtsFromArchive("~/thoughts", "my-project")
if err != nil {
    // Warn but continue - project unarchive succeeded
    fmt.Printf("Warning: Failed to restore thoughts: %v\n", err)
}

// Update symlinks
thoughtsPaths := GetThoughtsPaths("~/thoughts", "my-project")
if err := UpdateProjectSymlinks(thoughtsPaths.Docs, "/path/to/project"); err != nil {
    fmt.Printf("Warning: Failed to update symlinks: %v\n", err)
}
```

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

### SyncSettings

**Purpose:** Global sync configuration with default exclusion patterns  
**Location:** `internal/registry/types.go`  
**Fields:**
- `DefaultExclude` - Default patterns to exclude from sync (e.g., node_modules, .venv)

**Usage:**
```yaml
settings:
  sync:
    default_exclude:
      - node_modules
      - .venv
      - __pycache__
```

### ProjectSync

**Purpose:** Per-project sync configuration  
**Location:** `internal/registry/types.go`  
**Fields:**
- `Status` - Sync status (active/inactive, empty = inactive for backward compat)
- `Exclude` - Project-specific exclusion patterns (in addition to defaults)

**Usage:**
```yaml
projects:
  my-project:
    sync:
      status: active
      exclude:
        - .cache
        - dist/
```

### MachineConfig

**Purpose:** Machine-specific configuration for multi-machine sync  
**Location:** `internal/machine/config.go`  
**Fields:**
- `Name` - Machine identifier (defaults to hostname)
- `Role` - Machine role (storage or working-set)
- `StorageHost` - Hostname of storage machine (required for working-set)

**Key Methods:**
- `Validate() error` - Validates configuration
- `Load(path) (*MachineConfig, error)` - Loads config from file or returns default

**Usage:**
```yaml
# Storage machine
name: desktop-machine
role: storage

# Working-set machine
name: laptop
role: working-set
storage_host: desktop-machine
```

### Rsync

**Purpose:** Wrapper for rsync operations with Go interface  
**Location:** `internal/sync/rsync.go`  
**Key Types:**
- `Rsync` - Main wrapper struct with executor and lookPath
- `RsyncOptions` - Configuration for sync operations
- `RsyncResult` - Result containing output and exit code
- `Direction` - Enum for push/pull direction

**Key Methods:**
- `NewRsync()` - Create instance with default executor
- `NewRsyncWithExecutor(executor)` - Create instance with custom executor (for testing)
- `Push(opts)` - Sync local to remote
- `Pull(opts)` - Sync remote to local
- `DryRun(opts)` - Preview changes without syncing

### Conflict

**Purpose:** Represents a file conflict between local and remote  
**Location:** `internal/sync/conflict.go`  
**Fields:**
- `Path` - Relative path of conflicting file
- `Type` - Type of conflict (local-only, remote-only, both-modified)

**Key Types:**
- `ConflictType` - Enum for conflict types
- `Conflict` - Struct representing a single conflict

**Key Functions:**
- `DetectConflicts(localPath, remoteHost, remotePath, excludes)` - Detect all conflicts
- `DetectConflictsWithRsync(rsync, ...)` - Detect conflicts with custom Rsync instance

**Conflict Types:**
- `ConflictLocalOnly` - File exists only locally (would be pushed)
- `ConflictRemoteOnly` - File exists only remotely (would be pulled)
- `ConflictBothModified` - File modified on both sides (true conflict)

### GitOps

**Purpose:** Git operations for registry sync  
**Location:** `internal/gitops/git.go`  
**Fields:**
- `RepoPath` - Path to git repository

**Key Methods:**
- `New(repoPath)` - Create new GitOps instance
- `HasChanges()` - Check for uncommitted changes
- `Add(files...)` - Stage files (all if none specified)
- `Commit(message)` - Commit with automatic "overlord: " prefix
- `Push()` - Push to remote (gracefully handles no remote)
- `Pull()` - Pull with rebase (gracefully handles no remote)

**Usage:**
```go
git := gitops.New("~/.config/overlord")
if hasChanges, _ := git.HasChanges(); hasChanges {
    git.Add()
    git.Commit("activate my-project")
    git.Push()
}
```

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

internal/machine/config.go
    └── gopkg.in/yaml.v3
    └── os (path expansion, hostname)

internal/sync/rsync.go
    └── os/exec (command execution)
    └── bytes (output buffering)

internal/sync/conflict.go
    └── internal/sync/rsync.go (Rsync, RsyncOptions)
    └── strings (output parsing)

internal/gitops/git.go
    └── os/exec (git commands)
    └── strings (output parsing)

internal/cmd/thoughts.go
    └── os (file operations, stat, mkdir, rename, remove)
    └── path/filepath (path manipulation)
    └── strings (error checking)

internal/cmd/archive.go
    └── internal/cmd/thoughts.go (MoveThoughtsToArchive, UpdateProjectSymlinks)
    └── internal/registry (Load, Save)

internal/cmd/unarchive.go
    └── internal/cmd/thoughts.go (MoveThoughtsFromArchive, UpdateProjectSymlinks)
    └── internal/registry (Load, Save)
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

### Configuring Sync Exclusion Patterns

**Global defaults (applies to all projects):**
1. Edit `~/.config/overlord/registry.yaml`
2. Add patterns to `settings.sync.default_exclude`:
   ```yaml
   settings:
     sync:
       default_exclude:
         - node_modules
         - .venv
         - __pycache__
         - .git
   ```

**Per-project exclusions (in addition to defaults):**
1. Edit project entry in registry
2. Add patterns to `sync.exclude`:
   ```yaml
   projects:
     my-project:
       sync:
         status: active
         exclude:
           - .cache
           - dist/
           - build/
   ```

**Note:** Project-specific exclusions are additive to global defaults

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
make deps                           # Install dependencies (go mod download)
make build                          # Build the application
make install                        # Build and install to ~/.local/bin
make deploy [VPS=hostname]          # Cross-compile and deploy to VPS (default: poe-vps)
make run                            # Run the application (go run .)
make test                           # Run tests (go test -v ./...)
make bench                          # Run benchmarks
make fmt                            # Format code (go fmt ./...)
make vet                            # Vet code (go vet ./...)
make clean                          # Clean build artifacts
```

**Overlord Commands:**
```bash
make setup                          # Set up overlord directories and config
make config                         # Open registry in editor
make init [PATH=<path>] [LANG=go|py|ts|sol]  # Initialize directory with templates
```

**Sync Commands:**
```bash
make activate NAME=<name>           # Activate sync for a project
make sync [NAME=<name>]             # Sync all or specific project
make sync DRY_RUN=1                 # Preview sync changes
make sync FORCE=1                   # Force sync (skip conflict confirmation)
make deactivate NAME=<name>         # Deactivate sync for a project
```


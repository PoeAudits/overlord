# overlord - Project Management System

A Go-based CLI tool for managing Git worktrees, tmux sessions, and project registries.

## Architecture Overview

Layered CLI application: Cobra commands → Registry operations (fuzzy matching, filtering, validation) → Templates (embedded, language-specific) → YAML Store (atomic writes, backups) → Filesystem (`~/.config/overlord`).

**Core Principles:**
- Registry is the single source of truth for project metadata
- All operations validate before persisting
- Category-based organization (not language-based)
- Fuzzy matching for flexible project resolution
- Interactive TUI when ambiguous or no args provided
- Embedded templates for portability
- Automatic thoughts directory management during archive/unarchive

**Two Data Surfaces:**

| Repository | Purpose | Location |
|------------|---------|----------|
| `PoeAudits/overlord` | Source code (this Go project) | `~/Overlord/core/tools/overlord` |
| `~/.config/overlord/` Syncthing folder | Runtime registry artifacts (`registry.yaml`, `peers.yaml`, `registry.lock.yaml`) | `~/.config/overlord/` |
| `~/.local/share/overlord/backup` git repo | Optional disaster-recovery snapshots (`overlord backup`) | local repo with GitHub remote |

`~/.config/overlord/` is Syncthing-managed. `machine.yaml` is local-only and excluded via `.stignore`.

## Directory Structure

```
overlord/
├── cmd/overlord/main.go           # Entry point (only calls cmd.Execute())
├── internal/
│   ├── cmd/                       # Cobra command definitions
│   │   ├── root.go                # Root command + routing logic
│   │   ├── constants.go           # Named constants (max lengths, picker sizes)
│   │   ├── styles.go              # Shared terminal styles (NewStyles())
│   │   ├── helpers.go             # Shared helpers (loadRegistry, saveRegistry, countTrue, etc.)
│   │   ├── fileutil.go            # File operations helpers
│   │   ├── tmux.go                # Tmux session management (tmuxSession struct)
│   │   ├── list.go                # List projects with filtering
│   │   ├── info.go, new.go, add.go, rm.go
│   │   ├── open.go                # Open workspace (with TUI picker)
│   │   ├── worktree.go            # Git worktree + tmux management
│   │   ├── archive.go, unarchive.go, clear.go, thoughts.go
│   │   ├── init.go, setup.go, config.go
│   │   ├── activate.go, sync.go, deactivate.go
│   │   ├── sync_lock.go            # Registry lock commands (show/transfer/claim)
│   │   ├── doctor.go, backup.go
│   │   ├── syncthing_config_folder.go
│   │   └── env.go, detect.go
│   ├── registry/                  # Registry types and operations
│   │   ├── types.go               # Registry, Project, Category, Language types
│   │   ├── store.go               # Load/Save with atomic writes
│   │   └── resolve.go             # Fuzzy matching and resolution
│   ├── machine/config.go          # MachineConfig, Syncthing settings, Load function
│   ├── machine/peers.go           # Shared peer registry helpers
│   ├── machine/lock.go            # Registry lock artifact helpers
│   ├── gitops/git.go              # Git operations for registry sync
│   └── templates/
│       ├── templates.go           # Template loading and rendering
│       └── templates/             # Embedded template files (go:embed)
│           ├── base/              # Base templates (all languages)
│           ├── go/, python/, typescript/, solidity/
│           └── shared/.tmux.local
├── Makefile
└── go.mod
```

Tests are co-located (`*_test.go`). Example tests in `example_test.go` files.

## Key Patterns

### Shared Utilities (internal/cmd/)

Command files use shared utilities to reduce duplication:

**Constants** (`constants.go`): Named constants for magic numbers used across commands:
- `maxProjectNameLen` (64) — maximum project name length
- `maxDescDisplayLen` (40) — max description length in tables/pickers
- `maxPickerVisibleItems` (15) — default visible items in TUI picker
- `minPickerVisibleItems` (5) — minimum visible items in TUI picker
- `pickerHeaderFooterLines` (5) — lines reserved for picker header/footer
- `defaultWorktreeBaseDir` — base directory for worktrees (`~/Overlord/worktrees`)
- `worktreeAdjectives`, `worktreeNouns` — word lists for auto-generated branch names

**Styles** (`styles.go`): Centralized terminal styling via `NewStyles()`. Returns a `Styles` struct with pre-configured lipgloss styles for success, error, warning, info, dim, bold, and color constants. All command files call `NewStyles()` instead of defining inline styles.

**Helpers** (`helpers.go`): Common operations used across commands:
- `loadRegistry()` — loads registry with error handling
- `saveRegistry()` — saves registry with validation and lock-owner enforcement
- `confirmAction()` — interactive yes/no confirmation prompt
- `expandBaseDir()` — expands base_dir from registry settings
- `countTrue()` — variadic helper to count true boolean flags (used for validation)

**File Utilities** (`fileutil.go`): File system operations:
- `formatPathWithTilde()` — replaces home dir with `~` for display
- `moveDirectory()` — cross-device safe directory move (copy + remove)
- `expandProjectPath()` — resolves project path from registry
- `copyFileWithMode()` — copies file with mode preservation, cleans up partial files on failure

**Tmux Session Management** (`tmux.go`): Encapsulates tmux session operations via `tmuxSession` struct:
- `exists()` — checks if session exists
- `create()` — creates new detached session with "base" window
- `sourceLocal()` — sources `.tmux.local` file with environment variables
- `attachOrSwitch()` — attaches to session or switches if already in tmux

**Pattern:** Command files import these utilities and call them instead of duplicating logic. Path expansion uses `registry.ExpandPath()` (canonical implementation). Commands that manage tmux sessions (new.go, open.go) use the `tmuxSession` struct instead of inline exec.Command calls.

### Registry & Store

The registry (`internal/registry/types.go`) is a versioned YAML structure (version: 2) with cascading validation: `Registry → Settings + Projects → Project fields`. All types implement `Validate() error`. Enums (Category, Language, State) have `IsValid()` methods.

Registry saves (`internal/registry/store.go`) use atomic writes: validate → backup existing (`.bak`) → write temp (`.tmp`) → atomic rename → clean up on failure. Never write invalid data.

### Root Command Routing

`overlord` with no args → list active projects. `overlord <name>` → open project (shorthand for `overlord open <name>`). Subcommands exported as `RunXxx` for reuse. Location: `internal/cmd/root.go`.

**Error propagation:** Config initialization errors are stored in `configErr` and returned via `PersistentPreRunE`. This prevents `os.Exit()` calls outside main and allows proper error handling through Cobra's error chain.

### Fuzzy Matching

Three-tier matching in `internal/registry/resolve.go`: exact name (score: 1000) → exact alias (score: 900) → fuzzy match on names/aliases (min score: 50). Ambiguous matches trigger interactive TUI picker.

### Interactive TUI

Bubbletea picker in `internal/cmd/open.go`. Shown when: no args, multiple matches, or ambiguous. Real-time fuzzy filtering, arrow key navigation, Enter to select, Esc to cancel. Falls back to error if not TTY.

### Templates

Embedded via `go:embed all:templates` in `internal/templates/templates.go`. Fallback order: `templates/{lang}/` → `templates/base/` → `templates/shared/`. `.tmpl` files processed with `text/template`, others returned as-is. Variables: `{{.ProjectName}}`, `{{.Description}}`, etc.

### Enums

Categories and Languages are typed string constants with `IsValid()`, `ValidCategories()`/`ValidLanguages()` list methods. 

**Category.Path():** Uses package-level `categoryPaths` map for filesystem mapping (not a switch statement). This makes adding new categories simpler — just update the map and `ValidCategories()`.

**IsValid() methods:** Iterate over `ValidCategories()`/`ValidLanguages()` instead of duplicating the list in a switch. Single source of truth for valid values.

Values:
- **Categories:** contracts, web, services, ml, libs, cli, core-agents, core-tools, sandbox, config
- **Languages:** python, typescript, go, solidity, base

### Context Threading Pattern

All long-running operations accept `context.Context` as the first parameter. This enables future timeout/cancellation support without breaking signatures.

**Packages with context support:**
- `internal/gitops/git.go` — `Add(ctx)`, `Commit(ctx)`, `Push(ctx)`, `Pull(ctx)`
- `internal/ssh/mux.go` — `Connect(ctx)`, `Close(ctx)`

**Current usage:** Commands pass `context.Background()` to these methods. Future enhancements can use `context.WithTimeout()` for long operations (e.g., large file syncs, slow git operations).

**CommandExecutor interface:** Updated to accept context. Test mocks and real implementations both support context-aware execution.

### Context Structs for Complex Operations

When a function requires many parameters (>5), use a context struct to bundle related parameters. This improves readability and makes it easier to add new parameters without breaking signatures.

**When to use:** Functions with >5 parameters, especially when parameters fall into logical groups (config, options, paths, etc.).

## Multi-Machine Sync

### Machine Config

`internal/machine/config.go` — stored at `~/.config/overlord/machine.yaml` (gitignored).

Fields:
- `name` — machine hostname
- `role` — machine role (`main`, `vps`, `runner`, `aux`)
- `syncthing` — local sensitive settings (device_id, api_url, api_key, address)

Shared peer metadata is stored in `~/.config/overlord/peers.yaml`.

### Registry Lock

`~/.config/overlord/registry.lock.yaml` enforces single-writer registry mutations.

Lock commands:
- `overlord sync lock show`
- `overlord sync lock transfer <machine>`
- `overlord sync lock claim`

All commands that mutate `registry.yaml` are guarded via `saveRegistry()` lock checks.

If the file doesn't exist, Overlord creates one with the hostname only. Sync commands require Syncthing config to be filled in.

### Sync Infrastructure

**Syncthing REST client** (`internal/syncthing/`): manages devices and folders via the Syncthing REST API.

**Backup Git repo** (`internal/cmd/backup.go`): one-way disaster-recovery snapshots of `registry.yaml` + `peers.yaml` to a dedicated local git repo/remote. Not used in sync-critical flows.

### Sync Commands

**`overlord activate [project ...]`** (`internal/cmd/activate.go`):
- Supports single, many, or `--all`
- Ensures peer device and folder exist in Syncthing (local)
- Sets `sync.status = active` and `sync.method = syncthing`
- Validates not archived and enforces lock ownership before registry writes

**`overlord sync [project]`** (`internal/cmd/sync.go`):
- Shows Syncthing folder status for active projects
- No file transfer is initiated; sync is continuous via Syncthing

**`overlord deactivate [project ...]`** (`internal/cmd/deactivate.go`):
- Supports single, many, or `--all`
- Removes Syncthing folder locally
- Sets `sync.status = inactive` and `sync.method = syncthing`
- Enforces lock ownership before registry writes

**`overlord doctor`** (`internal/cmd/doctor.go`):
- Verifies machine config, Syncthing connectivity, registry/lock validity, peer reachability, and folder consistency.

**`overlord backup`** (`internal/cmd/backup.go`):
- Copies `registry.yaml` + `peers.yaml` into a dedicated backup repo, commits timestamped snapshots, and pushes one-way.

### Sync Ignore Patterns

Sync ignores are managed via `.stignore` with a shared `.stglobalignore` include. Overlord ensures `.stignore` exists when activating projects.

### Synced List Behavior

`overlord list --synced` shows active projects with sync status `active` and disk usage for local paths.

## Thoughts Management

`internal/cmd/thoughts.go` — automatic thoughts directory management during archive/unarchive.

**Structure:** `~/thoughts/projects/{project-name}/` with subdirs: plans, logs, docs (symlinks to project), research, sessions, handoffs, reviews, briefs. Archive location: `~/thoughts/archive/{project-name}/`.

**Archive flow:** Move project dir → move thoughts to archive → update symlinks to archived location.
**Unarchive flow:** Move project from archive → restore thoughts → update symlinks. Handles legacy archives.

**Rules:** Thoughts operations are secondary (warn on failure, don't fail archive/unarchive). Always update symlinks after moving. Clean up empty archive dirs.

## Worktree Management

`internal/cmd/worktree.go` — git worktree management with tmux session integration.

**Commands:**
- **`overlord worktree new [branch]`** — Create worktree + tmux session. Auto-generates adjective-noun branch names if not provided.
- **`overlord worktree list`** — List worktrees for current project with session status.
- **`overlord worktree attach [branch]`** — Attach to worktree's tmux session. Uses current branch if in worktree.
- **`overlord worktree remove [branch]`** — Remove worktree, branch, and session. Supports self-removal with confirmation.
- **Alias:** `overlord wt` is shorthand for `overlord worktree`.

**Directory Structure:**
- Worktrees created at: `~/Overlord/worktrees/{project}/{branch}/`
- Session name format: `{project}--{branch}` (slashes in branch replaced with dashes)

**Auto-generated Names:** Uses adjective-noun pairs (e.g., `swift-fix`, `bright-task`) from word lists in `constants.go`. Appends counter on collision (`swift-fix-2`).

**Context Detection:** Works from any git repository. Detects project name from directory. If in a worktree, resolves to main repo for operations.

**Session Setup:** Creates tmux session with single "base" window running `opencode`.

**Key Functions:**
- `detectProject()` — Detects project from cwd, handles worktree resolution
- `parseWorktreeList()` — Parses `git worktree list --porcelain` output
- `generateBranchName()` — Generates random adjective-noun names
- `worktreeSessionName()` — Formats `{project}--{branch}` session name
- `ensureUniqueBranch()` — Appends counter to avoid collisions

## Coding Conventions

### Naming

| Thing | Convention | Example |
|-------|------------|---------|
| Files | snake_case | `types.go`, `store.go` |
| Types | PascalCase | `Registry`, `Project` |
| Exported functions | PascalCase | `Load`, `Save` |
| Private functions | camelCase | `expandPath`, `copyFile` |
| Constants | PascalCase with prefix | `CategoryWeb`, `StateActive` |
| YAML tags | snake_case | `yaml:"base_dir"` |

### Error Handling

- Return errors, don't panic
- Wrap with context: `fmt.Errorf("failed to X: %w", err)`
- Validate before operations (fail fast)

### Testing

- Table-driven tests for validation
- Test both valid and invalid cases
- Use `t.Run()` for subtests
- Example tests in `example_test.go` for public APIs

## Do NOT

- Do NOT put business logic in `cmd/overlord/main.go` (only call Execute)
- Do NOT skip validation before saving registry
- Do NOT modify registry without going through Load/Save
- Do NOT use global variables for registry state
- Do NOT use `panic()` for expected errors
- Do NOT ignore errors (always check and handle)
- Do NOT use `interface{}` without good reason
- Do NOT mutate function parameters (return new values)
- Do NOT skip atomic write pattern (temp file + rename)
- Do NOT skip backup before overwriting
- Do NOT hardcode paths (use `registry.ExpandPath()` for ~ expansion)
- Do NOT skip table-driven tests for validation
- Do NOT test only happy paths (test error cases)
- Do NOT define inline styles in command files (use `NewStyles()`)
- Do NOT duplicate registry load/save logic (use `loadRegistry()`, `saveRegistry()`)
- Do NOT reintroduce git push/pull into sync command paths (`activate`, `deactivate`, `sync`, `setup --init`)
- Do NOT use magic numbers (define named constants in `constants.go`)
- Do NOT use `os.Exit()` outside main (propagate errors via return values)
- Do NOT leave resource leaks (clean up partial files on failure)

## Common Tasks

### Adding a New Category
1. Add constant to `internal/registry/types.go`
2. Update `ValidCategories()` list
3. Add path mapping to `categoryPaths` map
4. Update tests in `types_test.go`

### Adding a New Language
1. Add constant to `internal/registry/types.go`
2. Update `ValidLanguages()` list
3. Update tests in `types_test.go`

### Adding a New Registry Field
1. Add field to struct in `internal/registry/types.go` with YAML tag
2. Update `Validate()` if needed
3. Update tests and examples

### Adding a New Cobra Command
1. Create file in `internal/cmd/`
2. Define `cobra.Command`, add to root in `init()`
3. Implement logic
4. Update Makefile if needed

## Dependencies

| Package | Purpose |
|---------|---------|
| `context` | Context threading for cancellation/timeout support |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Configuration |
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/sahilm/fuzzy` | Fuzzy matching |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/bubbles` | TUI components |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `golang.org/x/term` | Terminal detection |

## Environment Variables Management

`internal/cmd/env.go` — manages environment variables via pass (password-store) integration.

**`.env.pass` template format:**
```
# Secrets from pass
API_KEY=pass:ai/anthropic-api-key
DB_URL=pass:project/db-url

# Literal config
NODE_ENV=development
PORT=3000
```

**Commands:**
- **`overlord env [project]`** — reads `.env.pass`, resolves `pass:` references, writes `.env` (0600 perms). Auto-detects project from cwd if no arg.
- **`overlord env generate [project]`** — scans pass store, generates `.env.pass` with all entries. Converts `kebab-case` to `UPPER_SNAKE_CASE`. Refuses to overwrite without `--force`. Auto-run during `overlord new`.

**Implementation notes:**
- `listPassEntries()` walks `$PASSWORD_STORE_DIR` (or `~/.password-store`), returns sorted `.gpg` files
- `passEntryToEnvVar()` converts pass entry names to env var names: `ai/anthropic-api-key` → `AI_ANTHROPIC_API_KEY`
- `parseEnvPassFile()` parses template, `resolveEnvEntries()` fetches from pass, `atomicWriteEnvFile()` writes with 0600
- Integrated into `overlord new` (step 2b) — generates `.env.pass` if pass store exists (best-effort)

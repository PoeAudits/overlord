# Overlord - Project Management System

## Overview

Overlord is a CLI-based project management system for organizing Python, TypeScript, and Solidity projects. It manages project lifecycle through status categories (active, lib, archive) and provides tmux workspace integration.

## Installation

### Fresh Installation

1. Clone the repository:
   ```bash
   git clone <repo-url> ~/overlord
   cd ~/overlord
   ```

2. Run the setup script:
   ```bash
   ./setup.sh
   ```

3. Verify installation:
   ```bash
   overlord --version
   overlord list
   ```

The setup script will:
- Create a symlink at `/usr/local/bin/overlord`
- Initialize an empty registry file
- Verify the installation

### Requirements

- Bash 4.0+
- Git
- Write access to `/usr/local/bin` (or sudo)
- `jq` for JSON processing
- `tmux` for workspace management
- `fzf` for fuzzy search (optional but recommended)

## Directory Structure

```
~/Work/
├── Python/
│   ├── active/          # Currently working on
│   ├── libs/            # Reusable libraries
│   └── archive/         # Completed/paused projects
├── Typescript/
│   ├── active/
│   ├── libs/
│   └── archive/
├── Solidity/
│   ├── active/
│   ├── libs/
│   └── archive/

~/bin/overlord/
├── overlord             # Main dispatcher
├── lib/
│   └── common.sh        # Shared helper functions
├── overlord-new         # Create new project
├── overlord-add         # Register existing project
├── overlord-init        # Initialize existing directory
├── overlord-list        # List projects
├── overlord-mv          # Move project status
├── overlord-rm          # Remove project from registry
├── overlord-open        # Open workspace
├── overlord-info        # Show project details
├── overlord-sync        # Propagate files to projects
├── overlord-config      # Edit registry.json
├── overlord-edit        # Edit overlord scripts
├── setup.sh             # Installation script
├── registry.json        # Project metadata (created by setup)
├── templates/
│   └── opencode-*.jsonc
├── tmux/
│   ├── base.tmux        # Base tmux template (non-language projects)
│   ├── python.tmux      # Python tmux template
│   ├── typescript.tmux  # TypeScript tmux template
│   └── solidity.tmux    # Solidity tmux template
├── makefiles/
│   ├── base.mk          # Git worktree commands (shared)
│   ├── python.mk        # Python/uv commands
│   ├── typescript.mk    # TypeScript/pnpm/bun commands
│   └── solidity.mk      # Solidity/forge commands
├── AGENTS.md            # This documentation
└── thoughts/
```

## Commands

### overlord (default: list)

Main dispatcher. Running `overlord` with no arguments lists active projects.

```bash
overlord                    # List active projects
overlord --help             # Show help
overlord --version          # Show version
overlord add <name> <path>  # Register existing project
```

### overlord new

Create a new project with language-specific initialization.

```bash
overlord new <name> --py|--ts|--sol [options]

# Language flags (required, one of):
--py, --python       # Python project (uses uv)
--ts, --typescript   # TypeScript project (uses pnpm)
--sol, --solidity    # Solidity project (uses forge)

# Options:
--lib                # Create in libs/ instead of active/
--no-git             # Skip git initialization
--no-open, -s        # Don't open workspace after creation
--force              # Overwrite existing project
```

**Examples:**
```bash
overlord new myapp --py              # New Python project
overlord new mylib --ts --lib        # New TypeScript library
overlord new mycontract --sol -s     # New Solidity, don't open
```

### overlord list

List projects from registry.

```bash
overlord list [options]

# Language filters (can be combined with OR logic):
--py, --python       # Python projects only
--ts, --typescript   # TypeScript projects only
--sol, --solidity    # Solidity projects only

# Status filters (AND logic with language filters):
--active             # Active projects (default)
--lib                # Library projects
--archive            # Archived projects
--all                # All projects

# Output:
--json               # Output as JSON
```

**Examples:**
```bash
overlord list                      # Active projects (default)
overlord list --all                # All projects
overlord list --py --lib           # Python libraries
overlord list --py --ts            # Python OR TypeScript active projects
overlord list --py --ts --all      # All Python and TypeScript projects
overlord list --archive            # Archived projects
```

### overlord add

Register existing project directories in the Overlord registry.

```bash
overlord add <name> <path> [options]

# Arguments:
name      # Project name (must be unique)
path      # Path to existing project directory

# Language flags (optional, auto-detects if omitted):
--py, --python       # Python project
--ts, --typescript   # TypeScript project
--sol, --solidity    # Solidity project

# Options:
--lib                # Mark as library instead of active
--alias <name>       # Add alias (can be used multiple times)
--force              # Overwrite existing project entry
```

**Examples:**
```bash
overlord add myproject /path/to/project                    # Auto-detect
overlord add mylib . --lib --alias ml --alias mylib        # Add current dir as library
overlord add overlord ~/bin/overlord --force                # Overwrite existing
```

### overlord init

Initialize existing directory with Overlord configuration.

```bash
overlord init [path] [options]

# Arguments:
path      # Directory to initialize (default: current directory)

# Language flags (optional, auto-detects if omitted):
--py, --python       # Python project
--ts, --typescript   # TypeScript project
--sol, --solidity    # Solidity project
--base               # Force base configuration (no language-specific)

# Options:
--lib                # Set status to 'lib' instead of 'active'
--name <name>        # Override project name (default: directory basename)
--alias <alias>      # Add alias (can be used multiple times)
--no-git             # Skip git initialization
--force              # Overwrite existing .tmux.local, Makefile, and .opencode/opencode.jsonc
```

**Language auto-detection:**
- `pyproject.toml` or `setup.py` → Python
- `package.json` → TypeScript
- `foundry.toml` → Solidity
- None detected → base

**Creates:**
- `.tmux.local` - Workspace configuration
- `Makefile` - Build/test commands
- `.opencode/opencode.jsonc` - AI assistant instructions (migrates existing from root if found)
- `thoughts/` - Development artifact structure (never overwritten)

**Examples:**
```bash
overlord init                              # Initialize current dir, auto-detect language
overlord init /path/to/repo --py           # Initialize specific path as Python
overlord init --base --name myproj         # Force base config with custom name
overlord init --ts --alias mp --lib        # TypeScript library with alias
```

### overlord mv

Move project between status categories. Physically moves directory and updates registry.

```bash
overlord mv <name> <status>

# Arguments:
name      # Project name or alias
status    # Target: active, lib, or archive
```

**Examples:**
```bash
overlord mv myproject active     # Activate project
overlord mv myproject lib        # Mark as library
overlord mv myproject archive    # Archive project
```

### overlord rm

Remove project from the Overlord registry (does not delete project files).

```bash
overlord rm <name> [options]

# Arguments:
name      # Project name, alias, or absolute path

# Options:
--force, -f          # Skip confirmation prompt
--help, -h           # Show this help
```

**Lookup priority:**
1. Project name
2. Project alias
3. Absolute path

**Behavior:**
- Creates registry backup at `~/bin/overlord/registry.json.bak`
- Shows confirmation prompt with project details (unless `--force` is used)
- Removes project from registry only; project files remain on disk

**Examples:**
```bash
overlord rm myproject                 # Remove by name
overlord rm mp                        # Remove by alias
overlord rm /path/to/project          # Remove by path
overlord rm myproject --force          # Skip confirmation
```

### overlord open

Open project workspace with tmux. Uses fzf for fuzzy search.

```bash
overlord open [name]

# Behavior:
# - No argument: Launch fzf picker (active/lib projects)
# - Exact match: Open immediately
# - No exact match: fzf pre-filtered with query
# - Archived projects cannot be opened (must move to active first)
```

**Examples:**
```bash
overlord open              # Interactive fzf picker
overlord open myproject    # Open exact match
overlord open mypr         # Fuzzy search
```

### overlord info

Show detailed project information.

```bash
overlord info <name>
```

**Output includes:**
- Name, language, status
- Path and aliases
- Creation date
- Directory existence check
- Git and .tmux.local status

### overlord config

Open registry.json in editor ($EDITOR or nvim).

```bash
overlord config
```

Use this to manually add aliases to projects:
```json
{
  "projects": {
    "poe-supabase-utils": {
      "lang": "python",
      "status": "lib",
      "path": "/home/thomas/Work/Python/libs/poe-supabase-utils",
      "created": "2024-12-15",
      "aliases": ["psu", "supabase"]
    }
  }
}
```

### overlord edit

Open overlord scripts directory in editor.

```bash
overlord edit
```

### overlord sync

Propagate Makefile templates, .opencode/opencode.jsonc, and thoughts/ directory to all registered projects.
Handles migration of legacy root-level `opencode.jsonc` to the `.opencode/` directory.
By default, only creates files if missing. Use `--force` to overwrite existing files.

```bash
overlord sync [options]

# Language filters:
--py, --python       # Sync only Python projects
--ts, --typescript   # Sync only TypeScript projects
--sol, --solidity    # Sync only Solidity projects

# Options:
--force              # Overwrite existing Makefile and opencode.jsonc
--dry-run            # Preview changes without writing files
```

**Examples:**
```bash
overlord sync                  # Sync all projects (create if missing)
overlord sync --force          # Sync all projects (overwrite existing)
overlord sync --py             # Sync only Python projects
overlord sync --dry-run        # Preview what would be synced
```

**Note:** Sync respects existing files by default. The `thoughts/` directory is always additive - existing content is never removed, even with `--force`.

## Makefile System

Each project gets a generated `Makefile` combining:
1. **base.mk** - Git worktree management (shared across all languages)
2. **Language-specific template** - Build/test/lint commands

### Worktree Commands (all projects)

```bash
make help                           # Show available commands
make worktree-new [BRANCH=feature]  # Create worktree + tmux session (auto-names if omitted)
make worktree-list                  # List all worktrees
make worktree-attach BRANCH=feature # Attach to worktree's tmux session
make worktree-sessions              # List tmux sessions for all worktrees
make worktree-remove BRANCH=feature # Remove worktree and kill tmux session
make worktree-setup                 # Run .worktree-setup.sh if present
```

**Auto-naming examples:**
```bash
make worktree-new                   # Creates swift_fix_00, bright_fix_01, etc.
make worktree-new BRANCH=my-feature # Creates my-feature worktree
```

**Naming system:**
- Auto-generated names follow the pattern: `{adjective}_{noun}_{counter:02d}`
- Provides 400 unique combinations (20 adjectives × 20 nouns)
- Counter file stored at `.worktrees/.counter` (automatically gitignored)

### Cross-Session Communication Commands

Send commands to and read output from worktree sessions without leaving your main workspace:

```bash
make worktree-send BRANCH=<name> WINDOW=<window> CMD="<command>"  # Send command to worktree session
make worktree-read BRANCH=<name> WINDOW=<window>                  # Capture visible pane content from worktree
```

**Examples:**
```bash
# Send test command from main repo to worktree
make worktree-send BRANCH=feature-auth WINDOW=shell CMD="make test"

# Read test output without attaching
make worktree-read BRANCH=feature-auth WINDOW=shell

# Send git command to git window
make worktree-send BRANCH=feature-auth WINDOW=git CMD="git status"
```

**Window validation:** Non-existent windows show helpful error messages with list of available windows in the session.

### Current Session Utilities Commands (run from within tmux)

Convenient shortcuts for working within your active tmux session without specifying session names:

```bash
make tmux-send WINDOW=<window> CMD="<command>"  # Send command to window in current session
make tmux-read WINDOW=<window>                  # Capture visible pane content from current session
make tmux-list                                  # List all windows in current tmux session
```

**Examples:**
```bash
# Send test command to shell window in current session
make tmux-send WINDOW=shell CMD="make test"

# Read test output
make tmux-read WINDOW=shell

# Show all available windows with active marker
make tmux-list
```

**Requirements:** Commands must be run from within an active tmux session. Running outside tmux will show a helpful error message.

### Python Commands

```bash
make install    # uv sync
make test       # uv run pytest
make lint       # uv run ruff check .
make format     # uv run ruff format .
make clean      # Remove build artifacts
```

### TypeScript Commands

```bash
make install    # pnpm/bun install (auto-detects)
make build      # pnpm/bun run build
make dev        # pnpm/bun run dev
make test       # pnpm/bun run test
make lint       # pnpm/bun run lint
make clean      # Remove dist, cache
```

### Solidity Commands

```bash
make install    # forge install
make build      # forge build
make test       # forge test
make test-v     # forge test -vvv
make gas        # forge test --gas-report
make coverage   # forge coverage
make clean      # forge clean
```

### Custom Setup Script

Create `.worktree-setup.sh` in your project root for custom worktree initialization:

```bash
#!/usr/bin/env bash
# .worktree-setup.sh - runs when you call `make worktree-setup`
uv sync
# or: pnpm install
# or: forge install
```

## OpenCode Configuration

Each project gets an `.opencode/opencode.jsonc` file with language-specific AI assistant instructions:

### Python Projects
```json
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/PYTHON_STYLEGUIDE.md"]
}
```

### TypeScript Projects
```json
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/TYPESCRIPT_STYLEGUIDE.md"]
}
```

### Solidity Projects
```json
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/SOLIDITY_STYLEGUIDE.md"]
}
```

### Base Projects (non-language-specific)
```json
{
  "instructions": []
}
```

Templates are stored in `~/bin/overlord/templates/opencode-{lang}.jsonc`.

## Thoughts Directory

Each project gets a `thoughts/` directory for organizing development artifacts:

```
thoughts/
├── tickets/    # Feature requests, bug reports
├── plans/      # Implementation plans
├── logs/       # Development logs, decisions
├── research/   # Research notes, findings
└── handoffs/   # Context for session handoffs
```

This directory is always created additively - existing content is never removed during initialization, sync, or any other operation, regardless of the `--force` flag.

## Registry Format

The registry (`~/bin/overlord/registry.json`) stores project metadata:

```json
{
  "settings": {
    "base_dir": "/home/thomas/Work"
  },
  "projects": {
    "project-name": {
      "lang": "python|typescript|solidity|base",
      "status": "active|lib|archive",
      "path": "/absolute/path/to/project",
      "created": "YYYY-MM-DD",
      "aliases": ["alias1", "alias2"]
    }
  }
}
```

## Tmux Integration

Each project can have a `.tmux.local` file for custom workspace setup. Templates are provided per language in `~/bin/overlord/tmux/`:
- `base.tmux` - Used for non-language-specific projects
- Language-specific templates - `python.tmux`, `typescript.tmux`, `solidity.tmux`

**Template variables:**
- `$TMUX_SESSION` - Session name
- `$TMUX_PROJECT_DIR` - Project root directory

**Modes:**
- `MODE=override` (default) - Template fully controls layout
- `MODE=merge` - Default layout first, then template additions

## Migration

For existing projects, use the migration script:

```bash
# Preview changes (default)
overlord-migrate-init --dry-run

# Execute migration
overlord-migrate-init --execute
```

**Migration behavior:**
- Creates active/, libs/, archive/ under each language
- Moves all projects to archive/ by default
- Moves `poe-*-utils` projects to libs/
- Generates registry.json
- Backs up existing registry

**Configuration Migration:**
Newer versions of Overlord automatically move `opencode.jsonc` to the `.opencode/` directory during `overlord init` or `overlord sync` operations.

## Design Decisions

1. **Status categories**: Three tiers (active/lib/archive) balance organization with simplicity
2. **Archive restriction**: Archived projects must be activated before opening to keep workspace clean
3. **Fuzzy search**: fzf integration for fast project selection
4. **Registry-based**: JSON registry enables metadata and aliases without filesystem complexity
5. **Language templates**: Per-language tmux configurations for appropriate dev environments
6. **Subcommand architecture**: Each command is a separate script for modularity and AI tool integration
7. **Worktree auto-naming**: Counter-based naming generates unique worktree names (400 combinations from 20 adjectives × 20 nouns), with tmux sessions created automatically for each worktree
8. **Integrated cleanup**: Removing a worktree automatically kills its associated tmux session, ensuring no orphaned sessions
9. **Cross-session communication**: `worktree-send` and `worktree-read` allow coordinating multiple worktrees from the main session without switching
10. **Session-local shortcuts**: `tmux-send`, `tmux-read`, and `tmux-list` provide convenient shortcuts for operations within the current session
11. **Centralized Helpers**: `lib/common.sh` provides unified logging, language detection, and template handling logic across all subcommands.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OVERLORD_BASE_DIR` | Registry or `$HOME/Work` | Root directory for project categories |
| `OVERLORD_CONFIG` | script location | Directory for registry and templates |
| `OVERLORD_BIN` | script location | Directory for overlord scripts |
| `OVERLORD_STRICT` | `false` | Enable strict error handling for templates |
| `EDITOR` | `nvim` | Editor for config/edit |

## Future Considerations

This system is designed to be operated by an AI agent. Each subcommand can be invoked as a tool:

- `overlord-new` - Create projects
- `overlord-add` - Register existing projects
- `overlord-list` - Query project state
- `overlord-mv` - Change project status
- `overlord-open` - Launch workspaces
- `overlord-info` - Get project details
- `overlord-sync` - Propagate Makefiles
- `overlord-config` - Modify registry

The `--json` flag on `overlord list` provides machine-readable output for AI consumption.

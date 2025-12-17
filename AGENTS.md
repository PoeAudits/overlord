# Overlord - Project Management System

## Overview

Overlord is a CLI-based project management system for organizing Python, TypeScript, and Solidity projects. It manages project lifecycle through status categories (active, lib, archive) and provides tmux workspace integration.

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

~/.config/overlord/
├── registry.json        # Project metadata
├── tmux/
│   ├── base.tmux        # Base tmux template (non-language projects)
│   ├── python.tmux      # Python tmux template
│   ├── typescript.tmux  # TypeScript tmux template
│   └── solidity.tmux    # Solidity tmux template
└── makefiles/
    ├── base.mk          # Git worktree commands (shared)
    ├── python.mk        # Python/uv commands
    ├── typescript.mk    # TypeScript/pnpm/bun commands
    └── solidity.mk      # Solidity/forge commands

~/bin/overlord/
├── overlord             # Main dispatcher
├── overlord-new         # Create new project
├── overlord-list        # List projects
├── overlord-mv          # Move project status
├── overlord-open        # Open workspace
├── overlord-info        # Show project details
├── overlord-sync        # Propagate Makefiles to projects
├── overlord-config      # Edit registry.json
├── overlord-edit        # Edit overlord scripts
├── overlord-migrate-init # One-time migration script
└── AGENTS.md            # This documentation
```

## Commands

### overlord (default: list)

Main dispatcher. Running `overlord` with no arguments lists active projects.

```bash
overlord                    # List active projects
overlord --help             # Show help
overlord --version          # Show version
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

Propagate Makefile templates to all registered projects. Combines `base.mk` (git worktree commands) with language-specific templates into each project's `Makefile`.

```bash
overlord sync [options]

# Language filters:
--py, --python       # Sync only Python projects
--ts, --typescript   # Sync only TypeScript projects
--sol, --solidity    # Sync only Solidity projects

# Options:
--dry-run            # Preview changes without writing files
```

**Examples:**
```bash
overlord sync                  # Sync all projects
overlord sync --py             # Sync only Python projects
overlord sync --dry-run        # Preview what would be synced
```

**Note:** Sync overwrites existing Makefiles. Projects can customize behavior via `.worktree-setup.sh` script which is called by `make worktree-setup`.

## Makefile System

Each project gets a generated `Makefile` combining:
1. **base.mk** - Git worktree management (shared across all languages)
2. **Language-specific template** - Build/test/lint commands

### Worktree Commands (all projects)

```bash
make help                           # Show available commands
make worktree-new BRANCH=feature    # Create worktree at .worktrees/feature
make worktree-list                  # List all worktrees
make worktree-remove BRANCH=feature # Remove worktree
make worktree-setup                 # Run .worktree-setup.sh if present
```

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

## Registry Format

The registry (`~/.config/overlord/registry.json`) stores project metadata:

```json
{
  "projects": {
    "project-name": {
      "lang": "python|typescript|solidity",
      "status": "active|lib|archive",
      "path": "/absolute/path/to/project",
      "created": "YYYY-MM-DD",
      "aliases": ["alias1", "alias2"]
    }
  }
}
```

## Tmux Integration

Each project can have a `.tmux.local` file for custom workspace setup. Templates are provided per language in `~/.config/overlord/tmux/`:
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

## Design Decisions

1. **Status categories**: Three tiers (active/lib/archive) balance organization with simplicity
2. **Archive restriction**: Archived projects must be activated before opening to keep workspace clean
3. **Fuzzy search**: fzf integration for fast project selection
4. **Registry-based**: JSON registry enables metadata and aliases without filesystem complexity
5. **Language templates**: Per-language tmux configurations for appropriate dev environments
6. **Subcommand architecture**: Each command is a separate script for modularity and AI tool integration

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OVERLORD_CONFIG` | `~/.config/overlord` | Config directory |
| `OVERLORD_BIN` | `~/bin/overlord` | Scripts directory |
| `EDITOR` | `nvim` | Editor for config/edit |

## Future Considerations

This system is designed to be operated by an AI agent. Each subcommand can be invoked as a tool:

- `overlord-new` - Create projects
- `overlord-list` - Query project state
- `overlord-mv` - Change project status
- `overlord-open` - Launch workspaces
- `overlord-info` - Get project details
- `overlord-sync` - Propagate Makefiles
- `overlord-config` - Modify registry

The `--json` flag on `overlord list` provides machine-readable output for AI consumption.

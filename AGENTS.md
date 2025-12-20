# Overlord - Project Management System

CLI for managing Python, TypeScript, and Solidity projects with tmux workspace integration.

## Quick Reference

```bash
overlord                     # List active projects
overlord <name>              # Open workspace (shortcut for 'overlord open')
overlord new <name> --py|--ts|--sol [--lib] [--force]
overlord add <name> <path> [--py|--ts|--sol] [--lib] [--alias <n>]
overlord init [path] [--py|--ts|--sol|--base] [--lib] [--name <n>] [--force]
overlord list [--py|--ts|--sol] [--active|--lib|--archive|--all] [--json] [--edit|-e]
overlord mv <name> active|lib|archive
overlord rm <name> [--force]
overlord open [name] [--fuzzy] [--force]
overlord info <name>
overlord sync [--py|--ts|--sol] [--force] [--dry-run]
overlord inject <name>       # Add project as path-based dependency
overlord detect              # Find unregistered projects
overlord config [--import <path>]
overlord edit
overlord uninstall [--dry-run] [--force]
```

## Installation & Directory Layout

Overlord installs to `~/.config/overlord` with a symlink at `/usr/local/bin/overlord`:

```
/usr/local/bin/overlord           # Symlink to ~/.config/overlord/overlord

~/.config/overlord/
├── overlord                       # Main dispatcher
├── overlord-*                     # Subcommands
├── lib/common.sh                  # Shared helpers (logging, detection, templates)
├── registry.json                  # Project metadata
├── templates/opencode-{base,python,typescript,solidity}.jsonc
├── tmux/{base,python,typescript,solidity}.tmux
└── makefiles/{base,python,typescript,solidity}.mk

~/Work/{Python,Typescript,Solidity}/{active,libs,archive}/  # Projects directory
```

## Registry Format

```json
{
  "settings": { "base_dir": "/home/thomas/Work" },
  "projects": {
    "name": {
      "lang": "python|typescript|solidity|base",
      "status": "active|lib|archive",
      "path": "/absolute/path",
      "created": "YYYY-MM-DD",
      "aliases": ["alias1"]
    }
  }
}
```

## Language Detection

Auto-detected by file presence: `pyproject.toml`/`setup.py` → Python, `package.json` → TypeScript, `foundry.toml` → Solidity.

## Project Initialization

`overlord init` and `overlord new` create:
- `.tmux.local` - Workspace layout
- `Makefile` - Combined from `base.mk` + language-specific `.mk`
- `.opencode/opencode.jsonc` - AI instructions (migrates legacy root-level file)
- `thoughts/{tickets,plans,logs,research,handoffs}/` - Never overwritten

## Makefile Commands

**All projects (from base.mk):**
```bash
make worktree-new [BRANCH=name]     # Create worktree + tmux session
make worktree-list                  # List worktrees
make worktree-attach BRANCH=name    # Attach to session
make worktree-remove BRANCH=name    # Remove worktree + kill session
make worktree-send BRANCH=x WINDOW=y CMD="z"   # Send command to worktree
make worktree-read BRANCH=x WINDOW=y           # Read worktree output
make tmux-send WINDOW=x CMD="y"     # Send to current session
make tmux-read WINDOW=x             # Read from current session
make tmux-list                      # List windows in current session
```

**Python:** `make install|test|lint|format|clean` (uses uv, ruff, pytest)
**TypeScript:** `make install|build|dev|test|lint|clean` (uses pnpm/bun)
**Solidity:** `make install|build|test|test-v|gas|coverage|clean` (uses forge)

## Tmux Templates

Templates use variables `$TMUX_SESSION` and `$TMUX_PROJECT_DIR`. Modes: `MODE=override` (default) or `MODE=merge`.

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OVERLORD_BASE_DIR` | Registry or `$HOME/Work` | Project root |
| `OVERLORD_CONFIG` | auto-detected from symlink | `~/.config/overlord` |
| `OVERLORD_BIN` | auto-detected from symlink | `~/.config/overlord` |
| `EDITOR` | `nvim` | Editor for config/edit |

## Installation & Uninstallation

**Installation:**
```bash
git clone <repo> ~/tmp/overlord-setup
~/tmp/overlord-setup/setup.sh
# Script will:
# 1. Move repo to ~/.config/overlord
# 2. Create symlink: /usr/local/bin/overlord → ~/.config/overlord/overlord
# 3. Initialize registry at ~/.config/overlord/registry.json
# 4. Automatically delete the temporary clone when done
```

**Updating:**
Simply re-run the setup.sh from a fresh clone to update to the latest version.

**Uninstalling:**
```bash
overlord uninstall [--dry-run] [--force]
# Removes: ~/.config/overlord/, /usr/local/bin/overlord symlink
# Preserves: All project directories and files
```

## Key Behaviors

- **Archived projects** must be moved to active/lib before opening
- **`overlord rm`** only removes from registry, not disk
- **`overlord sync`** creates files only if missing unless `--force`
- **`thoughts/`** is always additive, never overwritten
- **Lookup priority** for names: project name → alias → absolute path
- **`--json`** on `overlord list` for machine-readable output
- **`--edit`** / **`-e`** on `overlord list` to enter interactive project state editor
- **`overlord inject`** adds a registered project as a path-based local dependency (bash-native TOML parsing, zero external dependencies)

## Script Patterns

### Standard Header

```bash
#!/usr/bin/env bash
set -euo pipefail

OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
OVERLORD_BIN="${OVERLORD_BIN:-$OVERLORD_REPO}"

source "$OVERLORD_BIN/lib/common.sh"
```

### Common.sh Functions

```bash
# Logging (errors go to stderr)
log_info "message"      # Blue >
log_success "message"   # Green ✓
log_error "message"     # Red ✗
log_warning "message"   # Yellow !

# Language detection
LANGUAGE=$(detect_language "$PROJECT_PATH")  # Returns: python|typescript|solidity|base

# Template operations
copy_tmux_template "$PROJECT_PATH" "$LANGUAGE"
copy_opencode_template "$PROJECT_PATH" "$LANGUAGE"
generate_makefile "$PROJECT_PATH" "$LANGUAGE"
create_thoughts_dirs "$PROJECT_PATH"

# Validation
validate_project_name "$name"
PROJECT_PATH=$(validate_path "$path")  # Expands ~, ., validates exists
```

### Registry Operations

```bash
# Check existence
jq -e --arg name "$name" '.projects[$name]' "$OVERLORD_REGISTRY" >/dev/null 2>&1

# Find by name or alias
jq -r --arg q "$query" '
  .projects | to_entries[] | 
  select(.key == $q or (.value.aliases // [] | index($q))) |
  [.key, .value.lang, .value.status, .value.path] | @tsv
' "$OVERLORD_REGISTRY"

# Atomic update (always use temp file + mv)
tmp_file=$(mktemp)
trap 'rm -f "$tmp_file"' EXIT
jq --arg name "$name" ... "$OVERLORD_REGISTRY" > "$tmp_file"
mv "$tmp_file" "$OVERLORD_REGISTRY"
```

### Argument Parsing

```bash
PROJECT_NAME=""
LANGUAGE=""
FORCE=false
ALIASES=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --py|--python) LANGUAGE="python" ;;
    --force) FORCE=true ;;
    --alias) shift; ALIASES+=("$1") ;;
    --help|-h) usage; exit 0 ;;
    -*) log_error "Unknown option: $1"; exit 1 ;;
    *) PROJECT_NAME="$1" ;;
  esac
  shift
done
```

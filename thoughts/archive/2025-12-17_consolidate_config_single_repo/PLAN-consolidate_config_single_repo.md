# Consolidate Overlord Configuration into Single Repository - Implementation Plan

## Overview

Restructure Overlord so all configuration (templates, makefiles, scripts) lives in a single git repository that can be cloned and used across multiple computers. The main `overlord` command will be symlinked to `/usr/local/bin/` for PATH access, while all configuration and helper scripts remain in the cloned repository.

## Current State Analysis

**Current structure:**
```
~/bin/overlord/                    (git repo)
├── overlord                       (main dispatcher)
├── overlord-*                     (12 subcommand scripts)
├── AGENTS.md
├── thoughts/

~/.config/overlord/                (separate location)
├── registry.json
├── templates/
│   └── opencode-*.jsonc
├── tmux/
│   ├── base.tmux
│   ├── python.tmux
│   ├── typescript.tmux
│   └── solidity.tmux
└── makefiles/
    ├── base.mk
    ├── python.mk
    ├── typescript.mk
    └── solidity.mk
```

**Key constraints discovered:**
- All scripts use `OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"` to locate config files (overlord:8, overlord-add:7, overlord-init:7, etc.)
- Main dispatcher uses `OVERLORD_BIN="${OVERLORD_BIN:-$HOME/bin/overlord}"` to locate subcommands (overlord:10)
- Registry location is derived: `OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"` (overlord:9)
- One hardcoded path exists: `$HOME/bin/overlord-open` in overlord-new:406
- Templates/makefiles/tmux files are read from `$OVERLORD_CONFIG/` subdirectories

## Desired End State

After running setup script on a fresh clone:

```
/path/to/cloned/overlord-repo/
├── overlord                       (dispatcher - source)
├── overlord-*                     (all subcommand scripts)
├── setup.sh                       (new: installation script)
├── registry.json                  (gitignored, created by setup)
├── templates/
│   └── opencode-*.jsonc
├── tmux/
│   ├── base.tmux
│   ├── python.tmux
│   ├── typescript.tmux
│   └── solidity.tmux
├── makefiles/
│   ├── base.mk
│   ├── python.mk
│   ├── typescript.mk
│   └── solidity.mk
├── AGENTS.md
├── thoughts/

/usr/local/bin/overlord           (symlink → repo/overlord)
```

**Verification criteria:**
- Clone repo to new location
- Run `./setup.sh`
- `overlord` command works from any directory
- All subcommands locate config files correctly
- Registry operations work

## What We're NOT Doing

- Migrating existing user installations automatically (out of scope)
- Creating migration tools for existing `~/.config/overlord/` setups
- Backwards compatibility with old dual-location setup
- Publishing to package managers (homebrew, apt, etc.)

## Implementation Approach

**Auto-detection strategy:**
Scripts will use symlink resolution to find the repository directory without requiring environment variables. The main `overlord` dispatcher will:
1. Resolve its own symlink using `realpath "$0"` 
2. Find the repository directory using `dirname`
3. Set OVERLORD_CONFIG internally to point to the repo
4. Pass this configuration to all subcommands via environment

This ensures:
- No user environment variables required
- Works regardless of clone location
- Standard symlink approach (like homebrew CLIs)
- Automatic updates via `git pull`

---

## Phase 1: Repository Restructuring

### Overview
Move configuration files from `~/.config/overlord/` into the repository and update git configuration.

### Changes Required:

#### 1. Move configuration directories into repository
**Action**: Move files from `~/.config/overlord/` to `~/bin/overlord/`

```bash
# Move directories
mv ~/.config/overlord/templates ~/bin/overlord/
mv ~/.config/overlord/makefiles ~/bin/overlord/
mv ~/.config/overlord/tmux ~/bin/overlord/

# Note: Do NOT move registry.json - it's user-specific
```

**Files affected:**
- `templates/opencode-base.jsonc`
- `templates/opencode-python.jsonc`
- `templates/opencode-typescript.jsonc`
- `templates/opencode-solidity.jsonc`
- `makefiles/base.mk`
- `makefiles/python.mk`
- `makefiles/typescript.mk`
- `makefiles/solidity.mk`
- `tmux/base.tmux`
- `tmux/python.tmux`
- `tmux/typescript.tmux`
- `tmux/solidity.tmux`

#### 2. Update .gitignore
**File**: `.gitignore`

Add registry.json to gitignore:
```gitignore
# Worktrees directory (used for testing)
.worktrees/

# User-specific registry (created by setup.sh)
registry.json
registry.json.bak
registry.json.pre-migration.*
```

#### 3. Remove AGENTS.md from config directory
**Action**: Delete `~/.config/overlord/AGENTS.md` (duplicate exists in repo)

```bash
rm ~/.config/overlord/AGENTS.md
```

### Success Criteria:

#### Automated Verification:
- [x] All template files exist in repo: `ls -R templates/ makefiles/ tmux/`
- [x] `.gitignore` includes registry.json: `grep registry.json .gitignore`
- [x] Git status shows new files: `git status`

#### Manual Verification:
- [x] All 4 opencode templates present in `templates/`
- [x] All 4 makefiles present in `makefiles/`
- [x] All 4 tmux templates present in `tmux/`
- [x] `~/.config/overlord/` directory can be safely removed

---

## Phase 2: Script Path Resolution Updates

### Overview
Update all scripts to auto-detect repository location instead of relying on environment variables or hardcoded paths.

### Changes Required:

#### 1. Update main overlord dispatcher
**File**: `overlord`

Replace environment variable defaults (lines 8-10) with auto-detection:

```bash
# OLD (lines 8-10):
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
OVERLORD_BIN="${OVERLORD_BIN:-$HOME/bin/overlord}"

# NEW:
# Auto-detect repository directory (handles symlinked installation)
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
OVERLORD_BIN="$OVERLORD_REPO"
export OVERLORD_CONFIG OVERLORD_BIN
```

Update help text (line 44):
```bash
# OLD:
  OVERLORD_CONFIG   Config directory (default: ~/.config/overlord)

# NEW:
  OVERLORD_CONFIG   Config directory (default: auto-detected from script location)
```

#### 2. Update overlord-add
**File**: `overlord-add`

Replace lines 7-8:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
# Use parent-provided OVERLORD_CONFIG or auto-detect
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 3. Update overlord-config
**File**: `overlord-config`

Replace lines 6-7:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 4. Update overlord-edit
**File**: `overlord-edit`

Replace line 6:
```bash
# OLD:
OVERLORD_BIN="${OVERLORD_BIN:-$HOME/bin/overlord}"

# NEW:
OVERLORD_BIN="${OVERLORD_BIN:-$(cd "$(dirname "$(realpath "$0")")" && pwd)}"
```

#### 5. Update overlord-info
**File**: `overlord-info`

Replace lines 6-7:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 6. Update overlord-init
**File**: `overlord-init`

Replace lines 7-8:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 7. Update overlord-list
**File**: `overlord-list`

Replace lines 7-8:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 8. Update overlord-mv
**File**: `overlord-mv`

Replace lines 7-8:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 9. Update overlord-new
**File**: `overlord-new`

Replace lines 7-8:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

Replace hardcoded path at line 406:
```bash
# OLD:
    exec "$HOME/bin/overlord-open" "$PROJECT_NAME"

# NEW:
    exec "$OVERLORD_REPO/overlord-open" "$PROJECT_NAME"
```

#### 10. Update overlord-open
**File**: `overlord-open`

Replace lines 8-9:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 11. Update overlord-rm
**File**: `overlord-rm`

Replace lines 7-8:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 12. Update overlord-sync
**File**: `overlord-sync`

Replace lines 7-9:
```bash
# OLD:
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
MAKEFILE_DIR="$OVERLORD_CONFIG/makefiles"

# NEW:
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
MAKEFILE_DIR="$OVERLORD_CONFIG/makefiles"
```

### Success Criteria:

#### Automated Verification:
- [x] All scripts use auto-detection: `grep -l 'OVERLORD_REPO=' overlord*`
- [x] No hardcoded `$HOME/bin/overlord` references: `! grep -r '\$HOME/bin/overlord' overlord* || true`
- [x] Scripts are executable: `ls -l overlord* | grep '^-rwxr'`

#### Manual Verification:
- [x] Run `./overlord list` from repo directory works
- [x] Config files are found at correct location
- [x] No errors about missing directories

---

## Phase 3: Setup Script Creation

### Overview
Create an idempotent setup script that installs overlord on a fresh system.

### Changes Required:

#### 1. Create setup.sh script
**File**: `setup.sh`

```bash
#!/usr/bin/env bash
# setup.sh: Install overlord CLI tool
# Run this after cloning the repository

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}>${NC} $*"; }
log_success() { echo -e "${GREEN}✓${NC} $*"; }
log_error() { echo -e "${RED}✗${NC} $*" >&2; }
log_warning() { echo -e "${YELLOW}!${NC} $*"; }

# Detect repository directory
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_TARGET="/usr/local/bin/overlord"
REGISTRY_FILE="$REPO_DIR/registry.json"

echo ""
log_info "Overlord Setup"
log_info "Repository: $REPO_DIR"
echo ""

# Check if repository looks valid
if [[ ! -f "$REPO_DIR/overlord" ]]; then
  log_error "Repository structure invalid: overlord script not found"
  exit 1
fi

if [[ ! -d "$REPO_DIR/templates" ]] || [[ ! -d "$REPO_DIR/makefiles" ]] || [[ ! -d "$REPO_DIR/tmux" ]]; then
  log_error "Repository structure invalid: missing templates, makefiles, or tmux directories"
  exit 1
fi

# Check if /usr/local/bin is writable (or use sudo)
NEED_SUDO=false
if [[ ! -w "/usr/local/bin" ]]; then
  log_warning "/usr/local/bin requires sudo access"
  NEED_SUDO=true
fi

# Create symlink to /usr/local/bin/overlord
log_info "Installing overlord to $INSTALL_TARGET..."

if [[ -L "$INSTALL_TARGET" ]]; then
  # Existing symlink
  CURRENT_TARGET="$(readlink "$INSTALL_TARGET")"
  if [[ "$CURRENT_TARGET" == "$REPO_DIR/overlord" ]]; then
    log_success "Symlink already points to this repository"
  else
    log_warning "Existing symlink points to: $CURRENT_TARGET"
    read -r -p "Replace with this repository? (y/N): " response
    case "$response" in
      [yY]|[yY][eE][sS])
        if [[ "$NEED_SUDO" == true ]]; then
          sudo rm "$INSTALL_TARGET"
          sudo ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
        else
          rm "$INSTALL_TARGET"
          ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
        fi
        log_success "Symlink updated"
        ;;
      *)
        log_warning "Keeping existing symlink"
        ;;
    esac
  fi
elif [[ -f "$INSTALL_TARGET" ]]; then
  # Regular file exists
  log_error "Regular file exists at $INSTALL_TARGET"
  log_error "Please remove it manually and re-run setup"
  exit 1
else
  # Create new symlink
  if [[ "$NEED_SUDO" == true ]]; then
    sudo ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
  else
    ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
  fi
  log_success "Symlink created: $INSTALL_TARGET → $REPO_DIR/overlord"
fi

# Initialize registry.json if missing
if [[ ! -f "$REGISTRY_FILE" ]]; then
  log_info "Initializing registry..."
  echo '{"projects":{}}' > "$REGISTRY_FILE"
  log_success "Created empty registry: $REGISTRY_FILE"
else
  log_success "Registry already exists: $REGISTRY_FILE"
fi

# Verify installation
echo ""
log_info "Verifying installation..."

if command -v overlord &>/dev/null; then
  VERSION_OUTPUT="$(overlord --version 2>&1 || true)"
  log_success "overlord command is available"
  log_info "$VERSION_OUTPUT"
else
  log_error "overlord command not found in PATH"
  log_error "You may need to add /usr/local/bin to your PATH"
  exit 1
fi

# Success message
echo ""
log_success "Overlord installation complete!"
echo ""
log_info "Quick start:"
echo "  overlord new myproject --py      # Create new Python project"
echo "  overlord list                    # List projects"
echo "  overlord open                    # Open project workspace"
echo "  overlord --help                  # Show all commands"
echo ""
log_info "Configuration directory: $REPO_DIR"
log_info "Registry location: $REGISTRY_FILE"
echo ""
```

#### 2. Make setup.sh executable
**Action**: `chmod +x setup.sh`

#### 3. Update README or AGENTS.md with setup instructions
**File**: `AGENTS.md`

Add installation section after the "Overview" section:

```markdown
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
```

### Success Criteria:

#### Automated Verification:
- [x] `setup.sh` is executable: `test -x setup.sh`
- [x] `setup.sh` syntax is valid: `bash -n setup.sh`
- [x] Installation section exists in AGENTS.md: `grep "## Installation" AGENTS.md`

#### Manual Verification:
- [x] Run `./setup.sh` in a test directory
- [x] Symlink created at `/usr/local/bin/overlord`
- [x] `registry.json` created with empty projects object
- [x] `overlord --version` works from any directory
- [x] Script is idempotent (running twice doesn't break anything)

---

## Phase 4: Testing & Verification

### Overview
Comprehensive testing to ensure the consolidated repository works correctly.

### Testing Steps:

#### 1. Test fresh installation workflow

**Manual steps:**
```bash
# Simulate fresh installation
cd /tmp
git clone ~/bin/overlord overlord-test
cd overlord-test
./setup.sh

# Verify commands work
overlord --version
overlord list
overlord --help

# Test creating a project
overlord new test-project --py --no-open
overlord list
overlord info test-project

# Cleanup
overlord rm test-project --force
cd ~
rm -rf /tmp/overlord-test
```

#### 2. Test registry operations

**Manual steps:**
```bash
# From installed overlord
overlord new mytest --ts --no-open
overlord list --all
overlord mv mytest lib
overlord list --lib
overlord info mytest
overlord rm mytest --force
```

#### 3. Test config file access

**Manual steps:**
```bash
# Verify templates are accessible
overlord init /tmp/test-init-py --py --name test-init
ls -la /tmp/test-init-py/.tmux.local
ls -la /tmp/test-init-py/Makefile
ls -la /tmp/test-init-py/opencode.jsonc
overlord rm test-init --force
rm -rf /tmp/test-init-py

# Test sync command
overlord new sync-test --sol --no-open
rm ~/Work/Solidity/active/sync-test/Makefile
overlord sync --sol
ls -la ~/Work/Solidity/active/sync-test/Makefile
overlord rm sync-test --force
```

#### 4. Test symlink resolution

**Manual steps:**
```bash
# Verify symlink works
ls -la /usr/local/bin/overlord
readlink /usr/local/bin/overlord

# Test from different directories
cd /
overlord --version
cd /tmp
overlord list
cd ~
overlord list
```

#### 5. Verify no environment variable dependency

**Manual steps:**
```bash
# Unset env vars and test
unset OVERLORD_CONFIG OVERLORD_BIN
overlord list
overlord --version

# Verify OVERLORD_CONFIG override still works
OVERLORD_CONFIG=/tmp/test-config overlord list
```

### Success Criteria:

#### Automated Verification:
- [x] Symlink exists: `test -L /usr/local/bin/overlord`
- [x] Symlink target is correct: `readlink /usr/local/bin/overlord`
- [x] Registry file exists in repo: `test -f <repo>/registry.json`
- [x] All template files exist: `ls templates/*.jsonc makefiles/*.mk tmux/*.tmux`
- [x] No files remain in `~/.config/overlord/`: `! test -d ~/.config/overlord || ls -A ~/.config/overlord | wc -l -eq 0`

#### Manual Verification:
- [x] Fresh installation completes without errors
- [x] All commands work from any directory
- [x] Config files are found correctly
- [x] Registry operations succeed
- [x] Sync command propagates templates
- [x] No dependency on OVERLORD_CONFIG environment variable
- [x] Setup script is idempotent

---

## Performance Considerations

**Symlink resolution overhead:**
- `realpath` adds minimal overhead (<1ms per invocation)
- Only executed once per command at startup
- Negligible impact on user experience

**Auto-detection benefits:**
- No environment variable setup required
- Works regardless of clone location
- Simplifies documentation and onboarding

## Migration Notes

**For existing installations:**

Users with existing `~/bin/overlord` installations should:

1. Backup their registry:
   ```bash
   cp ~/.config/overlord/registry.json ~/overlord-registry-backup.json
   ```

2. Pull latest changes:
   ```bash
   cd ~/bin/overlord
   git pull
   ```

3. Run setup script:
   ```bash
   ./setup.sh
   ```

4. Copy registry back:
   ```bash
   cp ~/overlord-registry-backup.json ~/bin/overlord/registry.json
   ```

5. Optionally remove old config directory:
   ```bash
   rm -rf ~/.config/overlord
   ```

**Note:** This migration process is not automated (out of scope).

## References

- Original ticket: `thoughts/tickets/feature_consolidate_config_single_repo.md`
- Current overlord dispatcher: `overlord:1-98`
- Environment variable usage: `overlord:8-10`, `overlord-add:7-8`, etc.
- Symlink resolution pattern: Standard bash practice using `realpath` and `dirname`

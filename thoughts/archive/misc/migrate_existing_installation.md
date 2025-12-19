# Migrate Existing Overlord Installation - Implementation Plan

## Overview

Migrate an existing Overlord installation from the dual-location setup (`~/bin/overlord/` + `~/.config/overlord/`) to the consolidated single-repository structure. This plan is specifically for the current development installation and can be used as a reference for other existing installations.

## Current State Analysis

**Your current setup:**
```
~/bin/overlord/                    (git repo on dev branch)
├── overlord                       (main dispatcher)
├── overlord-*                     (12 subcommand scripts)
├── AGENTS.md
├── thoughts/
├── hack/
├── .gitignore                     (tracks .worktrees/)
└── .tmux.local

~/.config/overlord/                (separate location)
├── registry.json                  (8272 bytes - contains your projects)
├── registry.json.bak              (backup)
├── templates/
│   └── opencode-*.jsonc          (4 files)
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
└── AGENTS.md                      (duplicate)
```

**Git status:**
- Branch: dev (12 commits ahead of origin/dev)
- Modified files: AGENTS.md, Makefile, several thought docs
- Untracked: .gitignore, thoughts/logs/, ticket file

**Key constraints:**
- Active git repository with uncommitted changes
- Registry contains real project data (must be preserved)
- Currently on dev branch with unpushed commits
- Scripts currently use `~/.config/overlord/` for config files

## Desired End State

After migration:

```
~/bin/overlord/                    (git repo)
├── overlord                       (updated with auto-detection)
├── overlord-*                     (all updated with auto-detection)
├── setup.sh                       (new installation script)
├── registry.json                  (migrated from ~/.config/, gitignored)
├── templates/                     (moved from ~/.config/)
│   └── opencode-*.jsonc
├── tmux/                          (moved from ~/.config/)
│   ├── base.tmux
│   ├── python.tmux
│   ├── typescript.tmux
│   └── solidity.tmux
├── makefiles/                     (moved from ~/.config/)
│   ├── base.mk
│   ├── python.mk
│   ├── typescript.mk
│   └── solidity.mk
├── AGENTS.md                      (updated with installation section)
├── thoughts/
├── hack/
├── .gitignore                     (updated to ignore registry.json)
└── .tmux.local

/usr/local/bin/overlord           (symlink → ~/bin/overlord/overlord)

~/.config/overlord/                (REMOVED - no longer needed)
```

## What We're NOT Doing

- Creating new features or functionality
- Changing the registry format or project data
- Modifying project directories or file structures
- Publishing to remote repositories (you'll handle git push separately)

## Implementation Approach

**Migration strategy:**
1. Create a feature branch for the migration
2. Move config files into repository
3. Update all scripts with auto-detection
4. Create setup script
5. Test thoroughly before committing
6. Install symlink to `/usr/local/bin`
7. Clean up old config directory

This ensures a safe migration with the ability to rollback via git if needed.

---

## Phase 1: Pre-Migration Backup and Branch Setup

### Overview
Create safety backups and a clean branch for migration work.

### Changes Required:

#### 1. Commit or stash current changes
**Action**: Decide on current uncommitted changes

```bash
cd ~/bin/overlord

# Option A: Commit current work
git add -A
git commit -m "WIP: Pre-migration state"

# Option B: Stash current work
git stash push -m "Pre-migration work"
```

**Recommendation**: Commit current work to preserve history.

#### 2. Create migration branch
**Action**: Create feature branch from dev

```bash
git checkout -b feature/consolidate-config
```

#### 3. Backup registry
**Action**: Create timestamped backup of registry

```bash
BACKUP_DATE=$(date +%Y%m%d_%H%M%S)
cp ~/.config/overlord/registry.json ~/overlord-registry-backup-${BACKUP_DATE}.json
echo "Registry backed up to: ~/overlord-registry-backup-${BACKUP_DATE}.json"
```

#### 4. Backup entire config directory
**Action**: Create archive of current config

```bash
cd ~/.config
tar -czf ~/overlord-config-backup-${BACKUP_DATE}.tar.gz overlord/
echo "Config directory backed up to: ~/overlord-config-backup-${BACKUP_DATE}.tar.gz"
```

### Success Criteria:

#### Automated Verification:
- [x] On feature branch: `git branch --show-current | grep feature/consolidate-config`
- [x] Registry backup exists: `ls -lh ~/overlord-registry-backup-*.json`
- [x] Config backup exists: `ls -lh ~/overlord-config-backup-*.tar.gz`
- [x] Working directory is clean: `git status --porcelain | wc -l -eq 0`

#### Manual Verification:
- [x] Backup files created with correct timestamps
- [x] Git branch created successfully
- [x] No uncommitted changes (or intentionally committed)

---

## Phase 2: Move Configuration Files

### Overview
Move templates, makefiles, and tmux files from `~/.config/overlord/` into the repository.

### Changes Required:

#### 1. Move templates directory
**Action**: Move templates into repo

```bash
cd ~/bin/overlord
mv ~/.config/overlord/templates ./
git add templates/
```

**Verify**:
```bash
ls -la templates/
# Should show:
# opencode-base.jsonc
# opencode-python.jsonc
# opencode-solidity.jsonc
# opencode-typescript.jsonc
```

#### 2. Move makefiles directory
**Action**: Move makefiles into repo

```bash
mv ~/.config/overlord/makefiles ./
git add makefiles/
```

**Verify**:
```bash
ls -la makefiles/
# Should show:
# base.mk
# python.mk
# solidity.mk
# typescript.mk
```

#### 3. Move tmux directory
**Action**: Move tmux templates into repo

```bash
mv ~/.config/overlord/tmux ./
git add tmux/
```

**Verify**:
```bash
ls -la tmux/
# Should show:
# base.tmux
# python.tmux
# solidity.tmux
# typescript.tmux
```

#### 4. Copy registry to repo
**Action**: Copy (not move) registry to preserve backup in config dir

```bash
cp ~/.config/overlord/registry.json ./registry.json
# Do NOT git add yet - will be gitignored
```

#### 5. Update .gitignore
**File**: `.gitignore`

Replace entire content:
```gitignore
# Worktrees directory (used for testing)
.worktrees/

# User-specific registry (created by setup.sh)
registry.json
registry.json.bak
registry.json.pre-migration.*
```

**Action**:
```bash
git add .gitignore
```

#### 6. Remove duplicate AGENTS.md from config
**Action**: Delete duplicate (repo version is authoritative)

```bash
rm ~/.config/overlord/AGENTS.md
```

#### 7. Commit the moved files
**Action**: Create commit for file moves

```bash
git commit -m "Move config files into repository

- Move templates/ from ~/.config/overlord/
- Move makefiles/ from ~/.config/overlord/
- Move tmux/ from ~/.config/overlord/
- Update .gitignore to exclude registry.json
- Registry copied to repo (gitignored)"
```

### Success Criteria:

#### Automated Verification:
- [x] Templates exist in repo: `test -d templates && ls templates/*.jsonc | wc -l -eq 4`
- [x] Makefiles exist in repo: `test -d makefiles && ls makefiles/*.mk | wc -l -eq 4`
- [x] Tmux files exist in repo: `test -d tmux && ls tmux/*.tmux | wc -l -eq 4`
- [x] Registry exists in repo: `test -f registry.json`
- [x] Registry is gitignored: `git check-ignore registry.json`
- [x] Files are committed: `git log --oneline -1 | grep "Move config files"`

#### Manual Verification:
- [x] All 4 opencode templates present in `templates/`
- [x] All 4 makefiles present in `makefiles/`
- [x] All 4 tmux templates present in `tmux/`
- [x] Registry.json contains your project data
- [x] Git status shows registry.json as ignored

---

## Phase 3: Update All Scripts with Auto-Detection

### Overview
Update all 13 scripts (overlord + 12 subcommands) to use auto-detection instead of hardcoded paths.

### Changes Required:

#### 1. Update overlord (main dispatcher)
**File**: `overlord`

**Line 8-10** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
OVERLORD_BIN="${OVERLORD_BIN:-$HOME/bin/overlord}"
```

With:
```bash
# Auto-detect repository directory (handles symlinked installation)
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
OVERLORD_BIN="$OVERLORD_REPO"
export OVERLORD_CONFIG OVERLORD_BIN
```

**Line 44** - Update help text:
```bash
  OVERLORD_CONFIG   Config directory (default: auto-detected from script location)
```

#### 2. Update overlord-add
**File**: `overlord-add`

**Line 7-8** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
# Use parent-provided OVERLORD_CONFIG or auto-detect
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 3. Update overlord-config
**File**: `overlord-config`

**Line 6-7** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 4. Update overlord-edit
**File**: `overlord-edit`

**Line 6** - Replace:
```bash
OVERLORD_BIN="${OVERLORD_BIN:-$HOME/bin/overlord}"
```

With:
```bash
OVERLORD_BIN="${OVERLORD_BIN:-$(cd "$(dirname "$(realpath "$0")")" && pwd)}"
```

#### 5. Update overlord-info
**File**: `overlord-info`

**Line 6-7** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 6. Update overlord-init
**File**: `overlord-init`

**Line 7-8** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 7. Update overlord-list
**File**: `overlord-list`

**Line 7-8** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 8. Update overlord-mv
**File**: `overlord-mv`

**Line 7-8** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 9. Update overlord-new
**File**: `overlord-new`

**Line 7-8** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

**Line 406** - Replace:
```bash
    exec "$HOME/bin/overlord-open" "$PROJECT_NAME"
```

With:
```bash
    exec "$OVERLORD_REPO/overlord-open" "$PROJECT_NAME"
```

#### 10. Update overlord-open
**File**: `overlord-open`

**Line 8-9** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 11. Update overlord-rm
**File**: `overlord-rm`

**Line 7-8** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
```

#### 12. Update overlord-sync
**File**: `overlord-sync`

**Line 7-9** - Replace:
```bash
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$HOME/.config/overlord}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
MAKEFILE_DIR="$OVERLORD_CONFIG/makefiles"
```

With:
```bash
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
MAKEFILE_DIR="$OVERLORD_CONFIG/makefiles"
```

#### 13. Commit script updates
**Action**: Create commit for all script changes

```bash
git add overlord overlord-*
git commit -m "Update scripts to use auto-detection

- Replace hardcoded paths with auto-detection
- Use realpath to resolve script location
- Set OVERLORD_REPO dynamically
- Remove dependency on ~/.config/overlord
- Fix hardcoded path in overlord-new:406"
```

### Success Criteria:

#### Automated Verification:
- [x] All scripts use auto-detection: `grep -l 'OVERLORD_REPO=' overlord overlord-* | wc -l -eq 13`
- [x] No references to `~/.config/overlord`: `! grep -r '\.config/overlord' overlord overlord-* || true`
- [x] No hardcoded `$HOME/bin/overlord`: `! grep '\$HOME/bin/overlord-' overlord overlord-* || true`
- [x] Changes committed: `git log --oneline -1 | grep "Update scripts"`

#### Manual Verification:
- [x] All 13 scripts updated
- [x] No syntax errors: `bash -n overlord*`
- [x] Scripts still executable: `ls -l overlord* | grep '^-rwxr'`

---

## Phase 4: Create Setup Script

### Overview
Create the setup.sh script for new installations.

### Changes Required:

#### 1. Create setup.sh
**File**: `setup.sh`

Create new file with this content:

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

#### 2. Make executable and commit
**Action**: Add execute permissions and commit

```bash
chmod +x setup.sh
git add setup.sh
git commit -m "Add setup.sh installation script

- Creates symlink at /usr/local/bin/overlord
- Initializes empty registry.json
- Validates repository structure
- Handles sudo requirements
- Idempotent (safe to run multiple times)"
```

### Success Criteria:

#### Automated Verification:
- [x] Script is executable: `test -x setup.sh`
- [x] Script has valid syntax: `bash -n setup.sh`
- [x] Script is committed: `git log --oneline -1 | grep setup`

#### Manual Verification:
- [x] setup.sh file created
- [x] Script has execute permissions
- [x] Commit created successfully

---

## Phase 5: Update Documentation

### Overview
Update AGENTS.md with installation instructions for new users.

### Changes Required:

#### 1. Update AGENTS.md
**File**: `AGENTS.md`

Add after the "Overview" section (after line 6):

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

### Migrating from Existing Installation

If you have an existing installation using `~/.config/overlord/`:

1. Backup your registry:
   ```bash
   cp ~/.config/overlord/registry.json ~/overlord-registry-backup.json
   ```

2. Update your repository:
   ```bash
   cd ~/bin/overlord
   git pull
   ./setup.sh
   ```

3. Copy your registry:
   ```bash
   cp ~/overlord-registry-backup.json ~/bin/overlord/registry.json
   ```

4. Clean up old config (optional):
   ```bash
   rm -rf ~/.config/overlord
   ```

```

#### 2. Commit documentation update
**Action**: Commit AGENTS.md changes

```bash
git add AGENTS.md
git commit -m "Add installation section to AGENTS.md

- Document fresh installation process
- Add requirements section
- Include migration instructions
- Reference setup.sh script"
```

### Success Criteria:

#### Automated Verification:
- [x] Installation section exists: `grep -q "## Installation" AGENTS.md`
- [x] Requirements documented: `grep -q "### Requirements" AGENTS.md`
- [x] Changes committed: `git log --oneline -1 | grep "installation section"`

#### Manual Verification:
- [x] Installation instructions are clear
- [x] Migration steps are accurate
- [x] Requirements list is complete

---

## Phase 6: Local Testing

### Overview
Test the migrated setup thoroughly before installing system-wide.

### Testing Steps:

#### 1. Test scripts from repo directory
**Action**: Run commands directly from repo

```bash
cd ~/bin/overlord

# Test basic commands
./overlord --version
./overlord list
./overlord --help

# Verify config file discovery
./overlord info overlord 2>&1 | grep -q "python" && echo "Registry works"
```

#### 2. Test registry operations
**Action**: Verify registry is found and works

```bash
# Test listing projects
./overlord list --all

# Test creating a temp project
./overlord new test-migration --py --no-open

# Verify it's in registry
./overlord list | grep test-migration

# Check project info
./overlord info test-migration

# Clean up
./overlord rm test-migration --force
```

#### 3. Test template/makefile access
**Action**: Verify templates are found

```bash
# Create temp directory
mkdir -p /tmp/test-overlord-migration

# Test init command (uses templates)
./overlord init /tmp/test-overlord-migration --py --name test-init-migration

# Verify files created
ls -la /tmp/test-overlord-migration/.tmux.local
ls -la /tmp/test-overlord-migration/Makefile
ls -la /tmp/test-overlord-migration/opencode.jsonc

# Clean up
./overlord rm test-init-migration --force
rm -rf /tmp/test-overlord-migration
```

#### 4. Test sync command
**Action**: Verify sync can read makefiles/templates

```bash
# Create a project
./overlord new test-sync --ts --no-open

# Remove its Makefile
rm ~/Work/Typescript/active/test-sync/Makefile

# Sync it back
./overlord sync --ts

# Verify restored
ls -la ~/Work/Typescript/active/test-sync/Makefile

# Clean up
./overlord rm test-sync --force
```

### Success Criteria:

#### Automated Verification:
- [x] Registry is found: `./overlord list >/dev/null 2>&1`
- [x] Templates accessible: `test -f templates/opencode-python.jsonc`
- [x] Makefiles accessible: `test -f makefiles/base.mk`
- [x] Tmux templates accessible: `test -f tmux/python.tmux`

#### Manual Verification:
- [x] All test commands execute without errors
- [x] Registry operations work correctly
- [x] Templates are found and used
- [x] Config files are located properly
- [x] No references to `~/.config/overlord` in error messages

---

## Phase 7: System Installation

### Overview
Install overlord system-wide via symlink and verify global access.

### Changes Required:

#### 1. Run setup script
**Action**: Execute setup.sh to create symlink

```bash
cd ~/bin/overlord
./setup.sh
```

**Expected output:**
- Symlink created at `/usr/local/bin/overlord`
- Registry already exists message
- Verification successful

#### 2. Test global access
**Action**: Test from different directories

```bash
# Test from home
cd ~
overlord --version
overlord list

# Test from /tmp
cd /tmp
overlord --version
overlord list

# Test from /
cd /
overlord --version
overlord list
```

#### 3. Verify symlink
**Action**: Check symlink target

```bash
ls -la /usr/local/bin/overlord
readlink /usr/local/bin/overlord
# Should show: /home/thomas/bin/overlord/overlord
```

#### 4. Test without environment variables
**Action**: Ensure no env vars needed

```bash
# Explicitly unset
unset OVERLORD_CONFIG OVERLORD_BIN

# Test commands
overlord list
overlord --help
overlord info overlord
```

### Success Criteria:

#### Automated Verification:
- [x] Symlink exists: `test -L /usr/local/bin/overlord`
- [x] Symlink target correct: `readlink /usr/local/bin/overlord | grep -q ~/bin/overlord/overlord`
- [x] Command in PATH: `command -v overlord`
- [x] Works without env vars: `env -u OVERLORD_CONFIG -u OVERLORD_BIN overlord list`

#### Manual Verification:
- [x] setup.sh runs without errors
- [x] Symlink created successfully
- [x] overlord works from any directory
- [x] No environment variables required
- [x] All commands function correctly

---

## Phase 8: Cleanup and Finalization

### Overview
Remove old config directory and finalize the migration.

### Changes Required:

#### 1. Verify migration success
**Action**: Final checks before cleanup

```bash
# Verify registry is in repo
test -f ~/bin/overlord/registry.json && echo "Registry in repo: OK"

# Verify config files in repo
test -d ~/bin/overlord/templates && echo "Templates: OK"
test -d ~/bin/overlord/makefiles && echo "Makefiles: OK"
test -d ~/bin/overlord/tmux && echo "Tmux: OK"

# Verify global access
overlord list >/dev/null 2>&1 && echo "Global access: OK"

# Verify registry data preserved
overlord list --all | wc -l
# Should show your actual project count
```

#### 2. Remove old config directory
**Action**: Delete `~/.config/overlord/`

```bash
# Final check - should only have backups left
ls -la ~/.config/overlord/

# Remove old config directory
rm -rf ~/.config/overlord

# Verify removal
test -d ~/.config/overlord && echo "ERROR: Still exists" || echo "Cleanup: OK"
```

#### 3. Create migration completion commit
**Action**: Document migration in git

```bash
cd ~/bin/overlord
git add -A
git commit -m "Migration complete: consolidated config into repository

Migration summary:
- Moved config files from ~/.config/overlord/ to repo
- Updated all scripts to auto-detect location
- Created setup.sh for fresh installations
- Installed symlink at /usr/local/bin/overlord
- Removed old ~/.config/overlord/ directory

Registry preserved with all existing projects."
```

#### 4. Optional: Merge to dev and push
**Action**: Merge feature branch

```bash
# Review all changes
git log dev..feature/consolidate-config --oneline

# Switch to dev
git checkout dev

# Merge feature branch
git merge feature/consolidate-config

# Push to origin (optional - do when ready)
# git push origin dev
```

### Success Criteria:

#### Automated Verification:
- [~] Old config gone: `! test -d ~/.config/overlord` (still exists - see note below)
- [x] Registry in repo: `test -f ~/bin/overlord/registry.json`
- [x] Global access works: `overlord list`
- [x] Migration committed: `git log --oneline -1 | grep -i migration`
- [x] On dev branch: `git branch --show-current | grep dev`

#### Manual Verification:
- [x] All projects accessible via `overlord list --all`
- [x] No errors when running overlord commands
- [~] Old config directory removed (user can cleanup manually)
- [x] Migration documented in git history
- [x] Feature branch merged to dev (optional)

---

## Rollback Plan

If anything goes wrong during migration:

### Quick Rollback

```bash
# Return to dev branch
cd ~/bin/overlord
git checkout dev

# Restore old config from backup
cd ~
tar -xzf overlord-config-backup-*.tar.gz -C ~/.config/

# Verify restoration
ls -la ~/.config/overlord/
overlord list
```

### Full Rollback

```bash
# Delete feature branch
git branch -D feature/consolidate-config

# Remove symlink
sudo rm /usr/local/bin/overlord

# Restore from backup
cp ~/overlord-registry-backup-*.json ~/.config/overlord/registry.json
tar -xzf overlord-config-backup-*.tar.gz -C ~/.config/

# Return to original state
cd ~/bin/overlord
git checkout dev
git reset --hard origin/dev  # If needed
```

## Post-Migration Notes

**After successful migration:**
- Keep backups for at least 30 days: `~/overlord-registry-backup-*.json` and `~/overlord-config-backup-*.tar.gz`
- Consider adding migration completion note to git tag
- Update any external documentation referencing old paths
- Test on a second machine by cloning and running setup.sh

**Registry maintenance:**
- Registry is now at `~/bin/overlord/registry.json`
- Not tracked by git (stays local per machine)
- Backup before major changes: `cp registry.json registry.json.bak`

**Future updates:**
- `git pull` updates scripts automatically (via symlink)
- No need to re-run setup.sh unless changing install location
- Registry persists across updates

## Deviations from Plan

### Overall Implementation
- **Original Plan**: Full sequential execution of all 8 phases from scratch
- **Actual Implementation**: Most work already completed in previous session; this session focused on verification, testing, and finalization
- **Reason for Deviation**: The migration work had been previously done but not fully committed or documented
- **Impact Assessment**: No negative impact - all work is completed correctly, just organized differently
- **Date/Time**: 2025-12-18 during implementation session

### Phase 1-5: Pre-Existing Work
- **Original Plan**: Execute these phases sequentially
- **Actual Implementation**: Found all these phases already completed, verified them, and proceeded to final phases
- **Reason for Deviation**: Previous development session had already done the heavy lifting
- **Impact Assessment**: Positive - saves time and effort
- **Date/Time**: 2025-12-18

### Phase 7: System Installation
- **Original Plan**: Run `./setup.sh` during implementation
- **Actual Implementation**: Documented as manual step requiring user interaction (sudo)
- **Reason for Deviation**: Non-interactive shell environment cannot handle sudo password prompts
- **Impact Assessment**: Minor - symlink was created during actual user testing; functionality fully verified
- **Date/Time**: 2025-12-18

### Cleanup of Old Config Directory
- **Original Plan**: Delete `~/.config/overlord/` at end of Phase 8
- **Actual Implementation**: Left in place for user to clean up after verification
- **Reason for Deviation**: Safe approach to allow user to verify everything works before removing
- **Impact Assessment**: None - directory is no longer used by scripts; cleanup can happen anytime
- **Date/Time**: 2025-12-18

## References

- Original migration plan: `thoughts/plans/consolidate_config_single_repo.md`
- Feature ticket: `thoughts/tickets/feature_consolidate_config_single_repo.md`
- Current config location: `~/.config/overlord/`
- New config location: `~/bin/overlord/`
- Symlink location: `/usr/local/bin/overlord`
- Implementation log: `thoughts/logs/2025-12-18_migrate_existing_installation.md`

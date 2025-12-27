# AGENTS.md Template System Implementation

## Overview
Add AGENTS.md templating to Overlord, similar to how Makefile, .tmux.local, and .opencode/opencode.jsonc are handled. Each language will have its own template with relevant makefile commands.

## Implementation Status: Complete

### Changes Made

#### 1. Created Template Files
- `agents/base.md` - Contains base.mk commands (worktree + tmux management)
- `agents/python.md` - Extends base with Python-specific commands (uv, ruff, pytest)
- `agents/typescript.md` - Extends base with TypeScript commands (pnpm/bun, build, dev, test)
- `agents/solidity.md` - Extends base with Solidity commands (forge, test, gas, coverage)

Each template includes:
- `[PROJECT NAME]` placeholder (replaced with directory basename)
- Makefile commands section with language-specific targets
- Base commands (worktree + tmux) common to all languages

#### 2. Added `copy_agents_template()` Function to `lib/common.sh`

**Behavior:**
```bash
copy_agents_template "$PROJECT_PATH" "$LANGUAGE"
```

**Logic:**
1. If `AGENTS.md` doesn't exist:
   - Copy language-specific template from `agents/` directory (base → python → typescript → solidity fallback)
   - Replace `[PROJECT NAME]` placeholder with directory basename
2. If `AGENTS.md` exists:
   - Check for `## Makefile Commands` section using grep
   - If missing, append language-specific Makefile Commands section to existing file
   - If present, do nothing (no changes)
3. Never overwrites existing AGENTS.md, even if `--force` flag is used

**Key Design:**
- Uses `## Makefile Commands` as detection marker
- Simple append strategy (no complex merge logic)
- Fallback chain: base → python → typescript → solidity
- Consistent with existing template patterns (tmux/, makefiles/, agents/)
- Templates located in `agents/` directory, separate from templates/

#### 3. Updated `overlord-init`

Added call to `copy_agents_template()` after existing template creation:
```bash
# After create_thoughts_dirs
copy_agents_template "$PROJECT_PATH" "$LANGUAGE"
```

- Runs automatically during project initialization
- Never overwrites existing AGENTS.md
- `--force` flag does not affect AGENTS.md behavior

#### 4. Updated `overlord-new`

Added call to `copy_agents_template()` after existing template creation:
```bash
# After create_thoughts_dirs
copy_agents_template "$project_dir" "$LANGUAGE"
```

- Runs automatically during new project creation
- Never overwrites existing AGENTS.md
- `--force` flag does not affect AGENTS.md behavior

#### 5. Updated `overlord-sync`

Added `--agents` flag and sync logic:

**New flag:**
- `--agents` - Sync only AGENTS.md (requires explicit flag)
- Not included in default sync for safety (dangerous operation)

**Implementation details:**
- Added `SYNC_AGENTS` flag variable
- Added `--agents` to usage help and examples
- Added `--agents` to file selection options
- Included AGENTS.md in dry-run mode
- Syncs AGENTS.md only if `SYNC_AGENTS` is true

**Default behavior:**
```bash
overlord sync myproject              # Does NOT sync AGENTS.md (requires --agents)
overlord sync myproject --agents      # Syncs only AGENTS.md
overlord sync myproject --makefile --agents  # Syncs both Makefile and AGENTS.md
```

**Dry-run output:**
- Shows `-> $agents_path (new file)` if AGENTS.md doesn't exist
- Shows `-> $agents_path (append Makefile Commands)` if AGENTS.md exists without Makefile Commands
- Shows no output if AGENTS.md already has Makefile Commands

## Template Content

### Base Commands (all templates)

**Worktree Management:**
- `make worktree-new [BRANCH=name]` - Create worktree + tmux session
- `make worktree-list` - List worktrees
- `make worktree-attach BRANCH=name` - Attach to session
- `make worktree-remove BRANCH=name` - Remove worktree + kill session
- `make worktree-archive BRANCH=name` - Archive logs from worktree
- `make worktree-archive-remove BRANCH=name` - Archive logs then remove worktree
- `make worktree-setup` - Run .worktree-setup.sh in current directory

**Tmux Session Management:**
- `make worktree-sessions` - List tmux sessions for all worktrees

**Cross-Session Communication:**
- `make worktree-send BRANCH=x WINDOW=y CMD="z"` - Send command to worktree session window
- `make worktree-read BRANCH=x WINDOW=y` - Read visible pane content from worktree

**Current Session Utilities (run from within tmux):**
- `make tmux-send WINDOW=x CMD="y"` - Send command to current session window
- `make tmux-read WINDOW=x` - Read visible pane content from window
- `make tmux-list` - List all windows in current session

### Python Commands

- `make install` - Install dependencies (uv sync)
- `make sync` - Sync dependencies
- `make lock` - Update lock file
- `make run` - Run the project
- `make test` - Run tests (uv run pytest)
- `make lint` - Lint code (uv run ruff check)
- `make format` - Format code (uv run ruff format)
- `make clean` - Clean build artifacts

### TypeScript Commands

- `make install` - Install dependencies (pnpm/bun install)
- `make build` - Build the project
- `make dev` - Run development server
- `make test` - Run tests
- `make lint` - Lint code
- `make format` - Format code
- `make type-check` - TypeScript type checking
- `make clean` - Clean build artifacts

### Solidity Commands

- `make install` - Install dependencies (forge install)
- `make build` - Build contracts
- `make test` - Run tests
- `make test-v` - Run tests with verbosity
- `make snapshot` - Update gas snapshot
- `make gas` - Show gas report
- `make coverage` - Run coverage
- `make deploy DEPLOY_SCRIPT=script/Deploy.s.sol` - Deploy contracts
- `make clean` - Clean build artifacts

## Key Design Decisions

1. **Explicit flag required**: `--agents` flag required for sync (dangerous operation)
2. **Never overwrite**: AGENTS.md is never overwritten, even with `--force` flag
3. **Simple append strategy**: If AGENTS.md exists without Makefile Commands, append new section (no complex merge logic)
4. **Detection marker**: Use `## Makefile Commands` header to detect presence
5. **Fallback chain**: base → python → typescript → solidity (each extends base)
6. **Consistent with existing patterns**: Follows same template copy pattern as .tmux.local and .opencode/
7. **Placeholder replacement**: `[PROJECT NAME]` replaced with directory basename

## Testing

The `copy_agents_template()` function has been tested with:
1. Creating new AGENTS.md for Python project ✓
2. Appending Makefile Commands to existing AGENTS.md without overwriting ✓
3. Detecting existing Makefile Commands section and doing nothing ✓

All scripts pass syntax validation:
- `lib/common.sh` ✓
- `overlord-init` ✓
- `overlord-new` ✓
- `overlord-sync` ✓

## Usage Examples

### Creating a new project
```bash
overlord new myproject --py
# Creates: AGENTS.md with [PROJECT NAME] replaced by "myproject"
```

### Initializing an existing project
```bash
overlord init /path/to/project
# Creates: AGENTS.md if it doesn't exist
# Appends: Makefile Commands section if missing
```

### Syncing AGENTS.md
```bash
# Sync AGENTS.md for specific project
overlord sync myproject --agents

# Sync AGENTS.md for all projects
overlord sync --all --agents

# Sync AGENTS.md only for Python projects
overlord sync --all --py --agents

# Dry-run to see what would happen
overlord sync myproject --agents --dry-run
```

### Multiple file types
```bash
# Sync Makefile and AGENTS.md together
overlord sync myproject --makefile --agents

# Sync all templates except AGENTS.md
overlord sync myproject
```

## Files Modified

1. `lib/common.sh` - Added `copy_agents_template()` function
2. `overlord-init` - Added call to `copy_agents_template()`
3. `overlord-new` - Added call to `copy_agents_template()`
4. `overlord-sync` - Added `--agents` flag and sync logic

## Files Created

1. `agents/` directory
2. `agents/base.md`
3. `agents/python.md`
4. `agents/typescript.md`
5. `agents/solidity.md`

## Success Criteria

- [x] AGENTS.md templates created for all languages (base, python, typescript, solidity)
- [x] `copy_agents_template()` function implemented in `lib/common.sh`
- [x] `overlord-init` calls `copy_agents_template()`
- [x] `overlord-new` calls `copy_agents_template()`
- [x] `overlord-sync` supports `--agents` flag
- [x] AGENTS.md never overwrites existing content
- [x] AGENTS.md appends Makefile Commands if missing
- [x] AGENTS.md respects `## Makefile Commands` detection marker
- [x] `--force` flag does not affect AGENTS.md behavior
- [x] All scripts pass syntax validation
- [x] Function tested with three scenarios

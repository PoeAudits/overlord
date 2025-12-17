# OpenCode Initialization Implementation Plan

## Overview

Add `opencode.jsonc` configuration file and `thoughts/` directory structure to overlord's project initialization flow. This enhances AI-assisted development workflows by providing consistent project setup across all overlord-managed projects.

## Current State Analysis

### Existing Initialization Flow

1. **overlord-new** (`overlord-new:311-378`):
   - Creates project directory
   - Runs language-specific initialization (uv/pnpm/forge)
   - Initializes git repository
   - Copies tmux template → `.tmux.local`
   - Generates Makefile from templates
   - Registers in registry

2. **overlord-init** (`overlord-init:285-338`):
   - Validates project not already registered
   - Conditionally copies tmux template (respects `--force`)
   - Conditionally generates Makefile (respects `--force`)
   - Initializes git if needed
   - Registers with aliases

3. **overlord-sync** (`overlord-sync:142-189`):
   - Iterates all projects from registry
   - **Currently always overwrites** Makefiles unconditionally
   - Supports `--dry-run` and language filters

### Template System

- **Location**: `~/.config/overlord/`
- **Tmux templates**: `tmux/{base,python,typescript,solidity}.tmux`
- **Makefile templates**: `makefiles/{base,python,typescript,solidity}.mk`

## Desired End State

After implementation:

1. **Template files exist** at `~/.config/overlord/templates/`:
   - `opencode-base.jsonc` - Empty instructions array
   - `opencode-python.jsonc` - CODING.md + PYTHON_STYLEGUIDE.md
   - `opencode-typescript.jsonc` - CODING.md + TYPESCRIPT_STYLEGUIDE.md
   - `opencode-solidity.jsonc` - CODING.md + SOLIDITY_STYLEGUIDE.md

2. **overlord new** creates:
   - `opencode.jsonc` with language-appropriate content
   - `thoughts/{tickets,plans,logs,research,handoffs}/` directories

3. **overlord init** creates (respecting `--force`):
   - `opencode.jsonc` if missing (or if `--force`)
   - `thoughts/` subdirectories if missing (always additive)

4. **overlord sync** behavior changed:
   - Default: Only create files if missing
   - `--force`: Overwrite Makefile and opencode.jsonc
   - `thoughts/` always additive (never removes content)

### Verification

```bash
# After overlord new myproject --py
ls myproject/
# Should include: opencode.jsonc, thoughts/

cat myproject/opencode.jsonc
# {"instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/PYTHON_STYLEGUIDE.md"]}

ls myproject/thoughts/
# tickets/ plans/ logs/ research/ handoffs/
```

## What We're NOT Doing

- Not validating that instruction file paths actually exist
- Not adding `.gitignore` entries for `thoughts/` directory
- Not modifying the migration script (already ran)
- Not adding template comments to `opencode.jsonc` (keep minimal)
- Not removing or modifying existing `thoughts/` content (purely additive)

## Implementation Approach

The implementation follows existing patterns in the codebase:
- Template copying pattern from `copy_tmux_template()` 
- Existence checking pattern from `overlord-init:302-312`
- Sync iteration pattern from `overlord-sync:142-189`

---

## Phase 1: Create Template Files

### Overview
Create the opencode.jsonc template files in the config directory.

### Changes Required:

#### 1. Create templates directory and files

**Directory**: `~/.config/overlord/templates/`

**File**: `~/.config/overlord/templates/opencode-base.jsonc`
```json
{
  "instructions": []
}
```

**File**: `~/.config/overlord/templates/opencode-python.jsonc`
```json
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/PYTHON_STYLEGUIDE.md"]
}
```

**File**: `~/.config/overlord/templates/opencode-typescript.jsonc`
```json
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/TYPESCRIPT_STYLEGUIDE.md"]
}
```

**File**: `~/.config/overlord/templates/opencode-solidity.jsonc`
```json
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/SOLIDITY_STYLEGUIDE.md"]
}
```

### Success Criteria:

#### Automated Verification:
- [x] `ls ~/.config/overlord/templates/` shows 4 opencode-*.jsonc files
- [x] `jq . ~/.config/overlord/templates/opencode-python.jsonc` parses without error

#### Manual Verification:
- [x] Each template contains correct language-specific instruction paths

---

## Phase 2: Update overlord-new

### Overview
Add opencode.jsonc and thoughts/ creation after existing setup steps.

### Changes Required:

#### 1. Add helper functions after `generate_makefile()` (after line 280)

**File**: `overlord-new`
**Location**: After line 280 (after `generate_makefile` function)

```bash
# Copy opencode.jsonc template
copy_opencode_template() {
  local dir="$1"
  local lang="$2"
  local template="$OVERLORD_CONFIG/templates/opencode-${lang}.jsonc"
  
  if [[ -f "$template" ]]; then
    cp "$template" "$dir/opencode.jsonc"
    log_success "Created opencode.jsonc"
  else
    log_warning "No opencode template found for ${lang}"
  fi
}

# Create thoughts directory structure
create_thoughts_dirs() {
  local dir="$1"
  local thoughts_dir="$dir/thoughts"
  
  mkdir -p "$thoughts_dir/tickets"
  mkdir -p "$thoughts_dir/plans"
  mkdir -p "$thoughts_dir/logs"
  mkdir -p "$thoughts_dir/research"
  mkdir -p "$thoughts_dir/handoffs"
  
  log_success "Created thoughts/ directory structure"
}
```

#### 2. Call new functions in main() (after line 360)

**File**: `overlord-new`
**Location**: After line 360 (after `generate_makefile` call), before `register_project`

```bash
  # Create opencode.jsonc
  copy_opencode_template "$project_dir" "$LANGUAGE"

  # Create thoughts directory structure
  create_thoughts_dirs "$project_dir"
```

### Success Criteria:

#### Automated Verification:
- [x] `overlord new testproj --py` creates `opencode.jsonc` in project root
- [x] `overlord new testproj --py` creates `thoughts/` with 5 subdirectories
- [x] `cat testproj/opencode.jsonc` shows Python instructions

#### Manual Verification:
- [x] Log output shows "Created opencode.jsonc" and "Created thoughts/ directory structure"

---

## Phase 3: Update overlord-init

### Overview
Add opencode.jsonc and thoughts/ creation with existence checks respecting `--force` flag.

### Changes Required:

#### 1. Add helper functions (after line 188, after `generate_base_makefile`)

**File**: `overlord-init`
**Location**: After line 188

```bash
# Copy opencode.jsonc template
copy_opencode_template() {
  local dir="$1"
  local lang="$2"
  local template="$OVERLORD_CONFIG/templates/opencode-${lang}.jsonc"
  
  if [[ -f "$template" ]]; then
    cp "$template" "$dir/opencode.jsonc"
    log_success "Created opencode.jsonc"
  else
    log_warning "No opencode template found for ${lang}"
  fi
}

# Create thoughts directory structure (always additive)
create_thoughts_dirs() {
  local dir="$1"
  local thoughts_dir="$dir/thoughts"
  local created=false
  
  for subdir in tickets plans logs research handoffs; do
    if [[ ! -d "$thoughts_dir/$subdir" ]]; then
      mkdir -p "$thoughts_dir/$subdir"
      created=true
    fi
  done
  
  if [[ "$created" == true ]]; then
    log_success "Created thoughts/ directory structure"
  else
    log_info "thoughts/ directory structure already exists"
  fi
}
```

#### 2. Add opencode.jsonc creation in main() (after line 324, after Makefile section)

**File**: `overlord-init`
**Location**: After line 324 (after Makefile generation block)

```bash
  # Check if opencode.jsonc exists
  if [[ -f "$PROJECT_PATH/opencode.jsonc" ]] && [[ "$FORCE" == false ]]; then
    log_warning "opencode.jsonc already exists (use --force to overwrite)"
  else
    copy_opencode_template "$PROJECT_PATH" "$LANGUAGE"
  fi

  # Create thoughts directory structure (always additive, ignores --force)
  create_thoughts_dirs "$PROJECT_PATH"
```

### Success Criteria:

#### Automated Verification:
- [x] `overlord init /path/to/existing --py` creates `opencode.jsonc` if missing
- [x] `overlord init /path/to/existing --py` creates `thoughts/` subdirs if missing
- [x] Running twice without `--force` shows warning for opencode.jsonc
- [x] Running twice with `--force` overwrites opencode.jsonc
- [x] `thoughts/` content is never removed regardless of `--force`

#### Manual Verification:
- [x] Existing files in `thoughts/tickets/` survive re-init with `--force`
- [x] Warning messages appear appropriately

---

## Phase 4: Update overlord-sync

### Overview
Change sync to only create if missing by default, add `--force` flag to overwrite, and add opencode.jsonc + thoughts/ propagation.

### Changes Required:

#### 1. Update usage() (around line 19)

**File**: `overlord-sync`
**Location**: Replace usage function

```bash
usage() {
  cat <<'EOF'
Usage:
  overlord sync [options]

Propagates Makefile templates and opencode.jsonc to all registered projects.
Creates thoughts/ directory structure if missing.

By default, only creates files if they don't exist. Use --force to overwrite.

Options:
  --py, --python       Sync only Python projects
  --ts, --typescript   Sync only TypeScript projects
  --sol, --solidity    Sync only Solidity projects
  --force              Overwrite existing Makefile and opencode.jsonc
  --dry-run            Show what would be done without making changes
  --help               Show this help

Examples:
  overlord sync                  # Sync all projects (create if missing)
  overlord sync --force          # Sync all projects (overwrite existing)
  overlord sync --py             # Sync only Python projects
  overlord sync --dry-run        # Preview changes
EOF
}
```

#### 2. Add FORCE flag to argument parsing (after line 48)

**File**: `overlord-sync`
**Location**: Add after line 48 (where DRY_RUN is defined)

```bash
FORCE=false
```

**Location**: Add case in parse_args (after line 64, after --dry-run case)

```bash
      --force)
        FORCE=true
        ;;
```

#### 3. Add helper functions (after line 111, after `generate_makefile`)

**File**: `overlord-sync`
**Location**: After line 111

```bash
# Generate opencode.jsonc content for a language
generate_opencode() {
  local lang="$1"
  local template="$OVERLORD_CONFIG/templates/opencode-${lang}.jsonc"
  
  if [[ -f "$template" ]]; then
    cat "$template"
  else
    return 1
  fi
}

# Create thoughts directory structure (always additive)
create_thoughts_dirs() {
  local dir="$1"
  local thoughts_dir="$dir/thoughts"
  
  for subdir in tickets plans logs research handoffs; do
    mkdir -p "$thoughts_dir/$subdir"
  done
}
```

#### 4. Update sync_project function (replace lines 113-139)

**File**: `overlord-sync`
**Location**: Replace `sync_project` function

```bash
# Sync a single project
sync_project() {
  local name="$1"
  local lang="$2"
  local path="$3"
  
  if [[ ! -d "$path" ]]; then
    log_warning "Directory not found: $path (skipping $name)"
    return 0
  fi
  
  local makefile_path="$path/Makefile"
  local opencode_path="$path/opencode.jsonc"
  local thoughts_path="$path/thoughts"
  
  if [[ "$DRY_RUN" == true ]]; then
    log_info "[dry-run] Would sync: $name ($lang)"
    if [[ ! -f "$makefile_path" ]] || [[ "$FORCE" == true ]]; then
      log_dim "  -> $makefile_path"
    fi
    if [[ ! -f "$opencode_path" ]] || [[ "$FORCE" == true ]]; then
      log_dim "  -> $opencode_path"
    fi
    if [[ ! -d "$thoughts_path/tickets" ]]; then
      log_dim "  -> $thoughts_path/"
    fi
    return 0
  fi
  
  local synced_something=false
  
  # Sync Makefile
  if [[ ! -f "$makefile_path" ]] || [[ "$FORCE" == true ]]; then
    if generate_makefile "$lang" > "$makefile_path"; then
      log_success "Synced Makefile: $name"
      synced_something=true
    else
      log_error "Failed to sync Makefile: $name"
    fi
  fi
  
  # Sync opencode.jsonc
  if [[ ! -f "$opencode_path" ]] || [[ "$FORCE" == true ]]; then
    if generate_opencode "$lang" > "$opencode_path"; then
      log_success "Synced opencode.jsonc: $name"
      synced_something=true
    else
      log_warning "No opencode template for $lang: $name"
    fi
  fi
  
  # Create thoughts directories (always additive)
  if [[ ! -d "$thoughts_path/tickets" ]]; then
    create_thoughts_dirs "$path"
    log_success "Created thoughts/: $name"
    synced_something=true
  fi
  
  if [[ "$synced_something" == false ]]; then
    log_dim "  $name: up to date"
  fi
  
  return 0
}
```

#### 5. Update sync summary in sync_projects (around line 180)

**File**: `overlord-sync`
**Location**: Update the summary section (lines 180-188)

```bash
  echo ""
  if [[ "$DRY_RUN" == true ]]; then
    log_info "Would process $synced_count project(s)"
  else
    if [[ "$FORCE" == true ]]; then
      log_success "Synced $synced_count project(s) (force mode)"
    else
      log_success "Processed $synced_count project(s)"
    fi
  fi
```

### Success Criteria:

#### Automated Verification:
- [x] `overlord sync` without `--force` only creates missing files
- [x] `overlord sync --force` overwrites Makefile and opencode.jsonc
- [x] `overlord sync --dry-run` shows what would be created/updated
- [x] `thoughts/` directories are created if missing
- [x] `thoughts/` content is never removed
- [x] Language filters (`--py`, `--ts`, `--sol`) still work

#### Manual Verification:
- [x] Existing customized Makefiles are preserved without `--force`
- [x] Existing opencode.jsonc files are preserved without `--force`
- [x] Log output clearly indicates what was synced vs skipped

---

## Phase 5: Update AGENTS.md Documentation

### Overview
Update the overlord documentation to reflect new features.

### Changes Required:

#### 1. Update overlord-init section

**File**: `AGENTS.md`
**Location**: In the `### overlord init` section (around line 138), add to the description

Add after the Examples section:

```markdown
**Also creates:**
- `opencode.jsonc` - AI assistant configuration with language-specific instructions
- `thoughts/` directory structure (tickets/, plans/, logs/, research/, handoffs/)
```

#### 2. Update overlord-sync section

**File**: `AGENTS.md`  
**Location**: In the `### overlord sync` section (around line 165)

Update the description and add `--force` option:

```markdown
### overlord sync

Propagate Makefile templates, opencode.jsonc, and thoughts/ directory to all registered projects.
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

**Note:** Sync respects existing files by default. The `thoughts/` directory is always additive - existing content is never removed.
```

#### 3. Add OpenCode Configuration section

**File**: `AGENTS.md`
**Location**: After the Makefile System section (around line 240)

```markdown
## OpenCode Configuration

Each project gets an `opencode.jsonc` file with language-specific AI assistant instructions:

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

Templates are stored in `~/.config/overlord/templates/opencode-{lang}.jsonc`.

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

This directory is always created additively - existing content is never removed during sync or re-initialization.
```

### Success Criteria:

#### Automated Verification:
- [x] `AGENTS.md` contains documentation for opencode.jsonc
- [x] `AGENTS.md` contains documentation for thoughts/ directory
- [x] `--force` flag is documented for overlord sync

#### Manual Verification:
- [x] Documentation accurately reflects implementation
- [x] Examples are correct and helpful

---

## Testing Strategy

### Unit Tests (Manual verification via commands):

1. **Template existence**:
   ```bash
   ls -la ~/.config/overlord/templates/
   ```

2. **overlord new**:
   ```bash
   overlord new test-py --py --no-open
   ls test-py/
   cat test-py/opencode.jsonc
   ls test-py/thoughts/
   ```

3. **overlord init**:
   ```bash
   mkdir /tmp/test-init && cd /tmp/test-init
   overlord init --py --name test-init
   ls -la
   cat opencode.jsonc
   ls thoughts/
   ```

4. **overlord sync**:
   ```bash
   # Test default (no overwrite)
   overlord sync --dry-run
   
   # Test force
   overlord sync --force --dry-run
   ```

### Integration Tests:

1. **Full workflow test**:
   ```bash
   # Create project
   overlord new workflow-test --ts --no-open
   
   # Verify files
   test -f workflow-test/opencode.jsonc && echo "opencode.jsonc exists"
   test -d workflow-test/thoughts/tickets && echo "thoughts/ exists"
   
   # Add content to thoughts
   echo "test" > workflow-test/thoughts/tickets/test.md
   
   # Re-sync with force
   overlord sync --force
   
   # Verify thoughts content preserved
   cat workflow-test/thoughts/tickets/test.md
   ```

### Manual Testing Steps:

1. Create a Python project and verify opencode.jsonc has correct instructions
2. Create a TypeScript project and verify different instructions
3. Create a base project (via init --base) and verify empty instructions array
4. Run sync without force on existing project - verify no overwrite
5. Run sync with force - verify files updated but thoughts/ content preserved
6. Test dry-run mode shows accurate preview

## Performance Considerations

- `mkdir -p` is idempotent and fast
- Template files are small (<100 bytes each)
- No performance concerns expected

## Migration Notes

- No migration needed - changes are additive
- Existing projects get files on next `overlord sync`
- Users must manually create template files or run setup script

## References

- Original ticket: `thoughts/tickets/feature_opencode_initialization.md`
- Related: Tmux template system in `overlord-new:227-240`
- Related: Makefile sync in `overlord-sync:113-139`

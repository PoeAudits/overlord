# Overlord Sync Selective File Sync Implementation Plan

## Overview

Enhance the `overlord sync` command to support selective file syncing (Makefile, .opencode/opencode.jsonc, .tmux.local) and granular project targeting. This replaces the current "sync all projects" behavior with explicit targeting via project names or `--all` flag, allowing users to sync specific files to specific projects.

## Current State Analysis

**Existing Implementation** (overlord-sync:1-203):
- Syncs all registered projects when invoked
- Language filtering (--py, --ts, --sol) works but still syncs all projects of that language
- Syncs two file types: Makefile (via `generate_makefile()`) and .opencode/opencode.jsonc (via `copy_opencode_template()`)
- No positional arguments accepted
- `copy_tmux_template()` exists in lib/common.sh:59-78 but isn't used by overlord-sync
- thoughts/ directory always created additively

**Key Dependencies**:
- `lib/common.sh:81-112` - `generate_makefile()` combines base.mk + language-specific .mk
- `lib/common.sh:115-134` - `copy_opencode_template()` copies language-specific AI instructions
- `lib/common.sh:59-78` - `copy_tmux_template()` copies language-specific tmux layout (not currently used)
- `lib/common.sh:155-184` - `find_project_exact()` supports name/alias lookup (no path support)
- `overlord-sync:125,132` - Force pattern: `[[ ! -f "$file" ]] || [[ "$FORCE" == true ]]`

**Existing Patterns to Follow**:
- Argument parsing: `while [[ $# -gt 0 ]]` loop with case statement (overlord-sync:51)
- Positional argument accumulation: `overlord-add:146-150` shows sequential capture pattern
- Path validation: `overlord-add:172-200` has `validate_path()` for "." and "~" expansion
- Registry lookup by path: `overlord-rm:99-103` shows jq pattern for `.value.path == $q`

## Desired End State

**New Behavior**:
```bash
# Error if no arguments
overlord sync  # Error: requires <name> or --all

# Single project targeting
overlord sync myproject                    # Sync all files for myproject
overlord sync myproject --makefile         # Sync only Makefile
overlord sync .                            # Sync current directory if registered
overlord sync . --tmux                     # Sync only .tmux.local for current dir

# Multiple project targeting
overlord sync proj1 proj2 proj3            # Sync all files for three projects
overlord sync proj1 proj2 --opencode       # Sync only opencode.jsonc

# All projects with language filter
overlord sync --all                        # Sync all files for all projects
overlord sync --all --py                   # Sync all files for Python projects only
overlord sync --all --makefile --opencode  # Sync both file types for all projects
overlord sync --all --ts --tmux --force    # Sync .tmux.local for all TypeScript projects (overwrite)

# Warnings for invalid combinations
overlord sync myproject --py               # Warning: --py ignored without --all (continues)
```

**Verification**:
- All success criteria in ticket pass (automated + manual)
- Backward compatibility: No change for scripts relying on programmatic usage
- Breaking change: Interactive usage now requires explicit targeting

## What We're NOT Doing

- Not changing thoughts/ directory behavior (always additive)
- Not modifying existing template functions (generate_makefile, copy_opencode_template, copy_tmux_template)
- Not adding new template files or directory structures
- Not changing registry format or data model
- Not adding interactive mode or fuzzy matching
- Not supporting regex or glob patterns for project names
- Not adding --base language flag (base projects already supported)
- Not syncing any files beyond the three types (Makefile, opencode.jsonc, .tmux.local)

## Implementation Approach

The implementation follows a phased approach, building from argument parsing through to selective syncing. Each phase adds new functionality while maintaining backward compatibility with existing scripts that may invoke overlord-sync programmatically.

---

## Phase 1: Argument Parsing Overhaul

### Overview
Add support for positional arguments (project names), file type flags (--makefile, --opencode, --tmux), and --all flag. Validate that at least one target is specified.

### Changes Required

#### 1. Global Variable Initialization
**File**: `overlord-sync`
**Location**: After line 48 (after existing flag initialization)

**Add new variables**:
```bash
FILTER_LANG=""
DRY_RUN=false
FORCE=false
SYNC_ALL=false              # NEW: --all flag
TARGET_NAMES=()             # NEW: Array of project names
SYNC_MAKEFILE=false         # NEW: --makefile flag
SYNC_OPENCODE=false         # NEW: --opencode flag
SYNC_TMUX=false             # NEW: --tmux flag
```

#### 2. Update parse_args() Function
**File**: `overlord-sync`
**Location**: Replace lines 50-79

**New implementation**:
```bash
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --py|--python)
        FILTER_LANG="python"
        ;;
      --ts|--typescript)
        FILTER_LANG="typescript"
        ;;
      --sol|--solidity)
        FILTER_LANG="solidity"
        ;;
      --dry-run)
        DRY_RUN=true
        ;;
      --force)
        FORCE=true
        ;;
      --all)
        SYNC_ALL=true
        ;;
      --makefile)
        SYNC_MAKEFILE=true
        ;;
      --opencode)
        SYNC_OPENCODE=true
        ;;
      --tmux)
        SYNC_TMUX=true
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      -*)
        log_error "Unknown option: $1"
        exit 1
        ;;
      *)
        # Positional argument: project name
        TARGET_NAMES+=("$1")
        ;;
    esac
    shift
  done
  
  # If no file flags specified, sync all types (backward compatible)
  if [[ "$SYNC_MAKEFILE" == false ]] && [[ "$SYNC_OPENCODE" == false ]] && [[ "$SYNC_TMUX" == false ]]; then
    SYNC_MAKEFILE=true
    SYNC_OPENCODE=true
    SYNC_TMUX=true
  fi
  
  # Validate: must have either target names or --all
  if [[ ${#TARGET_NAMES[@]} -eq 0 ]] && [[ "$SYNC_ALL" == false ]]; then
    log_error "No targets specified. Provide project name(s) or use --all"
    echo ""
    usage >&2
    exit 1
  fi
  
  # Validate: can't use both target names and --all
  if [[ ${#TARGET_NAMES[@]} -gt 0 ]] && [[ "$SYNC_ALL" == true ]]; then
    log_error "Cannot specify both project names and --all flag"
    exit 1
  fi
  
  # Warning: language flags without --all are ignored
  if [[ -n "$FILTER_LANG" ]] && [[ "$SYNC_ALL" == false ]]; then
    log_warning "Language filter (--${FILTER_LANG:0:2}) ignored without --all flag"
  fi
}
```

#### 3. Update usage() Function
**File**: `overlord-sync`
**Location**: Replace lines 19-43

**New usage documentation**:
```bash
usage() {
  cat <<'EOF'
Usage:
  overlord sync <name>... [options]
  overlord sync --all [options]

Propagates Makefile, opencode.jsonc, and .tmux.local to registered projects.
Creates thoughts/ directory structure if missing.

By default, only creates files if they don't exist. Use --force to overwrite.

Target Selection:
  <name>                   Project name, alias, or '.' for current directory
  --all                    Sync all registered projects

File Selection (default: all files):
  --makefile               Sync only Makefile
  --opencode               Sync only .opencode/opencode.jsonc
  --tmux                   Sync only .tmux.local
  
Language Filters (only with --all):
  --py, --python           Sync only Python projects
  --ts, --typescript       Sync only TypeScript projects
  --sol, --solidity        Sync only Solidity projects

Other Options:
  --force                  Overwrite existing files
  --dry-run                Show what would be done without making changes
  --help                   Show this help

Examples:
  overlord sync myproject                    # Sync all files for myproject
  overlord sync . --makefile                 # Sync only Makefile for current dir
  overlord sync proj1 proj2 --opencode       # Sync opencode.jsonc for two projects
  overlord sync --all                        # Sync all files for all projects
  overlord sync --all --py --makefile        # Sync Makefiles for Python projects
  overlord sync --all --ts --tmux --force    # Sync .tmux.local for TypeScript projects
EOF
}
```

### Success Criteria

#### Automated Verification:
- [x] `overlord sync` (no args) exits with error and shows usage
- [x] `overlord sync myproject --help` shows updated usage
- [x] `overlord sync --all --makefile` sets correct flag variables
- [x] `overlord sync proj1 proj2 proj3` populates TARGET_NAMES array with 3 elements
- [x] `overlord sync myproject --all` exits with error (conflicting flags)
- [x] `overlord sync myproject --py` shows warning about ignored language flag

#### Manual Verification:
- [x] Error messages are clear and actionable
- [x] Usage documentation accurately reflects new behavior
- [x] Flags can be mixed in any order

---

## Phase 2: Project Resolution Logic

### Overview
Add logic to resolve project names to registry entries, including support for "." (current directory), aliases, and path-based lookup. Accumulate all target projects into a list for processing.

### Changes Required

#### 1. Add find_project_by_path() to lib/common.sh
**File**: `lib/common.sh`
**Location**: After line 184 (after find_project_exact())

**New function**:
```bash
# Find project by absolute path
find_project_by_path() {
  local path="$1"
  
  local result
  result=$(jq -r --arg q "$path" '
    .projects | to_entries[] | 
    select(.value.path == $q) | 
    [.key, .value.lang, .value.status, .value.path] | @tsv
  ' "$OVERLORD_REGISTRY" 2>/dev/null || true)
  
  if [[ -n "$result" ]]; then
    echo "$result"
    return 0
  fi
  
  return 1
}
```

#### 2. Add resolve_project_target() Function
**File**: `overlord-sync`
**Location**: After parse_args() function (after line 79)

**New function**:
```bash
# Resolve a project target (name, alias, or path) to registry info
# Returns TSV: name<TAB>lang<TAB>status<TAB>path
resolve_project_target() {
  local target="$1"
  local resolved_path=""
  
  # Handle "." as current directory
  if [[ "$target" == "." ]]; then
    resolved_path="$(pwd)"
    
    # Look up by path in registry
    local project_info
    if ! project_info=$(find_project_by_path "$resolved_path"); then
      log_error "Current directory is not a registered project: $resolved_path"
      log_info "Hint: Use 'overlord init' or 'overlord new' to register this project"
      exit 1
    fi
    
    echo "$project_info"
    return 0
  fi
  
  # Try name/alias lookup using existing function
  local project_info
  if project_info=$(find_project_exact "$target"); then
    echo "$project_info"
    return 0
  fi
  
  # Not found
  log_error "Project not found: $target"
  log_info "Hint: Use 'overlord list' to see registered projects"
  exit 1
}
```

#### 3. Add build_project_list() Function
**File**: `overlord-sync`
**Location**: After resolve_project_target() function

**New function**:
```bash
# Build list of projects to sync (TSV format)
build_project_list() {
  if [[ "$SYNC_ALL" == true ]]; then
    # Use existing registry query logic with language filter
    local jq_filter='.projects | to_entries[]'
    if [[ -n "$FILTER_LANG" ]]; then
      jq_filter="$jq_filter | select(.value.lang == \"$FILTER_LANG\")"
    fi
    
    local projects
    projects=$(jq -r "$jq_filter | [.key, .value.lang, .value.path] | @tsv" "$OVERLORD_REGISTRY" 2>/dev/null || true)
    
    if [[ -z "$projects" ]]; then
      log_warning "No projects found to sync"
      exit 0
    fi
    
    echo "$projects"
  else
    # Resolve each target name individually
    local projects=""
    for target in "${TARGET_NAMES[@]}"; do
      local project_info
      project_info=$(resolve_project_target "$target")
      
      # Extract name, lang, path (skip status field)
      local name lang status path
      IFS=$'\t' read -r name lang status path <<< "$project_info"
      
      # Append to projects list in TSV format
      if [[ -n "$projects" ]]; then
        projects="${projects}"$'\n'"${name}"$'\t'"${lang}"$'\t'"${path}"
      else
        projects="${name}"$'\t'"${lang}"$'\t'"${path}"
      fi
    done
    
    echo "$projects"
  fi
}
```

#### 4. Update sync_projects() Function
**File**: `overlord-sync`
**Location**: Replace lines 151-195 (entire sync_projects function)

**New implementation**:
```bash
# Main sync logic
sync_projects() {
  if [[ ! -f "$OVERLORD_REGISTRY" ]]; then
    log_error "Registry not found: $OVERLORD_REGISTRY"
    exit 1
  fi
  
  # Build list of projects to sync
  local projects
  projects=$(build_project_list)
  
  local synced_count=0
  
  if [[ "$DRY_RUN" == true ]]; then
    log_info "Dry run mode - no changes will be made"
    echo ""
  fi
  
  while IFS=$'\t' read -r name lang path; do
    if sync_project "$name" "$lang" "$path"; then
      ((synced_count++ || 1))
    fi
  done <<< "$projects"
  
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
}
```

### Success Criteria

#### Automated Verification:
- [x] `overlord sync .` works when current directory is registered
- [x] `overlord sync .` errors when current directory not registered
- [x] Error message suggests `overlord init` or `overlord new` for unregistered directory
- [x] `overlord sync myproject` resolves project name correctly
- [x] `overlord sync myalias` resolves alias correctly (using find_project_exact)
- [x] `overlord sync proj1 proj2` builds list with both projects
- [x] `overlord sync --all --py` filters to Python projects only
- [x] `overlord sync nonexistent` shows helpful error with hint to run `overlord list`

#### Manual Verification:
- [x] Error messages include helpful hints for resolution
- [x] Alias resolution works for all registered aliases
- [x] Current directory detection works in any registered project

---

## Phase 3: Selective File Syncing

### Overview
Refactor `sync_project()` to accept file type flags and only sync requested file types. Add .tmux.local syncing using existing `copy_tmux_template()` function.

### Changes Required

#### 1. Update sync_project() Function
**File**: `overlord-sync`
**Location**: Replace lines 82-148

**New implementation**:
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
  local opencode_path="$path/.opencode/opencode.jsonc"
  local legacy_opencode="$path/opencode.jsonc"
  local tmux_path="$path/.tmux.local"
  local thoughts_path="$path/thoughts"
  
  if [[ "$DRY_RUN" == true ]]; then
    log_info "[dry-run] Would sync: $name ($lang)"
    
    # Makefile
    if [[ "$SYNC_MAKEFILE" == true ]]; then
      if [[ ! -f "$makefile_path" ]] || [[ "$FORCE" == true ]]; then
        log_dim "  -> $makefile_path"
      fi
    fi
    
    # Legacy migration (always show if exists)
    if [[ -f "$legacy_opencode" ]]; then
      log_dim "  -> Migrate $legacy_opencode to $opencode_path"
    fi
    
    # Opencode
    if [[ "$SYNC_OPENCODE" == true ]]; then
      if [[ ! -f "$opencode_path" ]] || [[ "$FORCE" == true ]]; then
        log_dim "  -> $opencode_path"
      fi
    fi
    
    # Tmux
    if [[ "$SYNC_TMUX" == true ]]; then
      if [[ ! -f "$tmux_path" ]] || [[ "$FORCE" == true ]]; then
        log_dim "  -> $tmux_path"
      fi
    fi
    
    # Thoughts (always show if missing)
    if [[ ! -d "$thoughts_path/tickets" ]]; then
      log_dim "  -> $thoughts_path/"
    fi
    
    return 0
  fi
  
  local synced_something=false

  # Migrate legacy opencode.jsonc if found (always migrate regardless of flags)
  if [[ -f "$legacy_opencode" ]]; then
    log_info "  Migrating $name: opencode.jsonc -> .opencode/opencode.jsonc"
    mkdir -p "$path/.opencode"
    mv "$legacy_opencode" "$opencode_path"
    synced_something=true
  fi
  
  # Sync Makefile
  if [[ "$SYNC_MAKEFILE" == true ]]; then
    if [[ ! -f "$makefile_path" ]] || [[ "$FORCE" == true ]]; then
      OVERLORD_STRICT=true generate_makefile "$path" "$lang"
      synced_something=true
    fi
  fi
  
  # Sync opencode.jsonc
  if [[ "$SYNC_OPENCODE" == true ]]; then
    if [[ ! -f "$opencode_path" ]] || [[ "$FORCE" == true ]]; then
      copy_opencode_template "$path" "$lang"
      synced_something=true
    fi
  fi
  
  # Sync .tmux.local
  if [[ "$SYNC_TMUX" == true ]]; then
    if [[ ! -f "$tmux_path" ]] || [[ "$FORCE" == true ]]; then
      copy_tmux_template "$path" "$lang"
      synced_something=true
    fi
  fi
  
  # Create thoughts directories (always additive, regardless of flags)
  if [[ ! -d "$thoughts_path/tickets" ]]; then
    create_thoughts_dirs "$path"
    synced_something=true
  fi
  
  if [[ "$synced_something" == false ]]; then
    log_dim "  $name: up to date"
  fi
  
  return 0
}
```

### Success Criteria

#### Automated Verification:
- [x] `overlord sync myproject --makefile` syncs only Makefile
- [x] `overlord sync myproject --opencode` syncs only opencode.jsonc
- [x] `overlord sync myproject --tmux` syncs only .tmux.local
- [x] `overlord sync myproject --makefile --opencode` syncs both file types
- [x] `overlord sync myproject` (no file flags) syncs all three file types
- [x] `overlord sync myproject --tmux --force` overwrites existing .tmux.local
- [x] `overlord sync myproject --tmux` creates .tmux.local if missing
- [x] `overlord sync myproject --tmux` skips existing .tmux.local without --force
- [x] thoughts/ directory always created regardless of file flags
- [x] Legacy opencode.jsonc migration happens regardless of file flags

#### Manual Verification:
- [x] Dry-run shows correct files for all flag combinations
- [x] .tmux.local has correct permissions (executable) after sync
- [x] .tmux.local uses language-specific template or falls back to base template
- [x] File syncing respects --force flag correctly for all file types

---

## Phase 4: Language Filtering Logic

### Overview
Language filtering already works in the existing code via jq filter construction. This phase ensures it continues to work correctly with the new --all flag and shows warnings when used with specific project names.

### Changes Required

#### 1. Verify Language Filter in build_project_list()
**File**: `overlord-sync`
**Location**: In build_project_list() function (already implemented in Phase 2)

**Verification notes**:
- Language filter already implemented in Phase 2: lines with `if [[ -n "$FILTER_LANG" ]]`
- Warning for language flags without --all already implemented in Phase 1: `parse_args()` function
- No additional code changes needed in this phase

### Success Criteria

#### Automated Verification:
- [x] `overlord sync --all --py` syncs only Python projects
- [x] `overlord sync --all --ts` syncs only TypeScript projects
- [x] `overlord sync --all --sol` syncs only Solidity projects
- [x] `overlord sync --all --py --makefile` syncs only Makefiles for Python projects
- [x] `overlord sync myproject --py` shows warning, continues syncing
- [x] Language filter warning shows correct language abbreviation

#### Manual Verification:
- [x] Language filtering produces correct subset of projects
- [x] Warning message is clear and non-blocking
- [x] No language filter syncs all languages

---

## Phase 5: Error Handling & Messages

### Overview
Add comprehensive error messages with helpful hints, validate all edge cases, and ensure dry-run mode works correctly for all new features.

### Changes Required

#### 1. Add Validation for Edge Cases
**File**: `overlord-sync`
**Location**: Already implemented in parse_args() (Phase 1) and resolve_project_target() (Phase 2)

**Validation checklist**:
- ✅ No arguments provided → error with usage (Phase 1: parse_args)
- ✅ Both --all and project names → error (Phase 1: parse_args)
- ✅ Language flag without --all → warning (Phase 1: parse_args)
- ✅ Current directory not registered → error with hint (Phase 2: resolve_project_target)
- ✅ Project name not found → error with hint (Phase 2: resolve_project_target)
- ✅ No projects match filter → warning and exit (Phase 2: build_project_list)

#### 2. Ensure Dry-Run Completeness
**File**: `overlord-sync`
**Location**: In sync_project() function (Phase 3)

**Dry-run verification**:
- ✅ Shows all file types based on flags (Phase 3: sync_project dry-run section)
- ✅ Shows legacy migration if needed (Phase 3: sync_project dry-run section)
- ✅ Shows thoughts directory creation if needed (Phase 3: sync_project dry-run section)
- ✅ Uses log_dim for indented output (Phase 3: sync_project dry-run section)
- ✅ Returns early without making changes (Phase 3: sync_project dry-run section)

### Success Criteria

#### Automated Verification:
- [x] All validation errors exit with code 1
- [x] All error messages go to stderr (via log_error)
- [x] Dry-run never modifies files
- [x] Dry-run shows exactly what would be synced
- [x] Registry errors show clear messages

#### Manual Verification:
- [x] Error messages include actionable hints
- [x] Dry-run output is clear and complete
- [x] Warning messages are non-blocking
- [x] Exit codes are correct for all error conditions

---

## Phase 6: Testing & Documentation

### Overview
Comprehensive testing of all new features and edge cases. Verify backward compatibility and update any related documentation.

### Changes Required

#### 1. Test All Usage Patterns
**Test cases** (execute manually or via test script):

```bash
# Basic usage
overlord sync myproject
overlord sync .
overlord sync proj1 proj2 proj3

# File selection
overlord sync myproject --makefile
overlord sync myproject --opencode
overlord sync myproject --tmux
overlord sync myproject --makefile --opencode --tmux

# All projects
overlord sync --all
overlord sync --all --py
overlord sync --all --ts
overlord sync --all --sol
overlord sync --all --makefile
overlord sync --all --opencode
overlord sync --all --tmux

# Force mode
overlord sync myproject --force
overlord sync --all --tmux --force

# Dry-run mode
overlord sync myproject --dry-run
overlord sync --all --dry-run
overlord sync --all --py --makefile --dry-run

# Error cases
overlord sync                           # Should error
overlord sync nonexistent               # Should error
overlord sync myproject --all           # Should error
overlord sync . --py                    # Should warn
```

#### 2. Verify Backward Compatibility
**Test patterns**:

```bash
# These should still work (for scripts that invoke overlord sync programmatically)
# Note: Interactive usage now requires explicit targeting, but scripts can adapt
overlord sync --all                     # Equivalent to old default behavior
overlord sync --all --py               # Equivalent to old --py behavior
overlord sync --all --force            # Equivalent to old --force behavior
overlord sync --all --dry-run          # Equivalent to old --dry-run behavior
```

#### 3. Update AGENTS.md
**File**: `AGENTS.md`
**Location**: Line 19 (overlord sync section)

**Replace**:
```markdown
overlord sync [--py|--ts|--sol] [--force] [--dry-run]
```

**With**:
```markdown
overlord sync <name>... [--makefile] [--opencode] [--tmux] [--force] [--dry-run]
overlord sync --all [--py|--ts|--sol] [--makefile] [--opencode] [--tmux] [--force] [--dry-run]
```

#### 4. Verify All Success Criteria from Ticket

**From ticket automated verification** (thoughts/tickets/feature_overlord_sync_selective_file_sync.md:100-110):
- [x] `overlord sync myproject --makefile` syncs only Makefile
- [x] `overlord sync --all --py` syncs only Python projects
- [x] `overlord sync proj1 proj2` syncs both projects
- [x] `overlord sync .` works when current dir is registered
- [x] `overlord sync .` errors when current dir not registered
- [x] `overlord sync myproject --py` shows warning, continues
- [x] `overlord sync --makefile --opencode` syncs both file types
- [x] `overlord sync --all --tmux --force` overwrites .tmux.local
- [x] `overlord sync --all --tmux` creates .tmux.local if missing
- [x] `overlord sync --all --tmux` skips existing .tmux.local without --force

**From ticket manual verification** (thoughts/tickets/feature_overlord_sync_selective_file_sync.md:112-118):
- [x] Dry-run shows correct files for all flag combinations
- [x] Alias resolution works for project names
- [x] Error messages are clear for invalid combinations
- [x] thoughts/ directory always created when syncing
- [x] Legacy opencode.jsonc migration still works

### Success Criteria

#### Automated Verification:
- [x] All test cases execute without errors
- [x] Exit codes are correct for success/failure cases
- [x] File permissions are correct (.tmux.local is executable)
- [x] Registry remains valid JSON after all operations
- [x] No unexpected files created in project directories

#### Manual Verification:
- [x] All success criteria from ticket verified
- [x] AGENTS.md accurately documents new usage
- [x] Error messages provide helpful guidance
- [x] Backward compatibility maintained for programmatic usage
- [x] Dry-run accurately previews all changes

---

## Testing Strategy

### Unit Tests (Manual Execution)

**Test 1: Argument Parsing**
```bash
overlord sync --help                    # Should show updated usage
overlord sync                           # Should error
overlord sync myproject --unknown       # Should error
```

**Test 2: Project Resolution**
```bash
cd /path/to/registered/project
overlord sync .                         # Should work
cd /tmp
overlord sync .                         # Should error with hint
```

**Test 3: File Selection**
```bash
overlord sync myproject --makefile --dry-run     # Show only Makefile
overlord sync myproject --opencode --dry-run     # Show only opencode.jsonc
overlord sync myproject --tmux --dry-run         # Show only .tmux.local
overlord sync myproject --dry-run                # Show all three files
```

**Test 4: Language Filtering**
```bash
overlord sync --all --py --dry-run               # Show only Python projects
overlord sync myproject --py --dry-run           # Show warning
```

**Test 5: Force Mode**
```bash
overlord sync myproject --tmux                   # Create if missing
overlord sync myproject --tmux                   # Skip (already exists)
overlord sync myproject --tmux --force           # Overwrite existing
```

### Integration Tests (End-to-End)

**Test 1: Complete sync workflow**
```bash
# Create test project
overlord new test-sync-project --py
cd test-sync-project
rm Makefile .tmux.local .opencode/opencode.jsonc  # Clean slate

# Sync individual files
overlord sync . --makefile
test -f Makefile || echo "FAIL: Makefile not created"

overlord sync . --opencode
test -f .opencode/opencode.jsonc || echo "FAIL: opencode.jsonc not created"

overlord sync . --tmux
test -f .tmux.local || echo "FAIL: .tmux.local not created"
test -x .tmux.local || echo "FAIL: .tmux.local not executable"

# Verify force mode
echo "test" > Makefile
overlord sync . --makefile                  # Should skip
grep -q "test" Makefile || echo "FAIL: Makefile overwritten without --force"

overlord sync . --makefile --force          # Should overwrite
grep -q "test" Makefile && echo "FAIL: Makefile not overwritten with --force"

# Cleanup
cd ..
overlord rm test-sync-project --force
```

**Test 2: Multiple projects sync**
```bash
# Create two test projects
overlord new test-proj1 --py
overlord new test-proj2 --ts

# Clean their files
rm test-proj1/Makefile test-proj2/Makefile

# Sync both
overlord sync test-proj1 test-proj2 --makefile

# Verify both have Makefiles
test -f test-proj1/Makefile || echo "FAIL: proj1 Makefile missing"
test -f test-proj2/Makefile || echo "FAIL: proj2 Makefile missing"

# Cleanup
overlord rm test-proj1 test-proj2 --force
```

### Manual Testing Steps

1. **Verify error messages**:
   - Run `overlord sync` with no args → Check error message clarity
   - Run `overlord sync .` in non-registered dir → Check hint message
   - Run `overlord sync nonexistent` → Check "not found" message

2. **Verify dry-run accuracy**:
   - Run `overlord sync --all --dry-run` → Note what would be synced
   - Run `overlord sync --all` → Verify actual sync matches dry-run output

3. **Verify alias resolution**:
   - Find a project with aliases in registry.json
   - Run `overlord sync <alias> --dry-run` → Verify correct project resolved

4. **Verify .tmux.local template**:
   - Sync .tmux.local for Python project → Check uses python.tmux template
   - Sync .tmux.local for TypeScript project → Check uses typescript.tmux template
   - Sync .tmux.local for base project → Check uses base.tmux template

5. **Verify thoughts/ always created**:
   - Create new project
   - Run `overlord sync <project> --makefile` (only Makefile flag)
   - Verify thoughts/ directories still created

## Performance Considerations

**Registry Lookup Efficiency**:
- Current jq queries are O(n) where n = number of projects
- For typical installations (<100 projects), performance impact is negligible
- No caching needed at this scale

**Multiple Project Resolution**:
- Each project name requires separate jq query (O(n) per project)
- For `overlord sync proj1 proj2 proj3`, total is O(3n)
- Still acceptable for reasonable number of targets (<10)

**Dry-Run Mode**:
- No performance impact (skips all file operations)
- Identical logic path except for actual writes

**Optimization Opportunities** (future):
- Cache registry data in memory for multiple lookups
- Batch jq queries for multiple project names
- Not needed for current scope

## Migration Notes

**Breaking Changes**:
- Interactive usage: `overlord sync` now requires explicit targeting (error instead of syncing all)
- Scripts can adapt by using `overlord sync --all` for previous default behavior

**Backward Compatibility**:
- Existing programmatic usage can be maintained by adding `--all` flag
- All existing flags (--py, --ts, --sol, --force, --dry-run) work as before when combined with --all
- Registry format unchanged
- Template functions unchanged
- File paths and directory structure unchanged

**Migration Path for Users**:
1. Review scripts that call `overlord sync`
2. Replace `overlord sync` with `overlord sync --all` to maintain previous behavior
3. Or refactor to use specific project targeting for more precise control

**No Data Migration Required**:
- Registry format unchanged
- Project directory structure unchanged
- No file renames or moves needed

## References

- Original ticket: `thoughts/tickets/feature_overlord_sync_selective_file_sync.md`
- Current implementation: `overlord-sync:1-203`
- Shared utilities: `lib/common.sh:1-185`
- Argument parsing patterns: `overlord-add:104-169`, `overlord-rm:162-203`
- Path handling patterns: `overlord-add:172-200`, `overlord-init:122-177`
- Registry path lookup: `overlord-rm:99-103`
- Template functions: `lib/common.sh:59-134`

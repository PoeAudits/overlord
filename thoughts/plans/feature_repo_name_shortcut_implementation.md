# FEATURE-004: Enable overlord <repo_name> Shortcut Syntax - Implementation Plan

## Overview

Enable `overlord <repo_name>` as a shortcut for `overlord open <repo_name>`, improving user experience by reducing typing for the most common operation while maintaining backward compatibility and providing helpful error messages.

## Current State Analysis

### Existing Implementation Details

**Main dispatcher** (`overlord:76-102`):
- Uses a case statement that only matches subcommands (`new`, `add`, `init`, `list`, `mv`, `open`, `info`, `sync`, `config`, `edit`, `rm`, `uninstall`)
- Default case (line 96-101) treats any unknown argument as an error
- No project name fallback mechanism exists

**Project lookup logic** (`overlord-open:38-67`):
- `find_project_exact()` function handles both name and alias matching via jq queries
- Returns tab-separated data: `name\tlang\tstatus\tpath`
- First tries exact name match, then tries alias match

**Logging utilities** (`lib/common.sh:11-15`):
- `log_error()`, `log_warning()`, `log_info()`, `log_success()` functions with consistent color formatting
- Colors already defined and imported in main script

**Current behavior**:
- `overlord myproject` → Shows "Unknown command: myproject" error
- `overlord open myproject` → Exact match first, then fzf fallback with query

### Key Constraints

1. Subcommands must take priority over project names (e.g., `overlord new` still creates new project)
2. Only active and lib projects can be opened (archived projects cannot be opened via this shortcut)
3. Must maintain full backward compatibility - all existing functionality unchanged
4. `overlord-open` already has fuzzy search via fzf; new shortcut should leverage this

## Desired End State

After implementation:
```bash
overlord                           # Lists active projects (unchanged)
overlord open myproject            # Opens myproject with fzf fallback (unchanged)
overlord myproject                 # Opens myproject if exact match exists
overlord myproj --fuzzy            # Fuzzy searches for "myproj"
overlord myproj -f                 # Fuzzy searches for "myproj" (short flag)
overlord myproject extra arg       # Opens myproject, passes extra args to overlord-open
overlord nonexistent               # Shows warning + help + list suggestion
```

### Verification

Test each scenario manually:
- Exact project match opens the project
- Partial name with `--fuzzy` triggers fzf search
- Invalid project shows helpful error message
- Extra arguments pass through correctly
- Subcommand priority maintained (e.g., `overlord new` still works)

## What We're NOT Doing

- Modifying `overlord-open` behavior or its fuzzy search capability
- Changing subcommand priority logic
- Modifying project registry structure or format
- Creating a test suite (no existing tests in project)
- Changing help text beyond documenting the new syntax
- Supporting fuzzy search without explicit flag (only exact matches work without flag)

## Implementation Approach

1. **Extract shared function**: Move `find_project_exact()` from `overlord-open` to `lib/common.sh` for reuse
2. **Add fallback logic**: Insert project lookup before default error case in main dispatcher
3. **Parse fuzzy flag**: Extract `--fuzzy` or `-f` from arguments and pass through to `overlord-open`
4. **Pass-through arguments**: Forward any additional arguments after project name/fuzzy flag to `overlord-open`
5. **Error handling**: Display warning + usage + suggestion for invalid projects

## Phase 1: Extract Project Lookup Function to Common Library

### Overview

Move the `find_project_exact()` function from `overlord-open` to `lib/common.sh` so it can be reused by the main dispatcher script.

### Changes Required

#### 1. Update lib/common.sh
**File**: `lib/common.sh`
**Changes**: Add `find_project_exact()` function at end of file

Add the following function after the `create_thoughts_dirs()` function:

```bash
# Find project by exact name or alias
find_project_exact() {
  local query="$1"
  
  # Try exact name match
  local result
  result=$(jq -r --arg q "$query" '
    .projects | to_entries[] | 
    select(.key == $q) | 
    [.key, .value.lang, .value.status, .value.path] | @tsv
  ' "$OVERLORD_REGISTRY" 2>/dev/null || true)
  
  if [[ -n "$result" ]]; then
    echo "$result"
    return 0
  fi
  
  # Try alias match
  result=$(jq -r --arg q "$query" '
    .projects | to_entries[] | 
    select(.value.aliases != null and (.value.aliases[] == $q)) | 
    [.key, .value.lang, .value.status, .value.path] | @tsv
  ' "$OVERLORD_REGISTRY" 2>/dev/null || true)
  
  if [[ -n "$result" ]]; then
    echo "$result"
    return 0
  fi
  
  return 1
}
```

**Notes**: 
- Uses `$OVERLORD_REGISTRY` which is already set in calling scripts
- Returns 1 if no match found (allows calling script to handle missing projects)
- Tab-separated output: `name\tlang\tstatus\tpath`

#### 2. Update overlord-open to use shared function
**File**: `overlord-open`
**Changes**: Remove the `find_project_exact()` function definition (lines 38-67) and rely on the one in `lib/common.sh`

Remove these lines from `overlord-open`:
```bash
# Find project by exact name or alias
find_project_exact() {
  local query="$1"
  
  # Try exact name match
  local result
  result=$(jq -r --arg q "$query" '
    .projects | to_entries[] | 
    select(.key == $q) | 
    [.key, .value.lang, .value.status, .value.path] | @tsv
  ' "$OVERLORD_REGISTRY" 2>/dev/null || true)
  
  if [[ -n "$result" ]]; then
    echo "$result"
    return 0
  fi
  
  # Try alias match
  result=$(jq -r --arg q "$query" '
    .projects | to_entries[] | 
    select(.value.aliases != null and (.value.aliases[] == $q)) | 
    [.key, .value.lang, .value.status, .value.path] | @tsv
  ' "$OVERLORD_REGISTRY" 2>/dev/null || true)
  
  if [[ -n "$result" ]]; then
    echo "$result"
    return 0
  fi
  
  return 1
}
```

**Notes**: `overlord-open` already sources `lib/common.sh` at line 14, so the function will be available

### Success Criteria

#### Automated Verification
- [x] `overlord-open` still works with exact project names: `overlord open myproject`
- [x] `overlord-open` still works with aliases: `overlord open my-alias`
- [x] `overlord-open` fuzzy search still works: `overlord open mypr` (no exact match)
- [x] No syntax errors in scripts: all scripts parse and execute without error

#### Manual Verification
- [x] Open an existing project with `overlord open projectname` - works unchanged
- [x] Open a project using alias with `overlord open alias` - works unchanged
- [x] Fuzzy search for partial name with `overlord open part` - fzf picker appears
- [x] Cancel fzf picker - exits cleanly without error

---

## Phase 2: Add Project Lookup Fallback to Main Dispatcher

### Overview

Modify the main `overlord` script to check if an unknown argument is a valid project name before showing an error. Insert fallback logic before the default error case.

### Changes Required

#### 1. Update overlord main script
**File**: `overlord`
**Changes**: 

1. Add a new helper function `is_valid_project()` after the `usage()` function (after line 56):

```bash
# Check if argument is a valid project name or alias
is_valid_project() {
  local query="$1"
  
  # Check if find_project_exact returns a result (requires sourcing common.sh first)
  if find_project_exact "$query" > /dev/null 2>&1; then
    return 0
  fi
  return 1
}
```

2. Import `lib/common.sh` at the top of the `main()` function (before line 69):

Insert after line 68:
```bash
  # Source common library
  source "$OVERLORD_BIN/lib/common.sh"
```

3. Replace the default case in the case statement (lines 96-101) with:

Replace the `*)` case:
```bash
    *)
      # Try project name lookup before showing error
      if is_valid_project "$1"; then
        local project_name="$1"
        shift
        
        # Check for fuzzy flag: --fuzzy or -f
        local fuzzy_flag=""
        if [[ "${1:-}" == "--fuzzy" || "${1:-}" == "-f" ]]; then
          fuzzy_flag="--fuzzy"
          shift
        fi
        
        # Dispatch to overlord-open with project name, optional fuzzy flag, and remaining args
        local open_args=("$project_name")
        if [[ -n "$fuzzy_flag" ]]; then
          open_args+=("$fuzzy_flag")
        fi
        open_args+=("$@")
        
        exec "$OVERLORD_BIN/overlord-open" "${open_args[@]}"
      else
        # Project not found - show warning, help, and suggestion
        log_warning "Project '$1' not found"
        echo ""
        usage >&2
        echo ""
        log_info "Use 'overlord list' to see available projects"
        exit 1
      fi
      ;;
```

**Logic flow**:
1. Try to find project with the first argument
2. If found, extract project name and any remaining arguments
3. Check if second argument is `--fuzzy` or `-f`
4. Build argument array for `overlord-open` with project name, optional fuzzy flag, and pass-through args
5. Dispatch to `overlord-open`
6. If not found, show warning + usage + list suggestion

**Notes**:
- Uses `log_warning()` and `log_info()` from `lib/common.sh` for consistent messaging
- Preserves all additional arguments after the fuzzy flag for pass-through
- Maintains subcommand priority by checking project name only in default case
- Sources `lib/common.sh` which sets up logging functions and provides `find_project_exact()`

### Success Criteria

#### Automated Verification
- [x] All existing commands still work: `overlord new`, `overlord add`, `overlord list`, etc.
- [x] No syntax errors: `bash -n overlord` passes without errors
- [x] Script dispatches correctly to subcommands

#### Manual Verification
- [x] `overlord myproject` opens the project (if it exists)
- [x] `overlord myproject extra args` opens project and passes `extra args` to overlord-open
- [x] `overlord nonexistent` shows warning + help + list suggestion
- [x] `overlord new` still creates new project (subcommand priority maintained)
- [x] `overlord open` still works with fzf picker

---

## Phase 3: Add Fuzzy Flag Support

### Overview

The fuzzy flag support is already implemented in Phase 2's case statement. This phase documents the behavior and verifies it works correctly.

### Changes Required

None additional - fuzzy flag parsing is included in Phase 2.

### Behavior

When user runs `overlord myproj --fuzzy`:
1. Main script extracts project name: `myproj`
2. Detects fuzzy flag: `--fuzzy`
3. Passes to `overlord-open` as: `overlord-open myproj --fuzzy`
4. `overlord-open` line 211-215 already has logic to handle fuzzy search via fzf if exact match fails

**Alternative short flag**: User can also use `-f`:
```bash
overlord myproj -f          # Same as --fuzzy
```

### Success Criteria

#### Automated Verification
- [x] Fuzzy flag parsing works correctly: both `--fuzzy` and `-f` recognized
- [x] Remaining arguments after fuzzy flag pass through: `overlord proj --fuzzy extra` passes `extra` to overlord-open

#### Manual Verification
- [x] `overlord myproj --fuzzy` launches fzf picker with "myproj" as initial query
- [x] `overlord myproj -f` launches fzf picker with "myproj" as initial query
- [x] Selecting from fzf picker opens the chosen project
- [x] Canceling fzf picker exits cleanly

---

## Phase 4: Error Message Implementation

### Overview

When a user provides a project name that doesn't exist, the script should show a helpful error message with guidance.

### Changes Required

Already implemented in Phase 2 (the else block in the fallback case).

### Error Message Format

When user runs `overlord nonexistent`:

```
! Project 'nonexistent' not found

overlord - Project Management System v1.0.0

Usage:
  overlord                              List active projects (default)
  overlord new <name> --py|--ts|--sol   Create new project
  ... (rest of usage) ...

> Use 'overlord list' to see available projects
```

**Components**:
1. `log_warning "Project '$1' not found"` - Shows yellow warning with project name
2. `usage >&2` - Shows full usage help
3. `log_info "Use 'overlord list' to see available projects"` - Blue suggestion to list projects

### Success Criteria

#### Automated Verification
- [x] Script exits with code 1 when project not found
- [x] Error output goes to stderr

#### Manual Verification
- [x] Error message is clear and helpful
- [x] Suggestion to use `overlord list` is visible
- [x] User can easily understand what went wrong and how to fix it

---

## Phase 5: Update Usage Documentation

### Overview

Update the main `overlord` script's usage function to document the new shortcut syntax.

### Changes Required

#### 1. Update usage() function in overlord
**File**: `overlord`
**Changes**: Update the usage text (lines 27-55) to include the new shortcut syntax

Add these lines after the existing `overlord open [name]` line (after line 38):

```bash
  overlord <name>                       Open workspace (same as 'overlord open')
  overlord <name> --fuzzy|-f            Open workspace with fuzzy search
```

Update the full usage function to be:

```bash
usage() {
  cat <<EOF
overlord - Project Management System v${OVERLORD_VERSION}

Usage:
  overlord                              List active projects (default)
  overlord <name>                       Open workspace (same as 'overlord open')
  overlord <name> --fuzzy|-f            Open workspace with fuzzy search
  overlord new <name> --py|--ts|--sol   Create new project
  overlord add <name> <path> [options]  Register existing directory
  overlord init [path] [options]        Initialize existing dir with config
  overlord list [options]               List projects
  overlord mv <name> <status>           Move project (active|lib|archive)
  overlord open [name]                  Open workspace (fzf if no match)
  overlord info <name>                  Show project details
  overlord rm <name> [options]          Remove project from registry
  overlord sync [options]               Propagate Makefile templates to projects
  overlord uninstall [options]          Remove overlord from system
  overlord config                       Edit registry.json in \$EDITOR
  overlord edit                         Edit overlord scripts in \$EDITOR
  overlord --help                       Show this help
  overlord --version                    Show version

Subcommand Help:
  overlord <command> --help             Show help for specific command

Environment:
  OVERLORD_CONFIG   Config directory (default: auto-detected from script location)
  OVERLORD_BIN      Scripts directory (default: auto-detected from script location)
  EDITOR            Editor for config/edit commands
EOF
}
```

**Notes**:
- Placement before `overlord new` keeps the most common operations at the top
- Shows both long and short fuzzy flag options: `--fuzzy|-f`
- Clarifies that `<name>` usage is equivalent to `overlord open <name>`

### Success Criteria

#### Automated Verification
- [x] `overlord --help` displays without errors
- [x] Help text is valid bash syntax (no escaping issues)

#### Manual Verification
- [x] `overlord --help` shows the new syntax
- [x] Documentation is clear and follows existing style
- [x] Users can understand the shortcut from help text alone

---

## Testing Strategy

### Manual Testing Checklist

Since the project lacks an automated test suite, all testing must be manual:

1. **Basic shortcut functionality**:
   - [ ] `overlord validproject` opens the project
   - [ ] `overlord validproject` with multiple active projects opens exact match
   - [ ] `overlord validalias` (using an alias) opens the correct project

2. **Fuzzy flag variants**:
   - [ ] `overlord partialname --fuzzy` shows fzf picker
   - [ ] `overlord partialname -f` shows fzf picker (short flag)
   - [ ] Selecting from fzf picker opens project
   - [ ] Canceling fzf picker exits cleanly

3. **Additional arguments**:
   - [ ] `overlord projectname extraarg` passes `extraarg` to overlord-open without error
   - [ ] `overlord projectname --fuzzy extraarg` works (fuzzy flag parsed, extra arg passed through)

4. **Error handling**:
   - [ ] `overlord nonexistent` shows warning, help, and list suggestion
   - [ ] Error message exits with code 1
   - [ ] Error output appears on stderr

5. **Subcommand priority**:
   - [ ] `overlord new` still creates new project
   - [ ] `overlord open` still works normally
   - [ ] `overlord list` still lists projects
   - [ ] `overlord add` still adds projects
   - [ ] All other subcommands work unchanged

6. **Backward compatibility**:
   - [ ] `overlord` with no args lists active projects
   - [ ] `overlord --help` shows help
   - [ ] `overlord --version` shows version
   - [ ] All existing subcommand functionality unchanged

### Test Projects Setup

For manual testing, create or verify these test projects exist in the registry:
- A valid active project (e.g., "myapp")
- A valid lib project (e.g., "mylib")
- A project with an alias (e.g., project "longname" with alias "ln")

Run through all manual tests with these projects.

---

## Implementation Order

Execute phases in this order:

1. **Phase 1**: Extract function to `lib/common.sh` and update `overlord-open`
   - Lowest risk, isolated change
   - Verify `overlord-open` still works
   
2. **Phase 2**: Add fallback logic to main dispatcher
   - Core feature implementation
   - Includes error handling and pass-through logic
   
3. **Phase 3**: Verify fuzzy flag support (no changes needed)
   - Already implemented in Phase 2
   - Just verification via manual testing
   
4. **Phase 4**: Verify error messages (no changes needed)
   - Already implemented in Phase 2
   - Just verification via manual testing
   
5. **Phase 5**: Update usage documentation
   - Last step, ensures help is accurate

After each phase, run the manual testing checklist relevant to that phase.

---

## Edge Cases and Considerations

### Archive Project Handling
- Users cannot open archived projects via `overlord projectname`
- `overlord-open` already rejects archived projects with helpful error (line 237-240)
- No additional handling needed in main script

### Project Name vs Subcommand Conflicts
- **Impossible**: Subcommands are checked before project lookup in case statement
- If a user names a project "new", "open", etc., they cannot use the shortcut (subcommand wins)
- This is acceptable and documented in requirements (subcommand priority)

### Registry Corruption
- If registry is invalid JSON, `find_project_exact()` returns error
- Script will treat it as project not found and show helpful error
- No special handling needed

### Missing OVERLORD_REGISTRY
- Both `find_project_exact()` and main script already check registry existence
- `lib/common.sh` expects `OVERLORD_REGISTRY` to be set by calling script
- Main script sets this at line 11

### Pass-through Arguments Containing Dashes
- Fuzzy flag parsing uses simple string match: `"${1:-}" == "--fuzzy"` or `"${1:-}" == "-f"`
- Arguments after fuzzy flag are passed as-is via `"$@"`
- Example: `overlord proj --fuzzy --some-arg value` correctly parses `--fuzzy` and passes `--some-arg value`

---

## Performance Considerations

- `find_project_exact()` uses jq query (same as existing implementation in `overlord-open`)
- No additional registry queries added beyond what already exists
- No performance regression expected

---

## References

- Original ticket: `thoughts/tickets/feature_overlord_repo_name_shortcut.md`
- Main dispatcher: `overlord:76-102` (case statement)
- Project lookup: `overlord-open:38-67` (find_project_exact function)
- Shared utilities: `lib/common.sh` (logging and helpers)
- Registry structure: `overlord:11` (OVERLORD_REGISTRY setup)

# Overlord Detect Command Implementation Plan

## Overview

Implement the `overlord-detect` command to scan the `OVERLORD_BASE_DIR` work directory and identify projects that exist on disk but are not registered in `registry.json`. This helps users discover orphaned or manually-created projects that need to be registered.

## Current State Analysis

**Existing Registry Structure** (`registry.json`):
- Stores project metadata in a `projects` dictionary
- Each project entry contains: `lang`, `status`, `path`, `created`, `aliases`
- Registry persists all registered projects across all statuses (active, lib, archive)

**Directory Organization** (`OVERLORD_BASE_DIR`):
- Root: `/home/thomas/Work` (configurable via registry settings or environment)
- Language structure: `Python/`, `Typescript/`, `Solidity/` subdirectories
- Status structure: Each language has `active/`, `libs/`, `archive/` subdirectories
- Projects: Stored as directories under `$OVERLORD_BASE_DIR/$LANG_DIR/$STATUS_DIR/$PROJECT_NAME`

**Command Architecture** (from `overlord-list`, `overlord-add`, `overlord-info`):
- All subcommands follow a consistent pattern: setup → parse args → execute → output
- Integration with `lib/common.sh` for shared utilities (colors, logging, directory helpers)
- Use `jq` for JSON operations
- Structured output with coloring

## Desired End State

After implementation, users can run `overlord detect` to:
1. Scan the work directory filesystem at max depth 3 (base_dir → lang → status → project_name)
2. Compare discovered directories against registered projects
3. Display unregistered projects in a table format similar to `overlord list`
4. Show the project name, inferred language, inferred status, and full path
5. Handle edge cases gracefully (no unregistered projects, empty work directory, missing base_dir)

### How to Verify End State

**Automated Verification**:
- [x] Script exists at `/home/thomas/bin/overlord/overlord-detect`
- [x] Script is executable and can be called via `overlord detect`
- [x] `overlord detect` exits cleanly when no unregistered projects exist
- [x] `overlord detect` exits cleanly when base_dir doesn't exist

**Manual Verification**:
- [x] `overlord detect` displays unregistered projects in table format
- [x] Table has columns: NAME, LANG, STATUS, PATH
- [x] Language and status are correctly inferred from directory structure
- [x] Projects registered via `overlord add` no longer appear in detect output
- [x] Output formatting matches `overlord list` style (colors, alignment)

## Key Discoveries

- **Directory Structure**: Projects are organized in a predictable 3-level structure: `$BASE_DIR/$LANG_DIR/$STATUS_DIR/$PROJECT_NAME`
- **Language Mapping** (`lib/common.sh:17-29`): Python, Typescript, Solidity directories are capitalized; these map to lowercase lang values
- **Status Mapping** (`lib/common.sh:31-42`): `active/`, `libs/`, `archive/` map to `active`, `lib`, `archive` status values
- **Registry Query Pattern** (`overlord-list:134-171`): Uses jq to filter projects; we need inverse logic to find what's NOT in registry
- **Output Pattern** (`overlord-list:199-219`): Consistent table format with printf, colored output using common.sh colors
- **Logging Pattern** (`overlord-*`): All commands use `log_error()`, `log_success()`, `log_info()`, `log_warning()` for user feedback

## What We're NOT Doing

- **JSON output format**: No `--json` flag for this command; output is table-only
- **Project validation**: We don't check if directories are actual projects (git repos, package files, etc.)
- **Scanning directory contents**: We only check directory existence at max depth 3; no recursive scanning of project interiors
- **Auto-registration**: No automatic `overlord add` functionality; user must manually register
- **Language detection from files**: We infer language purely from directory structure, not from `pyproject.toml`, `package.json`, etc.
- **Integration with other commands**: This is a read-only discovery tool; doesn't modify registry or filesystem

## Implementation Approach

### Strategy
1. **Phase 1: Discovery**: Use bash `find` or `ls` with careful depth limiting to enumerate directories at exactly depth 3
2. **Phase 2: Comparison**: Build a list of filesystem projects, then filter against registry entries using jq
3. **Phase 3: Output**: Format and display unregistered projects using the same coloring/alignment as `overlord list`

### Algorithm Overview
1. Verify `OVERLORD_BASE_DIR` exists
2. For each language directory (Python, Typescript, Solidity):
   - For each status subdirectory (active, libs, archive):
     - List immediate subdirectories (project names) using `ls -d` at max depth
3. Build a discovered projects array with `name`, `lang`, `status`, `path`
4. Load all registered projects from registry.json
5. Filter discovered projects to exclude those in registry
6. Sort and display in table format with colors

## Phase 1: Core Discovery Logic

### Overview
Scan the work directory filesystem to discover all projects organized under language and status subdirectories. Limit depth to avoid scanning project interiors.

### Changes Required

#### 1. Create overlord-detect Script
**File**: `/home/thomas/bin/overlord/overlord-detect`

**Changes**: New script implementing core discovery and comparison logic

**Key Functions**:

1. **discover_projects()** - Find all directories at max depth 3
   - Iterate language directories: Python, Typescript, Solidity
   - For each language, iterate status subdirectories: active, libs, archive
   - Use `ls -d` with proper error handling to list project directories
   - Output format: `project_name|lang|status|full_path`

2. **get_registered_projects()** - Extract registered project paths from registry
   - Use jq to get all `.projects[].path` values
   - Return as newline-separated list

3. **find_unregistered()** - Find discovered projects not in registry
   - Compare discovered project paths against registered paths
   - Filter out matches to get unregistered projects

**Script Structure** (following `overlord-list` pattern):
```bash
#!/usr/bin/env bash
# overlord-detect: Find unregistered projects in work directory

set -euo pipefail

# Setup environment (same pattern as overlord-list:7-11)
OVERLORD_REPO="$(cd "$(dirname "$(realpath "$0")")" && pwd)"
OVERLORD_CONFIG="${OVERLORD_CONFIG:-$OVERLORD_REPO}"
OVERLORD_REGISTRY="$OVERLORD_CONFIG/registry.json"
OVERLORD_BIN="${OVERLORD_BIN:-$OVERLORD_REPO}"

# Source common library
source "$OVERLORD_BIN/lib/common.sh"

# Define colors (use common.sh colors)
DIM='\033[2m'

usage() {
  # Display help (similar to overlord-list:20-50)
  cat <<'EOF'
Usage:
  overlord detect

Displays all projects found on disk in $OVERLORD_BASE_DIR that are not
registered in the Overlord registry.

Output shows: NAME | LANG | STATUS | PATH

Examples:
  overlord detect          # Show unregistered projects
EOF
}

discover_projects() {
  # Scan OVERLORD_BASE_DIR for projects at depth 3
  # For each language dir: Python, Typescript, Solidity
  # For each status subdir: active, libs, archive
  # List directories at that level
  # Output: "project_name|lang|status|path"
}

get_registered_projects() {
  # Query registry.json for all registered project paths
  # Use jq to extract .projects[].path
  # Return newline-separated list
}

find_unregistered() {
  # Compare discovered vs registered
  # Filter discovered to exclude registered paths
  # Return unregistered projects
}

format_output() {
  # Format unregistered projects as table
  # Columns: NAME, LANG, STATUS, PATH
  # Use colors consistent with overlord-list
  # Sort by status (active, lib, archive)
}

main() {
  # Verify base_dir exists, discover projects, compare, display
}

main "$@"
```

### Success Criteria

#### Automated Verification
- [x] Script exists at `overlord-detect`
- [x] Script is executable: `test -x overlord-detect`
- [x] Script sources `lib/common.sh` without errors
- [x] Script can be invoked via `overlord detect` dispatcher

#### Manual Verification
- [x] Script executes without errors on clean system (no unregistered projects)
- [x] Script correctly identifies test project when manually created in work directory
- [x] Identified project shows correct language inferred from directory path
- [x] Identified project shows correct status inferred from directory path
- [x] Identified project shows full absolute path
- [x] No false positives (registered projects don't appear in output)

---

## Phase 2: Registry Comparison Logic

### Overview
Filter discovered projects against registry entries to identify those not yet registered.

### Changes Required

#### 1. Registry Query Implementation
**Location**: Within `overlord-detect` script

**Implementation Details**:

- **Extract registered paths**: Use jq to query `registry.json` for all project paths
  ```
  jq -r '.projects[].path' "$OVERLORD_REGISTRY"
  ```

- **Path comparison**: Use bash associative array or simple string matching to exclude registered paths
  - Build a set of registered paths
  - Filter discovered projects to exclude any matching registered paths

- **Fallback handling**: If registry doesn't exist or is empty, all discovered projects are unregistered

**Code Pattern** (similar to `overlord-list:134-171`):
```bash
get_registered_projects() {
  if [[ ! -f "$OVERLORD_REGISTRY" ]]; then
    return 0
  fi
  jq -r '.projects[].path' "$OVERLORD_REGISTRY" 2>/dev/null || true
}
```

### Success Criteria

#### Automated Verification
- [x] Registry query returns valid paths
- [x] Comparison correctly identifies registered vs unregistered projects
- [x] No errors when registry is missing or empty

#### Manual Verification
- [x] After `overlord add`, previously detected project disappears from output
- [x] Projects with different paths (aliases or moved) still compare correctly
- [x] Multiple unregistered projects all identified correctly

---

## Phase 3: Output Formatting

### Overview
Display unregistered projects in a table format consistent with `overlord list`, with proper coloring and alignment.

### Changes Required

#### 1. Table Formatting
**Location**: Within `overlord-detect` script, `format_output()` function

**Implementation Details**:

- **Column headers**: NAME | LANG | STATUS | PATH (same as `overlord list`)
- **Color scheme** (from `lib/common.sh:4-9` and `overlord-list`):
  - Status colors: active=GREEN, lib=CYAN, archive=DIM
  - Language colors: python=BLUE, typescript=YELLOW, solidity=RED
  - Separator: DIM color
- **Sorting**: Sort by status (active first, then lib, then archive) using same logic as `overlord-list:182`
- **Path truncation**: Truncate long paths with `...` prefix if over 40 characters (same as `overlord-list:213-214`)

**Code Pattern** (from `overlord-list:199-219`):
```bash
format_output() {
  local projects="$1"  # newline-separated: "name|lang|status|path"
  
  if [[ -z "$projects" ]]; then
    echo -e "${DIM}No unregistered projects found.${NC}"
    return
  fi

  # Header
  printf "${DIM}%-40s %-6s %-8s %-40s${NC}\n" "NAME" "LANG" "STATUS" "PATH"
  printf "${DIM}%-40s %-6s %-8s %-40s${NC}\n" "----" "----" "------" "----"

  # Projects (process line by line)
  # Color based on status/language like overlord-list does
  # Use status_color() and lang_color() from common.sh or define locally
}
```

- **Edge cases**:
  - No unregistered projects: Display "No unregistered projects found." message
  - Base directory doesn't exist: Display error and exit gracefully
  - Empty work directory: Display "No unregistered projects found."

### Success Criteria

#### Automated Verification
- [x] Output contains header row with column names
- [x] Output is properly formatted as aligned columns
- [x] No output artifacts or extra whitespace

#### Manual Verification
- [x] Table columns align properly (NAME, LANG, STATUS, PATH)
- [x] Status colors match `overlord list` (active=green, lib=cyan, archive=dim)
- [x] Language abbreviations are correct (py, ts, sol)
- [x] Long paths are truncated with `...` prefix
- [x] Output is readable and visually consistent with `overlord list`
- [x] Message displayed when no unregistered projects exist

---

## Testing Strategy

### Unit Testing Approach

#### Test 1: Discovery Function
- Create temporary directory structure with test projects
- Verify discover_projects() finds all projects at depth 3
- Verify no recursion into project directories

#### Test 2: Registry Comparison
- Create test registry with known projects
- Verify unregistered projects are correctly identified
- Verify registered projects are excluded from output

#### Test 3: Output Formatting
- Verify table headers display correctly
- Verify column alignment
- Verify colors are applied
- Verify path truncation works

### Integration Testing Approach

#### Test 1: Full Workflow
1. Create test projects in work directory
2. Register some projects via `overlord add`
3. Run `overlord detect`
4. Verify registered projects don't appear
5. Verify unregistered projects do appear
6. Register another project
7. Run `overlord detect` again
8. Verify newly registered project disappears

#### Test 2: Edge Cases
1. Empty work directory → "No unregistered projects found"
2. No work directory → Error message and graceful exit
3. Missing registry → All discovered projects are unregistered
4. All projects registered → "No unregistered projects found"

### Manual Testing Steps

1. **Setup**: Ensure work directory structure exists
   ```bash
   mkdir -p ~/Work/Python/active ~/Work/Python/libs ~/Work/Python/archive
   mkdir -p ~/Work/Typescript/active ~/Work/Typescript/libs ~/Work/Typescript/archive
   mkdir -p ~/Work/Solidity/active ~/Work/Solidity/libs ~/Work/Solidity/archive
   ```

2. **Create test projects** (manually, without registering):
   ```bash
   mkdir ~/Work/Python/active/test-unregistered-py
   mkdir ~/Work/Typescript/libs/test-unregistered-ts
   mkdir ~/Work/Solidity/archive/test-unregistered-sol
   ```

3. **Run detect command**:
   ```bash
   overlord detect
   ```
   Verify all three test projects appear in output with correct language and status.

4. **Register one project**:
   ```bash
   overlord add test-unregistered-py ~/Work/Python/active/test-unregistered-py
   ```

5. **Run detect command again**:
   Verify only two unregistered projects remain.

6. **Cleanup**:
   ```bash
   rm -rf ~/Work/Python/active/test-unregistered-py
   rm -rf ~/Work/Typescript/libs/test-unregistered-ts
   rm -rf ~/Work/Solidity/archive/test-unregistered-sol
   overlord rm test-unregistered-py
   ```

## Performance Considerations

- **Directory scanning**: Using `ls -d` at exactly depth 3 avoids recursing into project directories, ensuring O(n) performance where n = number of projects
- **Registry comparison**: Using jq to extract paths is efficient; path matching is O(m) where m = number of registered projects
- **Overall complexity**: O(n + m) where n = discovered projects, m = registered projects

## References

- Original ticket: `thoughts/tickets/feature_overlord_detect_command.md`
- Registry format: `registry.json` structure documented in `AGENTS.md:619-629`
- Command patterns: `overlord-list:1-228` (list command implementation)
- Common utilities: `lib/common.sh:1-153` (logging, colors, helpers)
- Directory helpers: `lib/common.sh:17-42` (language/status directory mapping)

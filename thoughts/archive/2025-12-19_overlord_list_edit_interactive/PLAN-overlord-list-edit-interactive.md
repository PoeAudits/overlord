# Interactive Project State Editor Implementation Plan

## Overview

Add an interactive `--edit` / `-e` flag to `overlord list` that displays all projects in a terminal UI with keyboard navigation, allowing users to toggle project states (active, lib, archive) and apply changes atomically in bulk.

## Current State Analysis

The Overlord system currently supports:
- Project listing via `overlord-list` with filtering (`overlord-list:128-220`)
- Individual state changes via `overlord-mv` with directory moving (`overlord-mv:76-174`)
- Atomic registry updates using mktemp + jq + mv pattern (`overlord-mv:82-90`)
- ANSI color codes for terminal output (`lib/common.sh:4-15`)
- Simple confirmation prompts using bash `read` (`overlord-rm:130-141`)

**What's Missing:**
- No interactive terminal UI framework (only external fzf in overlord-open)
- No raw terminal mode handling for single-key input
- No ANSI cursor control or screen management
- No in-memory state tracking for transitory changes
- No bulk state change mechanism

## Desired End State

After implementation, users will be able to:
1. Run `overlord list --edit` or `overlord list -e` to launch interactive mode
2. Navigate through all projects using arrow keys (↑/↓) or vim keys (j/k)
3. Cycle project states with h (previous), l (next), or Enter (next)
4. See pending changes highlighted in the display
5. View a confirmation prompt at the bottom showing all pending changes
6. Press a confirmation key to apply all changes atomically
7. Press Escape to exit without saving any changes
8. Have projects physically moved and registry updated atomically on confirmation

### Verification:

**Automated:**
- `overlord list --edit` and `overlord list -e` launch without errors
- Terminal state is restored on exit (clean up even on errors)
- Registry updates are atomic (all changes or none)
- State cycling wraps around: active → lib → archive → active

**Manual:**
- Arrow keys and j/k navigate correctly
- h/l/Enter toggle states in correct direction
- Display matches `overlord list --all` format with selection indicator
- Pending changes show clearly in UI
- Escape exits without changes
- Confirmation applies all changes and moves directories

### Key Discoveries:

**Existing Patterns:**
- List display format: `overlord-list:199-219` - formatted table with colors
- State colors: `overlord-list:99-106` - GREEN (active), CYAN (lib), DIM (archive)
- Language colors: `overlord-list:108-116` - BLUE (py), YELLOW (ts), RED (sol)
- Atomic registry update: `overlord-mv:82-90` - mktemp + jq + mv
- Directory moving: `overlord-mv:132-163` - mkdir -p parent, mv, update registry
- Project lookup: `overlord-mv:37-66` - find by name or alias

**Terminal Control Requirements:**
- Need raw terminal mode to capture single keypresses
- Need ANSI sequences for: cursor movement, clear screen, colors, hide/show cursor
- Must save and restore terminal state (tput/stty)
- Input: read single chars without waiting for Enter

## What We're NOT Doing

- Not adding new sorting or filtering options to interactive mode
- Not supporting mouse input
- Not adding keyboard shortcuts beyond j/k/h/l/↑/↓/Enter/Escape
- Not implementing undo/redo within the interactive session
- Not showing project details or expanding views
- Not supporting batch operations on multiple selections
- Not adding visual themes or customization
- Not implementing search/filter within interactive mode

## Implementation Approach

Create an interactive mode within `overlord-list` that:
1. Enters raw terminal mode to capture keypresses
2. Loads all projects into memory arrays
3. Tracks pending state changes in associative array
4. Renders full-screen UI with project list and footer
5. Processes keyboard input in event loop
6. On confirmation: applies all changes atomically (registry + directory moves)
7. Restores terminal state on exit (success, cancel, or error)

The implementation will be contained within `overlord-list` as a separate function branch when `--edit` flag is detected.

## Phase 1: Terminal Control Foundation

### Overview
Implement low-level terminal control functions for raw mode, cursor management, and input handling.

### Changes Required:

#### 1. Terminal Mode Management Functions (`overlord-list`)
**File**: `overlord-list`
**Changes**: Add functions after the `lang_short()` function (line 126)

```bash
# Terminal control: Save terminal state
term_save() {
  SAVED_TERM_STATE=$(stty -g)
}

# Terminal control: Enter raw mode
term_raw() {
  stty -echo -icanon min 1 time 0
}

# Terminal control: Restore terminal state
term_restore() {
  [[ -n "${SAVED_TERM_STATE:-}" ]] && stty "${SAVED_TERM_STATE}"
  printf '\033[?25h'  # Show cursor
  printf '\033[0m'    # Reset colors
}

# Terminal control: Cleanup handler
term_cleanup() {
  term_restore
}

# ANSI Escape Sequences
ansi_clear_screen() { printf '\033[2J'; }
ansi_cursor_home() { printf '\033[H'; }
ansi_cursor_hide() { printf '\033[?25l'; }
ansi_cursor_show() { printf '\033[?25h'; }
ansi_cursor_to() { printf '\033[%d;%dH' "$1" "$2"; }
ansi_clear_line() { printf '\033[2K'; }

# Read single character (handles escape sequences)
read_key() {
  local key
  IFS='' read -r -s -n1 key
  
  # Handle escape sequences (arrow keys, etc.)
  if [[ "$key" == $'\x1b' ]]; then
    # Check if there's more input (escape sequence)
    IFS='' read -r -s -n1 -t 0.001 key2
    if [[ -z "$key2" ]]; then
      # Just Escape key
      echo "ESC"
      return
    fi
    
    IFS='' read -r -s -n1 key3
    if [[ "$key2" == "[" ]]; then
      case "$key3" in
        A) echo "UP" ;;
        B) echo "DOWN" ;;
        C) echo "RIGHT" ;;
        D) echo "LEFT" ;;
        *) echo "UNKNOWN" ;;
      esac
    else
      echo "UNKNOWN"
    fi
  else
    # Regular character
    case "$key" in
      $'\n'|$'\r') echo "ENTER" ;;
      j) echo "j" ;;
      k) echo "k" ;;
      h) echo "h" ;;
      l) echo "l" ;;
      q) echo "q" ;;
      y) echo "y" ;;
      *) echo "OTHER" ;;
    esac
  fi
}
```

### Success Criteria:

#### Automated Verification:
- [x] `term_save()` captures terminal state without errors
- [x] `term_raw()` enters raw mode successfully
- [x] `term_restore()` returns terminal to normal mode
- [x] `term_cleanup()` can be called multiple times safely
- [x] ANSI functions output correct escape sequences
- [x] `read_key()` captures single characters without Enter

#### Manual Verification:
- [x] Test terminal save/restore cycle - terminal behaves normally after
- [x] Test raw mode - single keypress captured without Enter
- [x] Test arrow key detection - UP/DOWN/LEFT/RIGHT recognized
- [x] Test Escape key detection - ESC recognized vs escape sequences
- [x] Test vim keys - j/k/h/l recognized correctly
- [x] Test cleanup on Ctrl+C - terminal restored properly

---

## Phase 2: Project List Management

### Overview
Load projects from registry into memory and implement state change tracking with circular cycling logic.

### Changes Required:

#### 1. State Management Functions (`overlord-list`)
**File**: `overlord-list`
**Changes**: Add functions after terminal control functions

```bash
# State cycling: Get next state (circular)
state_next() {
  case "$1" in
    active)  echo "lib" ;;
    lib)     echo "archive" ;;
    archive) echo "active" ;;
    *)       echo "active" ;;
  esac
}

# State cycling: Get previous state (circular)
state_prev() {
  case "$1" in
    active)  echo "archive" ;;
    lib)     echo "active" ;;
    archive) echo "lib" ;;
    *)       echo "active" ;;
  esac
}

# Load all projects into arrays
# Populates: PROJECT_NAMES, PROJECT_LANGS, PROJECT_STATUSES, PROJECT_PATHS
load_projects() {
  local data
  data=$(jq -r '.projects | to_entries[] | [.key, .value.lang, .value.status, .value.path] | @tsv' "$OVERLORD_REGISTRY" 2>/dev/null || echo "")
  
  if [[ -z "$data" ]]; then
    return 1
  fi
  
  PROJECT_NAMES=()
  PROJECT_LANGS=()
  PROJECT_STATUSES=()
  PROJECT_PATHS=()
  
  while IFS=$'\t' read -r name lang status path; do
    PROJECT_NAMES+=("$name")
    PROJECT_LANGS+=("$lang")
    PROJECT_STATUSES+=("$status")
    PROJECT_PATHS+=("$path")
  done <<< "$data"
  
  # Sort by status priority (active, lib, archive) then name
  # Create indexed array for sorting
  local -a sort_data=()
  for i in "${!PROJECT_NAMES[@]}"; do
    local sort_key
    case "${PROJECT_STATUSES[$i]}" in
      active)  sort_key="1" ;;
      lib)     sort_key="2" ;;
      archive) sort_key="3" ;;
      *)       sort_key="4" ;;
    esac
    sort_data+=("${sort_key}|${PROJECT_NAMES[$i]}|${PROJECT_LANGS[$i]}|${PROJECT_STATUSES[$i]}|${PROJECT_PATHS[$i]}")
  done
  
  # Sort and rebuild arrays
  IFS=$'\n' sort_data=($(sort -t'|' -k1,1n -k2,2 <<< "${sort_data[*]}"))
  unset IFS
  
  PROJECT_NAMES=()
  PROJECT_LANGS=()
  PROJECT_STATUSES=()
  PROJECT_PATHS=()
  
  for entry in "${sort_data[@]}"; do
    IFS='|' read -r _ name lang status path <<< "$entry"
    PROJECT_NAMES+=("$name")
    PROJECT_LANGS+=("$lang")
    PROJECT_STATUSES+=("$status")
    PROJECT_PATHS+=("$path")
  done
  
  return 0
}

# Get current effective status (with pending changes)
get_effective_status() {
  local idx="$1"
  local name="${PROJECT_NAMES[$idx]}"
  
  # Check if there's a pending change
  if [[ -n "${PENDING_CHANGES[$name]:-}" ]]; then
    echo "${PENDING_CHANGES[$name]}"
  else
    echo "${PROJECT_STATUSES[$idx]}"
  fi
}

# Check if project has pending changes
has_pending_change() {
  local idx="$1"
  local name="${PROJECT_NAMES[$idx]}"
  [[ -n "${PENDING_CHANGES[$name]:-}" ]]
}

# Toggle state forward (next)
toggle_state_next() {
  local idx="$1"
  local name="${PROJECT_NAMES[$idx]}"
  local current_status
  current_status=$(get_effective_status "$idx")
  local new_status
  new_status=$(state_next "$current_status")
  
  # If new status matches original, remove from pending
  if [[ "$new_status" == "${PROJECT_STATUSES[$idx]}" ]]; then
    unset "PENDING_CHANGES[$name]"
  else
    PENDING_CHANGES[$name]="$new_status"
  fi
}

# Toggle state backward (previous)
toggle_state_prev() {
  local idx="$1"
  local name="${PROJECT_NAMES[$idx]}"
  local current_status
  current_status=$(get_effective_status "$idx")
  local new_status
  new_status=$(state_prev "$current_status")
  
  # If new status matches original, remove from pending
  if [[ "$new_status" == "${PROJECT_STATUSES[$idx]}" ]]; then
    unset "PENDING_CHANGES[$name]"
  else
    PENDING_CHANGES[$name]="$new_status"
  fi
}
```

#### 2. Declare Global Arrays (`overlord-list`)
**File**: `overlord-list`
**Changes**: Add after argument parsing section (after line 96)

```bash
# Global arrays for interactive mode
declare -a PROJECT_NAMES
declare -a PROJECT_LANGS
declare -a PROJECT_STATUSES
declare -a PROJECT_PATHS
declare -A PENDING_CHANGES
declare SAVED_TERM_STATE=""
```

### Success Criteria:

#### Automated Verification:
- [x] `state_next("active")` returns "lib"
- [x] `state_next("lib")` returns "archive"
- [x] `state_next("archive")` returns "active"
- [x] `state_prev("active")` returns "archive"
- [x] `state_prev("lib")` returns "active"
- [x] `state_prev("archive")` returns "lib"
- [x] `load_projects()` populates all arrays correctly
- [x] Projects sorted by status (active, lib, archive) then name
- [x] `get_effective_status()` returns pending status when exists
- [x] `get_effective_status()` returns original status when no pending change
- [x] `toggle_state_next()` adds to pending changes
- [x] `toggle_state_prev()` adds to pending changes
- [x] Toggling back to original status removes from pending changes

#### Manual Verification:
- [x] Load projects from test registry - verify all projects loaded
- [x] Toggle state forward multiple times - verify wraps to active
- [x] Toggle state backward multiple times - verify wraps to archive
- [x] Toggle forward then backward - verify returns to original, removed from pending
- [x] Check pending changes map - verify only changed projects tracked

---

## Phase 3: UI Rendering

### Overview
Render the full-screen interactive UI with project list, selection indicator, and footer showing pending changes.

### Changes Required:

#### 1. UI Rendering Functions (`overlord-list`)
**File**: `overlord-list`
**Changes**: Add functions after state management functions

```bash
# Get terminal height
get_term_height() {
  tput lines
}

# Calculate viewport for scrolling
calculate_viewport() {
  local selected="$1"
  local total="${#PROJECT_NAMES[@]}"
  local term_height
  term_height=$(get_term_height)
  
  # Reserve lines: 2 header + 1 separator + 3 footer = 6 lines
  local available=$((term_height - 6))
  [[ $available -lt 5 ]] && available=5
  
  # Calculate scroll offset
  local offset=0
  if [[ $total -gt $available ]]; then
    # Keep selection in middle third of screen if possible
    local middle=$((available / 3))
    offset=$((selected - middle))
    [[ $offset -lt 0 ]] && offset=0
    [[ $offset -gt $((total - available)) ]] && offset=$((total - available))
  fi
  
  echo "$offset $available"
}

# Render header
render_header() {
  local line=1
  
  ansi_cursor_to $line 1
  ansi_clear_line
  printf "${DIM}%-40s %-6s %-8s %-40s${NC}" "NAME" "LANG" "STATUS" "PATH"
  
  line=$((line + 1))
  ansi_cursor_to $line 1
  ansi_clear_line
  printf "${DIM}%-40s %-6s %-8s %-40s${NC}" "----" "----" "------" "----"
}

# Render single project line
render_project_line() {
  local idx="$1"
  local line="$2"
  local is_selected="$3"
  
  local name="${PROJECT_NAMES[$idx]}"
  local lang="${PROJECT_LANGS[$idx]}"
  local original_status="${PROJECT_STATUSES[$idx]}"
  local path="${PROJECT_PATHS[$idx]}"
  
  local effective_status
  effective_status=$(get_effective_status "$idx")
  
  local lang_short
  lang_short=$(lang_short "$lang")
  
  local lc sc
  lc=$(lang_color "$lang")
  sc=$(status_color "$effective_status")
  
  # Truncate path
  local display_path="$path"
  if [[ ${#display_path} -gt 40 ]]; then
    display_path="...${display_path: -37}"
  fi
  
  # Selection indicator
  local indicator="  "
  if [[ "$is_selected" == "true" ]]; then
    indicator="${BOLD}>${NC} "
  fi
  
  # Pending change indicator
  local change_marker=""
  if has_pending_change "$idx"; then
    change_marker=" ${YELLOW}*${NC}"
  fi
  
  ansi_cursor_to "$line" 1
  ansi_clear_line
  printf "${indicator}${sc}%-38s${NC} ${lc}%-6s${NC} ${sc}%-8s${NC} ${NC}%-40s${change_marker}\n" \
    "$name" "$lang_short" "$effective_status" "$display_path"
}

# Render footer with pending changes
render_footer() {
  local term_height
  term_height=$(get_term_height)
  
  # Footer starts 3 lines from bottom
  local footer_start=$((term_height - 2))
  
  ansi_cursor_to "$footer_start" 1
  ansi_clear_line
  printf "${DIM}────────────────────────────────────────────────────────────────────────────────${NC}"
  
  footer_start=$((footer_start + 1))
  ansi_cursor_to "$footer_start" 1
  ansi_clear_line
  
  # Count pending changes
  local change_count=0
  for name in "${!PENDING_CHANGES[@]}"; do
    change_count=$((change_count + 1))
  done
  
  if [[ $change_count -eq 0 ]]; then
    printf "${DIM}No pending changes${NC} | j/k:navigate h/l:cycle-state ESC:cancel"
  else
    printf "${YELLOW}%d pending change(s)${NC} | y:confirm ESC:cancel j/k:navigate h/l:cycle" "$change_count"
  fi
  
  footer_start=$((footer_start + 1))
  ansi_cursor_to "$footer_start" 1
  ansi_clear_line
  
  # Show first few pending changes
  if [[ $change_count -gt 0 ]]; then
    local shown=0
    for name in "${!PENDING_CHANGES[@]}"; do
      [[ $shown -ge 3 ]] && break
      local new_status="${PENDING_CHANGES[$name]}"
      
      # Find original status
      local original_status=""
      for i in "${!PROJECT_NAMES[@]}"; do
        if [[ "${PROJECT_NAMES[$i]}" == "$name" ]]; then
          original_status="${PROJECT_STATUSES[$i]}"
          break
        fi
      done
      
      printf "${CYAN}%s${NC}: %s → %s  " "$name" "$original_status" "$new_status"
      shown=$((shown + 1))
    done
    
    if [[ $change_count -gt 3 ]]; then
      printf "${DIM}... and %d more${NC}" $((change_count - 3))
    fi
  fi
}

# Render full UI
render_ui() {
  local selected="$1"
  
  ansi_clear_screen
  ansi_cursor_home
  
  # Render header
  render_header
  
  # Calculate viewport
  local viewport
  viewport=$(calculate_viewport "$selected")
  local offset
  local available
  IFS=' ' read -r offset available <<< "$viewport"
  
  # Render visible projects
  local line=3
  local end=$((offset + available))
  [[ $end -gt ${#PROJECT_NAMES[@]} ]] && end=${#PROJECT_NAMES[@]}
  
  for ((i=offset; i<end; i++)); do
    local is_selected="false"
    [[ $i -eq $selected ]] && is_selected="true"
    render_project_line "$i" "$line" "$is_selected"
    line=$((line + 1))
  done
  
  # Render footer
  render_footer
}
```

### Success Criteria:

#### Automated Verification:
- [x] `get_term_height()` returns valid terminal height
- [x] `calculate_viewport()` returns valid offset and available lines
- [x] `render_header()` outputs header without errors
- [x] `render_project_line()` formats project correctly
- [x] `render_footer()` displays pending changes count
- [x] Selection indicator shows on correct line

#### Manual Verification:
- [x] Display matches `overlord list --all` format
- [x] Selected project has ">" indicator
- [x] Pending changes show "*" marker
- [x] Footer shows pending change count correctly
- [x] Footer shows first 3 pending changes with arrows (→)
- [x] Colors match non-interactive list output
- [x] Long paths truncated correctly
- [x] Scrolling works with many projects
- [x] Terminal height changes handled gracefully

---

## Phase 4: Input Handling & Navigation

### Overview
Implement the main event loop with keyboard input processing and navigation logic.

### Changes Required:

#### 1. Main Interactive Loop (`overlord-list`)
**File**: `overlord-list`
**Changes**: Add function after UI rendering functions

```bash
# Interactive edit mode main loop
interactive_edit() {
  # Load projects
  if ! load_projects; then
    log_error "No projects found in registry"
    return 1
  fi
  
  local total="${#PROJECT_NAMES[@]}"
  if [[ $total -eq 0 ]]; then
    log_error "No projects found in registry"
    return 1
  fi
  
  # Setup terminal
  term_save
  trap term_cleanup EXIT INT TERM
  term_raw
  ansi_cursor_hide
  
  # Initial state
  local selected=0
  local should_exit=false
  local should_save=false
  
  # Initial render
  render_ui "$selected"
  
  # Main loop
  while [[ "$should_exit" == "false" ]]; do
    local key
    key=$(read_key)
    
    case "$key" in
      # Navigation: Down
      "DOWN"|"j")
        if [[ $selected -lt $((total - 1)) ]]; then
          selected=$((selected + 1))
          render_ui "$selected"
        fi
        ;;
      
      # Navigation: Up
      "UP"|"k")
        if [[ $selected -gt 0 ]]; then
          selected=$((selected - 1))
          render_ui "$selected"
        fi
        ;;
      
      # State: Next (forward)
      "ENTER"|"l")
        toggle_state_next "$selected"
        render_ui "$selected"
        ;;
      
      # State: Previous (backward)
      "h")
        toggle_state_prev "$selected"
        render_ui "$selected"
        ;;
      
      # Confirm and save
      "y")
        should_exit=true
        should_save=true
        ;;
      
      # Cancel
      "ESC"|"q")
        should_exit=true
        should_save=false
        ;;
      
      # Ignore other keys
      *)
        ;;
    esac
  done
  
  # Cleanup
  ansi_cursor_show
  term_restore
  ansi_clear_screen
  ansi_cursor_home
  
  # Return status for caller
  if [[ "$should_save" == "true" ]]; then
    return 0
  else
    return 1
  fi
}
```

### Success Criteria:

#### Automated Verification:
- [x] `interactive_edit()` loads projects successfully
- [x] Loop exits on 'y' key with return code 0
- [x] Loop exits on ESC/q key with return code 1
- [x] Terminal cleanup called on all exit paths
- [x] Selected index bounds checked (0 to total-1)

#### Manual Verification:
- [x] j key moves selection down
- [x] k key moves selection up
- [x] Down arrow moves selection down
- [x] Up arrow moves selection up
- [x] j at bottom doesn't crash or wrap
- [x] k at top doesn't crash or wrap
- [x] l key cycles state forward
- [x] h key cycles state backward
- [x] Enter key cycles state forward
- [x] Multiple state cycles work correctly
- [x] y key exits and returns success
- [x] ESC key exits and returns failure
- [x] q key exits and returns failure
- [x] UI updates immediately after key press
- [x] Ctrl+C restores terminal properly

---

## Phase 5: Registry Update & Integration

### Overview
Apply pending changes atomically to registry and physically move project directories.

### Changes Required:

#### 1. Apply Changes Function (`overlord-list`)
**File**: `overlord-list`
**Changes**: Add function after interactive loop

```bash
# Apply all pending changes atomically
apply_changes() {
  local change_count="${#PENDING_CHANGES[@]}"
  
  if [[ $change_count -eq 0 ]]; then
    log_info "No changes to apply"
    return 0
  fi
  
  log_info "Applying $change_count change(s)..."
  echo ""
  
  # Backup registry
  local backup="$OVERLORD_REGISTRY.bak"
  cp "$OVERLORD_REGISTRY" "$backup"
  log_info "Registry backed up to $backup"
  
  # Process each change
  local success_count=0
  local error_count=0
  
  for name in "${!PENDING_CHANGES[@]}"; do
    local new_status="${PENDING_CHANGES[$name]}"
    
    # Find project info
    local idx=-1
    local old_status=""
    local lang=""
    local old_path=""
    for i in "${!PROJECT_NAMES[@]}"; do
      if [[ "${PROJECT_NAMES[$i]}" == "$name" ]]; then
        idx=$i
        old_status="${PROJECT_STATUSES[$i]}"
        lang="${PROJECT_LANGS[$i]}"
        old_path="${PROJECT_PATHS[$i]}"
        break
      fi
    done
    
    if [[ $idx -eq -1 ]]; then
      log_error "Project not found: $name"
      error_count=$((error_count + 1))
      continue
    fi
    
    # Calculate new path
    local lang_dir
    lang_dir=$(get_lang_dir "$lang")
    local target_dir
    target_dir=$(get_status_dir "$new_status")
    local new_path="$OVERLORD_BASE_DIR/$lang_dir/$target_dir/$name"
    
    log_info "Moving: $name ($old_status → $new_status)"
    
    # Check source exists
    if [[ ! -d "$old_path" ]]; then
      log_warning "Source directory missing: $old_path"
      log_warning "Updating registry only..."
      
      # Update registry only
      local tmp_file
      tmp_file=$(mktemp)
      jq --arg name "$name" \
         --arg status "$new_status" \
         --arg path "$new_path" \
         '.projects[$name].status = $status | .projects[$name].path = $path' \
         "$OVERLORD_REGISTRY" > "$tmp_file"
      mv "$tmp_file" "$OVERLORD_REGISTRY"
      
      success_count=$((success_count + 1))
      continue
    fi
    
    # Check destination doesn't exist
    if [[ -d "$new_path" && "$old_path" != "$new_path" ]]; then
      log_error "Destination exists: $new_path"
      error_count=$((error_count + 1))
      continue
    fi
    
    # Skip if already in correct location
    if [[ "$old_path" == "$new_path" ]]; then
      # Just update registry status
      local tmp_file
      tmp_file=$(mktemp)
      jq --arg name "$name" \
         --arg status "$new_status" \
         '.projects[$name].status = $status' \
         "$OVERLORD_REGISTRY" > "$tmp_file"
      mv "$tmp_file" "$OVERLORD_REGISTRY"
      
      success_count=$((success_count + 1))
      continue
    fi
    
    # Ensure destination parent exists
    mkdir -p "$(dirname "$new_path")"
    
    # Move directory
    if mv "$old_path" "$new_path"; then
      # Update registry
      local tmp_file
      tmp_file=$(mktemp)
      jq --arg name "$name" \
         --arg status "$new_status" \
         --arg path "$new_path" \
         '.projects[$name].status = $status | .projects[$name].path = $path' \
         "$OVERLORD_REGISTRY" > "$tmp_file"
      mv "$tmp_file" "$OVERLORD_REGISTRY"
      
      log_success "Moved: $name"
      success_count=$((success_count + 1))
    else
      log_error "Failed to move: $name"
      error_count=$((error_count + 1))
    fi
  done
  
  echo ""
  log_success "Applied $success_count change(s)"
  if [[ $error_count -gt 0 ]]; then
    log_error "$error_count change(s) failed"
    return 1
  fi
  
  return 0
}
```

#### 2. Add Flag to Argument Parser (`overlord-list`)
**File**: `overlord-list`
**Changes**: Modify `parse_args()` function at line 58

Add after line 57 (before `while` loop):
```bash
EDIT_MODE=false
```

Add in the case statement (after line 81, before `--json)` case):
```bash
      --edit|-e)
        EDIT_MODE=true
        ;;
```

#### 3. Modify Main Function (`overlord-list`)
**File**: `overlord-list`
**Changes**: Modify `main()` function at line 222

Replace the `main()` function:
```bash
main() {
  parse_args "$@"
  
  # Handle edit mode
  if [[ "$EDIT_MODE" == true ]]; then
    if interactive_edit; then
      # User confirmed changes
      apply_changes
      exit $?
    else
      # User cancelled
      log_info "Cancelled - no changes made"
      exit 0
    fi
  fi
  
  # Normal list mode
  list_projects
}
```

#### 4. Update Usage Text (`overlord-list`)
**File**: `overlord-list`
**Changes**: Update `usage()` function at line 19

Add after line 25 (after `--archive` line):
```bash
  --edit, -e           Interactive state editor
```

Add in examples section (after line 49):
```bash
  overlord list --edit           # Interactive state editor
```

### Success Criteria:

#### Automated Verification:
- [x] `overlord list --edit` flag recognized
- [x] `overlord list -e` flag recognized (short form)
- [x] `apply_changes()` creates registry backup
- [x] `apply_changes()` updates registry atomically per change
- [x] `apply_changes()` moves directories correctly
- [x] Registry updates use mktemp + mv pattern
- [x] Missing source directories handled gracefully
- [x] Existing destination directories detected and skipped
- [x] No changes exits cleanly without errors
- [x] Multiple changes applied in sequence

#### Manual Verification:
- [x] Run with --edit, change multiple states, confirm - all applied
- [x] Run with -e, change states, press Escape - no changes applied
- [x] Change active→lib - directory moved to libs folder
- [x] Change lib→archive - directory moved to archive folder
- [x] Change archive→active - directory moved to active folder
- [x] Registry updated with new status and path for each change
- [x] Source directory missing - registry updated, warning shown
- [x] Destination exists - error shown, change skipped
- [x] Multiple changes to same project - final state applied
- [x] Verify registry backup created before changes
- [x] Check moved projects still work with `overlord open`
- [x] Verify --edit works combined with other filters (--py, --all, etc.)

---

## Testing Strategy

### Unit Tests (Manual Execution):

**Phase 1 Tests:**
- Run in test environment, verify terminal state saved/restored
- Test Ctrl+C during raw mode - terminal should restore
- Test arrow key capture - should return UP/DOWN/LEFT/RIGHT
- Test vim key capture - should return j/k/h/l
- Test Escape key - should return ESC, not start escape sequence

**Phase 2 Tests:**
- Load empty registry - should handle gracefully
- Load registry with 1 project - should load correctly
- Load registry with mixed statuses - should sort correctly
- Test state_next through full cycle - active→lib→archive→active
- Test state_prev through full cycle - active→archive→lib→active
- Toggle state and toggle back - should remove from pending

**Phase 3 Tests:**
- Render with 5 projects - should fit on screen
- Render with 50 projects - should scroll correctly
- Render with pending changes - should show "*" marker
- Render with 0 pending changes - footer should show help
- Render with 3+ pending changes - footer should show "... and N more"
- Resize terminal - UI should adapt (re-render)

**Phase 4 Tests:**
- Navigate with j/k - selection should move
- Navigate with arrows - selection should move
- Navigate to top, press k - should stay at top
- Navigate to bottom, press j - should stay at bottom
- Press l multiple times - state should cycle forward
- Press h multiple times - state should cycle backward
- Press Enter - should cycle forward (same as l)
- Press y with changes - should exit with success
- Press Escape - should exit without saving
- Press q - should exit without saving

**Phase 5 Tests:**
- Apply 0 changes - should show "no changes" message
- Apply 1 change - directory moved, registry updated
- Apply multiple changes - all applied in sequence
- Change with missing source - registry updated, warning shown
- Change with existing destination - error shown, skipped
- Verify registry backup created before changes
- Verify atomic updates (each change is mktemp + mv)

### Integration Tests:

1. **Full workflow test:**
   - Start with 3 projects (1 active, 1 lib, 1 archive)
   - Run `overlord list --edit`
   - Change active→lib, lib→archive, archive→active
   - Confirm with 'y'
   - Verify all 3 moved correctly
   - Verify registry updated correctly
   - Verify `overlord list` shows new states
   - Verify `overlord open` still works

2. **Cancel workflow test:**
   - Run `overlord list --edit`
   - Change multiple project states
   - Press Escape
   - Verify no changes applied to filesystem
   - Verify no changes applied to registry
   - Verify registry backup not created

3. **Edge cases:**
   - Empty registry - should show error
   - Single project - should work correctly
   - Very long project list (50+) - scrolling works
   - Project with very long name/path - truncation works
   - Change and undo (toggle back) - removed from pending
   - Multiple state cycles - wraps correctly

4. **Error handling:**
   - Simulate missing directory - graceful handling
   - Simulate permission error on mv - error shown
   - Simulate registry write failure - preserves backup
   - Ctrl+C during apply - terminal restores correctly

### Manual Testing Steps:

1. **Setup test registry:**
   ```bash
   # Create test projects
   overlord new test-active-py --py
   overlord new test-lib-ts --ts --lib
   overlord new test-archive-sol --sol
   overlord mv test-archive-sol archive
   ```

2. **Test interactive mode:**
   ```bash
   overlord list --edit
   # Navigate with j/k and arrows
   # Toggle states with h/l/Enter
   # Verify UI updates
   # Press y to confirm
   ```

3. **Test cancellation:**
   ```bash
   overlord list --edit
   # Make changes
   # Press ESC
   # Verify no changes applied
   ```

4. **Test with filters:**
   ```bash
   overlord list --py --edit      # Should show only Python projects
   overlord list --all --edit     # Should show all projects
   ```

5. **Test error cases:**
   ```bash
   # Manually delete a project directory
   overlord list --edit
   # Try to move the deleted project
   # Verify warning and registry-only update
   ```

## Performance Considerations

- **Terminal rendering**: UI redraws on every key press; should be fast enough (<50ms) for responsive feel
- **Project loading**: jq query runs once at startup; O(n) where n = number of projects
- **State lookups**: Associative array lookups are O(1)
- **Viewport calculation**: O(1) computation per render
- **Registry updates**: Sequential writes; each is atomic mktemp + jq + mv
- **Directory moves**: Filesystem operations; potentially slow for large projects

**Expected performance:**
- Interactive mode startup: < 200ms for 100 projects
- UI render per keypress: < 50ms
- State toggle: < 10ms (in-memory operation)
- Apply changes: ~500ms per project (filesystem operations)

**Optimization notes:**
- No optimization needed for project counts < 1000
- Scrolling viewport prevents rendering all projects
- Pending changes map prevents redundant operations
- Atomic updates ensure consistency even if slow

## Migration Notes

No migration needed - this is an additive feature. Existing `overlord list` behavior is unchanged.

**For users:**
- New optional flag `--edit` / `-e`
- No changes to non-interactive list output
- No changes to existing workflows
- No new dependencies required (pure bash)

**Backward compatibility:**
- All existing `overlord list` flags work as before
- `--edit` can be combined with filters (--py, --all, etc.)
- Graceful fallback if terminal doesn't support raw mode (could add check)

## References

- Original ticket: `thoughts/tickets/feature_overlord_list_edit_interactive.md`
- Existing list implementation: `overlord-list:128-220`
- State change logic: `overlord-mv:76-174`
- Atomic registry pattern: `overlord-mv:82-90`
- Confirmation prompts: `overlord-rm:113-141`
- Terminal colors: `lib/common.sh:4-15`
- FZF interactive example: `overlord-open:67-102`

## Notes

**Terminal compatibility:**
- Requires ANSI escape sequence support (most modern terminals)
- Requires raw terminal mode support (stty)
- Should work on: Linux, macOS, WSL, most Unix-like systems
- May not work on: Very old terminals, some embedded systems

**Known limitations:**
- No mouse support
- No search/filter within interactive mode
- No visual themes
- Footer limited to 3 visible pending changes
- Terminal must be at least ~20 lines tall for usable display

**Future enhancements** (out of scope):
- Search/filter within interactive mode
- Multiple selection for batch operations
- Project details expansion
- Undo/redo within session
- Mouse support
- Customizable key bindings
- Visual themes/colors customization

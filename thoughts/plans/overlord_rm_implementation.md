# Add `overlord rm` Command - Implementation Plan

## Overview
Create a new `overlord rm` command that removes projects from the Overlord registry without deleting actual project files. This provides a safe way to untrack projects while following the existing command patterns and color schemes.

## Current State
- No way to remove projects from registry except manual editing via `overlord config`
- Project lookup by name/alias exists in `overlord-mv` (lines 71-100) and `overlord-info` (lines 42-71)
- Registry manipulation using jq exists in `overlord-mv` (lines 103-125)
- Confirmation prompts are used in bash with `read -p`

## Changes Required

### 1. Create `overlord-rm` Script
**File**: `~/bin/overlord/overlord-rm` (new file)

**What to create:**
New bash script with the following structure:
- Standard header with `set -euo pipefail`
- Color definitions matching other commands (RED, GREEN, YELLOW, BLUE, NC)
- Logging functions: `log_info`, `log_success`, `log_error`, `log_warning`
- `find_project()` function supporting:
  - Name lookup (exact match)
  - Alias lookup
  - Path lookup (new - match against `.value.path`)
- `show_confirmation()` function displaying:
  - Project name, language (with color), status (with color), path
  - Prompt: "Remove this project from registry? (y/N): "
- `remove_from_registry()` function using:
  - Backup registry to `.bak` file
  - `jq 'del(.projects[$name])'` to remove entry
  - Atomic write using temp file pattern from `overlord-mv`
- Argument parsing:
  - `--help` / `-h` shows usage
  - `--force` / `-f` skips confirmation
  - First positional arg is identifier (name/alias/path)
- Main flow:
  1. Find project by identifier
  2. Show confirmation (unless `--force`)
  3. Remove from registry
  4. Success message

**Why:**
Follows exact patterns from existing commands, ensuring consistency in UX and maintainability.

### 2. Update Main Dispatcher
**File**: `~/bin/overlord/overlord`

**What to change:**
Add `rm` to the subcommand routing (line 74):
```bash
new|list|mv|open|info|sync|config|edit|rm)
```

**Why:**
Routes `overlord rm` to `overlord-rm` script using existing dispatcher pattern.

### 3. Update Dispatcher Usage Text
**File**: `~/bin/overlord/overlord`

**What to change:**
Add to usage text after line 28:
```bash
  overlord rm <name> [options]          Remove project from registry
```

**Why:**
Documents the new command in help output.

---

## Implementation Details

### Project Lookup Priority
When identifier could match multiple (e.g., name matches but also a path):
1. Try exact name match first
2. Try alias match second
3. Try path match last

If identifier is ambiguous (matches multiple via different methods), show error with suggestions.

### Path Lookup Implementation
Add to `find_project()` after alias lookup:
```bash
# Try path match
result=$(jq -r --arg q "$query" '
  .projects | to_entries[] | 
  select(.value.path == $q) | 
  [.key, .value.lang, .value.status, .value.path] | @tsv
' "$OVERLORD_REGISTRY" 2>/dev/null || true)
```

### Confirmation Format
```
Removing project from registry:

  Name:     myproject
  Language: python
  Status:   active
  Path:     /home/user/Work/Python/active/myproject

Remove this project from registry? (y/N):
```

Use color formatting from `overlord-info` for language and status.

### Registry Removal Pattern
```bash
backup_registry() {
  local backup="$OVERLORD_REGISTRY.bak"
  cp "$OVERLORD_REGISTRY" "$backup"
  log_info "Registry backed up to $backup"
}

remove_from_registry() {
  local name="$1"
  local tmp_file
  tmp_file=$(mktemp)
  
  jq --arg name "$name" 'del(.projects[$name])' \
    "$OVERLORD_REGISTRY" > "$tmp_file"
  
  mv "$tmp_file" "$OVERLORD_REGISTRY"
}
```

---

## Out of Scope
- Deleting actual project files from disk
- Bulk removal with `--all` flag
- Interactive project selection (just direct removal)
- Checking if project is open in tmux
- "Did you mean?" suggestions for fuzzy matching

---

## Success Criteria

### Automated Verification
- [ ] Script is executable: `chmod +x ~/bin/overlord/overlord-rm`
- [ ] `overlord rm --help` shows usage text
- [ ] `overlord rm nonexistent` shows error "Project not found: nonexistent"
- [ ] `overlord rm testproject` prompts for confirmation with project details
- [ ] Entering 'n' at confirmation leaves project in registry
- [ ] Entering 'y' at confirmation removes project from registry
- [ ] `overlord rm testproject -f` removes without prompt
- [ ] `overlord list --all` doesn't show removed project
- [ ] Registry backup created at `~/.config/overlord/registry.json.bak`

### Manual Verification
- [ ] Confirmation shows colored output (language and status colors match `overlord-info`)
- [ ] Project directory still exists on disk after removal
- [ ] Can verify with `ls /path/to/project` that files are untouched
- [ ] Registry is valid JSON after removal (can run `jq . registry.json`)
- [ ] Removal by project name works
- [ ] Removal by project alias works  
- [ ] Removal by absolute path works
- [ ] Error message clear when project not found

### Edge Cases
- [ ] Removing non-existent project shows clear error
- [ ] Removing when registry.json is empty or missing projects key
- [ ] Force flag works correctly
- [ ] Confirmation defaults to 'N' (pressing Enter doesn't remove)

---

## References
- Ticket: `thoughts/tickets/feature_overlord_rm.md`
- Pattern reference: `overlord-mv` for registry manipulation
- Pattern reference: `overlord-info` for project lookup and display formatting
- Pattern reference: `overlord-list` for color schemes

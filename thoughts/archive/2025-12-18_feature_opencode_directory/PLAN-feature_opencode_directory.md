# Move opencode.jsonc to .opencode Directory - Implementation Plan

## Overview
Change the placement of `opencode.jsonc` from the project root to a dedicated `.opencode/` directory. This improves project organization and reduces clutter in the root directory.

## Current State
- `opencode.jsonc` is created directly in the project root.
- Key function: `copy_opencode_template` in `lib/common.sh` (lines 115-130).
- Referenced in `overlord-init`, `overlord-new`, and `overlord-sync`.

## Changes Required

### 1. Update Common Library
**File**: `lib/common.sh`

**What to change:**
Update `copy_opencode_template` to create the `.opencode` directory and copy the template there.

```bash
copy_opencode_template() {
  local dir="$1"
  local lang="$2"
  local target_dir="$dir/.opencode"
  local target_file="$target_dir/opencode.jsonc"
  local template="$OVERLORD_BIN/templates/opencode-${lang}.jsonc"
  local base_template="$OVERLORD_BIN/templates/opencode-base.jsonc"
  
  mkdir -p "$target_dir"
  
  if [[ -f "$template" ]]; then
    cp "$template" "$target_file"
    log_success "Created .opencode/opencode.jsonc (${lang})"
  elif [[ -f "$base_template" ]]; then
    cp "$base_template" "$target_file"
    log_success "Created .opencode/opencode.jsonc (base)"
  # ...
```

**Why:**
Centralizes the directory creation and file placement logic used by all initialization scripts.

### 2. Update Initialization and Migration Logic
**File**: `overlord-init`

**What to change:**
Update existence check and add migration for existing root-level `opencode.jsonc`.

```bash
  # Check for opencode.jsonc (new location or legacy root location)
  local opencode_path="$PROJECT_PATH/.opencode/opencode.jsonc"
  local legacy_opencode="$PROJECT_PATH/opencode.jsonc"

  if [[ -f "$legacy_opencode" ]]; then
    log_info "Migrating opencode.jsonc to .opencode/ directory..."
    mkdir -p "$PROJECT_PATH/.opencode"
    mv "$legacy_opencode" "$opencode_path"
  fi

  if [[ -f "$opencode_path" ]] && [[ "$FORCE" == false ]]; then
    log_warning ".opencode/opencode.jsonc already exists (use --force to overwrite)"
  else
    copy_opencode_template "$PROJECT_PATH" "$LANGUAGE"
  fi
```

### 3. Update Sync Logic
**File**: `overlord-sync`

**What to change:**
Update `opencode_path` and add migration support during sync.

```bash
  local opencode_path="$path/.opencode/opencode.jsonc"
  local legacy_opencode="$path/opencode.jsonc"
  
  if [[ -f "$legacy_opencode" ]]; then
    log_info "  Migrating $name: opencode.jsonc -> .opencode/opencode.jsonc"
    mkdir -p "$path/.opencode"
    mv "$legacy_opencode" "$opencode_path"
  fi
```

### 4. Update Documentation
**File**: `AGENTS.md`

**What to change:**
Update references to `opencode.jsonc` location in the "OpenCode Configuration" section and other relevant areas.

---

## Out of Scope
- Moving other configuration files (`.tmux.local`, `Makefile`) into `.opencode/`.
- Changing the content or structure of the `opencode.jsonc` templates themselves.

## Success Criteria

### Automated Verification
- [x] `overlord new testproj --py` creates `.opencode/opencode.jsonc`.
- [x] `overlord init` in a directory with `opencode.jsonc` moves it to `.opencode/`.
- [x] `overlord sync` moves legacy files to the new directory across all projects.

### Manual Verification
- [x] Verify root directory is cleaner after migration.
- [x] Verify `.opencode/opencode.jsonc` contains correct language-specific instructions.

## References
- Ticket: `thoughts/tickets/feature_opencode_directory.md`
- Research: `thoughts/research/2025-12-18_feature_opencode_directory.md`

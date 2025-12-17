# Overlord Init Command - Implementation Plan

## Overview
Create `overlord init` command to initialize existing directories with Overlord configuration files (Makefile, .tmux.local) and register them in the registry. This enables bringing cloned or existing repos into the Overlord ecosystem without creating new directories or running language-specific initialization.

## Current State
- `overlord-new` creates new projects and runs language init (lines 142-209)
- No `overlord-add` exists yet (different ticket)
- Existing helper functions in `overlord-new` can be reused:
  - `init_git()` at lines 212-225
  - `copy_tmux_template()` at lines 227-240
  - `generate_makefile()` at lines 252-280
  - `register_project()` at lines 283-309
- `base.tmux` template exists at `~/.config/overlord/tmux/base.tmux`
- Main dispatcher at `overlord` routes subcommands (line 74)

## Changes Required

### 1. Create `overlord-init` Script
**File**: `~/bin/overlord/overlord-init` (new file)

**What to create:**
New bash script following `overlord-new` structure but with these key differences:

**Header and shared code** (reuse from overlord-new):
- Same shebang, set options, color definitions (lines 1-16)
- Same log functions (lines 42-45)
- Same helper functions: `get_lang_dir()`, `get_status_dir()`, `get_lang_makefile()` (lines 125-249)
- Same action functions: `init_git()`, `copy_tmux_template()`, `generate_makefile()`, `register_project()` (lines 212-309)

**New argument parsing:**
```bash
parse_args() {
  # Default to current directory
  PROJECT_PATH="$(pwd)"
  PROJECT_NAME=""  # Empty = derive from directory name
  LANGUAGE=""      # Empty = auto-detect
  STATUS="active"
  INIT_GIT=true
  FORCE=false
  ALIASES=()
  
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --py|--python)     LANGUAGE="python" ;;
      --ts|--typescript) LANGUAGE="typescript" ;;
      --sol|--solidity)  LANGUAGE="solidity" ;;
      --base)            LANGUAGE="base" ;;
      --lib)             STATUS="lib" ;;
      --no-git)          INIT_GIT=false ;;
      --force)           FORCE=true ;;
      --name)
        shift
        PROJECT_NAME="$1"
        ;;
      --alias)
        shift
        ALIASES+=("$1")
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
        # First positional arg is path
        PROJECT_PATH="$1"
        ;;
    esac
    shift
  done
  
  # Resolve absolute path
  PROJECT_PATH="$(cd "$PROJECT_PATH" && pwd)"
  
  # Derive name from directory if not specified
  if [[ -z "$PROJECT_NAME" ]]; then
    PROJECT_NAME="$(basename "$PROJECT_PATH")"
  fi
  
  # Auto-detect language if not specified
  if [[ -z "$LANGUAGE" ]]; then
    LANGUAGE=$(detect_language "$PROJECT_PATH")
  fi
}
```

**New language detection function:**
```bash
detect_language() {
  local dir="$1"
  
  if [[ -f "$dir/pyproject.toml" ]] || [[ -f "$dir/setup.py" ]]; then
    echo "python"
  elif [[ -f "$dir/package.json" ]]; then
    echo "typescript"
  elif [[ -f "$dir/foundry.toml" ]]; then
    echo "solidity"
  else
    echo "base"
  fi
}
```

**Modified main function:**
```bash
main() {
  parse_args "$@"
  validate_project_name "$PROJECT_NAME"
  
  log_info "Initializing project: ${PROJECT_NAME}"
  log_info "Location: $PROJECT_PATH"
  log_info "Language: $LANGUAGE"
  log_info "Status: $STATUS"
  echo ""
  
  # Check if already registered
  if project_exists_in_registry "$PROJECT_NAME"; then
    log_warning "Project '$PROJECT_NAME' already registered in registry"
    log_info "Skipping initialization (use different --name or edit registry)"
    exit 0
  fi
  
  # Check if .tmux.local exists
  if [[ -f "$PROJECT_PATH/.tmux.local" ]] && [[ "$FORCE" == false ]]; then
    log_warning ".tmux.local already exists (use --force to overwrite)"
  else
    # Copy appropriate template
    if [[ "$LANGUAGE" == "base" ]]; then
      copy_tmux_template "$PROJECT_PATH" "base"
    else
      copy_tmux_template "$PROJECT_PATH" "$LANGUAGE"
    fi
  fi
  
  # Check if Makefile exists
  if [[ -f "$PROJECT_PATH/Makefile" ]] && [[ "$FORCE" == false ]]; then
    log_warning "Makefile already exists (use --force to overwrite)"
  else
    # Generate appropriate Makefile
    if [[ "$LANGUAGE" == "base" ]]; then
      generate_base_makefile "$PROJECT_PATH"
    else
      generate_makefile "$PROJECT_PATH" "$LANGUAGE"
    fi
  fi
  
  # Initialize git if needed
  if [[ "$INIT_GIT" == true ]]; then
    init_git "$PROJECT_PATH"
    echo ""
  fi
  
  # Register in registry (with aliases if provided)
  register_project_with_aliases "$PROJECT_NAME" "$LANGUAGE" "$STATUS" "$PROJECT_PATH" "${ALIASES[@]}"
  
  echo ""
  log_success "Project initialized successfully!"
  log_info "Run 'overlord open ${PROJECT_NAME}' to open workspace"
}
```

**New helper functions:**
```bash
project_exists_in_registry() {
  local name="$1"
  jq -e --arg name "$name" '.projects[$name] != null' "$OVERLORD_REGISTRY" >/dev/null 2>&1
}

generate_base_makefile() {
  local dir="$1"
  local makefile_dir="$OVERLORD_CONFIG/makefiles"
  local base_file="$makefile_dir/base.mk"
  
  if [[ ! -f "$base_file" ]]; then
    log_warning "Base makefile template not found"
    return 0
  fi
  
  cp "$base_file" "$dir/Makefile"
  log_success "Generated Makefile (base only)"
}

register_project_with_aliases() {
  local name="$1"
  local lang="$2"
  local status="$3"
  local path="$4"
  shift 4
  local aliases=("$@")
  local date
  date=$(date +%Y-%m-%d)
  
  local tmp_file
  tmp_file=$(mktemp)
  
  local aliases_json="[]"
  if [[ ${#aliases[@]} -gt 0 ]]; then
    aliases_json=$(printf '%s\n' "${aliases[@]}" | jq -R . | jq -s .)
  fi
  
  jq --arg name "$name" \
     --arg lang "$lang" \
     --arg status "$status" \
     --arg path "$path" \
     --arg date "$date" \
     --argjson aliases "$aliases_json" \
     '.projects[$name] = {
       "lang": $lang,
       "status": $status,
       "path": $path,
       "created": $date,
       "aliases": $aliases
     }' "$OVERLORD_REGISTRY" > "$tmp_file"
  
  mv "$tmp_file" "$OVERLORD_REGISTRY"
  log_success "Registered project in registry"
}
```

**Usage function:**
```bash
usage() {
  cat <<'EOF'
Usage:
  overlord init [path] [options]

Arguments:
  path                 Directory to initialize (default: current directory)

Language flags (optional, auto-detected if not specified):
  --py, --python       Python project
  --ts, --typescript   TypeScript project
  --sol, --solidity    Solidity project
  --base               Force base configuration (no language-specific)

Options:
  --lib                Set status to 'lib' instead of 'active'
  --name <name>        Override project name (default: directory basename)
  --alias <alias>      Add alias (can be used multiple times)
  --no-git             Skip git initialization
  --force              Overwrite existing .tmux.local and Makefile
  --help               Show this help

Language auto-detection:
  - pyproject.toml or setup.py → Python
  - package.json → TypeScript
  - foundry.toml → Solidity
  - None detected → base

Examples:
  overlord init                              # Initialize current dir, auto-detect language
  overlord init /path/to/repo --py           # Initialize specific path as Python
  overlord init --base --name myproj         # Force base config with custom name
  overlord init --ts --alias mp --lib        # TypeScript library with alias
EOF
}
```

**Why:**
This provides full initialization capability for existing directories while reusing proven code from `overlord-new`. The key differences are: no directory creation, no language-specific init commands, language auto-detection, and base-only mode support.

### 2. Update Main Dispatcher
**File**: `~/bin/overlord/overlord`

**What to change:**
Add `init` to the case statement routing.

```bash
# Line 74 - add 'init' to the case pattern
new|list|mv|open|info|sync|config|edit|init)
```

**Why:**
Routes `overlord init` to the new `overlord-init` script.

### 3. Make Script Executable
**Command**: `chmod +x ~/bin/overlord/overlord-init`

**Why:**
Script must be executable to be invoked by the dispatcher.

---

## Out of Scope
- Creating `overlord add` command (separate ticket)
- Running language-specific initialization (uv init, pnpm init, forge init)
- Moving projects between directories (that's `overlord mv`)
- Updating existing registry entries (init skips if already registered)

## Success Criteria

### Automated Verification
- [x] Tests pass: `overlord init --help` shows usage
- [x] Auto-detection: Create temp dir with `pyproject.toml`, run `overlord init`, verify Python config created
- [x] Auto-detection: Create temp dir with `package.json`, run `overlord init`, verify TypeScript config created
- [x] Auto-detection: Create temp dir with `foundry.toml`, run `overlord init`, verify Solidity config created
- [x] Auto-detection: Create temp empty dir, run `overlord init`, verify base config created
- [x] Force base: Create temp dir with `package.json`, run `overlord init --base`, verify base config created
- [x] No git: Run `overlord init --no-git` in non-git dir, verify no `.git` created
- [x] Force overwrite: Create `.tmux.local`, run `overlord init --force --py`, verify file overwritten
- [x] Already registered: Run `overlord init` twice, verify second warns and skips
- [x] Aliases: Run `overlord init --alias foo --alias bar`, verify registry contains aliases
- [x] Custom name: Run `overlord init --name customname`, verify registry uses custom name
- [x] Registry check: Verify project appears in `overlord list` after init
- [x] Dispatcher: `overlord init --help` works (dispatcher routes correctly)

### Manual Verification
- [ ] Clone external repo, run `overlord init`, verify Makefile and .tmux.local created
- [ ] Run `overlord open <project>` on initialized project, verify workspace opens
- [ ] Run `make help` in initialized project, verify commands available
- [ ] Verify tmux workspace has correct layout based on template

## References
- Ticket: `thoughts/tickets/feature_overlord_init.md`
- Reference code: `overlord-new` (lines 142-309 for reusable functions)
- Templates: `~/.config/overlord/tmux/` (base.tmux, python.tmux, typescript.tmux, solidity.tmux)
- Templates: `~/.config/overlord/makefiles/` (base.mk, python.mk, typescript.mk, solidity.mk)

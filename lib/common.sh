#!/usr/bin/env bash
# lib/common.sh: Shared logic for Overlord scripts

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log_info() { echo -e "${BLUE}>${NC} $*"; }
log_success() { echo -e "${GREEN}✓${NC} $*"; }
log_error() { echo -e "${RED}✗${NC} $*" >&2; }
log_warning() { echo -e "${YELLOW}!${NC} $*"; }

# Get language directory name
get_lang_dir() {
  case "$1" in
    python)     echo "Python" ;;
    typescript) echo "Typescript" ;;
    solidity)   echo "Solidity" ;;
    base)       echo "Base" ;;
    *)
      log_error "Unknown language: $1"
      exit 1
      ;;
  esac
}

# Get status directory name
get_status_dir() {
  case "$1" in
    active)  echo "active" ;;
    lib)     echo "libs" ;;
    archive) echo "archive" ;;
    *)
      log_error "Invalid status: $1 (must be active, lib, or archive)"
      exit 1
      ;;
  esac
}

# Detect language from project files
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

# Copy tmux template with fallback
copy_tmux_template() {
  local dir="$1"
  local lang="$2"
  local template="$OVERLORD_BIN/tmux/${lang}.tmux"
  local base_template="$OVERLORD_BIN/tmux/base.tmux"
  
  # Try language-specific template first
  if [[ -f "$template" ]]; then
    cp "$template" "$dir/.tmux.local"
    chmod +x "$dir/.tmux.local"
    log_success "Copied ${lang} tmux template"
  elif [[ -f "$base_template" ]]; then
    # Fall back to base template
    cp "$base_template" "$dir/.tmux.local"
    chmod +x "$dir/.tmux.local"
    log_success "Copied base tmux template"
  else
    log_warning "No tmux template found for ${lang} or base"
  fi
}

# Generate Makefile with fallback
generate_makefile() {
  local dir="$1"
  local lang="$2"
  local makefile_dir="$OVERLORD_BIN/makefiles"
  local base_file="$makefile_dir/base.mk"
  local lang_file="$makefile_dir/${lang}.mk"
  
  if [[ ! -f "$base_file" ]]; then
    if [[ "${OVERLORD_STRICT:-false}" == "true" ]]; then
        log_error "Base makefile template not found at $base_file"
        exit 1
    fi
    log_warning "Base makefile template not found"
    return 1
  fi
  
  # For base language or if language-specific is missing, use base only
  if [[ "$lang" == "base" ]] || [[ ! -f "$lang_file" ]]; then
    cp "$base_file" "$dir/Makefile"
    log_success "Generated Makefile (base only)"
    return 0
  fi
  
  # Combine base + language-specific
  {
    cat "$base_file"
    echo ""
    cat "$lang_file"
  } > "$dir/Makefile"
  
  log_success "Generated Makefile (${lang})"
}

# Copy opencode.jsonc template with fallback
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
  else
    log_warning "No opencode template found for ${lang} or base"
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
  fi
}

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

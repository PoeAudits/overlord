#!/usr/bin/env bash
# setup.sh: Install overlord CLI tool
# Run this after cloning the repository

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}>${NC} $*"; }
log_success() { echo -e "${GREEN}✓${NC} $*"; }
log_error() { echo -e "${RED}✗${NC} $*" >&2; }
log_warning() { echo -e "${YELLOW}!${NC} $*"; }

# Detect repository directory
REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_TARGET="/usr/local/bin/overlord"
REGISTRY_FILE="$REPO_DIR/registry.json"

echo ""
log_info "Overlord Setup"
log_info "Repository: $REPO_DIR"
echo ""

# Check if repository looks valid
if [[ ! -f "$REPO_DIR/overlord" ]]; then
  log_error "Repository structure invalid: overlord script not found"
  exit 1
fi

if [[ ! -d "$REPO_DIR/templates" ]] || [[ ! -d "$REPO_DIR/makefiles" ]] || [[ ! -d "$REPO_DIR/tmux" ]]; then
  log_error "Repository structure invalid: missing templates, makefiles, or tmux directories"
  exit 1
fi

# Check for required dependencies
echo ""
log_info "Checking dependencies..."

REQUIRED_DEPS=("git" "jq" "tmux")
OPTIONAL_DEPS=("fzf")

MISSING_REQUIRED=()
MISSING_OPTIONAL=()

for dep in "${REQUIRED_DEPS[@]}"; do
  if ! command -v "$dep" &>/dev/null; then
    MISSING_REQUIRED+=("$dep")
  fi
done

for dep in "${OPTIONAL_DEPS[@]}"; do
  if ! command -v "$dep" &>/dev/null; then
    MISSING_OPTIONAL+=("$dep")
  fi
done

if [[ ${#MISSING_REQUIRED[@]} -gt 0 ]]; then
  log_error "Missing required dependencies: ${MISSING_REQUIRED[*]}"
  log_error "Please install them and re-run setup"
  exit 1
fi

if [[ ${#MISSING_OPTIONAL[@]} -gt 0 ]]; then
  log_warning "Missing optional dependencies: ${MISSING_OPTIONAL[*]}"
  log_warning "These are recommended for better experience but not required"
fi

log_success "All required dependencies found"

# Check if /usr/local/bin is writable (or use sudo)
NEED_SUDO=false
if [[ ! -w "/usr/local/bin" ]]; then
  echo ""
  log_info "Symlink Installation Requires Administrator Access"
  echo "Overlord needs to create a symlink at /usr/local/bin/overlord to make the"
  echo "'overlord' command available globally from your terminal. This directory"
  echo "requires administrator (sudo) privileges to write to for system-wide commands."
  echo ""
  log_warning "You will be prompted for your password to complete the installation"
  NEED_SUDO=true
fi

# Create symlink to /usr/local/bin/overlord
log_info "Installing overlord to $INSTALL_TARGET..."

if [[ -L "$INSTALL_TARGET" ]]; then
  # Existing symlink
  CURRENT_TARGET="$(readlink "$INSTALL_TARGET")"
  if [[ "$CURRENT_TARGET" == "$REPO_DIR/overlord" ]]; then
    log_success "Symlink already points to this repository"
  else
    log_warning "Existing symlink points to: $CURRENT_TARGET"
    read -r -p "Replace with this repository? (y/N): " response
    case "$response" in
      [yY]|[yY][eE][sS])
        if [[ "$NEED_SUDO" == true ]]; then
          sudo rm "$INSTALL_TARGET"
          sudo ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
        else
          rm "$INSTALL_TARGET"
          ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
        fi
        log_success "Symlink updated"
        ;;
      *)
        log_warning "Keeping existing symlink"
        ;;
    esac
  fi
elif [[ -f "$INSTALL_TARGET" ]]; then
  # Regular file exists
  log_error "Regular file exists at $INSTALL_TARGET"
  log_error "Please remove it manually and re-run setup"
  exit 1
else
  # Create new symlink
  if [[ "$NEED_SUDO" == true ]]; then
    sudo ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
  else
    ln -s "$REPO_DIR/overlord" "$INSTALL_TARGET"
  fi
  log_success "Symlink created: $INSTALL_TARGET → $REPO_DIR/overlord"
fi

# Get base directory for projects
DEFAULT_BASE_DIR="$HOME/Work"
echo ""
log_info "Base Directory Configuration"
log_info "Projects will be organized under: Python/, Typescript/, Solidity/"
echo ""

read -r -p "Enter base directory for projects [$DEFAULT_BASE_DIR]: " USER_BASE_DIR
BASE_DIR="${USER_BASE_DIR:-$DEFAULT_BASE_DIR}"

# Create base directory if it doesn't exist
if [[ ! -d "$BASE_DIR" ]]; then
  log_info "Creating base directory: $BASE_DIR"
  mkdir -p "$BASE_DIR"
  log_success "Created: $BASE_DIR"
fi

# Initialize registry.json if missing
if [[ ! -f "$REGISTRY_FILE" ]]; then
  log_info "Initializing registry..."
  # Create registry with settings section including base_dir
  cat > "$REGISTRY_FILE" << EOF
{
  "settings": {
    "base_dir": "$BASE_DIR"
  },
  "projects": {}
}
EOF
  log_success "Created registry: $REGISTRY_FILE"
else
  # Check if registry has settings.base_dir, add if missing
  if ! jq -e '.settings.base_dir' "$REGISTRY_FILE" &>/dev/null; then
    log_info "Updating registry with base_dir setting..."
    # Backup existing registry
    cp "$REGISTRY_FILE" "$REGISTRY_FILE.bak"
    # Add settings.base_dir to existing registry
    tmp_file=$(mktemp)
    jq --arg base_dir "$BASE_DIR" '.settings = (.settings // {}) | .settings.base_dir = $base_dir' "$REGISTRY_FILE" > "$tmp_file" && mv "$tmp_file" "$REGISTRY_FILE"
    log_success "Updated registry with base_dir: $BASE_DIR"
  else
    log_success "Registry already exists: $REGISTRY_FILE"
  fi
fi

# Verify installation
echo ""
log_info "Verifying installation..."

if command -v overlord &>/dev/null; then
  VERSION_OUTPUT="$(overlord --version 2>&1 || true)"
  log_success "overlord command is available"
  log_info "$VERSION_OUTPUT"
else
  log_error "overlord command not found in PATH"
  log_error "You may need to add /usr/local/bin to your PATH"
  exit 1
fi

# Show base directory info
BASE_DIR_FROM_REGISTRY=$(jq -r '.settings.base_dir' "$REGISTRY_FILE" 2>/dev/null || echo "$BASE_DIR")

# Success message
echo ""
log_success "Overlord installation complete!"
echo ""
log_info "Configuration:"
echo "  Base directory: $BASE_DIR_FROM_REGISTRY"
echo "  Registry: $REGISTRY_FILE"
echo "  Config directory: $REPO_DIR"
echo ""
log_info "Quick start:"
echo "  overlord new myproject --py      # Create new Python project"
echo "  overlord list                    # List projects"
echo "  overlord open                    # Open project workspace"
echo "  overlord --help                  # Show all commands"
echo ""

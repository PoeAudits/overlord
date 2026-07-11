# overlord

A Go-based project management CLI for organizing development projects with category-based structure, tmux workspace integration, and AI-first registry design.

## Overview

Overlord v2 is a complete rewrite from Bash to Go, providing a modern CLI for managing development projects. It organizes projects by category (not language), maintains a rich YAML registry with metadata, and integrates seamlessly with tmux workspaces. The registry is designed to be both human-readable and AI-friendly, with embedded project templates for quick initialization.

## Features

- **Category-Based Organization** - Projects organized by purpose (web, services, cli, etc.) not language
- **Rich Project Registry** - YAML-based registry with metadata (category, language, tags, status, aliases)
- **Fuzzy Matching** - Smart project resolution with fuzzy search and alias support
- **Interactive TUI Picker** - Beautiful terminal UI for project selection with real-time filtering
- **Embedded Templates** - Language-specific project templates (Makefile, README, AGENTS.md, etc.)
- **Tmux Workspace Integration** - Automatic session creation and management with .tmux.local support
- **Global Thoughts Directory** - Centralized thoughts directory for cross-project notes
- **Archive Management** - Archive and restore projects without losing metadata
- **Source Backups** - Timestamped backup zips for registry projects tagged `backup`
- **Atomic Operations** - Safe registry updates with automatic backups and validation
- **Syncthing Sync Management** - Continuous bidirectional sync via Syncthing with per-project activation

## Quick Start

```bash
# Build and install
make build
make install
overlord setup

# List active projects
overlord

# Open a project (with interactive picker)
overlord open

# Open a specific project
overlord myproject

# Create a new project
overlord new my-web-app --category=web --ts

# Attach the current directory to local OpenCode
oc

# Show project details
overlord info my-web-app
```

## Installation

### Prerequisites

- Go 1.25.5 or later
- Git (for worktree management and backup command)
- Tmux (for session management)
- Syncthing (for continuous project + registry artifact sync)

### Build from Source

```bash
# Clone the source code repository
git clone git@github.com:PoeAudits/overlord.git
cd overlord

# Build the binary
make build

# Install to ~/.local/bin
make install

# Run initial setup (creates directories, config, and installs the managed oc shim)
overlord setup
```

### Deploy to VPS

```bash
# Cross-compile and deploy to VPS (default: poe-vps)
make deploy

# Deploy to specific host
make deploy VPS=my-server
```

## Repositories

Overlord uses two separate repositories:

| Repository | Purpose | Location |
|------------|---------|----------|
| [`PoeAudits/overlord`](https://github.com/PoeAudits/overlord) | Source code (Go project) | `~/Overlord/core/tools/overlord` |
| [`PoeAudits/overlord-config`](https://github.com/PoeAudits/overlord-config) | Optional disaster-recovery backup target | `~/.local/share/overlord/backup` |

Operational multi-machine sync is Syncthing-first (not git-based).

## Configuration

The config directory (`~/.config/overlord/`) is a Syncthing shared folder (`overlord-config`):

- `registry.yaml` - Project registry with metadata (shared across machines)
- `peers.yaml` - Shared non-sensitive machine peer metadata
- `registry.lock.yaml` - Single-writer lock artifact for registry mutations
- `machine.yaml` - Machine-specific sensitive sync configuration (`.stignore`d, NOT shared)
- `.stignore` - Excludes machine-specific files such as `machine.yaml`

Run `overlord setup --init` on a new machine to initialize local machine config, configure the Syncthing config folder, and reconcile active projects.

### Registry Structure

The registry uses a versioned YAML format:

```yaml
version: 2
settings:
  base_dir: ~/Overlord
  thoughts_dir: ~/thoughts
projects:
  my-project:
    path: projects/web/my-project
    category: web
    lang: typescript
    created: "2026-01-23"
    description: "My awesome project"
    aliases: ["mp"]
    tags: ["react", "nextjs"]
    status:
      state: active
    sync:
      status: active
      method: syncthing
```

### Categories

- `contracts` - Smart contracts and blockchain projects
- `web` - Web applications and frontends
- `services` - Backend services and APIs
- `ml` - Machine learning projects
- `libs` - Libraries and packages
- `cli` - Command-line tools
- `core-agents` - Core AI agents
- `core-tools` - Core development tools
- `core-evals` - Core evaluation infrastructure
- `sandbox` - Experimental projects

### Languages

- `python` - Python projects
- `typescript` - TypeScript/JavaScript projects
- `go` - Go projects
- `solidity` - Solidity smart contracts
- `base` - Language-agnostic or multi-language projects

### Machine Configuration

For Syncthing integration, `~/.config/overlord/machine.yaml` stores local-only values:

```yaml
name: desktop-machine
role: main
syncthing:
  device_id: "XXXX-XXXX-XXXX-XXXX"
  api_url: "http://localhost:8384"
  api_key: "<key>"
  address: "tcp://100.x.x.x:22000"
```

Shared peers are defined in `peers.yaml` (not in `machine.yaml`).

## Usage

### Core Commands

#### List Projects

```bash
# List active projects (default)
overlord
overlord list

# Filter by category
overlord list --category=web

# Filter by tag
overlord list --tag=api --tag=production

# Show archived projects
overlord list --archived

# Show all projects (active + archived)
overlord list --all

# Output as JSON
overlord list --json
```

#### Open Project Workspace

```bash
# Interactive picker (fuzzy search)
overlord open

# Open by name
overlord open my-project

# Open by partial name (fuzzy match)
overlord open myproj

# Open by alias
overlord open mp

# Shorthand (same as 'overlord open')
overlord my-project
```

The `open` command:
- Creates a new tmux session if it doesn't exist
- Attaches to existing session if already running
- Sources `.tmux.local` file if present in project directory
- Switches client if already in tmux, otherwise attaches

#### Attach Current Directory to OpenCode

```bash
# Preferred stable entrypoint
oc

# Direct Overlord command
overlord code
```

The `oc` shim delegates to `overlord code`, which:
- Ensures the local OpenCode stack is up
- Resolves the local OpenCode URL from Overlord config
- Attaches the current working directory to the local OpenCode server
- Uses the same OpenCode binary selection as `overlord start`

#### Managed T3 Code Service

Overlord can manage T3 Code as a first-class stack service above OpenCode. OpenCode remains the local agent server for `oc` and `overlord code`; T3 Code is the local/remote control plane exposed through Tailscale Serve.

```yaml
services:
  opencode:
    enabled: true
    settings:
      host: 127.0.0.1
      port: "4096"
  t3code:
    enabled: true
    settings:
      host: 127.0.0.1
      port: "3773"
      workdir: /home/thomas/Overlord/core/tools/code
      base_dir: ~/.local/share/t3code
  tailscale:
    enabled: true
    settings:
      target: http://127.0.0.1:3773
```

Overlord checks for the built server artifact, seeds T3 Code's OpenCode provider setting to `http://127.0.0.1:4096`, then runs the equivalent of:

```bash
T3CODE_TAILSCALE_SERVE=false node apps/server/dist/bin.mjs serve --host 127.0.0.1 --port 3773 --base-dir ~/.local/share/t3code
```

Run it from `/home/thomas/Overlord/core/tools/code` after building T3 Code. If `apps/server/dist/bin.mjs` is missing, the T3 Code window prints the required build command (`bun run --cwd apps/web build && bun run --cwd apps/server build`) and exits. Do not enable T3 Code's own Tailscale Serve flag; Overlord owns `tailscale serve --bg` and should proxy the remote root URL to `http://127.0.0.1:3773`, not to OpenCode.

#### Show Project Info

```bash
# Show detailed project information
overlord info my-project

# Works with fuzzy matching
overlord info myproj
```

Displays:
- Full project path
- Category and language
- Description
- Status (active/archived)
- Tags and aliases
- Creation date

#### Create New Project

```bash
# Create with interactive prompts
overlord new my-project

# Create with all options
overlord new my-web-app \
  --category=web \
  --lang=typescript \
  --description="My awesome web app" \
  --tag=react \
  --tag=nextjs \
  --alias=mwa

# Dry run (show what would be created)
overlord new my-project --dry-run
```

Creates:
- Project directory structure
- Registry entry with metadata
- Template files (Makefile, README.md, AGENTS.md, .tmux.local, opencode.jsonc)

#### Register Existing Project

```bash
# Add existing directory to registry
overlord add my-project /path/to/project \
  --category=services \
  --lang=go \
  --description="Existing service"

# Path can be relative to base_dir
overlord add my-lib libs/my-lib --category=libs --lang=python
```

#### Remove Project

```bash
# Remove from registry (keeps files)
overlord rm my-project

# Confirm removal
overlord rm my-project --force
```

**Note:** This only removes the registry entry. Project files are not deleted.

#### Archive/Unarchive Projects

```bash
# Archive a project (marks as archived, keeps in registry)
overlord archive my-project

# Restore archived project
overlord unarchive my-project

# Restore to different category
overlord unarchive my-project --category=sandbox
```

Archived projects:
- Remain in registry with `status: archived`
- Hidden from default `list` output
- Cannot be opened until restored
- Useful for inactive projects you want to track

#### Backup Projects

```bash
# Create ~/overlord-backup-YYYYMMDD-HHMMSS.zip
overlord backup

# Preview what would be backed up without writing a zip
overlord backup --dry-run
overlord backup --dry-run --files

# Manage the backup set
overlord backup list
overlord backup add my-project another-project
overlord backup rm old-project
```

The backup set is stored as a `backup` tag on registry projects. For git repositories, the backup includes tracked files plus untracked files that are not ignored by `.gitignore`. Files matching `.env*` are always excluded, along with common top-level generated/dependency directories such as `node_modules`, `.venv`, `lib`, `broadcast`, `cache`, `out`, and `artifacts`.

#### Environment Variables

Overlord integrates with [pass](https://www.passwordstore.org/) (the standard Unix password manager) for secure environment variable management.

**Generate `.env.pass` template:**
```bash
# Auto-detect project from current directory
overlord env generate

# Generate for specific project
overlord env generate my-project

# Overwrite existing .env.pass
overlord env generate --force
```

The `generate` command scans your pass store and creates a `.env.pass` template:
```
# Environment variables — generated by overlord env generate
ANTHROPIC_API_KEY=pass:ai/anthropic-api-key
OPENAI_API_KEY=pass:ai/openai-api-key
NODE_ENV=development
```

**Pull secrets into `.env`:**
```bash
# Auto-detect project from current directory
overlord env

# Pull for specific project
overlord env my-project
```

The `.env.pass` template format:
- `KEY=pass:entry-name` — fetched from pass store
- `KEY=literal-value` — written as-is to `.env`
- Comments (lines starting with `#`) are ignored

The `.env` file is created with `0600` permissions (owner read/write only) for security.

**Auto-generation during project creation:**
When you run `overlord new`, it automatically generates `.env.pass` if your pass store exists.

#### Sync Commands

Overlord manages Syncthing-based sync for working across machines with lock-guarded registry writes.

**Activate Sync:**
```bash
# Mark one project for sync
overlord activate my-project

# Bulk activation
overlord activate app api worker
overlord activate --all

# Works with fuzzy matching and aliases
overlord activate mp
```

The `activate` command:
- Ensures the peer device and shared folder exist in local Syncthing
- Sets `sync.status = active` and `sync.method = syncthing`
- Enforces registry lock ownership before mutating `registry.yaml`

**Sync Status:**
```bash
# Sync all active projects
overlord sync

# Sync a specific project
overlord sync my-project
```

The `sync` command:
- Shows Syncthing folder status for active projects
- Does not initiate transfers (sync is continuous via Syncthing)

**Deactivate Sync:**
```bash
# Deactivate one project
overlord deactivate my-project

# Bulk deactivation
overlord deactivate app api worker
overlord deactivate --all
```

The `deactivate` command:
- Removes the Syncthing folder locally
- Sets `sync.status = inactive` and `sync.method = syncthing`
- Enforces registry lock ownership before mutating `registry.yaml`

**Lock Commands:**
```bash
overlord sync lock show
overlord sync lock transfer <machine>
overlord sync lock claim
```

**Verification and Backup:**
```bash
overlord doctor
overlord backup
```

**Sync Workflow Example:**
```bash
# On desktop:
overlord sync lock show
overlord activate my-web-app
overlord sync                   # Check status

# Work on the project...
# Make changes, commit, etc.

overlord sync my-web-app        # Check sync status

# When done working on this machine:
overlord deactivate my-web-app
```

### Project Resolution

Overlord uses smart resolution to find projects:

1. **Exact name match** - Highest priority
2. **Exact alias match** - Second priority  
3. **Fuzzy matching** - Falls back to fuzzy search on names and aliases

```bash
# All of these can work:
overlord open overlord              # Exact name
overlord open ov2                   # Alias
overlord open ovlrd                 # Fuzzy match
```

If multiple projects match with equal scores, an interactive picker is shown.

### Interactive Picker

The picker appears when:
- Running `overlord open` with no arguments
- Multiple projects match a query equally
- Running `overlord` with no arguments (lists projects)

Features:
- Real-time fuzzy filtering
- Arrow keys or j/k for navigation
- Enter to select
- Esc to cancel
- Shows project name, category, and description

### Templates

New projects are initialized with language-specific templates:

| Template | Description |
|----------|-------------|
| `Makefile` | Build commands and worktree management |
| `README.md` | Project documentation (with variables filled) |
| `AGENTS.md` | AI agent documentation (with variables filled) |
| `.tmux.local` | Tmux session setup script |
| `opencode.jsonc` | OpenCode configuration |

Templates support variable substitution:
- `{{.ProjectName}}` - Project name
- `{{.Description}}` - Project description
- `{{.Date}}` - Creation date (YYYY-MM-DD)
- `{{.Category}}` - Project category
- `{{.Language}}` - Programming language

### Thoughts Directory

The registry includes a global `thoughts_dir` setting (default: `~/thoughts`):

```yaml
settings:
  base_dir: ~/Overlord
  thoughts_dir: ~/thoughts
```

This directory is intended for:
- Cross-project notes and ideas
- Daily logs and journals
- Planning documents
- Anything that doesn't belong to a specific project

Access it from any project's `.tmux.local` via `$THOUGHTS_DIR` environment variable.

## Makefile Commands

### Development

```bash
make build                          # Build the application
make install                        # Install to ~/.local/bin
make run                            # Run the application (go run .)
make test                           # Run tests
make bench                          # Run benchmarks
make fmt                            # Format code
make vet                            # Vet code
make clean                          # Clean build artifacts
```

### Sync Commands

```bash
make activate NAME=<name>           # Activate sync for a project
make sync [NAME=<name>]             # Sync all or specific project
make deactivate NAME=<name>         # Deactivate sync for a project
make sync-lock-show                 # Show lock owner
make sync-lock-transfer MACHINE=<name>
make sync-lock-claim                # Force-claim lock locally
make doctor                         # Run diagnostic checks
make backup                         # Push disaster-recovery backup
```

### Environment Variables

```bash
make env-pull [NAME=<name>]         # Pull .env from .env.pass template
make env-generate [NAME=<name>]     # Generate .env.pass from pass store
make env-generate FORCE=1           # Overwrite existing .env.pass
```

```

See `make help` for the complete list of worktree commands.

## Development

### Project Structure

```
overlord/
├── cmd/overlord/              # Application entry point
│   └── main.go                # Calls cmd.Execute()
├── internal/                  # Private application code
│   ├── cmd/                   # Cobra command definitions
│   │   ├── root.go            # Root command + routing
│   │   ├── list.go            # List projects
│   │   ├── info.go            # Show project info
│   │   ├── new.go             # Create new project
│   │   ├── add.go             # Register existing project
│   │   ├── rm.go              # Remove project
│   │   ├── open.go            # Open workspace (with TUI picker)
│   │   ├── archive.go         # Archive project
│   │   └── unarchive.go       # Restore project
│   ├── registry/              # Registry types and operations
│   │   ├── types.go           # Registry, Project, Category, Language
│   │   ├── store.go           # Load/Save with atomic writes
│   │   └── resolve.go         # Fuzzy matching and resolution
│   └── templates/             # Embedded templates
│       ├── templates.go       # Template loading and rendering
│       └── templates/         # Template files (embedded)
│           ├── base/          # Base templates (all languages)
│           ├── go/            # Go-specific templates
│           ├── python/        # Python-specific templates
│           ├── typescript/    # TypeScript-specific templates
│           ├── solidity/      # Solidity-specific templates
│           └── shared/        # Shared files (.tmux.local)
├── Makefile                   # Build and worktree commands
└── go.mod                     # Go module definition
```

### Running Tests

```bash
# Run all tests
make test

# Run benchmarks
make bench

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test -v ./internal/registry/
```

### Code Quality

```bash
# Format code
make fmt

# Run static analysis
make vet

# Run all checks
make fmt && make vet && make test
```

### Key Design Decisions

1. **Category over Language** - Projects organized by purpose (web, services, cli) not language
2. **Fuzzy Matching** - Smart resolution with exact match → alias → fuzzy fallback
3. **Embedded Templates** - Templates compiled into binary for portability
4. **Interactive TUI** - Bubbletea-based picker for better UX
5. **Atomic Writes** - Registry updates use temp file + rename for safety
6. **Validation Everywhere** - All types validate before persistence

## License

[License information to be added]

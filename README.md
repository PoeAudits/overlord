# overlord-v2

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
- **Atomic Operations** - Safe registry updates with automatic backups and validation

## Quick Start

```bash
# Build and install
make build
make install

# List active projects
overlord

# Open a project (with interactive picker)
overlord open

# Open a specific project
overlord myproject

# Create a new project
overlord new my-web-app --category=web --lang=typescript

# Show project details
overlord info my-web-app
```

## Installation

### Prerequisites

- Go 1.25.5 or later
- Git (for worktree management)
- Tmux (for session management)

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd overlord-v2

# Build the binary
make build

# Install to ~/.local/bin (copies to ~/.local/bin/overlord)
make install

# Verify installation
overlord --version
```

## Configuration

Overlord stores its configuration and registry at `~/.config/overlord/`:

- `config.yaml` - Application configuration (managed by Viper)
- `registry.yaml` - Project registry with metadata

### Registry Structure

The registry uses a versioned YAML format:

```yaml
version: 2
settings:
  base_dir: ~/Work
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
- `sandbox` - Experimental projects

### Languages

- `python` - Python projects
- `typescript` - TypeScript/JavaScript projects
- `go` - Go projects
- `solidity` - Solidity smart contracts
- `base` - Language-agnostic or multi-language projects

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
- Template files (Makefile, README.md, AGENTS.md, .tmux.local, .opencode/opencode.jsonc)

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

### Project Resolution

Overlord uses smart resolution to find projects:

1. **Exact name match** - Highest priority
2. **Exact alias match** - Second priority  
3. **Fuzzy matching** - Falls back to fuzzy search on names and aliases

```bash
# All of these can work:
overlord open overlord-v2           # Exact name
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
| `.opencode/opencode.jsonc` | OpenCode configuration |

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
  base_dir: ~/Work
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

### Worktree Management

The Makefile includes commands for Git worktree management (used during development):

```bash
make worktree-new [BRANCH=name]     # Create worktree + tmux session
make worktree-list                  # List all worktrees
make worktree-attach BRANCH=name    # Attach to session
make worktree-remove BRANCH=name    # Remove worktree + kill session
make worktree-archive BRANCH=name   # Archive logs from worktree
```

See `make help` for the complete list of worktree commands.

## Development

### Project Structure

```
overlord-v2/
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

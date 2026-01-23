# Overlord v2 Context

## Document Purpose

This document provides essential context for developing Overlord v2 by synthesizing relevant information from the larger infrastructure project. It covers:
- **Why** Overlord v2 is being built
- **What** it replaces (Overlord v1)
- **How** it integrates with the broader system
- **Where** it fits in the multi-machine architecture

---

## Project Background

### Motivation

Overlord v2 is part of a larger shift toward **AI-first, location-independent development**:

1. **AI capability inflection** - Models are now capable enough to help build and maintain infrastructure
2. **Development style shift** - Moved from writing code to orchestrating AI agents via OpenCode
3. **Location independence** - Need to access development environment from anywhere
4. **Background agent work** - Dispatch tasks while mobile, check results later

### Key Requirements

**Functional:**
- Fresh VPS provisionable to working state in <1 hour via scripts
- Dispatch AI agents from mobile, check results later
- Overlord v2 manages projects across machines with registry sync
- Path resolver plugin handles fuzzy/voice-transcribed file references

**Non-Functional:**
- Recovery time: <1 hour from bare VPS
- Monthly cost: <$50 (target: ~$10-15)
- Security: Tailscale-only access, SSH keys, sensitive data on desktop only

### Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Project Structure | Category-based, not language-based | AI agents don't care about language; cross-language projects common |
| Thoughts Directory | Global only, organized by project | Centralized knowledge base; per-project thoughts eliminated |
| Registry Metadata | Description + tags + status flags | Rich metadata for agent discovery and filtering |
| Registry Format | YAML | More token-efficient than JSON, human-readable |
| Template Storage | Embedded in binary | Simpler deployment, versioned with code |
| Installation | XDG compliant | `~/.local/bin/` + `~/.config/overlord/` |

---

## Overlord v1 (Current Implementation)

### Overview

**Location:** `~/.config/overlord/`

**Language:** Bash (shell scripts)

**Documentation:** See `/home/thomas/.config/overlord/AGENTS.md`

### Current Features

- Project registry (`registry.json`)
- Language detection (Python, TypeScript, Solidity, Go)
- tmux session templates
- Makefile generation
- OpenCode config templates
- thoughts/ directory creation (per-project)

### Commands

```bash
overlord                     # List projects
overlord <name>              # Open workspace
overlord new <name> --py     # Create new Python project
overlord sync <name>         # Sync templates to project
overlord list --json         # Machine-readable list
```

### Current Directory Structure

```
~/Work/
├── Python/
│   ├── active/
│   ├── libs/
│   ├── tools/
│   └── archive/
├── Typescript/
│   └── ...
├── Solidity/
│   └── ...
└── Go/
    └── ...
```

**Problem:** Language-based organization doesn't align with how agents think about projects or how modern multi-language projects work.

### What Works Well

- Simple project creation workflow
- tmux integration is solid
- Template system is functional
- OpenCode config generation

### What Needs Improvement

- **Language-based structure** - Not AI-first, awkward for multi-language projects
- **Bash implementation** - Hard to extend, no type safety
- **Local-only** - No multi-machine awareness
- **Per-project thoughts** - Scattered, hard for orchestration agents to find
- **Limited metadata** - Just name, language, path
- **No fuzzy matching** - Exact names only
- **No voice-friendly features** - Poor handling of transcription errors

---

## Overlord v2 Improvements

### New Features

1. **Category-based organization** - contracts, web, services, ml, libs, cli, core-agents, core-tools, sandbox
2. **Global thoughts directory** - Centralized knowledge base at `~/thoughts/`
3. **Rich metadata** - Description, tags, status (active/archived), creation date, aliases
4. **Multi-machine awareness** - Registry sync across machines (design TBD)
5. **Fuzzy matching** - For voice input and quick access
6. **Interactive picker** - fzf-style project selection
7. **Archive management** - Dedicated archive/unarchive commands
8. **Embedded templates** - Single binary deployment
9. **Go implementation** - Type safety, better testing, single binary

### Migration Path

From v1 language-based to v2 category-based:

| Current Location | New Location |
|------------------|--------------|
| `Python/active/{ml-project}/` | `projects/ml/{project}/` |
| `Python/active/{other}/` | `projects/services/` or `projects/cli/` |
| `Typescript/active/{web}/` | `projects/web/{project}/` |
| `Solidity/active/` | `projects/contracts/{project}/` |
| `Go/tools/` | `projects/cli/` or `core/tools/` |
| `{Lang}/libs/` | `projects/libs/{project}/` (flat) |
| `{Lang}/archive/` | `archive/{project}/` |

---

## Multi-Machine Architecture

### Machine Topology

```
                              TAILSCALE NETWORK
 ┌─────────────────────────────────────────────────────────────────┐
 │                                                                 │
 │   ┌─────────────┐     ┌─────────────┐     ┌─────────────┐      │
 │   │   Desktop   │     │  Cloud VPS  │     │ Home Server │      │
 │   │  (Omarchy)  │────▶│  (Primary)  │◀────│   (Heavy)   │      │
 │   │             │     │             │     │             │      │
 │   │ - Hyprland  │     │ - Docker    │     │ - Long jobs │      │
 │   │ - Hyper     │     │ - tmux      │     │ - Background│      │
 │   │   Whisper   │     │ - OpenCode  │     │   tasks     │      │
 │   │ - Local GPU │     │ - neovim    │     │             │      │
 │   │ - Sensitive │     │ - Projects  │     │             │      │
 │   │   data      │     │             │     │             │      │
 │   └─────────────┘     └─────────────┘     └─────────────┘      │
 │          │                   ▲                   ▲              │
 │          │                   │                   │              │
 │          ▼                   │                   │              │
 │   ┌─────────────┐     ┌─────────────┐                          │
 │   │   Laptop    │     │   Mobile    │                          │
 │   │  (Omarchy)  │────▶│  (Android)  │                          │
 │   │             │     │             │                          │
 │   │ - Same as   │     │ - SSH/Termux│                          │
 │   │   desktop   │     │ - Voice-to- │                          │
 │   │   (dotfiles)│     │   text      │                          │
 │   └─────────────┘     └─────────────┘                          │
 │                                                                 │
 └─────────────────────────────────────────────────────────────────┘
```

### Primary Development Server (Cloud VPS)

**Purpose:** Main development environment accessible from anywhere

**Specs (Starting Point):**
- Hetzner CPX11: 2 vCPU (AMD), 2GB RAM, 40GB NVMe - €4.35/mo
- Location: Hillsboro, Oregon (US West)

**Operating System:** Ubuntu 24.04 LTS (headless)

**Access:** SSH via Tailscale only (no public SSH)

### What Runs Where

| Task | Primary Location | Fallback | Reason |
|------|-----------------|----------|--------|
| Code editing (neovim) | VPS | Desktop | Centralized, accessible everywhere |
| OpenCode agents | VPS | Desktop | Same as editing |
| Git operations | VPS | Desktop | Where code lives |
| Heavy compilation | Home Server | VPS | Free compute, offload VPS |
| Long-running scripts | Home Server | VPS | Don't tie up primary dev server |
| Voice-to-text | Desktop (Hyper Whisper) | Mobile (built-in) | Desktop has GPU |
| Secret management | Desktop | - | High-trust zone |

### Security Model

```
┌─────────────────────────────────────────────────────────────────┐
│                     HIGH TRUST (Desktop Only)                    │
│                                                                  │
│   - Crypto private keys                                         │
│   - 1Password vault                                             │
│   - SSH private keys (master)                                   │
│   - Any credentials that can't be rotated easily                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Secrets injected at runtime
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     MEDIUM TRUST (VPS)                           │
│                                                                  │
│   - API keys (rotatable): OpenAI, Anthropic, GitHub, etc.       │
│   - SSH keys for git operations                                 │
│   - Environment-specific secrets (managed via pass/GPG)         │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Overlord v2 Multi-Machine Considerations

1. **Registry sync** - How to keep project registry in sync across machines?
   - Option 1: Git-based (registry.yaml in a repo)
   - Option 2: Tailscale file sync
   - Option 3: Hybrid (git for history, local cache for speed)

2. **Project "ownership"** - Some projects primarily live on VPS, others on desktop
   - Need metadata: `primary_machine: vps` or `primary_machine: desktop`
   - Remote operations: `overlord open <project>` should SSH to correct machine

3. **Remote project operations**
   - Open project on remote machine: trigger tmux session via SSH
   - Create project on specific machine: `overlord new <name> --on=vps`
   - Sync project between machines: `overlord sync <name> --to=desktop`

---

## Integration Points

### tmux (Terminal Multiplexer)

**Configuration:** `~/.config/tmux/tmux.conf` (in dotfiles)

**Overlord Integration:**
- `.tmux.local` per-project templates
- Makefile commands for tmux operations
- `overlord open` creates/attaches tmux sessions

**Multi-machine:** tmux sessions run on VPS, accessible from any device via SSH

### OpenCode (AI Coding Agent)

**Configuration:**
- Global: `~/.config/opencode/opencode.jsonc`
- Per-project: `.opencode/opencode.jsonc`

**Overlord Integration:**
- Overlord generates per-project `.opencode/opencode.jsonc` with appropriate settings
- Skills loaded from `~/.agents/skills/`
- OpenCode needs permission to access:
  - `~/Work/` (all subdirectories)
  - `~/thoughts/`
  - `~/files/`
  - `~/.agents/` (read-only for skills)

**Multi-machine:** OpenCode runs on VPS, same configuration via dotfiles

### Dispatch (Agent Orchestration)

**Location:** `~/Work/core/agents/dispatch/` (will move to this location)

**Current Status:** Active development, Go rewrite from TypeScript

**Purpose:** CLI for orchestrating AI agents - allows agents to dispatch other agents

**Commands:**
```bash
dispatch run <agent> "prompt"           # Run single agent
dispatch parallel <agent1> <agent2>     # Run agents in parallel
dispatch workflow <file.yaml>           # Run multi-step workflow
```

**Overlord Integration:**
- Dispatch can use Overlord to discover projects: `dispatch run worker "$(overlord list --json)"`
- Orchestration agents need access to full project registry
- Workflow definitions may reference projects by name or category

### Agent Skills

**Location:** `~/.agents/skills/`

**Sync Tool:** `~/.agents/bin/sync-skills`

**Purpose:** Modular knowledge packages for AI agents

**Overlord-Relevant Skills:**
- `thoughts-directory` - How to use global thoughts directory
- `go-coding-guidelines` - For Overlord v2 development
- `go-backend-development` - CLI structure, file operations
- `go-testing` - Testing Overlord v2
- `tools` - General tool development patterns

**Multi-machine:** Skills synced via dotfiles to all machines

### Secrets Management (pass + GPG)

**Strategy:** Each machine has its own GPG key, secrets encrypted to all keys

**Structure:**
```
~/.password-store/
├── .gpg-id                    # All machine key IDs
├── dev/
│   ├── openai-api-key.gpg
│   ├── anthropic-api-key.gpg
│   ├── github-token.gpg
│   └── hetzner-api-token.gpg
└── personal/
    └── ...
```

**Shell Integration:**
```bash
# Loaded at shell start
export OPENAI_API_KEY=$(pass show dev/openai-api-key 2>/dev/null)
export ANTHROPIC_API_KEY=$(pass show dev/anthropic-api-key 2>/dev/null)
```

**Overlord v2:** No direct integration needed, but API keys available for agent operations

---

## Dotfiles & Reproducibility

### Dotfiles Management

**Tool:** Chezmoi (with templating)

**Repository Structure:**
```
dotfiles/
├── .chezmoi.toml.tmpl              # Machine config (mode: server|desktop)
├── dot_config/
│   ├── nvim/
│   ├── tmux/
│   ├── opencode/
│   ├── overlord/                   # Overlord v2 config
│   └── git/
├── tools.yaml                      # Tool installation definitions
└── install.sh                      # Bootstrap script
```

**Installation Modes:**
- `server` - Headless VPS (Ubuntu)
- `desktop` - Omarchy workstation (Arch + Hyprland)

**Bootstrap:**
```bash
curl -fsSL <dotfiles-url>/install.sh | bash -s -- --mode server
```

### Tool Installation

**Defined in `tools.yaml`:**
- Go 1.22+
- Bun (TypeScript/JavaScript)
- uv (Python)
- Foundry (Solidity)
- Tailscale (networking)
- tmux, neovim, starship

**Extensible:** Add new tools by editing `tools.yaml`

### Overlord v2 Deployment

**Installation:**
1. Binary built from Go source: `make build`
2. Installed to `~/.local/bin/overlord`
3. Configuration at `~/.config/overlord/`
4. Registry at `~/.config/overlord/registry.yaml`

**Dotfiles Integration:**
- Overlord v2 config and registry included in dotfiles
- Fresh machine: dotfiles install + Overlord binary = fully configured

---

## Voice Input Considerations

### Desktop (Hyper Whisper)

**Status:** Working on desktop

**Flow:**
1. User speaks
2. Hyper Whisper (local, GPU-accelerated) transcribes
3. Text sent to SSH session on VPS
4. OpenCode receives text, agent executes

**Quality:** High (local model, low latency)

### Mobile (Android Built-in)

**Flow:**
1. User speaks
2. Android voice-to-text transcribes
3. Text sent via Termux SSH to VPS
4. **Path Resolution Plugin** (future) handles fuzzy matching
5. OpenCode receives resolved paths, agent executes

**Quality:** Lower (transcription errors common)

### Path Resolution Plugin (Phase 5)

**Purpose:** OpenCode plugin for fuzzy path resolution

**Requirements:**
- Pattern matching for common structures (thoughts, plans, etc.)
- Fuzzy matching for voice-to-text errors
- Configurable path mappings
- Integration with Overlord registry (project aliases, categories)

**Example:**
- Input: "thoughts about auth" (voice transcription)
- Output: `~/thoughts/plans/myproject/2026-01-15_auth_implementation.md`

**Overlord v2 Integration:**
- Plugin queries Overlord registry for project names and aliases
- Uses category information for path resolution
- Handles fuzzy project name matching

---

## Implementation Phases (Context)

### Where Overlord v2 Fits

From the larger infrastructure project phases:

**Phase 1-3: Foundation** (not Overlord-specific)
- VPS setup, networking, dotfiles

**Phase 4: Overlord v2** ← This is the focus
- Duration: 1-2 weeks
- Design multi-machine registry format
- Implement core functionality (see implementation-plan.md)
- Migrate from v1

**Phase 5: Path Resolution Plugin**
- Integrates with Overlord v2 for project resolution
- Voice-friendly path matching

**Phase 6: Polish**
- Backup system, monitoring, documentation

---

## Reference Documents

For complete details, see:

1. **`implementation-plan.md`** - Full orchestration plan (6 phases, 16 tasks)
2. **`filesystem-design-reference.md`** - Complete filesystem structure and design rationale
3. Original source files in `~/.config/opencode/dev/`:
   - `README.md` - Infrastructure project overview
   - `architecture.md` - Detailed machine topology and networking
   - `tools-inventory.md` - Complete tool catalog

---

## Quick Reference

### Key Paths

```
~/Work/                              # All project work
├── core/                            # Essential infrastructure
│   ├── agents/                      # dispatch, etc.
│   └── tools/                       # overlord v2 lives here
├── projects/                        # Active development
│   ├── contracts/                   # Solidity
│   ├── web/                         # Web apps
│   ├── services/                    # Backend APIs
│   ├── ml/                          # ML/data
│   ├── libs/                        # Libraries (flat)
│   └── cli/                         # CLI tools
├── sandbox/                         # Experiments
└── archive/                         # Archived projects

~/thoughts/                          # Global thoughts directory
├── projects/                        # Project docs (symlinks)
├── plans/                           # Implementation plans
├── research/                        # Reusable knowledge
├── logs/                            # Execution logs
├── sessions/                        # Conversation exports
└── archive/                         # Old work

~/.agents/                           # Agent infrastructure
├── skills/                          # Knowledge packages
├── bin/                             # Agent utilities
└── workflows/                       # Dispatch workflows

~/.config/overlord/                  # Overlord v2 config
└── registry.yaml                    # Project registry
```

### Category Mapping

| Category | Purpose | Path |
|----------|---------|------|
| `core/agents` | Agent orchestration | `~/Work/core/agents/` |
| `core/tools` | System utilities | `~/Work/core/tools/` |
| `projects/contracts` | Smart contracts | `~/Work/projects/contracts/` |
| `projects/web` | Web frontends | `~/Work/projects/web/` |
| `projects/services` | Backend APIs | `~/Work/projects/services/` |
| `projects/ml` | ML/data processing | `~/Work/projects/ml/` |
| `projects/libs` | Shared libraries | `~/Work/projects/libs/` |
| `projects/cli` | CLI tools | `~/Work/projects/cli/` |
| `sandbox` | Experiments | `~/Work/sandbox/` |
| `archive` | Archived projects | `~/Work/archive/` |

### Commands (Overlord v2)

```bash
overlord list [--json] [--category=<cat>] [--tag=<tag>]
overlord info <name>
overlord new <name> --category=<cat> --lang=<lang>
overlord add <name> <path>
overlord rm <name> [--force]
overlord open <name>              # or just: overlord <name>
overlord archive <name>
overlord unarchive <name> --category=<cat>
```

---

*This context document is synthesized from the larger infrastructure project documentation. For implementation details, see `implementation-plan.md`.*

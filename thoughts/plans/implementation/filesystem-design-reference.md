# Filesystem Design

## Status: DESIGN COMPLETE - READY FOR IMPLEMENTATION

This document defines the AI-first filesystem structure for multi-machine development. The design optimizes for autonomous agent operation, cross-project orchestration, and seamless access from any device.

## Design Principles

1. **AI-first navigation**: Structure for how agents think, not language or legacy conventions
2. **Autonomous operation**: Pre-defined permission boundaries eliminate runtime prompts
3. **Flat where possible**: Minimize path depth for easier specification and voice input
4. **Registry over filesystem**: Use Overlord v2 registry for metadata, not directory structure
5. **Global knowledge base**: Centralized thoughts directory for cross-project work

## Directory Structure Overview

```
~/
├── Work/                   # All project work
│   ├── core/               # Essential infrastructure (protected)
│   ├── projects/           # Active development work
│   ├── sandbox/            # Experiments, prototypes
│   └── archive/            # Archived projects (read-only)
│
├── thoughts/               # Global thoughts (cross-project knowledge)
│   ├── projects/           # Project documentation index (symlinks)
│   ├── plans/              # Implementation plans
│   ├── research/           # Reusable knowledge base
│   ├── logs/               # Execution logs
│   ├── sessions/           # Conversation exports
│   └── archive/            # Completed work
│
├── files/                  # Shared files (not code)
│   ├── data/               # Datasets, cached API responses
│   └── documents/          # PDFs, reference materials
│
└── .agents/                # Agent infrastructure
    ├── skills/             # Knowledge packages
    ├── bin/                # Agent utilities
    └── workflows/          # Dispatch workflow definitions
```

## Project Directory (`~/Work/`)

### Structure

```
~/Work/
├── core/                   # Essential infrastructure (cannot be archived)
│   ├── agents/             # Agent tooling (dispatch, orchestration)
│   │   └── dispatch/
│   └── tools/              # System utilities
│       └── overlord/
│
├── projects/               # All active development work
│   ├── contracts/          # Blockchain/Solidity smart contracts
│   ├── web/                # Web applications, frontends
│   ├── services/           # Backend APIs, running services
│   ├── ml/                 # ML, data processing, research
│   ├── libs/               # Shared libraries (all languages, flat)
│   └── cli/                # Command-line tools
│
├── sandbox/                # Experiments, prototypes, throwaway
│   └── {experiment-name}/
│
└── archive/                # Archived projects (read-only, hidden from agents)
    └── {project-name}/
```

### Category Definitions

| Category | Purpose | Examples |
|----------|---------|----------|
| `core/agents` | Agent orchestration and tooling | dispatch, skills configs |
| `core/tools` | System utilities | overlord, system scripts |
| `projects/contracts` | Smart contracts, blockchain | Ulti, tokenjar, energy-coin |
| `projects/web` | Web frontends, full-stack apps | Blog-X402, bags, poly |
| `projects/services` | Backend APIs, running services | cdp-agent |
| `projects/ml` | ML, data processing, research | OracleDspy, signals, judgenetwork |
| `projects/libs` | Reusable libraries (any language) | dspy-utils, offchain-lib, supabase-utils |
| `projects/cli` | Command-line tools | gopa |
| `sandbox` | Experiments, prototypes | Quick tests, learning projects |
| `archive` | Archived projects | Completed or abandoned work |

### Design Decisions

**Why not language-based organization?**
- Agents don't care about language; they care about what a project does
- Cross-language projects are increasingly common with AI assistance
- Language is metadata, not a navigational concern

**Why nested under `projects/`?**
- Cleaner top-level (4 dirs vs 8+)
- Clear separation: infrastructure (`core/`) vs work (`projects/`)
- Sandbox and archive are distinct lifecycle states, not categories

**Why flat `libs/`?**
- Registry handles language/domain metadata
- Simpler paths: `~/Work/projects/libs/dspy-utils/`
- Query by tag: `overlord list --tag=python --category=libs`

## Global Thoughts Directory (`~/thoughts/`)

### Structure

```
~/thoughts/
├── projects/               # Project documentation index
│   └── {project-name}/
│       ├── README.md       # Symlink → project's README.md
│       └── AGENTS.md       # Symlink → project's AGENTS.md
│
├── plans/                  # Implementation plans (organized by project)
│   └── {project-name}/
│       └── {plan-name}.md  # Plans with status in frontmatter
│
├── research/               # Reusable knowledge base (organized by topic)
│   └── {topic}/            # e.g., github-api/, openai-api/, supabase/
│       └── {subtopic}.md
│
├── logs/                   # Execution logs (post-action summaries)
│   └── {project-name}/
│       └── {date}_{action}.md
│
├── sessions/               # Conversation exports (for prompt optimization)
│   └── {project-name}/
│       └── {date}_{title}.md
│
└── archive/                # Completed/old work
    └── {date}_{slug}/
```

### Purpose of Each Subdirectory

**`projects/`** - Project Documentation Index
- Symlinks to each project's README.md and AGENTS.md
- Allows agents to look up project info without navigating to the project
- Auto-syncs when project docs are updated (symlinks, not copies)
- Created by Overlord v2 when registering a project

**`plans/`** - Implementation Plans
- Organized by project name
- Plans have status in frontmatter (pending, in_progress, completed)
- Managed via `save-plan` and `lookup-plan` tools
- Stays in place after execution (marked complete, not moved)

**`research/`** - Reusable Knowledge Base
- Organized by topic, not project
- Cross-project research (APIs, technologies, patterns)
- Avoids re-researching the same topics
- Examples: `github-api/`, `openai-api/`, `supabase/`, `dspy/`

**`logs/`** - Execution Logs
- Post-execution summaries written by agent or user
- Documents what was done, issues encountered, deviations from plan
- Different from sessions (logs are summaries, sessions are full transcripts)

**`sessions/`** - Conversation Exports
- Full conversation exports from OpenCode
- Format: user prompts + assistant responses + tool names (not results)
- Named by date and session title
- Input for prompt optimization work

**`archive/`** - Completed Work
- Old plans, logs, research that's no longer active
- Organized by date and slug

### Per-Project Thoughts: Eliminated

The new system uses **global thoughts only**. Per-project `thoughts/` directories are eliminated.

**Rationale:**
- Plans and logs are already organized by project within global thoughts
- Easier for orchestration agents to find information
- Single location for all planning artifacts
- Project directories stay focused on code

**Migration:** Existing per-project thoughts migrate to global thoughts, organized by project name.

## Files Directory (`~/files/`)

### Structure

```
~/files/
├── data/                   # Datasets, cached API responses, test fixtures
└── documents/              # PDFs, text files, reference materials
```

### Purpose

- **Global shared files** that aren't code
- Not project-specific (project data stays in project)
- Accessible by all agents with appropriate permissions

### Design Decision

Files are **global only**, not organized by project. Rationale:
- Datasets and documents are often shared across projects
- Project-specific data lives in the project directory
- Simpler model: `~/files/` is for truly shared resources

## Agent Infrastructure (`~/.agents/`)

### Structure

```
~/.agents/
├── skills/                 # Agent skill packages
│   ├── thoughts-directory/
│   ├── python-coding-guidelines/
│   └── ...
│
├── bin/                    # Agent utilities, executable scripts
│   └── {script-name}
│
└── workflows/              # Dispatch workflow definitions (YAML)
    └── {workflow-name}.yaml
```

### Purpose

- `skills/`: Knowledge packages that agents can load
- `bin/`: Executable scripts for agent operations
- `workflows/`: Dispatch workflow definitions for orchestration

## Permissions Model

### Agent Permission Boundaries

| Agent Type | Can Read | Can Write |
|------------|----------|-----------|
| Regular OpenCode | Current project, `~/thoughts/`, `~/files/` | Current project, `~/thoughts/` |
| Orchestration (via dispatch) | All of `~/Work/`, `~/thoughts/`, `~/.agents/`, `~/files/` | All of `~/Work/`, `~/thoughts/` |

### OpenCode Configuration

OpenCode should be configured to allow:
- `~/Work/` (all subdirectories)
- `~/thoughts/`
- `~/files/`
- `~/.agents/` (read for skills)

This eliminates permission prompts for normal operations.

### Archive Semantics

- **Hidden from agent discovery**: Not listed by default in Overlord queries
- **Read-only**: Agents instructed not to modify archived projects
- **Explicit unarchive required**: Must move out of archive before modifying
- **AGENTS.md instruction**: Include warning about archived projects

```markdown
## Archive Policy
Do not modify projects in `~/Work/archive/`. If you need to work on an archived 
project, first unarchive it: `overlord mv {project} {category}`
```

## Plan Management Tools

### save-plan

Saves a plan to the global thoughts directory.

```bash
save-plan --project {name} --name {plan-name}
# Reads from stdin or file
# Saves to ~/thoughts/plans/{project}/{plan-name}.md
# Adds frontmatter: status, date, project
```

### lookup-plan

Retrieves plans from the global thoughts directory.

```bash
lookup-plan --latest                    # Most recent plan for current project
lookup-plan --latest --project {name}   # Most recent for specified project
lookup-plan --name {plan-name}          # Specific plan by name
lookup-plan --list                      # List all plans for project (by recency)
```

### Plan Frontmatter

```yaml
---
project: project-name
status: pending | in_progress | completed
created: 2026-01-22
completed: 2026-01-23  # if applicable
---
```

## Session Export Format

### Location

`~/thoughts/sessions/{project-name}/{date}_{title}.md`

### Format

- User prompts + assistant responses
- Tool call names included (not full results - too verbose)
- OpenCode export option: exclude tool call results
- Named by date and session title (not just ID)

### Purpose

Input for prompt optimization. Allows analysis of:
- Which prompts worked well
- Common patterns in successful sessions
- Areas for improvement

## Project Structure (Per-Project)

Each project contains:

```
{project}/
├── README.md               # Project description (symlinked to thoughts)
├── AGENTS.md               # Agent instructions (symlinked to thoughts)
├── Makefile                # Standard commands
├── .tmux.local             # tmux workspace config
├── .opencode/              # OpenCode project config
│   └── opencode.jsonc
└── {source files}
```

**Note:** No per-project `thoughts/` directory. All planning artifacts go to global `~/thoughts/`.

## Migration Strategy

### Current Structure
```
~/Work/{Python,Typescript,Solidity,Go}/{active,libs,tools,archive}/{project}/
```

### New Structure
```
~/Work/{core,projects,sandbox,archive}/...
```

### Migration Steps

1. **Create new directory structure**
2. **Map each project to new location** (see migration table below)
3. **Move projects with `git mv`** to preserve history
4. **Update Overlord registry** with new paths and categories
5. **Create symlinks** in `~/thoughts/projects/` for each project
6. **Migrate per-project thoughts** to global `~/thoughts/plans/{project}/`
7. **Verify and test**

### Category Mapping

| Current Status | Current Lang Dir | New Location |
|----------------|------------------|--------------|
| active | Python (ML-related) | `projects/ml/` |
| active | Python (other) | `projects/services/` or `projects/cli/` |
| active | Typescript (web) | `projects/web/` |
| active | Typescript (backend) | `projects/services/` |
| active | Solidity | `projects/contracts/` |
| active | Go (tools) | `projects/cli/` or `core/agents/` |
| lib | any | `projects/libs/` |
| tool | any | `projects/cli/` or `core/tools/` |
| archive | any | `archive/` |

### Special Cases

| Project | Current Location | New Location | Rationale |
|---------|-----------------|--------------|-----------|
| overlord | ~/.config/overlord | ~/Work/core/tools/overlord | System tool |
| dispatch | ~/Work/Go/tools/dispatch | ~/Work/core/agents/dispatch | Agent infrastructure |
| opencode | ~/.config/opencode | stays in ~/.config | OpenCode config, not a project |
| skills | ~/.agents/skills | stays in ~/.agents | Agent infrastructure |

## Voice-to-Text Considerations

### Deferred

Detailed voice-to-text path resolution is deferred. Current mitigations:
- Simpler path structure (fewer levels)
- Plan management tools (don't need to specify full paths)
- Overlord aliases and fuzzy find

### Future: Path Resolution Plugin

When implemented, the plugin will need:
- Knowledge of directory structure
- Project aliases from registry
- Fuzzy matching for plan names
- Resolution of `@` symbol references

## References

- [Overlord v2](./overlord-v2.md) - Registry design and project management
- [Architecture](./architecture.md) - System architecture and machine topology
- [Tools Inventory](./tools-inventory.md) - Related tools

# Orchestration Plan: Overlord v2

## Overview

**Objective:** Rewrite Overlord from Bash to Go as a single-binary CLI tool for managing development projects in a new category-based, AI-first filesystem structure, with YAML registry, embedded templates, and global thoughts directory integration.

**Phases:** 6 phases, 16 total tasks

**Estimated Complexity:** Complex

**Key Skills Required:**
- `go-coding-guidelines` - All Go implementation tasks
- `go-backend-development` - CLI structure, file operations
- `go-testing` - Unit tests for core functionality

---

## Phase 1: Project Foundation ⏳

**Status:** ⏳ Pending
**Goal:** Set up Go project structure, define types, and implement YAML registry operations

**Dependencies:** None

**Parallel Execution:** Tasks 1.1 and 1.2 can run in parallel

### Task 1.1: Initialize Go Project with Cobra CLI ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Create a new Go project for Overlord v2 with Cobra CLI framework. Set up the basic project structure with cmd/, internal/, and pkg/ directories following Go conventions.

**Context:**
From the planning brief:
- Single binary deployment to `~/.local/bin/overlord`
- Configuration stored at `~/.config/overlord/`
- Use Cobra for CLI framework, Viper for configuration
- Go 1.21+ required

**References:**
- Cobra CLI patterns: https://github.com/spf13/cobra
- Go project layout: https://github.com/golang-standards/project-layout
- XDG Base Directory specification for config paths

**Success Criteria:**
- [ ] Go module initialized (`go.mod` with appropriate module name)
- [ ] Cobra root command created with basic `--help` and `--version` flags
- [ ] Project structure: `cmd/overlord/main.go`, `internal/`, `pkg/`
- [ ] Makefile with `build`, `install`, `test`, `clean` targets
- [ ] Binary builds and runs `overlord --help`

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Do not add commands beyond the root command in this task
- Keep dependencies minimal (Cobra, Viper only for now)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 1.2: Define Core Types and Registry Schema ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Define Go types for the registry schema including Project, Settings, and Registry structures. Include YAML struct tags and validation logic.

**Context:**
From the planning brief, the registry schema (YAML format):

```yaml
version: 2
settings:
  base_dir: /home/thomas/Work
  thoughts_dir: /home/thomas/thoughts
  
projects:
  project-name:
    path: /absolute/path
    category: contracts|web|services|ml|libs|cli|core-agents|core-tools|sandbox
    lang: python|typescript|go|solidity|base
    created: "2026-01-22"
    description: "Brief description"
    aliases: [alias1, alias2]
    tags: [tag1, tag2]
    status:
      state: active|archived
```

Category to path mapping:
| Category | Filesystem Path |
|----------|-----------------|
| `contracts` | `projects/contracts/` |
| `web` | `projects/web/` |
| `services` | `projects/services/` |
| `ml` | `projects/ml/` |
| `libs` | `projects/libs/` |
| `cli` | `projects/cli/` |
| `core-agents` | `core/agents/` |
| `core-tools` | `core/tools/` |
| `sandbox` | `sandbox/` |

**References:**
- go-yaml library: https://github.com/go-yaml/yaml

**Success Criteria:**
- [ ] `internal/registry/types.go` with Registry, Settings, Project, Status structs
- [ ] YAML struct tags on all fields
- [ ] Category enum/constants with path mapping function
- [ ] Language enum/constants (python, typescript, go, solidity, base)
- [ ] Validation methods for required fields

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Use `gopkg.in/yaml.v3` for YAML handling
- All date fields should use string format "YYYY-MM-DD"
- Status.State should only allow "active" or "archived"

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 1.3: Implement YAML Registry Read/Write Operations ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement functions to load and save the registry YAML file, including automatic backup on write and handling of missing/new registry files.

**Context:**
- Registry location: `~/.config/overlord/registry.yaml`
- Should create default registry if none exists
- Should backup before writes (registry.yaml.bak)
- Must handle concurrent access safely (file locking or atomic writes)

**References:**
- Task 1.2 types (depends on completion)
- Existing v1 backup behavior in `/home/thomas/.config/overlord/lib/common.sh`

**Success Criteria:**
- [ ] `internal/registry/store.go` with Load() and Save() functions
- [ ] Creates `~/.config/overlord/` directory if missing
- [ ] Creates default registry with empty projects if file missing
- [ ] Backs up existing registry before overwriting
- [ ] Atomic write (write to temp, rename)
- [ ] Unit tests for load/save operations

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- Do not use external database libraries
- Preserve YAML comments if possible (nice to have)

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 2: Core Read Commands ⏳

**Status:** ⏳ Pending
**Goal:** Implement read-only commands (list, info) with dual output modes

**Dependencies:** Phase 1

**Parallel Execution:** Tasks 2.1 and 2.2 can run in parallel after Phase 1

### Task 2.1: Implement `overlord list` Command ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Implement the list command with filtering options and dual output modes (human-friendly table and JSON for agents).

**Context:**
Command specification:
```bash
overlord list [options]

Options:
  --json               # JSON output for programmatic use
  --category=<cat>     # Filter by category
  --tag=<tag>          # Filter by tag (can repeat)
  --archived           # Include archived projects
  --all                # Include everything
```

Default behavior: List active projects, exclude archived.

Human-friendly output should use colors/formatting (Lipgloss).
JSON output should be clean array of project objects.

**References:**
- Lipgloss for styling: https://github.com/charmbracelet/lipgloss
- Task 1.2 types for Project structure

**Success Criteria:**
- [ ] `cmd/overlord/list.go` with Cobra command
- [ ] Filters work correctly (category, tag, archived)
- [ ] Human output: formatted table with colors (name, category, description, status)
- [ ] JSON output: clean array with `--json` flag
- [ ] No output if no projects match filters
- [ ] `overlord` with no subcommand defaults to `overlord list`

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Table should be readable in 80-column terminal
- Truncate long descriptions in table view
- JSON output should not include ANSI escape codes

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 2.2: Implement `overlord info` Command ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement the info command to show detailed information about a single project, including thoughts directory paths.

**Context:**
Command specification:
```bash
overlord info <name> [--json]
```

Human output format:
```
Project: dispatch
Path: /home/thomas/Work/core/agents/dispatch
Category: core-agents
Language: go
Description: CLI for orchestrating AI agents across projects
Tags: go, cli, agents
Status: active
Created: 2026-01-21
Aliases: -

Thoughts:
  Plans: ~/thoughts/plans/dispatch/
  Logs: ~/thoughts/logs/dispatch/
  Sessions: ~/thoughts/sessions/dispatch/
```

**References:**
- Task 1.2 types for Project structure
- Thoughts directory structure from filesystem-design.md

**Success Criteria:**
- [ ] `cmd/overlord/info.go` with Cobra command
- [ ] Resolves project by name or alias
- [ ] Shows all project fields in formatted output
- [ ] Shows thoughts directory paths (whether they exist or not)
- [ ] `--json` flag outputs structured JSON
- [ ] Error message if project not found

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Use same styling approach as list command
- Thoughts paths should use `~` shorthand in human output

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 3: Template System ⏳

**Status:** ⏳ Pending
**Goal:** Create embedded template system for project initialization

**Dependencies:** Phase 1

**Parallel Execution:** Task 3.1 must complete before 3.2

### Task 3.1: Create Template Embedding System ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Create the template embedding system using Go's embed package. Templates include .tmux.local, Makefile, AGENTS.md, README.md, and .opencode/opencode.jsonc for each language.

**Context:**
Languages to support: Python, TypeScript, Go, Solidity, base (fallback)

Templates needed per language:
- `Makefile` - Language-specific build commands
- `.tmux.local` - Workspace layout (can be same for all)
- `AGENTS.md` - Agent instructions (base + language-specific sections)
- `README.md` - Basic project readme template
- `.opencode/opencode.jsonc` - OpenCode configuration

Makefiles should have consistent commands across languages:
- `make install` - Install dependencies
- `make build` - Build project
- `make test` - Run tests
- `make lint` - Run linter
- `make format` - Format code
- `make clean` - Clean build artifacts

Remove worktree-related commands from Makefiles (not needed).

**References:**
- Existing v1 templates at `/home/thomas/.config/overlord/templates/`, `makefiles/`, `tmux/`, `agents/`
- Go embed documentation: https://pkg.go.dev/embed

**Success Criteria:**
- [ ] `internal/templates/` directory with embedded files
- [ ] `templates/` source directory with all template files
- [ ] Go embed directives to include templates in binary
- [ ] Function to get template by language and type
- [ ] Template rendering with variable substitution (project name, date, etc.)
- [ ] Makefiles have consistent commands, no worktree stuff

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Templates must be embedded (no external file dependencies)
- Use Go's text/template for variable substitution
- Keep templates simple and maintainable

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 3.2: Create Language-Specific Templates ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Create the actual template content for each supported language, based on existing v1 templates but simplified and updated for the new system.

**Context:**
For each language (python, typescript, go, solidity, base):

**Makefile commands to include:**
- Python: uses `uv`, `ruff`, `pytest`
- TypeScript: uses `pnpm` or `bun`, `eslint`
- Go: uses standard `go` commands
- Solidity: uses `forge` (Foundry)
- Base: minimal, just clean target

**.tmux.local:**
- Simple layout suitable for agent and human use
- Main editor pane, terminal pane
- Don't overcomplicate with 3-pane setup

**AGENTS.md:**
- Project-specific agent instructions
- Include Makefile command reference
- Include project structure hints

**References:**
- Existing v1 templates at `/home/thomas/.config/overlord/`
- Task 3.1 template system (depends on completion)

**Success Criteria:**
- [ ] Python templates: Makefile (uv/ruff/pytest), AGENTS.md
- [ ] TypeScript templates: Makefile (pnpm/eslint), AGENTS.md
- [ ] Go templates: Makefile (go build/test/vet), AGENTS.md
- [ ] Solidity templates: Makefile (forge), AGENTS.md
- [ ] Base templates: Minimal Makefile, generic AGENTS.md
- [ ] Shared: .tmux.local, README.md template, .opencode/opencode.jsonc
- [ ] All templates render correctly with test project

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Keep templates concise - agents will read these
- Makefile commands should be self-documenting (use help target)
- Don't include v1 worktree commands

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 4: Project Management Commands ⏳

**Status:** ⏳ Pending
**Goal:** Implement commands for creating, registering, and removing projects

**Dependencies:** Phase 1, Phase 3

**Parallel Execution:** Tasks 4.2, 4.3, 4.4 can run in parallel after 4.1

### Task 4.1: Implement `overlord new` Command ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Implement the new command to create a new project with full setup including directory creation, git init, templates, thoughts integration, and registry update.

**Context:**
Command specification:
```bash
overlord new <name> [options]

Options:
  --category=<cat>     # Category (required or prompted)
  --py|--ts|--go|--sol # Language shorthand
  --description="..."  # Project description
  --no-git             # Skip git initialization
  --no-open            # Don't open workspace after creation
```

On creation:
1. Validate name (no spaces, valid characters)
2. Create directory at `~/Work/{category-path}/{name}/`
3. Initialize git repository (unless --no-git)
4. Create templates: .tmux.local, Makefile, .opencode/opencode.jsonc, AGENTS.md, README.md
5. Create global thoughts structure:
   - `~/thoughts/projects/{name}/` with symlinks to README.md and AGENTS.md
   - `~/thoughts/plans/{name}/`
   - `~/thoughts/logs/{name}/`
   - `~/thoughts/sessions/{name}/`
6. Register in registry
7. Open workspace (unless --no-open)

**References:**
- Task 1.2 for registry types
- Task 1.3 for registry operations
- Task 3.1/3.2 for templates
- Filesystem design: /home/thomas/.config/opencode/dev/filesystem-design.md

**Success Criteria:**
- [ ] `cmd/overlord/new.go` with Cobra command
- [ ] Creates correct directory structure based on category
- [ ] Initializes git with initial commit
- [ ] Renders and writes all templates
- [ ] Creates thoughts directories with correct symlinks
- [ ] Registers project in registry
- [ ] Opens tmux workspace (calls tmux with .tmux.local)
- [ ] Handles errors gracefully (cleanup on failure)

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Must create parent directories if they don't exist
- Symlinks must be relative, not absolute (for portability)
- Git init should not fail if git is not installed (warn only)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 4.2: Implement `overlord add` Command ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement the add command to register an existing project directory in the registry.

**Context:**
Command specification:
```bash
overlord add <name> <path> [options]

Options:
  --category=<cat>     # Category (auto-detect or specify)
  --py|--ts|--go|--sol # Language (auto-detect or specify)
  --description="..."  # Project description
  --alias=<alias>      # Add alias
```

Should:
1. Validate path exists and is a directory
2. Auto-detect language from files (pyproject.toml, package.json, go.mod, foundry.toml)
3. Auto-detect category from path if in standard location
4. Create thoughts structure (same as new)
5. Register in registry

**References:**
- Language detection logic from v1: `/home/thomas/.config/overlord/AGENTS.md` (detect_language function)
- Task 4.1 for thoughts creation logic

**Success Criteria:**
- [ ] `cmd/overlord/add.go` with Cobra command
- [ ] Validates path exists
- [ ] Auto-detects language from project files
- [ ] Creates thoughts directories with symlinks
- [ ] Registers project in registry
- [ ] Error if project name already exists

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Do not modify existing project files
- If README.md or AGENTS.md don't exist, create placeholder symlinks anyway (will be broken but ready)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 4.3: Implement `overlord rm` Command ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement the rm command to remove a project from the registry (not from disk by default).

**Context:**
Command specification:
```bash
overlord rm <name> [--force]
```

Default: Remove from registry only, project directory remains.
With --force: Also delete project directory (with confirmation prompt).

Thoughts directories should remain (orphaned but preserved for history).

**References:**
- Task 1.3 for registry operations

**Success Criteria:**
- [ ] `cmd/overlord/rm.go` with Cobra command
- [ ] Removes project from registry
- [ ] Does not touch disk by default
- [ ] `--force` deletes directory with confirmation prompt
- [ ] Error if project not found
- [ ] Success message indicating what was done

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Never delete without explicit confirmation for --force
- Thoughts directories are never deleted

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 4.4: Implement Project Name Resolution with Fuzzy Matching ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement a shared utility for resolving project names that supports exact match, alias match, and fuzzy matching.

**Context:**
Resolution priority:
1. Exact project name match
2. Exact alias match
3. Fuzzy match on name and aliases

This utility will be used by: info, rm, open, archive, unarchive commands.

**References:**
- Fuzzy matching library: https://github.com/sahilm/fuzzy
- Task 1.2 for Project types

**Success Criteria:**
- [ ] `internal/registry/resolve.go` with resolution functions
- [ ] Exact match returns single project
- [ ] Alias match returns single project
- [ ] Fuzzy match returns ranked list of candidates
- [ ] Function to get single best match (for non-interactive use)
- [ ] Function to get all matches above threshold (for interactive picker)

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Fuzzy matching should be fast (cached index if needed)
- Should handle empty registry gracefully

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 5: Project Opening ⏳

**Status:** ⏳ Pending
**Goal:** Implement interactive project opening with fuzzy finder

**Dependencies:** Phase 4 (specifically Task 4.4)

**Parallel Execution:** None (single complex task)

### Task 5.1: Implement `overlord open` Command with Interactive Picker ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Implement the open command that opens a project workspace in tmux. When multiple matches exist, show an interactive fuzzy finder (fzf-style).

**Context:**
Command specification:
```bash
overlord <name>           # Shorthand
overlord open <name>      # Explicit
overlord open             # Interactive picker if no name
```

Behavior:
- Exact match: open immediately
- Multiple matches: show interactive picker
- No name given: show picker with all active projects

Opening a project:
1. Check if tmux session exists for project
2. If exists, attach to it
3. If not, create new session using .tmux.local or default layout
4. cd to project directory

**References:**
- Bubbletea for TUI: https://github.com/charmbracelet/bubbletea
- Bubbles (components): https://github.com/charmbracelet/bubbles (list, textinput)
- Task 4.4 for fuzzy matching

**Success Criteria:**
- [ ] `cmd/overlord/open.go` with Cobra command
- [ ] Root command (no subcommand) triggers open behavior
- [ ] Exact match opens project immediately
- [ ] Interactive fuzzy finder for multiple matches (Bubbletea)
- [ ] Picker shows: name, category, description
- [ ] Opens/attaches tmux session correctly
- [ ] Error if archived project selected (suggest unarchive)

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Picker must be keyboard navigable (j/k or arrows)
- Typing filters the list in real-time
- Enter selects, Escape cancels
- Must work in both interactive and piped contexts

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 6: Archive Commands and Polish ⏳

**Status:** ⏳ Pending
**Goal:** Implement archive/unarchive and polish CLI output

**Dependencies:** Phase 4, Phase 5

**Parallel Execution:** Tasks 6.1 and 6.2 can run in parallel; 6.3 after both

### Task 6.1: Implement `overlord archive` Command ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement the archive command to move a project to the archive directory and update its status.

**Context:**
Command specification:
```bash
overlord archive <name>
```

On archive:
1. Move project directory to `~/Work/archive/{name}/`
2. Update registry: path and status.state = "archived"
3. Keep symlinks in thoughts (may become broken, that's ok)
4. Print confirmation

**References:**
- Task 1.3 for registry operations
- Filesystem design for archive location

**Success Criteria:**
- [ ] `cmd/overlord/archive.go` with Cobra command
- [ ] Moves project directory to archive location
- [ ] Updates registry with new path and archived state
- [ ] Handles case where project is already archived
- [ ] Error if project not found

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Use os.Rename for move (same filesystem)
- If cross-filesystem, copy then delete
- Confirm before archiving

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 6.2: Implement `overlord unarchive` Command ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Implement the unarchive command to restore a project from archive to an active category.

**Context:**
Command specification:
```bash
overlord unarchive <name> --category=<cat>
```

On unarchive:
1. Prompt for category if not provided (interactive picker)
2. Move project from `~/Work/archive/{name}/` to `~/Work/{category-path}/{name}/`
3. Update registry: path and status.state = "active"
4. Update symlinks in thoughts to point to new location
5. Print confirmation

**References:**
- Task 6.1 for archive implementation
- Task 1.2 for category mappings

**Success Criteria:**
- [ ] `cmd/overlord/unarchive.go` with Cobra command
- [ ] Prompts for category if not provided
- [ ] Moves project to correct category path
- [ ] Updates registry with new path and active state
- [ ] Updates thoughts symlinks
- [ ] Error if project not in archive

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Category is required (prompt if not given)
- Validate category is valid before moving

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 6.3: Polish CLI Output and Add Help Text ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Polish all CLI output with consistent styling, improve help text, and ensure good UX across all commands.

**Context:**
- Use Lipgloss for consistent colors and styling
- All commands should have clear, helpful `--help` output
- Error messages should be actionable
- Success messages should confirm what was done

**References:**
- Lipgloss styling: https://github.com/charmbracelet/lipgloss
- All previously implemented commands

**Success Criteria:**
- [ ] Consistent color scheme across commands
- [ ] All commands have descriptive help text
- [ ] Examples in help text for complex commands
- [ ] Error messages include suggested fixes
- [ ] Success messages are clear and concise
- [ ] `overlord --help` shows command overview

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Colors should work on light and dark terminals
- Output should degrade gracefully if terminal doesn't support colors

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Execution Notes

### Parallelization Opportunities
- **Phase 1:** Tasks 1.1 and 1.2 can run in parallel (no dependencies between them)
- **Phase 2:** Tasks 2.1 and 2.2 can run in parallel after Phase 1
- **Phase 4:** Tasks 4.2, 4.3, 4.4 can run in parallel after 4.1
- **Phase 6:** Tasks 6.1 and 6.2 can run in parallel

### Critical Path
1. Task 1.1 (project setup) → Task 1.3 (registry ops) → Task 4.1 (new command)
2. Task 3.1 (template system) → Task 3.2 (template content) → Task 4.1 (new command)
3. Task 4.4 (fuzzy matching) → Task 5.1 (open command)

The longest path is: 1.1 → 1.2 → 1.3 → 3.1 → 3.2 → 4.1 → 5.1

### Risk Points
- **Task 3.2 (Templates):** May need iteration to get templates right for agent use
- **Task 5.1 (Interactive Picker):** Bubbletea has a learning curve, may take longer
- **Task 4.1 (overlord new):** Most complex task, many integration points

### Review Checkpoints
- After Phase 1 completion (foundation solid before building on it)
- After Phase 3 completion (templates correct before using in new command)
- After Phase 4 completion (core functionality complete)
- Final review after Phase 6

### Documentation Checkpoints
- After Phase 1 review passes (document basic usage)
- After Phase 4 review passes (document all commands)
- After Phase 6 review passes (final documentation)

---

## Task Summary Table

| Phase | Task | Name | Subagent | Dependencies | Parallel? | Status |
|-------|------|------|----------|--------------|-----------|--------|
| 1 | 1.1 | Initialize Go Project with Cobra CLI | worker | none | yes (with 1.2) | ⏳ |
| 1 | 1.2 | Define Core Types and Registry Schema | worker | none | yes (with 1.1) | ⏳ |
| 1 | 1.3 | Implement YAML Registry Read/Write | worker | 1.1, 1.2 | no | ⏳ |
| 2 | 2.1 | Implement `overlord list` Command | executor | Phase 1 | yes (with 2.2) | ⏳ |
| 2 | 2.2 | Implement `overlord info` Command | worker | Phase 1 | yes (with 2.1) | ⏳ |
| 3 | 3.1 | Create Template Embedding System | executor | Phase 1 | no | ⏳ |
| 3 | 3.2 | Create Language-Specific Templates | executor | 3.1 | no | ⏳ |
| 4 | 4.1 | Implement `overlord new` Command | executor | Phase 1, Phase 3 | no | ⏳ |
| 4 | 4.2 | Implement `overlord add` Command | worker | 4.1 | yes (with 4.3, 4.4) | ⏳ |
| 4 | 4.3 | Implement `overlord rm` Command | worker | 4.1 | yes (with 4.2, 4.4) | ⏳ |
| 4 | 4.4 | Implement Fuzzy Matching Utility | worker | Phase 1 | yes (with 4.2, 4.3) | ⏳ |
| 5 | 5.1 | Implement `overlord open` with Picker | executor | 4.4 | no | ⏳ |
| 6 | 6.1 | Implement `overlord archive` Command | worker | Phase 4 | yes (with 6.2) | ⏳ |
| 6 | 6.2 | Implement `overlord unarchive` Command | worker | Phase 4 | yes (with 6.1) | ⏳ |
| 6 | 6.3 | Polish CLI Output and Help Text | worker | 6.1, 6.2 | no | ⏳ |

---

## Execution Status

_This section is updated by the orchestrator during execution._

**Last Updated:** not started
**Current Phase:** -
**Current Task:** -

### Progress
- Phases complete: 0 of 6
- Tasks complete: 0 of 16

### Divergences from Plan
_(none yet)_

### Handoff History
_(none)_

---

## Appendix: Key Decisions Reference

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | Single binary, excellent CLI libs, good for structured data |
| Registry format | YAML | More token-efficient than JSON, human-readable |
| Template storage | Embedded in binary | Simpler deployment, versioned with code |
| Template organization | Language-based | Build tools determined by language, not category |
| Fuzzy finder | Interactive (fzf-style) | Better UX when multiple matches |
| Installation | XDG compliant | `~/.local/bin/` + `~/.config/overlord/` |
| Archive commands | `archive`/`unarchive` | Clearer than overloaded `mv` command |

## Appendix: Deferred Items

These items are explicitly out of scope for this plan:
- `overlord search` command
- `overlord tag` / `overlord describe` commands
- `overlord sync` command
- Migration commands from v1
- Agent-specific documentation format (starting with JSON)
- Worktree management (separate `~/Worktrees/` directory idea)

## Appendix: Future Considerations

**Noted for future iterations:**
- Worktree directory structure (`~/Worktrees/` mirroring `~/Work/`)
- Agent-optimized output format (alternative to JSON)
- Tmux template refinement for agent-first workflows
- Shell completions (bash, zsh, fish)

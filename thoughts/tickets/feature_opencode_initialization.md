---
type: feature
priority: medium
created: 2025-12-16T00:00:00Z
created_by: Opus
status: implemented
tags: [overlord, initialization, opencode, project-setup]
keywords: [overlord-new, overlord-init, overlord-sync, opencode.jsonc, thoughts directory, templates]
patterns: [template generation, directory creation, file initialization, language-specific configuration]
---

# FEATURE-001: Add OpenCode Configuration and Thoughts Directory to Project Initialization

## Description
Enhance overlord project initialization to automatically create an `opencode.jsonc` configuration file and a standardized `thoughts/` directory structure when creating or initializing projects. This provides a consistent development environment setup across all overlord-managed projects.

## Context
Currently, overlord creates projects with language-specific tooling (uv, pnpm, forge) and tmux configuration. To improve AI-assisted development workflows, projects need:
1. An `opencode.jsonc` file that references language-specific coding guidelines
2. A `thoughts/` directory structure for organizing development artifacts (tickets, plans, logs, research, handoffs)

This enhancement integrates with the existing project initialization flow where `.tmux.local` and other setup files are created.

## Requirements

### Functional Requirements
- Create `opencode.jsonc` in project root during `overlord new` and `overlord init`
- Create `thoughts/` directory structure with 5 subdirectories: `tickets/`, `plans/`, `logs/`, `research/`, `handoffs/`
- Generate language-specific instruction file references in `opencode.jsonc`:
  - **Python projects**: `["~/.config/opencode/CODING.md", "~/.config/opencode/PYTHON_STYLEGUIDE.md"]`
  - **TypeScript projects**: `["~/.config/opencode/CODING.md", "~/.config/opencode/TYPESCRIPT_STYLEGUIDE.md"]`
  - **Solidity projects**: `["~/.config/opencode/CODING.md", "~/.config/opencode/SOLIDITY_STYLEGUIDE.md"]`
- Store `opencode.jsonc` templates in `~/.config/overlord/templates/` (similar to tmux templates)
- Skip creation if `opencode.jsonc` or `thoughts/` already exists (with warning message)
- Extend `overlord sync` to propagate `opencode.jsonc` and `thoughts/` to all projects
- Output logging messages when creating files/directories

### Non-Functional Requirements
- No validation of instruction file paths (create regardless of whether files exist)
- Leave partial state if `thoughts/` subdirectory creation fails
- Maintain consistency with existing overlord initialization patterns
- All subdirectories in `thoughts/` should be empty (no template files or READMEs)

## Current State
- `overlord new` creates projects with language tooling and `.tmux.local`
- `overlord init` exists and can initialize existing projects
- `overlord sync` propagates Makefiles to projects
- No `opencode.jsonc` or `thoughts/` directory structure exists

## Desired State
- `overlord new` creates projects with `opencode.jsonc` and `thoughts/` directory
- `overlord init` can add `opencode.jsonc` and `thoughts/` to existing projects
- `overlord sync` ensures all projects have current `opencode.jsonc` template and `thoughts/` structure
- Consistent logging output for file/directory creation

## Research Context

### Keywords to Search
- `overlord-new` - Main project creation script
- `overlord-init` - Project initialization script (may need documentation update)
- `overlord-sync` - Makefile propagation script (needs extension)
- `.tmux.local` - Existing file creation pattern to model after
- `~/.config/overlord/templates/` - Template storage location
- `python.tmux`, `typescript.tmux`, `solidity.tmux` - Language-specific template examples

### Patterns to Investigate
- Template file generation based on language type
- Directory creation with error handling
- File existence checking and skip logic
- Output/logging patterns in overlord commands
- How language type is determined in `overlord new`
- How `overlord sync` iterates through projects

### Key Decisions Made
- **Location**: `opencode.jsonc` in project root (not `.opencode/` folder)
- **Template storage**: Use `~/.config/overlord/templates/` directory
- **Instruction paths**: Always point to `~/.config/opencode/` (hardcoded)
- **Overwrite behavior**: Skip if exists, show warning, don't overwrite
- **Error handling**: Leave partial state if directory creation fails
- **Sync integration**: Extend `overlord sync` to handle these files
- **No migration**: One-time migration script already ran, no need to update it
- **No comments**: Keep `opencode.jsonc` minimal without explanatory comments
- **Languages**: Python, TypeScript, Solidity only (no C++)

## Implementation Notes

### Template Format
The `opencode.jsonc` template should follow this structure:
```jsonc
{
  "instructions": ["~/.config/opencode/CODING.md", "~/.config/opencode/<LANGUAGE>_STYLEGUIDE.md"]
}
```

### Directory Structure to Create
```
<project-root>/
├── opencode.jsonc
└── thoughts/
    ├── tickets/
    ├── plans/
    ├── logs/
    ├── research/
    └── handoffs/
```

### Commands to Modify
1. **overlord-new**: Add opencode.jsonc and thoughts/ creation after .tmux.local setup
2. **overlord-init**: Add opencode.jsonc and thoughts/ creation logic
3. **overlord-sync**: Extend to check/create opencode.jsonc and thoughts/ in all projects

### Template Files to Create
- `~/.config/overlord/templates/opencode-python.jsonc`
- `~/.config/overlord/templates/opencode-typescript.jsonc`
- `~/.config/overlord/templates/opencode-solidity.jsonc`

## Success Criteria

### Automated Verification
- [ ] `overlord new myproject --py` creates `opencode.jsonc` with Python instructions
- [ ] `overlord new myproject --ts` creates `opencode.jsonc` with TypeScript instructions
- [ ] `overlord new myproject --sol` creates `opencode.jsonc` with Solidity instructions
- [ ] All projects get `thoughts/` directory with 5 subdirectories
- [ ] Running `overlord new` twice skips existing `opencode.jsonc` with warning
- [ ] Running `overlord new` twice skips existing `thoughts/` with warning
- [ ] `overlord init` adds missing `opencode.jsonc` to existing projects
- [ ] `overlord sync` propagates `opencode.jsonc` and `thoughts/` to all projects
- [ ] Logging output shows what was created

### Manual Verification
- [ ] Created `opencode.jsonc` has correct language-specific instructions
- [ ] `thoughts/` subdirectories are empty
- [ ] Templates exist in `~/.config/overlord/templates/`
- [ ] Warning messages appear when skipping existing files
- [ ] Partial directory creation doesn't crash the script

## Related Information
- Related to existing tmux template system
- Follows overlord's status-based project organization (active/lib/archive)
- Integrates with existing Makefile sync mechanism

## Notes
- The `overlord init` command may need documentation updates in AGENTS.md
- Consider whether `.gitignore` should exclude parts of `thoughts/` directory (out of scope for this ticket)
- Users can manually edit `opencode.jsonc` after creation to customize instruction files

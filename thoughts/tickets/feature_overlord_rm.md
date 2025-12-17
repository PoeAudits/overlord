---
type: feature
priority: medium
created: 2025-12-16T00:00:00Z
status: implemented
tags: [cli, registry, project-management]
keywords: [overlord-rm, remove, unregister, registry.json, jq delete]
patterns: [confirmation prompt, jq deletion, lookup by name/path/alias]
---

# FEATURE-002: Add `overlord rm` Command to Remove Projects from Registry

## Description
Create a new `overlord rm` command that removes projects from the Overlord registry without deleting the actual project files. This provides a safe way to untrack projects that are no longer needed in the workspace system.

## Context
As users add more projects to the registry, they need a way to remove entries for projects they no longer want to manage through Overlord. This is purely a registry operation - the actual files remain untouched. This is distinct from `overlord mv <name> archive` which keeps the project in the registry but changes its status.

## Requirements

### Functional Requirements
- Create new script `overlord-rm` in `~/bin/overlord/`
- Command syntax: `overlord rm <identifier> [options]`
- Support lookup by:
  - Project name (primary)
  - Project path (absolute path)
  - Project alias
- Require confirmation before removal showing:
  - Project name
  - Project path
  - Current status (active/lib/archive)
  - Language
- `--force` / `-f` flag to skip confirmation
- Remove entry from `registry.json` using jq
- Update main `overlord` dispatcher to route `rm` subcommand
- Does NOT delete any files on disk
- Does NOT check if project is open in tmux (removal is safe regardless)

### Non-Functional Requirements
- Follow existing code patterns from other overlord commands
- Use same color scheme and logging functions
- Clear error messages if project not found
- Support `--help` flag

## Current State
- No way to remove projects from registry
- Only option is to manually edit `registry.json` via `overlord config`

## Desired State
- `overlord rm myproject` prompts for confirmation then removes
- `overlord rm /path/to/project` removes by path lookup
- `overlord rm myalias` removes by alias lookup
- `overlord rm myproject -f` removes without confirmation

## Research Context

### Keywords to Search
- `OVERLORD_REGISTRY` - Registry file path
- `jq` - JSON manipulation patterns
- `.projects` - Registry structure
- `aliases` - Alias lookup in registry

### Patterns to Investigate
- Registry reading in `overlord-list` lines 118-149
- jq manipulation in `overlord-new` lines 292-308
- Project lookup patterns (by name, need to add path/alias lookup)
- Confirmation prompts in bash (read -p)

### Key Decisions Made
- Registry-only operation (never touches files)
- No `--all` flag for bulk removal
- No archive option (use `overlord mv` for that)
- No guidance message after removal
- Supports name, path, and alias lookup
- Confirmation required by default (shows project details)

## Success Criteria

### Automated Verification
- [ ] `overlord rm --help` shows usage
- [ ] `overlord rm nonexistent` shows error "project not found"
- [ ] `overlord rm testproj` prompts for confirmation
- [ ] `overlord rm testproj -f` removes without prompt
- [ ] Project no longer appears in `overlord list --all` after removal
- [ ] Removal by path works
- [ ] Removal by alias works

### Manual Verification
- [ ] Confirmation shows name, path, status, language
- [ ] Actual project directory still exists after removal
- [ ] Can re-add removed project with `overlord add`

## Related Information
- Reference: `overlord-mv` for registry manipulation patterns
- Reference: `overlord-info` for project lookup patterns
- Inverse of: `overlord add` command

## Notes
- Lookup order when identifier matches multiple: name > alias > path (or error if ambiguous)
- Consider showing "Did you mean?" suggestions if no exact match found

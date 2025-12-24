---
type: feature
priority: medium
created: 2025-12-23T00:00:00Z
status: reviewed
tags: [cli, sync, file-management, overlord-command]
keywords: overlord sync, selective sync, makefile sync, tmux sync, opencode sync
patterns: overlord-sync, sync_project, generate_makefile, copy_tmux_template, copy_opencode_template
---
# FEATURE-001: Add selective file sync to overlord command

## Description
Enhance the `overlord sync` command to support selective file syncing and more granular project targeting. Currently, the command syncs all projects with all files (Makefile, opencode.jsonc) and has no way to sync specific file types or target specific projects.

## Context
Users need more control over what gets synced to their projects. Common use cases:
- Only syncing Makefile changes after template updates
- Re-syncing tmux configuration without touching other files
- Targeting a specific project rather than all projects
- Syncing all projects of a specific language

Current behavior syncs all files to all projects, which is inefficient and potentially destructive when users only want specific updates.

## Requirements

### Functional Requirements
- Add `--makefile` flag to sync only Makefile
- Add `--opencode` flag to sync only opencode.jsonc
- Add `--tmux` flag to sync only .tmux.local
- Add `--all` flag to sync all registered projects
- Require either `<name>` positional argument or `--all` flag (default to no-ops)
- Support multiple project names: `overlord sync proj1 proj2 proj3`
- Support project aliases (resolve via registry)
- Support `.` for current directory (verify it's registered)
- Language flags (`--py`, `--ts`, `--sol`) filter `--all` targets
- Language flags without `--all` should warn and be ignored
- `--force` flag required to overwrite existing files for all sync types
- Multiple file flags combine: `--makefile --opencode` syncs both
- If no file flags specified, sync all types (current default behavior)
- `.tmux.local` sync uses existing `copy_tmux_template()` from common.sh
- `thoughts/` directory remains always additive (no change)

### Non-Functional Requirements
- Maintain backward compatibility where reasonable
- Preserve existing dry-run behavior for all new flags
- Follow existing overlord argument parsing patterns
- Exit with clear error messages for invalid combinations
- Support all existing language types: python, typescript, solidity, base

## Current State
```bash
# Current usage (syncs all projects, all files)
overlord sync              # Sync all projects
overlord sync --py         # Sync all Python projects
overlord sync --force      # Overwrite all files
```

Existing sync targets:
- Makefile (from base.mk + language-specific .mk)
- opencode.jsonc (from templates/)
- thoughts/ directory structure
- Legacy opencode.jsonc migration

## Desired State
```bash
# New usage patterns
overlord sync myproject                        # Sync all files for myproject
overlord sync myproject --makefile              # Sync only Makefile
overlord sync proj1 proj2 --opencode           # Sync opencode.jsonc for two projects
overlord sync . --tmux                         # Sync .tmux.local for current project
overlord sync --all                            # Sync all files for all projects
overlord sync --all --py --makefile           # Sync Makefiles for all Python projects
overlord sync --all --ts --tmux --force       # Sync .tmux.local for all TS projects (overwrite)
overlord sync myproject --py                   # Warning: --py ignored without --all
```

## Research Context

### Keywords to Search
- overlord sync - Current implementation patterns
- parse_args - Argument parsing patterns in overlord commands
- positional arguments - How other commands handle required args
- find_project_exact - Project lookup from common.sh
- validate_path - Path validation patterns

### Patterns to Investigate
- `while [[ $# -gt 0 ]]` - Argument parsing loops
- `jq -r --arg` - Registry query patterns for single project
- `if [[ -n "$FILTER_LANG" ]]` - Language filtering logic
- `OVERLORD_STRICT=true` - Strict mode patterns

### Key Decisions Made
- Default changed from sync-all to explicit targeting - Prevents accidental bulk syncs
- Language flags warn without --all - Clear user feedback rather than silent ignore
- .tmux.local sync added - Completes the trio of core config files
- thoughts/ remains always additive - No need to complicate with flags

## Success Criteria

### Automated Verification
- [ ] `overlord sync myproject --makefile` syncs only Makefile
- [ ] `overlord sync --all --py` syncs only Python projects
- [ ] `overlord sync proj1 proj2` syncs both projects
- [ ] `overlord sync .` works when current dir is registered
- [ ] `overlord sync .` errors when current dir not registered
- [ ] `overlord sync myproject --py` shows warning, continues
- [ ] `overlord sync --makefile --opencode` syncs both file types
- [ ] `overlord sync --all --tmux --force` overwrites .tmux.local
- [ ] `overlord sync --all --tmux` creates .tmux.local if missing
- [ ] `overlord sync --all --tmux` skips existing .tmux.local without --force

### Manual Verification
- [ ] Dry-run shows correct files for all flag combinations
- [ ] Alias resolution works for project names
- [ ] Error messages are clear for invalid combinations
- [ ] thoughts/ directory always created when syncing
- [ ] Legacy opencode.jsonc migration still works

## Related Information
- AGENTS.md: Overlord command reference
- lib/common.sh: Shared helper functions
- overlord-sync: Current implementation

## Notes
- When implementing, consider reusing existing `find_project_exact()` for alias support
- The `copy_tmux_template()` function already exists in common.sh, just need to call it
- Language filter logic needs to work for both --all and single project (warn if single)
- Verify path exists before syncing (already done in current implementation)
- Consider adding verbose flag to show what was skipped vs synced

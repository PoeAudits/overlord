---
type: feature
priority: high
created: 2025-12-16T00:00:00Z
status: archived
tags: [worktree, tmux, automation, makefile, base.mk]
keywords: [worktree, tmux, base.mk, session management, counter-based naming, overlord sync]
patterns: [makefile targets, tmux session creation, git worktree, cross-session commands]
---

# FEATURE-006: Worktree + Tmux Integration in base.mk

## Description
Extend Overlord's `base.mk` to integrate tmux session management with git worktrees. When a worktree is created, it will automatically create a corresponding tmux session. Enable cross-session communication between the main repository and worktree sessions.

## Context
Currently, git worktrees and tmux sessions are managed separately. Developers must manually create tmux sessions after creating worktrees and manage the relationship between them. This feature automates the entire workflow, enabling:
- Automatic tmux session creation when worktrees are created
- Counter-based deterministic naming for auto-generated worktree names
- Cross-session command execution (main repo can send commands to worktree sessions)
- Unified cleanup (removing worktree also kills tmux session)

This enhances the development workflow for working on multiple branches simultaneously with dedicated workspace sessions per worktree.

## Requirements

### Functional Requirements

#### Worktree Commands
- `make worktree-new [BRANCH=name]` - Create worktree + tmux session
  - Auto-generates name if BRANCH omitted using counter-based naming
  - Creates git worktree in `.worktrees/BRANCH`
  - Copies `.gitignore` to worktree
  - Runs `.worktree-setup.sh` if exists (abort and cleanup on failure)
  - Creates tmux session named BRANCH
  - Uses `.tmux.local` if exists, else default layout (editor/shell/git)
  
- `make worktree-list` - List all git worktrees

- `make worktree-remove BRANCH=name` - Remove worktree and kill tmux session
  - Force kills tmux session without prompting
  - Removes git worktree
  
- `make worktree-attach BRANCH=name` - Attach to worktree's tmux session
  - Auto-detects if inside tmux (switch-client vs attach)
  
- `make worktree-sessions` - List all tmux sessions for worktrees
  - Shows window count and attachment status
  
- `make worktree-setup` - Run `.worktree-setup.sh` in current directory

#### Cross-Session Commands (Main Repo → Worktree)
- `make worktree-send BRANCH=name WINDOW=win CMD="..."` - Send command to worktree session
- `make worktree-read BRANCH=name WINDOW=win` - Capture visible pane content from worktree

#### Tmux Commands (Current Session)
- `make tmux-send WINDOW=win CMD="..."` - Send command to window in current session
- `make tmux-read WINDOW=win` - Capture visible pane content from window
- `make tmux-list` - List windows in current session

#### Name Generation System
- Store counter in `.worktrees/.counter` (gitignored)
- Format: `{adjective}_{noun}_{counter:02d}`
- 20 adjectives × 20 nouns = 400 unique combinations
- Algorithm: `adjective_index = counter % 20`, `noun_index = (counter / 20) % 20`
- Examples: `swift_fix_00`, `bright_fix_01`, `swift_task_20`, etc.

Word lists:
```
ADJECTIVES: swift bright clever smooth quick clean sharp neat cool fast
            bold calm clear crisp fresh light prime ready smart steady

NOUNS: fix task work dev patch branch code build test run
       flow sync push pull draft spike probe check scan forge
```

### Non-Functional Requirements
- Replace entire `~/.config/overlord/makefiles/base.mk` file
- Maintain backward compatibility with existing `worktree-setup` command
- All commands fail gracefully with clear error messages
- Counter file is machine-local (gitignored)
- Setup failure causes complete rollback (worktree removed, branch deleted if created)
- Read mode captures visible pane content only (no full scrollback)

## Current State
- `base.mk` has basic worktree commands (new, list, remove, setup)
- No tmux integration
- No auto-naming capability
- No cross-session communication
- Worktrees and tmux sessions managed independently

## Desired State
- Single command creates both worktree and tmux session
- Auto-generated names follow deterministic pattern
- Main repo can send commands to and read from worktree sessions
- Cleanup is atomic (worktree + session removed together)
- Seamless integration with existing `.tmux.local` templates

## Research Context

### Keywords to Search
- `base.mk` - Current worktree commands
- `overlord-open` - Tmux session creation patterns (lines 140-204)
- `.tmux.local` - Tmux template usage
- `TMUX_SESSION`, `TMUX_PROJECT_DIR` - Template variables
- `overlord sync` - Command to propagate base.mk changes
- `create_worktree.sh` - Reference for name generation (lines 12-21)

### Patterns to Investigate
- Tmux session creation in `overlord-open` lines 140-204
- Template variable export and usage
- Counter file management (atomic read/increment/write)
- Error handling and rollback in Makefile
- tmux send-keys and capture-pane commands

### Key Decisions Made
- Session naming: Branch/worktree name only (no project prefix)
- Name uniqueness: Counter-based with 400 combinations
- Window targeting: Explicit `WINDOW=` required for send/read
- Files to copy: `.gitignore` only
- Missing `.tmux.local`: Use default layout (editor/shell/git)
- Cleanup: Force kill tmux session without prompting
- Read mode: Visible pane content only
- Setup failure: Remove worktree, abort, and log error
- Counter file: Gitignored (machine-local state)

## Success Criteria

### Automated Verification
- [ ] `make help` shows all new commands
- [ ] `make worktree-new` creates auto-named worktree and tmux session
- [ ] `make worktree-new BRANCH=test` creates named worktree and tmux session
- [ ] Counter increments correctly (`.worktrees/.counter` updated)
- [ ] Worktree receives `.gitignore` copy
- [ ] `.worktree-setup.sh` runs if present
- [ ] Setup failure triggers rollback
- [ ] `make worktree-sessions` lists sessions with status
- [ ] `make worktree-send` successfully sends commands
- [ ] `make worktree-read` captures pane content
- [ ] `make worktree-attach` attaches to session
- [ ] `make worktree-remove` removes both worktree and session
- [ ] `make tmux-send` and `tmux-read` work in current session
- [ ] `make tmux-list` shows windows

### Manual Verification
- [ ] Auto-generated names follow pattern (swift_fix_00, bright_fix_01, etc.)
- [ ] Tmux session uses `.tmux.local` layout if present
- [ ] Default layout (editor/shell/git) works when no `.tmux.local`
- [ ] Can work on multiple worktrees simultaneously
- [ ] Cross-session commands execute correctly
- [ ] Cleanup is complete (no orphaned sessions or worktrees)

## Related Information
- Primary file: `~/.config/overlord/makefiles/base.mk`
- Reference: `/home/thomas/bin/overlord/overlord-open` lines 140-204
- Reference: `/home/thomas/.config/overlord/tmux/*.tmux` templates
- Reference: `/home/thomas/bin/overlord/hack/create_worktree.sh` lines 12-21
- Post-implementation: Run `overlord sync` to propagate to all projects

## Implementation Details

### Files to Modify
1. `~/.config/overlord/makefiles/base.mk` - Replace entire file with new implementation

### Files to Reference (Read Only)
1. `/home/thomas/bin/overlord/overlord-open` - Lines 140-204 for tmux session creation
2. `/home/thomas/.config/overlord/tmux/python.tmux` - Template structure
3. `/home/thomas/bin/overlord/hack/create_worktree.sh` - Name generation reference

### Implementation Steps
1. Read current `base.mk` to understand existing structure
2. Read `overlord-open` lines 140-204 for tmux session creation pattern
3. Replace `base.mk` with complete implementation 
4. Ensure `.worktrees/.counter` is gitignored (check if `.worktrees/` pattern covers it)
5. Run `overlord sync` to propagate changes to all projects
6. Test in sample project (see Testing Plan below)

### Testing Plan
```bash
# 1. Sync changes to projects
overlord sync

# 2. Navigate to test project
cd ~/Work/Python/active/some-project

# 3. Test auto-named worktree creation
make worktree-new
# Expected: Creates swift_fix_00, copies .gitignore, creates tmux session

# 4. Test named worktree creation
make worktree-new BRANCH=test-feature
# Expected: Creates test-feature worktree and tmux session

# 5. Test worktree sessions listing
make worktree-sessions
# Expected: Lists both worktrees with tmux status

# 6. Test cross-session commands
make worktree-send BRANCH=swift_fix_00 WINDOW=shell CMD="echo hello"
make worktree-read BRANCH=swift_fix_00 WINDOW=shell
# Expected: "hello" appears in output

# 7. Test attach
make worktree-attach BRANCH=swift_fix_00
# Expected: Attaches to session

# 8. Test cleanup
make worktree-remove BRANCH=swift_fix_00
make worktree-remove BRANCH=test-feature
# Expected: Both worktrees and sessions removed
```

## Notes
- This is a **breaking change** to `base.mk` - replaces existing content entirely
- Must run `overlord sync` after implementation to propagate to all registered projects
- Counter file (`.worktrees/.counter`) should be added to `.gitignore` if not covered by `.worktrees/` pattern

## Future Enhancements (Not in Current Scope)
1. Worktree-specific tmux template (`.tmux.worktree`) for different layouts
2. Project-prefixed session names for global uniqueness across projects
3. Session persistence (save/restore tmux layouts)
4. Interactive worktree browser with fzf (similar to `overlord open`)

## Checklist for Implementation
- [ ] Read `~/.config/overlord/makefiles/base.mk` (current state)
- [ ] Read `/home/thomas/bin/overlord/overlord-open` lines 140-204 (tmux reference)
- [ ] Replace `base.mk` with implementation from 
- [ ] Verify `.worktrees/.counter` is gitignored
- [ ] Run `overlord sync` to propagate changes
- [ ] Test in sample project using Testing Plan
- [ ] Verify all 11 new commands work correctly
- [ ] Test error handling (setup failure, missing session, etc.)

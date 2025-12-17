# Core Worktree + Tmux Integration Implementation Plan

## Overview

Replace Overlord's `base.mk` with enhanced worktree commands that automatically create and manage tmux sessions. This enables developers to create worktrees with auto-generated names and corresponding tmux workspaces in a single command, with unified cleanup that removes both the worktree and its session.

## Current State Analysis

### Existing Implementation
- **File**: `/home/thomas/.config/overlord/makefiles/base.mk:1-60`
- Current commands: `worktree-new`, `worktree-list`, `worktree-remove`, `worktree-setup`
- Requires manual tmux session management separate from worktree creation
- No auto-naming capability (BRANCH parameter always required)
- No integration with `.tmux.local` templates
- No setup script failure handling or rollback mechanism

### Tmux Session Pattern
- **Reference**: `/home/thomas/bin/overlord/overlord-open:140-204`
- Pattern: Check existence → Create session → Apply `.tmux.local` or default layout → Attach
- Template variables: `TMUX_SESSION`, `TMUX_PROJECT_DIR`
- Modes: `override` (default) or `merge`
- Default layout: editor (nvim), shell, git windows

### Missing Infrastructure
- No `.gitignore` file exists in `/home/thomas/bin/overlord/` repository
- Counter file `.worktrees/.counter` needs to be ignored
- No name generation system currently exists

## Desired End State

After implementing this plan:

1. Developers can run `make worktree-new` without arguments to create auto-named worktrees with tmux sessions
2. Developers can run `make worktree-new BRANCH=feature` to create named worktrees with tmux sessions
3. Counter-based naming provides 400 deterministic name combinations
4. Setup script failures trigger complete rollback (worktree + branch removal)
5. Removing a worktree also kills its tmux session automatically
6. Developers can attach to or list worktree sessions easily
7. `.worktrees/.counter` is gitignored across all overlord-managed projects

### Verification:
- `make worktree-new` creates worktree with auto-generated name and tmux session
- `make worktree-sessions` shows the session with status
- `tmux ls` shows session named after worktree
- Attaching to session shows layout from `.tmux.local` or default layout
- `make worktree-remove BRANCH=name` removes both worktree and session
- Counter increments correctly in `.worktrees/.counter`
- `.gitignore` exists and excludes `.worktrees/`

## What We're NOT Doing

- Cross-session communication commands (`worktree-send`, `worktree-read`) - deferred to Plan 2
- Current session utility commands (`tmux-send`, `tmux-read`, `tmux-list`) - deferred to Plan 3
- Worktree-specific tmux templates (`.tmux.worktree`)
- Project-prefixed session names for global uniqueness
- Session persistence (save/restore tmux layouts)
- Interactive worktree browser with fzf
- Counter overflow handling (>400 worktrees)

## Implementation Approach

This plan replaces the entire `base.mk` file with a new implementation that embeds:
1. Counter management functions (read, increment, write with atomic operations)
2. Name generation logic (deterministic adjective_noun_XX pattern)
3. Tmux session creation integrated into worktree creation
4. Rollback mechanism for setup failures
5. Unified cleanup that removes both worktrees and sessions

The approach follows the existing pattern from `overlord-open` for tmux session creation, adapting it for worktree-specific paths and session names.

## Phase 1: Create .gitignore for Overlord Repository

### Overview
Create a `.gitignore` file in the overlord repository to prevent committing the `.worktrees/` directory and counter file that will be created in each project.

### Changes Required:

#### 1. Create .gitignore
**File**: `/home/thomas/bin/overlord/.gitignore`
**Action**: Create new file

```gitignore
# Worktrees directory (used for testing)
.worktrees/
```

**Rationale**: The overlord repository may be used to test worktree functionality, and the `.worktrees/` pattern covers both the directory and all contents including `.worktrees/.counter`.

### Success Criteria:

#### Automated Verification:
- [ ] `.gitignore` file exists: `test -f /home/thomas/bin/overlord/.gitignore`
- [ ] File contains `.worktrees/` pattern: `grep -q '\.worktrees/' /home/thomas/bin/overlord/.gitignore`
- [ ] Git ignores worktrees directory: `cd /home/thomas/bin/overlord && mkdir -p .worktrees && git status --porcelain | grep -qv '.worktrees'`

#### Manual Verification:
- [ ] Creating `.worktrees/` directory in overlord repo doesn't show in `git status`
- [ ] Creating `.worktrees/.counter` file doesn't show in `git status`

---

## Phase 2: Implement Counter Management System

### Overview
Add counter management functions to `base.mk` that provide atomic read/increment/write operations for the `.worktrees/.counter` file. This enables deterministic name generation.

### Changes Required:

#### 1. Add Counter Variables and Helper Functions
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After variable definitions, before targets

Add counter directory and helper shell functions:

```makefile
# Counter file for auto-generated worktree names
COUNTER_FILE := $(WORKTREE_DIR)/.counter

# Shell functions for counter management
define read_counter
$(shell [ -f $(COUNTER_FILE) ] && cat $(COUNTER_FILE) || echo 0)
endef

define increment_counter
$(shell \
	mkdir -p $(WORKTREE_DIR); \
	COUNTER=$$([ -f $(COUNTER_FILE) ] && cat $(COUNTER_FILE) || echo 0); \
	NEXT=$$((COUNTER + 1)); \
	echo $$NEXT > $(COUNTER_FILE); \
	echo $$COUNTER \
)
endef
```

**Key Design Decisions**:
- Counter starts at 0 if file doesn't exist
- `increment_counter` returns the current value, then increments for next use
- `mkdir -p` ensures counter directory exists before writing
- Counter persists across worktree operations (machine-local state)

### Success Criteria:

#### Automated Verification:
- [ ] Counter initializes to 0: Create test directory, call `increment_counter`, verify returns 0
- [ ] Counter increments: Call twice, verify returns 0 then 1
- [ ] Counter file created: Verify `.worktrees/.counter` exists after first increment
- [ ] Counter persists: Read counter, create new make process, verify same value

#### Manual Verification:
- [ ] Counter survives project restarts (not session-dependent)
- [ ] Counter file contains expected integer value when inspected

---

## Phase 3: Implement Name Generation Logic

### Overview
Add deterministic name generation using the counter value to select from predefined word lists. Implements collision detection by checking tmux sessions and appending counter if needed.

### Changes Required:

#### 1. Add Word Lists and Generation Function
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After counter functions, before targets

```makefile
# Word lists for name generation (20 adjectives × 20 nouns = 400 combinations)
ADJECTIVES := swift bright clever smooth quick clean sharp neat cool fast \
              bold calm clear crisp fresh light prime ready smart steady

NOUNS := fix task work dev patch branch code build test run \
         flow sync push pull draft spike probe check scan forge

# Generate deterministic name from counter
# Usage: $(call generate_name,counter_value)
define generate_name
$(shell \
	COUNTER=$(1); \
	ADJECTIVES="swift bright clever smooth quick clean sharp neat cool fast bold calm clear crisp fresh light prime ready smart steady"; \
	NOUNS="fix task work dev patch branch code build test run flow sync push pull draft spike probe check scan forge"; \
	ADJ_ARRAY=($$ADJECTIVES); \
	NOUN_ARRAY=($$NOUNS); \
	ADJ_IDX=$$((COUNTER % 20)); \
	NOUN_IDX=$$((COUNTER / 20 % 20)); \
	printf "%s_%s_%02d" "$${ADJ_ARRAY[$$ADJ_IDX]}" "$${NOUN_ARRAY[$$NOUN_IDX]}" $$COUNTER \
)
endef

# Check if tmux session exists
# Usage: $(call session_exists,session_name)
define session_exists
$(shell tmux has-session -t $(1) 2>/dev/null && echo 1 || echo 0)
endef

# Generate unique name (handles collisions)
# Returns: unique_name
define generate_unique_name
$(shell \
	COUNTER=$$($(increment_counter)); \
	NAME=$$($(call generate_name,$$COUNTER)); \
	while [ "$$($(call session_exists,$$NAME))" = "1" ]; do \
		NAME="$${NAME}_$$COUNTER"; \
	done; \
	echo $$NAME \
)
endef
```

**Key Design Decisions**:
- Algorithm: `adj_idx = counter % 20`, `noun_idx = (counter / 20) % 20`
- Format: `{adjective}_{noun}_{counter:02d}` (e.g., `swift_fix_00`, `bright_fix_01`)
- Collision handling: Append current counter value if session exists
- Examples: `swift_fix_00`, `swift_fix_20`, `swift_fix_40` (every 20 increments cycles adjectives)

### Success Criteria:

#### Automated Verification:
- [ ] Counter 0 generates `swift_fix_00`: Test `generate_name` with counter=0
- [ ] Counter 1 generates `bright_fix_01`: Test with counter=1
- [ ] Counter 20 generates `swift_task_20`: Test with counter=20 (noun advances)
- [ ] Collision detection works: Create session `swift_fix_00`, verify next name appends counter
- [ ] Session existence check: Create session, verify `session_exists` returns 1

#### Manual Verification:
- [ ] Generated names are human-readable and follow pattern
- [ ] Collision handling produces acceptable names (original_XX format)
- [ ] Name generation is deterministic (same counter = same name)

---

## Phase 4: Replace worktree-new with Integrated Implementation

### Overview
Replace the existing `worktree-new` target with a comprehensive implementation that handles both auto-naming and explicit naming, creates tmux sessions, runs setup scripts, and rolls back on failure.

### Changes Required:

#### 1. Replace worktree-new Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Replace existing `worktree-new` target (lines 20-36)

```makefile
# Create a new worktree with tmux session
# Usage: make worktree-new [BRANCH=<name>]
# If BRANCH omitted, generates name automatically
worktree-new:
	@# Determine branch name (auto-generate or use provided)
	@BRANCH_NAME="$(BRANCH)"; \
	if [ -z "$$BRANCH_NAME" ]; then \
		COUNTER=$$($(increment_counter)); \
		BRANCH_NAME=$$($(call generate_name,$$COUNTER)); \
		while tmux has-session -t "$$BRANCH_NAME" 2>/dev/null; do \
			BRANCH_NAME="$${BRANCH_NAME}_$$COUNTER"; \
		done; \
		echo "Auto-generated name: $$BRANCH_NAME"; \
	fi; \
	\
	WORKTREE_PATH="$(WORKTREE_DIR)/$$BRANCH_NAME"; \
	BRANCH_CREATED=0; \
	\
	mkdir -p $(WORKTREE_DIR); \
	\
	if git show-ref --verify --quiet refs/heads/$$BRANCH_NAME; then \
		echo "Branch '$$BRANCH_NAME' exists, creating worktree..."; \
		if ! git worktree add $$WORKTREE_PATH $$BRANCH_NAME 2>/dev/null; then \
			echo "Error: Failed to create worktree"; \
			exit 1; \
		fi; \
	else \
		echo "Creating new branch '$$BRANCH_NAME' and worktree..."; \
		if ! git worktree add -b $$BRANCH_NAME $$WORKTREE_PATH 2>/dev/null; then \
			echo "Error: Failed to create worktree"; \
			exit 1; \
		fi; \
		BRANCH_CREATED=1; \
	fi; \
	\
	if [ -f .gitignore ]; then \
		echo "Copying .gitignore to worktree..."; \
		cp .gitignore $$WORKTREE_PATH/; \
	fi; \
	\
	if [ -f .worktree-setup.sh ]; then \
		echo "Running .worktree-setup.sh..."; \
		chmod +x .worktree-setup.sh; \
		if ! (cd $$WORKTREE_PATH && ../.worktree-setup.sh); then \
			echo "Error: Setup script failed, rolling back..."; \
			git worktree remove --force $$WORKTREE_PATH 2>/dev/null || true; \
			if [ $$BRANCH_CREATED -eq 1 ]; then \
				git branch -D $$BRANCH_NAME 2>/dev/null || true; \
			fi; \
			exit 1; \
		fi; \
	else \
		echo "Warning: No .worktree-setup.sh found (continuing anyway)"; \
	fi; \
	\
	echo "Creating tmux session: $$BRANCH_NAME"; \
	$(MAKE) _create_worktree_session BRANCH=$$BRANCH_NAME WORKTREE_PATH=$$WORKTREE_PATH; \
	\
	echo ""; \
	echo "Worktree created successfully!"; \
	echo "  Branch: $$BRANCH_NAME"; \
	echo "  Path: $$WORKTREE_PATH"; \
	echo "  Session: $$BRANCH_NAME"; \
	echo ""; \
	echo "Attach with: make worktree-attach BRANCH=$$BRANCH_NAME"
```

#### 2. Add Internal Tmux Session Creation Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `worktree-new`, add new internal target

```makefile
# Internal: Create tmux session for worktree (called by worktree-new)
# Usage: make _create_worktree_session BRANCH=name WORKTREE_PATH=path
_create_worktree_session:
	@SESSION="$(BRANCH)"; \
	WORKTREE_DIR="$(WORKTREE_PATH)"; \
	\
	if tmux has-session -t "$$SESSION" 2>/dev/null; then \
		echo "Note: Tmux session '$$SESSION' already exists"; \
		exit 0; \
	fi; \
	\
	tmux new-session -d -s "$$SESSION" -c "$$WORKTREE_DIR" -n base; \
	\
	LOCAL_RC="$$WORKTREE_DIR/.tmux.local"; \
	MODE="override"; \
	\
	if [ -f "$$LOCAL_RC" ]; then \
		MODE_LINE=$$(grep -E '^[[:space:]]*MODE=' "$$LOCAL_RC" 2>/dev/null | head -n1 || true); \
		if [ -n "$$MODE_LINE" ]; then \
			MODE=$$(echo "$$MODE_LINE" | sed -E 's/^[[:space:]]*MODE=//; s/[[:space:]]*$$//'); \
		fi; \
	fi; \
	\
	if [ -f "$$LOCAL_RC" ] && [ "$$MODE" = "override" ]; then \
		TMUX_SESSION="$$SESSION" TMUX_PROJECT_DIR="$$WORKTREE_DIR" bash "$$LOCAL_RC"; \
	else \
		tmux rename-window -t "$${SESSION}:base" "editor"; \
		tmux send-keys -t "$${SESSION}:editor" "nvim" C-m; \
		tmux new-window -t "$$SESSION:" -n shell -c "$$WORKTREE_DIR"; \
		tmux new-window -t "$$SESSION:" -n git -c "$$WORKTREE_DIR"; \
		tmux send-keys -t "$${SESSION}:git" "git status" C-m; \
		\
		if [ -f "$$LOCAL_RC" ] && [ "$$MODE" = "merge" ]; then \
			TMUX_SESSION="$$SESSION" TMUX_PROJECT_DIR="$$WORKTREE_DIR" bash "$$LOCAL_RC"; \
		fi; \
	fi; \
	\
	tmux select-window -t "$${SESSION}:editor"
```

**Key Design Decisions**:
- Auto-naming: If no BRANCH provided, generate name and handle collisions inline
- Rollback tracking: `BRANCH_CREATED` flag determines if branch should be deleted on failure
- Setup execution: Run from worktree directory using relative path to parent script
- Tmux session: Created in worktree directory, uses branch name as session name
- Template support: Respects `.tmux.local` MODE (override/merge) same as `overlord-open`
- Missing setup: Warns but continues (not fatal)
- Setup failure: Complete rollback of worktree and branch

### Success Criteria:

#### Automated Verification:
- [ ] Auto-naming creates worktree: `make worktree-new` without BRANCH parameter succeeds
- [ ] Named creation works: `make worktree-new BRANCH=test` creates worktree
- [ ] Worktree directory exists: Check `.worktrees/BRANCH/` exists
- [ ] Git worktree registered: `git worktree list | grep BRANCH`
- [ ] Tmux session created: `tmux has-session -t BRANCH`
- [ ] Counter increments: Verify `.worktrees/.counter` increments after auto-naming
- [ ] .gitignore copied: Check `.worktrees/BRANCH/.gitignore` exists if source exists

#### Manual Verification:
- [ ] Setup script runs if present and worktree succeeds
- [ ] Setup failure triggers rollback (worktree and branch removed)
- [ ] Tmux session uses `.tmux.local` layout if present
- [ ] Default layout (editor/shell/git) created when no `.tmux.local`
- [ ] All session windows start in worktree directory (`.worktrees/BRANCH/`)
- [ ] Missing setup script shows warning but continues
- [ ] Auto-generated names follow pattern and are unique

---

## Phase 5: Update worktree-remove with Session Cleanup

### Overview
Enhance `worktree-remove` to force-kill the associated tmux session before removing the worktree, providing unified cleanup.

### Changes Required:

#### 1. Replace worktree-remove Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Replace existing `worktree-remove` target (lines 43-49)

```makefile
# Remove a worktree and kill its tmux session
# Usage: make worktree-remove BRANCH=<name>
worktree-remove:
ifndef BRANCH
	$(error BRANCH is required. Usage: make worktree-remove BRANCH=<branch-name>)
endif
	@echo "Removing worktree and tmux session: $(BRANCH)"; \
	\
	if tmux has-session -t "$(BRANCH)" 2>/dev/null; then \
		echo "Killing tmux session: $(BRANCH)"; \
		tmux kill-session -t "$(BRANCH)"; \
	else \
		echo "No tmux session found: $(BRANCH)"; \
	fi; \
	\
	if [ -d "$(WORKTREE_DIR)/$(BRANCH)" ]; then \
		echo "Removing worktree: $(BRANCH)"; \
		git worktree remove $(WORKTREE_DIR)/$(BRANCH); \
		echo "Worktree removed: $(BRANCH)"; \
	else \
		echo "Warning: Worktree directory not found: $(WORKTREE_DIR)/$(BRANCH)"; \
	fi; \
	\
	echo "Cleanup complete: $(BRANCH)"
```

**Key Design Decisions**:
- Session killed first (force, no prompt) before worktree removal
- Missing session is not fatal (just logs message)
- Missing worktree warns but continues
- Always attempts both operations (independent cleanup)

### Success Criteria:

#### Automated Verification:
- [ ] Session killed: Create worktree with session, remove, verify session gone with `tmux has-session`
- [ ] Worktree removed: Verify `.worktrees/BRANCH/` directory gone
- [ ] Git worktree unregistered: `git worktree list | grep -v BRANCH`
- [ ] Missing session handled: Remove worktree without session, verify no error
- [ ] Missing worktree handled: Try to remove non-existent worktree, verify warning

#### Manual Verification:
- [ ] Both worktree and session removed in single command
- [ ] No prompts or confirmations required (force kill)
- [ ] Works correctly when session attached (kills and detaches)
- [ ] Error messages are clear and actionable

---

## Phase 6: Add worktree-attach Command

### Overview
Add a command to attach to a worktree's tmux session, with auto-detection of whether we're already in tmux (switch-client vs attach).

### Changes Required:

#### 1. Add worktree-attach Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `worktree-remove`

```makefile
# Attach to worktree's tmux session
# Usage: make worktree-attach BRANCH=<name>
worktree-attach:
ifndef BRANCH
	$(error BRANCH is required. Usage: make worktree-attach BRANCH=<branch-name>)
endif
	@if ! tmux has-session -t "$(BRANCH)" 2>/dev/null; then \
		echo "Error: Tmux session '$(BRANCH)' does not exist"; \
		echo "Create worktree first: make worktree-new BRANCH=$(BRANCH)"; \
		exit 1; \
	fi; \
	\
	if [ -n "$$TMUX" ]; then \
		echo "Switching to session: $(BRANCH)"; \
		tmux switch-client -t "$(BRANCH)"; \
	else \
		echo "Attaching to session: $(BRANCH)"; \
		tmux attach -t "$(BRANCH)"; \
	fi
```

**Key Design Decisions**:
- Auto-detects if inside tmux via `$TMUX` environment variable
- Inside tmux: Uses `switch-client` (changes current client)
- Outside tmux: Uses `attach` (creates new client)
- Missing session: Clear error with suggestion to create worktree
- Pattern matches `overlord-open:150-154` and `overlord-open:199-203`

### Success Criteria:

#### Automated Verification:
- [ ] Validates session exists: Try to attach to non-existent session, verify error
- [ ] Error message helpful: Check error mentions creating worktree first

#### Manual Verification:
- [ ] Attaching from outside tmux works (creates new client)
- [ ] Switching from inside tmux works (changes session without nesting)
- [ ] Session opens to editor window (last selected window)
- [ ] All windows accessible and in correct worktree directory

---

## Phase 7: Add worktree-sessions Command

### Overview
Add a command to list all tmux sessions for worktrees, showing window counts and attachment status.

### Changes Required:

#### 1. Add worktree-sessions Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `worktree-attach`

```makefile
# List tmux sessions for all worktrees
# Usage: make worktree-sessions
worktree-sessions:
	@echo "Tmux sessions for worktrees:"; \
	echo ""; \
	if ! git worktree list --porcelain 2>/dev/null | grep -q '^worktree'; then \
		echo "No worktrees found"; \
		exit 0; \
	fi; \
	\
	git worktree list --porcelain | while IFS= read -r line; do \
		if echo "$$line" | grep -q '^worktree'; then \
			WORKTREE_PATH=$$(echo "$$line" | sed 's/^worktree //'); \
			BRANCH_NAME=$$(basename "$$WORKTREE_PATH"); \
			\
			if echo "$$WORKTREE_PATH" | grep -q "$(WORKTREE_DIR)"; then \
				if tmux has-session -t "$$BRANCH_NAME" 2>/dev/null; then \
					WINDOWS=$$(tmux list-windows -t "$$BRANCH_NAME" 2>/dev/null | wc -l); \
					ATTACHED=$$(tmux list-sessions 2>/dev/null | grep "^$$BRANCH_NAME:" | grep -o 'attached' || echo "detached"); \
					printf "  %-30s  %d windows  %s\n" "$$BRANCH_NAME" $$WINDOWS "$$ATTACHED"; \
				else \
					printf "  %-30s  (no session)\n" "$$BRANCH_NAME"; \
				fi; \
			fi; \
		fi; \
	done; \
	echo ""
```

**Key Design Decisions**:
- Uses `git worktree list --porcelain` for machine-readable output
- Filters to only worktrees in `.worktrees/` directory
- Shows window count using `tmux list-windows`
- Shows attachment status by parsing `tmux list-sessions`
- Formatted output: name (30 chars), window count, status

### Success Criteria:

#### Automated Verification:
- [ ] Lists existing worktree sessions: Create worktree, verify it appears in list
- [ ] Shows window count: Verify output includes window count
- [ ] Shows attachment status: Create attached and detached sessions, verify correct status
- [ ] Handles no worktrees: Run with empty `.worktrees/`, verify "No worktrees found"
- [ ] Handles no sessions: Create worktree without session, verify "(no session)" shown

#### Manual Verification:
- [ ] Output is readable and well-formatted
- [ ] Window count is accurate
- [ ] Attachment status correct for attached and detached sessions
- [ ] Only shows sessions for worktrees in `.worktrees/` (not main repo)

---

## Phase 8: Update Help Documentation

### Overview
Update the help text and .PHONY declarations to document all new commands with clear usage examples.

### Changes Required:

#### 1. Update .PHONY Declaration
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Near top of file (line 6)

```makefile
.PHONY: help worktree-new worktree-list worktree-remove worktree-setup \
        worktree-attach worktree-sessions _create_worktree_session
```

#### 2. Update help Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Replace existing `help` target (lines 12-18)

```makefile
# Show available commands
help:
	@echo "Worktree + Tmux Commands:"
	@echo ""
	@echo "Worktree Management:"
	@echo "  make worktree-new [BRANCH=<name>]     Create worktree + tmux session (auto-names if BRANCH omitted)"
	@echo "  make worktree-list                    List all worktrees"
	@echo "  make worktree-remove BRANCH=<name>    Remove worktree and kill tmux session"
	@echo "  make worktree-setup                   Run .worktree-setup.sh in current directory"
	@echo ""
	@echo "Tmux Session Management:"
	@echo "  make worktree-attach BRANCH=<name>    Attach to worktree's tmux session"
	@echo "  make worktree-sessions                List tmux sessions for all worktrees"
	@echo ""
	@echo "Examples:"
	@echo "  make worktree-new                     Create worktree with auto-generated name"
	@echo "  make worktree-new BRANCH=fix-auth     Create worktree named 'fix-auth'"
	@echo "  make worktree-attach BRANCH=fix-auth  Attach to 'fix-auth' session"
	@echo "  make worktree-remove BRANCH=fix-auth  Remove worktree and session"
	@echo ""
```

**Key Design Decisions**:
- Grouped commands by category (Worktree Management, Tmux Session Management)
- Shows optional parameters with `[BRANCH=<name>]` syntax
- Includes practical examples section
- Internal target `_create_worktree_session` in .PHONY but not in help (implementation detail)

### Success Criteria:

#### Automated Verification:
- [ ] Help displays: `make help` runs without error
- [ ] All commands documented: Verify help output includes all 6 user-facing commands
- [ ] .PHONY complete: Verify all targets declared

#### Manual Verification:
- [ ] Help text is clear and well-organized
- [ ] Examples are practical and cover common use cases
- [ ] Optional vs required parameters clearly indicated
- [ ] Command descriptions are concise but complete

---

## Phase 9: Propagate Changes and Testing

### Overview
Propagate the new `base.mk` to all projects using `overlord sync --force` and validate against success criteria with manual testing.

### Changes Required:

#### 1. Propagate to All Projects
**Command**: Run `overlord sync --force`
**Purpose**: Overwrite existing `Makefile` in all registered projects with new combined `base.mk + language.mk`

```bash
cd /home/thomas/bin/overlord
overlord sync --force
```

**Expected Behavior**:
- Reads registry from `~/.config/overlord/registry.json`
- For each project: Generates `Makefile` = `base.mk` + `{python,typescript,solidity}.mk`
- Overwrites existing Makefiles (--force flag)
- Logs success for each project synced

#### 2. Verify Propagation
**Commands**:
```bash
# Check a sample Python project
cd ~/Work/Python/active/some-project
grep -q "worktree-attach" Makefile && echo "Propagation successful"

# Check that counter functions exist
grep -q "increment_counter" Makefile && echo "Counter functions present"
```

### Success Criteria:

#### Automated Verification:
- [ ] `overlord sync --force` completes without errors
- [ ] Sample project Makefile contains new commands: `grep worktree-attach ~/Work/Python/active/*/Makefile`
- [ ] Sample project Makefile contains counter functions: `grep increment_counter ~/Work/Python/active/*/Makefile`

#### Manual Verification:
- [ ] All registered projects received updated Makefile
- [ ] Existing language-specific commands still present (e.g., `make test` in Python projects)
- [ ] No syntax errors in generated Makefiles

---

## Testing Strategy

### Unit Tests (per Phase):

**Phase 1 (Counter Management)**:
```bash
# Test counter initialization
cd /tmp/test-overlord
git init
# Copy base.mk with counter functions
make -f base.mk worktree-new  # Should create counter file
test -f .worktrees/.counter && echo "Counter file created"
cat .worktrees/.counter  # Should show "1"
```

**Phase 3 (Name Generation)**:
```bash
# Test deterministic naming
# Counter 0 -> swift_fix_00
# Counter 1 -> bright_fix_01
# Counter 20 -> swift_task_20
# Verify by creating worktrees and checking names
```

**Phase 4 (worktree-new)**:
```bash
# Test auto-naming
make worktree-new
git worktree list | grep swift_fix_00

# Test explicit naming
make worktree-new BRANCH=test-feature
git worktree list | grep test-feature

# Test setup script success
echo '#!/bin/bash\necho "Setup OK"' > .worktree-setup.sh
make worktree-new BRANCH=setup-test

# Test setup script failure and rollback
echo '#!/bin/bash\nexit 1' > .worktree-setup.sh
make worktree-new BRANCH=rollback-test
# Should fail and remove worktree
! test -d .worktrees/rollback-test && echo "Rollback successful"
```

**Phase 5 (worktree-remove)**:
```bash
# Test removal
make worktree-new BRANCH=remove-test
make worktree-remove BRANCH=remove-test
! tmux has-session -t remove-test && echo "Session removed"
! test -d .worktrees/remove-test && echo "Worktree removed"
```

**Phase 6 (worktree-attach)**:
```bash
# Test attach
make worktree-new BRANCH=attach-test
make worktree-attach BRANCH=attach-test
# Manually verify session attached
```

**Phase 7 (worktree-sessions)**:
```bash
# Test listing
make worktree-new
make worktree-new BRANCH=test-2
make worktree-sessions
# Should show both sessions with window counts
```

### Integration Tests:

**Full Workflow Test**:
```bash
cd ~/Work/Python/active/test-project

# 1. Create auto-named worktree
make worktree-new
FIRST_NAME=$(git worktree list | grep .worktrees | head -n1 | awk '{print $1}' | xargs basename)

# 2. Verify session exists
tmux has-session -t "$FIRST_NAME"

# 3. Create named worktree
make worktree-new BRANCH=feature-x

# 4. List sessions
make worktree-sessions
# Should show both sessions

# 5. Attach to first worktree
make worktree-attach BRANCH="$FIRST_NAME"
# Verify inside correct session

# 6. Remove both
make worktree-remove BRANCH="$FIRST_NAME"
make worktree-remove BRANCH=feature-x

# 7. Verify cleanup
! tmux has-session -t "$FIRST_NAME"
! tmux has-session -t feature-x
! test -d .worktrees/"$FIRST_NAME"
! test -d .worktrees/feature-x
```

### Manual Testing Steps:

1. **Test Auto-Naming**:
   - Run `make worktree-new` three times
   - Verify names follow pattern: `swift_fix_00`, `bright_fix_01`, `clever_fix_02`
   - Verify counter increments in `.worktrees/.counter`

2. **Test Tmux Integration**:
   - Create worktree: `make worktree-new BRANCH=test`
   - List sessions: `tmux ls` - should show `test` session
   - Attach: `make worktree-attach BRANCH=test`
   - Verify inside worktree directory: `pwd` should show `.worktrees/test`
   - Check windows: Should have editor, shell, git
   - Verify nvim auto-started in editor window

3. **Test .tmux.local**:
   - Create `.tmux.local` in project root with custom layout
   - Create worktree: `make worktree-new`
   - Verify custom layout applied

4. **Test Setup Script**:
   - Create `.worktree-setup.sh` that installs dependencies
   - Create worktree: `make worktree-new`
   - Verify setup ran (check for installed packages)
   - Create failing setup script: `echo 'exit 1' > .worktree-setup.sh`
   - Try to create worktree - should fail and rollback
   - Verify worktree not in `git worktree list`

5. **Test Cleanup**:
   - Create worktree with session
   - Remove: `make worktree-remove BRANCH=name`
   - Verify both worktree and session gone
   - `git worktree list` should not show it
   - `tmux ls` should not show it

6. **Test Edge Cases**:
   - Create worktree, manually kill session, try to remove worktree (should warn but succeed)
   - Create worktree, manually remove directory, try `worktree-remove` (should warn but continue)
   - Try to attach to non-existent session (should error with helpful message)

## Performance Considerations

- **Counter file I/O**: Each auto-named worktree creation reads/writes counter file once (minimal overhead)
- **Tmux session creation**: Adds ~500ms per worktree creation (acceptable for developer workflow)
- **Name generation**: Pure shell arithmetic, negligible performance impact
- **Collision detection**: Only loops if session name exists (rare in practice)

## Migration Notes

### For Existing Projects:

1. **Run propagation**: `overlord sync --force` overwrites all project Makefiles
2. **Backup if needed**: Projects with custom Makefile modifications will lose changes
3. **Existing worktrees**: Not affected - can be managed with new commands immediately
4. **Counter starts at 0**: First auto-named worktree will be `swift_fix_00` regardless of existing worktrees

### Backward Compatibility:

- **Preserved commands**: `worktree-list` and `worktree-setup` unchanged (fully compatible)
- **Changed behavior**: `worktree-new` and `worktree-remove` now manage tmux sessions (non-breaking - additive)
- **New requirement**: Tmux must be installed and available in PATH
- **Breaking change**: `worktree-new` now allows BRANCH to be optional (was required)

### Rollback Plan:

If issues arise:
1. Restore old `base.mk` from git history: `git show HEAD~1:~/.config/overlord/makefiles/base.mk > ~/.config/overlord/makefiles/base.mk`
2. Re-sync projects: `overlord sync --force`
3. Manually remove counter files: `find ~/Work -name .counter -path '*/.worktrees/*' -delete`

## References

- Original ticket: `thoughts/tickets/feature_worktree_tmux_integration.md`
- Current `base.mk`: `/home/thomas/.config/overlord/makefiles/base.mk:1-60`
- Tmux pattern reference: `/home/thomas/bin/overlord/overlord-open:140-204`
- Name generation reference: `/home/thomas/bin/overlord/hack/create_worktree.sh:12-21`
- Template examples: `/home/thomas/.config/overlord/tmux/{base,python}.tmux`
- Sync mechanism: `/home/thomas/bin/overlord/overlord-sync:97-119`

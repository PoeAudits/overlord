# Current Session Utilities for Tmux Implementation Plan

## Overview

Add current session utility commands to `base.mk` that enable developers to interact with windows in their active tmux session without manually specifying session names. These commands auto-detect the current session and provide convenient shortcuts for common operations.

## Current State Analysis

### Existing Implementation
- **Plan 1 Status**: Core worktree + tmux integration provides session management
- **Plan 2 Status**: Cross-session commands enable remote control of worktree sessions
- **Session Detection**: `$TMUX` environment variable available when inside tmux session
- **Existing Pattern**: `worktree-send` and `worktree-read` provide template for window operations

### Current Session Context
- When inside tmux: `$TMUX` environment variable contains session info
- Outside tmux: `$TMUX` is unset or empty
- Current session accessible via `tmux display-message -p '#S'`
- Window list accessible via `tmux list-windows` (uses current session by default)

### Current Gaps
- No convenient commands for current session window operations
- Developers must use full `worktree-send` syntax even for local operations
- No quick way to list windows in current session
- Window targeting requires remembering exact window names

## Desired End State

After implementing this plan:

1. Developers can send commands to current session windows: `make tmux-send WINDOW=shell CMD="make test"`
2. Developers can read output from current session windows: `make tmux-read WINDOW=shell`
3. Developers can list all windows in current session: `make tmux-list`
4. Commands fail gracefully when not in tmux with helpful error message
5. Commands validate window existence before operations
6. Tab completion friendly (short command names, predictable pattern)

### Verification:
- `make tmux-send WINDOW=shell CMD="echo test"` executes in current session
- `make tmux-read WINDOW=shell` captures output from current session
- `make tmux-list` shows all windows with active marker
- Commands outside tmux show helpful error message
- Window validation shows available windows on error

## What We're NOT Doing

- Pane-level targeting (window-level only)
- Session switching or creation (use `worktree-attach` or `overlord open`)
- History search or scrollback navigation
- Window creation or deletion
- Layout management or window arrangement
- Multi-window operations in single command
- Output streaming or monitoring

## Implementation Approach

This plan adds three new Makefile targets (`tmux-send`, `tmux-read`, `tmux-list`) to `base.mk` that:
1. Check if running inside tmux session (`$TMUX` environment variable)
2. Auto-detect current session name using `tmux display-message -p '#S'`
3. Reuse validation and execution patterns from Plan 2 (`worktree-send`, `worktree-read`)
4. Provide simpler interface (no BRANCH parameter needed)
5. Fail gracefully outside tmux with actionable error

The approach maintains consistency with existing commands while optimizing for the common case of working within a single session.

## Phase 1: Add tmux-send Command

### Overview
Implement `tmux-send` to execute commands in current session windows. Similar to `worktree-send` but auto-detects session and optimized for local use.

### Changes Required:

#### 1. Add tmux-send Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `worktree-read` target

```makefile
# Send command to window in current tmux session
# Usage: make tmux-send WINDOW=<window> CMD="<command>"
tmux-send:
ifndef WINDOW
	$(error WINDOW is required. Usage: make tmux-send WINDOW=<window> CMD="<command>")
endif
ifndef CMD
	$(error CMD is required. Usage: make tmux-send WINDOW=<window> CMD="<command>")
endif
	@if [ -z "$$TMUX" ]; then \
		echo "Error: Not running inside tmux session"; \
		echo ""; \
		echo "This command must be run from within a tmux session."; \
		echo "Use 'overlord open' or 'make worktree-attach' to enter a session."; \
		exit 1; \
	fi; \
	\
	SESSION=$$(tmux display-message -p '#S'); \
	WINDOW_NAME="$(WINDOW)"; \
	COMMAND="$(CMD)"; \
	\
	if ! tmux list-windows -t "$$SESSION" 2>/dev/null | grep -q "^[0-9]*: $$WINDOW_NAME"; then \
		echo "Error: Window '$$WINDOW_NAME' does not exist in current session '$$SESSION'"; \
		echo ""; \
		echo "Available windows:"; \
		tmux list-windows -t "$$SESSION" 2>/dev/null | awk '{print "  " $$2}' | sed 's/\*$$//'; \
		exit 1; \
	fi; \
	\
	echo "Sending to $$SESSION:$$WINDOW_NAME: $$COMMAND"; \
	tmux send-keys -t "$$SESSION:$$WINDOW_NAME" "$$COMMAND" C-m
```

**Key Design Decisions**:
- Two required parameters: WINDOW (window name), CMD (command to execute)
- Session auto-detection: Uses `tmux display-message -p '#S'` to get current session
- Tmux check: Validates `$TMUX` environment variable before proceeding
- Outside tmux error: Suggests using `overlord open` or `worktree-attach`
- Window validation: Same pattern as `worktree-send` for consistency
- Target syntax: Auto-builds `session:window` format internally

**Example Usage**:
```bash
# From within main project session
make tmux-send WINDOW=shell CMD="make test"

# From within worktree session
make tmux-send WINDOW=git CMD="git status"

# Send editor command
make tmux-send WINDOW=editor CMD=":w"
```

### Success Criteria:

#### Automated Verification:
- [ ] Parameter validation: Call without WINDOW, verify error message
- [ ] Parameter validation: Call without CMD, verify error message
- [ ] Tmux detection: Run outside tmux, verify error with helpful message
- [ ] Session auto-detection: Run inside session, verify correct session targeted
- [ ] Window validation: Send to non-existent window, verify error and window list
- [ ] Command execution: Send `echo test`, verify command executes

#### Manual Verification:
- [ ] Command executes in correct window of current session
- [ ] Error message outside tmux is clear and actionable
- [ ] Window list in error shows all available windows
- [ ] Session name correctly detected in various session contexts
- [ ] Works in main project sessions and worktree sessions
- [ ] Complex commands work (same as `worktree-send`)

---

## Phase 2: Add tmux-read Command

### Overview
Implement `tmux-read` to capture visible pane content from current session windows. Simplified version of `worktree-read` for local use.

### Changes Required:

#### 1. Add tmux-read Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `tmux-send` target

```makefile
# Read visible pane content from window in current tmux session
# Usage: make tmux-read WINDOW=<window>
tmux-read:
ifndef WINDOW
	$(error WINDOW is required. Usage: make tmux-read WINDOW=<window>)
endif
	@if [ -z "$$TMUX" ]; then \
		echo "Error: Not running inside tmux session"; \
		echo ""; \
		echo "This command must be run from within a tmux session."; \
		echo "Use 'overlord open' or 'make worktree-attach' to enter a session."; \
		exit 1; \
	fi; \
	\
	SESSION=$$(tmux display-message -p '#S'); \
	WINDOW_NAME="$(WINDOW)"; \
	\
	if ! tmux list-windows -t "$$SESSION" 2>/dev/null | grep -q "^[0-9]*: $$WINDOW_NAME"; then \
		echo "Error: Window '$$WINDOW_NAME' does not exist in current session '$$SESSION'"; \
		echo ""; \
		echo "Available windows:"; \
		tmux list-windows -t "$$SESSION" 2>/dev/null | awk '{print "  " $$2}' | sed 's/\*$$//'; \
		exit 1; \
	fi; \
	\
	echo "Reading from $$SESSION:$$WINDOW_NAME:"; \
	echo "----------------------------------------"; \
	tmux capture-pane -t "$$SESSION:$$WINDOW_NAME" -p
```

**Key Design Decisions**:
- One required parameter: WINDOW (window name)
- Same tmux detection and session auto-detection as `tmux-send`
- Same validation pattern for consistency
- Uses `tmux capture-pane -p` to print directly to stdout
- Captures visible pane only (not scrollback)
- Adds separator line for readability

**Example Usage**:
```bash
# Read shell window output
make tmux-read WINDOW=shell

# Read git window output
make tmux-read WINDOW=git

# Read editor window
make tmux-read WINDOW=editor
```

### Success Criteria:

#### Automated Verification:
- [ ] Parameter validation: Call without WINDOW, verify error message
- [ ] Tmux detection: Run outside tmux, verify error with helpful message
- [ ] Session auto-detection: Run inside session, verify correct session targeted
- [ ] Window validation: Read from non-existent window, verify error and window list
- [ ] Content capture: Send command, then read, verify output captured

#### Manual Verification:
- [ ] Captured content matches visible pane
- [ ] Only visible content captured (not scrollback)
- [ ] Output is readable and properly formatted
- [ ] Error messages consistent with `tmux-send`
- [ ] Works in different window types
- [ ] Separator line improves readability

---

## Phase 3: Add tmux-list Command

### Overview
Implement `tmux-list` to display all windows in the current session with their status (active/inactive). Helps users discover available windows for targeting with `tmux-send` and `tmux-read`.

### Changes Required:

#### 1. Add tmux-list Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `tmux-read` target

```makefile
# List all windows in current tmux session
# Usage: make tmux-list
tmux-list:
	@if [ -z "$$TMUX" ]; then \
		echo "Error: Not running inside tmux session"; \
		echo ""; \
		echo "This command must be run from within a tmux session."; \
		echo "Use 'overlord open' or 'make worktree-attach' to enter a session."; \
		exit 1; \
	fi; \
	\
	SESSION=$$(tmux display-message -p '#S'); \
	echo "Windows in session '$$SESSION':"; \
	echo ""; \
	tmux list-windows -t "$$SESSION" | while IFS=: read -r index rest; do \
		NAME=$$(echo "$$rest" | awk '{print $$1}' | sed 's/\*$$//'); \
		IS_ACTIVE=$$(echo "$$rest" | grep -q '\*' && echo " (active)" || echo ""); \
		printf "  %s%s\n" "$$NAME" "$$IS_ACTIVE"; \
	done; \
	echo ""
```

**Key Design Decisions**:
- No parameters required (operates on current session)
- Same tmux detection as other commands for consistency
- Displays session name in output for context
- Shows window names only (not indices) for clarity
- Marks active window with "(active)" suffix
- Formatted output: indented list for readability

**Example Output**:
```
Windows in session 'myproject':

  editor (active)
  shell
  git

```

### Success Criteria:

#### Automated Verification:
- [ ] Tmux detection: Run outside tmux, verify error with helpful message
- [ ] Session auto-detection: Run inside session, verify correct session name shown
- [ ] Window listing: Create session with 3 windows, verify all listed
- [ ] Active marker: Verify active window marked correctly

#### Manual Verification:
- [ ] Output is clean and readable
- [ ] Window names match actual windows
- [ ] Active window correctly identified
- [ ] Session name shown for context
- [ ] Works with custom window names from `.tmux.local`

---

## Phase 4: Update Help Documentation and Propagate

### Overview
Update help text and .PHONY declarations to document the new current session utilities, then propagate to all projects.

### Changes Required:

#### 1. Update .PHONY Declaration
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Near top of file

```makefile
.PHONY: help worktree-new worktree-list worktree-remove worktree-setup \
        worktree-attach worktree-sessions worktree-send worktree-read \
        tmux-send tmux-read tmux-list \
        _create_worktree_session
```

#### 2. Update help Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Replace existing `help` target

```makefile
# Show available commands
help:
	@echo "Worktree + Tmux Commands:"
	@echo ""
	@echo "Worktree Management:"
	@echo "  make worktree-new [BRANCH=<name>]        Create worktree + tmux session (auto-names if BRANCH omitted)"
	@echo "  make worktree-list                       List all worktrees"
	@echo "  make worktree-remove BRANCH=<name>       Remove worktree and kill tmux session"
	@echo "  make worktree-setup                      Run .worktree-setup.sh in current directory"
	@echo ""
	@echo "Tmux Session Management:"
	@echo "  make worktree-attach BRANCH=<name>       Attach to worktree's tmux session"
	@echo "  make worktree-sessions                   List tmux sessions for all worktrees"
	@echo ""
	@echo "Cross-Session Communication:"
	@echo "  make worktree-send BRANCH=<name> WINDOW=<window> CMD=\"<command>\""
	@echo "                                           Send command to worktree session window"
	@echo "  make worktree-read BRANCH=<name> WINDOW=<window>"
	@echo "                                           Read visible pane content from worktree"
	@echo ""
	@echo "Current Session Utilities (run from within tmux):"
	@echo "  make tmux-send WINDOW=<window> CMD=\"<command>\""
	@echo "                                           Send command to current session window"
	@echo "  make tmux-read WINDOW=<window>           Read visible pane content from window"
	@echo "  make tmux-list                           List all windows in current session"
	@echo ""
	@echo "Examples:"
	@echo "  make worktree-new                        Create worktree with auto-generated name"
	@echo "  make worktree-new BRANCH=fix-auth        Create worktree named 'fix-auth'"
	@echo "  make tmux-send WINDOW=shell CMD=\"make test\""
	@echo "  make tmux-read WINDOW=shell              Read test output"
	@echo "  make tmux-list                           Show available windows"
	@echo "  make worktree-send BRANCH=fix-auth WINDOW=shell CMD=\"make test\""
	@echo "  make worktree-attach BRANCH=fix-auth     Attach to 'fix-auth' session"
	@echo ""
```

**Key Design Decisions**:
- New section: "Current Session Utilities" clearly labeled with usage context
- Grouped by scope: worktree management, session management, cross-session, current session
- Examples show typical workflow with current session utilities
- Help note indicates tmux requirement for current session commands
- Maintains consistent formatting with previous help sections

#### 3. Propagate to All Projects
**Command**: Run `overlord sync --force`

```bash
cd /home/thomas/bin/overlord
overlord sync --force
```

**Expected Behavior**:
- Overwrites all project Makefiles with updated `base.mk + language.mk`
- All projects gain `tmux-send`, `tmux-read`, `tmux-list` commands
- Help text updated across all projects

### Success Criteria:

#### Automated Verification:
- [ ] .PHONY includes new targets: `grep tmux-send base.mk`
- [ ] Help text includes new section: `make help | grep "Current Session Utilities"`
- [ ] Help shows all three commands: `make help | grep -E "tmux-(send|read|list)"`
- [ ] Sync completes: `overlord sync --force` exits with code 0
- [ ] Sample project updated: `grep tmux-send ~/Work/Python/active/*/Makefile`

#### Manual Verification:
- [ ] Help text is well-organized with clear sections
- [ ] Examples demonstrate practical usage
- [ ] All registered projects received updates
- [ ] No syntax errors in generated Makefiles
- [ ] Current session utilities note is clear

---

## Testing Strategy

### Unit Tests (per Command):

**Phase 1 (tmux-send)**:
```bash
# Test outside tmux (must run from non-tmux shell)
cd ~/Work/Python/active/test-project
! make tmux-send WINDOW=shell CMD="echo test" 2>&1 | grep -q "Not running inside tmux"

# Test inside tmux
tmux new-session -d -s test-session
tmux send-keys -t test-session "cd ~/Work/Python/active/test-project" C-m
tmux send-keys -t test-session "make tmux-send WINDOW=shell CMD='echo hello'" C-m
# Verify command executed
tmux capture-pane -t test-session -p | grep -q "hello"
tmux kill-session -t test-session
```

**Phase 2 (tmux-read)**:
```bash
# Test parameter validation
tmux new-session -d -s test-session
tmux send-keys -t test-session "cd ~/Work/Python/active/test-project" C-m
tmux send-keys -t test-session "make tmux-read" C-m
# Verify error shown
tmux capture-pane -t test-session -p | grep -q "WINDOW is required"
tmux kill-session -t test-session
```

**Phase 3 (tmux-list)**:
```bash
# Test window listing
tmux new-session -d -s test-session
tmux new-window -t test-session -n shell
tmux new-window -t test-session -n git
tmux send-keys -t test-session "cd ~/Work/Python/active/test-project" C-m
tmux send-keys -t test-session "make tmux-list" C-m
# Verify all windows listed
OUTPUT=$(tmux capture-pane -t test-session -p)
echo "$OUTPUT" | grep -q "editor"
echo "$OUTPUT" | grep -q "shell"
echo "$OUTPUT" | grep -q "git"
tmux kill-session -t test-session
```

### Integration Tests:

**Full Current Session Workflow**:
```bash
cd ~/Work/Python/active/test-project

# Start tmux session
overlord open test-project

# Inside tmux session (manual steps):
# 1. List available windows
make tmux-list

# 2. Send test command
make tmux-send WINDOW=shell CMD="echo 'test output 123'"

# 3. Read output
make tmux-read WINDOW=shell
# Should show: test output 123

# 4. Send another command
make tmux-send WINDOW=git CMD="git status"

# 5. Read git output
make tmux-read WINDOW=git

# 6. Try non-existent window
make tmux-send WINDOW=fake CMD="test"
# Should error and list available windows
```

**Combined Workflow (Current + Cross-Session)**:
```bash
# From main project session
overlord open main-project

# Create worktree
make worktree-new BRANCH=feature-x

# Send command to worktree from main
make worktree-send BRANCH=feature-x WINDOW=shell CMD="make install"

# Send command to current session window
make tmux-send WINDOW=shell CMD="echo 'Working in main'"

# Read from both
make worktree-read BRANCH=feature-x WINDOW=shell
make tmux-read WINDOW=shell

# List current session windows
make tmux-list
```

### Manual Testing Steps:

1. **Test Outside Tmux**:
   - Exit all tmux sessions
   - Run `make tmux-send WINDOW=shell CMD="test"`
   - Verify error message is helpful
   - Verify suggests using `overlord open`

2. **Test Inside Main Project Session**:
   - Open project: `overlord open test-project`
   - List windows: `make tmux-list`
   - Verify shows editor, shell, git (or custom windows)
   - Send command: `make tmux-send WINDOW=shell CMD="ls -la"`
   - Verify command executes in shell window
   - Read output: `make tmux-read WINDOW=shell`
   - Verify output matches what's visible

3. **Test Inside Worktree Session**:
   - Create worktree: `make worktree-new`
   - Attach: `make worktree-attach BRANCH=swift_fix_00`
   - List windows: `make tmux-list`
   - Send command: `make tmux-send WINDOW=shell CMD="pwd"`
   - Verify shows worktree directory path
   - Read output: `make tmux-read WINDOW=shell`

4. **Test Error Handling**:
   - Try non-existent window: `make tmux-send WINDOW=fake CMD="test"`
   - Verify error shows available windows
   - Verify window list is accurate
   - Try invalid command: `make tmux-send WINDOW=shell CMD="nonexistent-command"`
   - Verify command sent (error appears in target window)

5. **Test Window Name Edge Cases**:
   - Create custom window: `tmux new-window -n "my-window"`
   - Verify `make tmux-list` shows it
   - Send to it: `make tmux-send WINDOW=my-window CMD="echo test"`
   - Verify works with hyphenated names

6. **Test Active Window Marking**:
   - Switch between windows: `tmux select-window -t shell`
   - Run `make tmux-list`
   - Verify shell marked as active
   - Switch to editor: `tmux select-window -t editor`
   - Run `make tmux-list`
   - Verify editor now marked as active

## Performance Considerations

- **Session Auto-Detection**: `tmux display-message -p '#S'` is instant (<5ms)
- **Window Listing**: `tmux list-windows` returns immediately
- **Command Execution**: Same as Plan 2 (near-instant)
- **No Overhead**: Auto-detection adds negligible latency vs explicit session parameter
- **Shell Overhead**: Minimal - single subshell for session detection

## Migration Notes

### For Existing Projects:

1. **Prerequisites**: Plans 1 and 2 should be implemented first (but not strictly required)
2. **Propagation**: Run `overlord sync --force` after updating `base.mk`
3. **Backward Compatibility**: Purely additive - no breaking changes
4. **Tmux Context Required**: Commands only work inside tmux sessions

### Usage Patterns:

**Before (Plan 2 only)**:
- Must specify full session and window: `make worktree-send BRANCH=myproject WINDOW=shell CMD="test"`
- Even when working in same session: `make worktree-send BRANCH=myproject WINDOW=shell CMD="test"`
- No quick way to list available windows

**After (Plan 3)**:
- Inside session, use shortcuts: `make tmux-send WINDOW=shell CMD="test"`
- List windows quickly: `make tmux-list`
- Cross-session commands still available when needed

### Common Use Cases:

1. **Local Development**: Use `tmux-send` and `tmux-read` for current workspace
2. **Multi-Session Coordination**: Use `worktree-send` for remote worktrees
3. **Window Discovery**: Use `tmux-list` to see available targets
4. **Quick Iterations**: Send/read in tight loop for rapid feedback

### Best Practices:

- **Inside Session**: Use `tmux-*` commands (shorter, more convenient)
- **Outside Session**: Use `worktree-*` commands (explicit session targeting)
- **Automation**: Prefer `worktree-*` commands (explicit is better than implicit)
- **Interactive**: Prefer `tmux-*` commands (fewer keystrokes)

## References

- Original ticket: `thoughts/tickets/feature_worktree_tmux_integration.md`
- Plan 1: `thoughts/plans/worktree_tmux_core_integration.md`
- Plan 2: `thoughts/plans/worktree_cross_session_communication.md`
- Base Makefile: `/home/thomas/.config/overlord/makefiles/base.mk`
- Tmux man page sections: display-message, list-windows

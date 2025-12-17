# Cross-Session Communication for Worktrees Implementation Plan

## Overview

Extend `base.mk` with cross-session communication commands that enable the main repository session to send commands to and read output from worktree tmux sessions. This allows developers to control and monitor worktree sessions without leaving their main workspace.

## Current State Analysis

### Existing Implementation
- **Plan 1 Status**: Core worktree + tmux integration provides foundation
- **Available Commands**: `worktree-new`, `worktree-remove`, `worktree-attach`, `worktree-sessions`
- **Session Naming**: Worktree sessions named after branch (e.g., `swift_fix_00`, `feature-auth`)
- **Window Structure**: Default layout has `editor`, `shell`, `git` windows (or custom via `.tmux.local`)

### Tmux Cross-Session Capabilities
- `tmux send-keys -t session:window` - Send keystrokes to specific window in any session
- `tmux capture-pane -t session:window -p` - Capture visible pane content from any session
- `tmux list-windows -t session` - List all windows in a session (for validation)
- Target syntax: `session:window` format (e.g., `swift_fix_00:shell`)

### Current Gaps
- No way to execute commands in worktree sessions from main repo
- No way to read output from worktree sessions without attaching
- No validation that target windows exist before attempting operations

## Desired End State

After implementing this plan:

1. Developers can send commands to worktree sessions: `make worktree-send BRANCH=fix-auth WINDOW=shell CMD="make test"`
2. Developers can read output from worktree sessions: `make worktree-read BRANCH=fix-auth WINDOW=shell`
3. Commands validate session and window existence before attempting operations
4. Clear error messages guide users when sessions or windows don't exist
5. Visible pane content captured (not full scrollback history)

### Verification:
- `make worktree-send BRANCH=name WINDOW=shell CMD="echo hello"` executes in worktree session
- `make worktree-read BRANCH=name WINDOW=shell` captures and displays output
- Sending to non-existent session shows error with helpful message
- Sending to non-existent window shows error and lists available windows
- Captured content matches what's visible in the pane when attached

## What We're NOT Doing

- Current session utility commands (`tmux-send`, `tmux-read`, `tmux-list`) - deferred to Plan 3
- Full scrollback capture (only visible pane content)
- Bidirectional communication (worktree → main repo)
- Command output streaming (only snapshot capture)
- Multiple window targeting in single command
- File transfer between sessions
- Session-to-session piping

## Implementation Approach

This plan adds two new Makefile targets (`worktree-send`, `worktree-read`) to `base.mk` that:
1. Validate required parameters (BRANCH, WINDOW, CMD for send)
2. Check session exists using `tmux has-session`
3. Check window exists using `tmux list-windows`
4. Execute tmux commands with proper target syntax
5. Provide clear error messages with actionable suggestions

The approach follows Makefile best practices with parameter validation and graceful error handling.

## Phase 1: Add worktree-send Command

### Overview
Implement `worktree-send` to execute commands in worktree session windows from the main repository. Includes validation of session and window existence with helpful error messages.

### Changes Required:

#### 1. Add worktree-send Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `worktree-sessions` target

```makefile
# Send command to worktree's tmux session window
# Usage: make worktree-send BRANCH=<name> WINDOW=<window> CMD="<command>"
worktree-send:
ifndef BRANCH
	$(error BRANCH is required. Usage: make worktree-send BRANCH=<name> WINDOW=<window> CMD="<command>")
endif
ifndef WINDOW
	$(error WINDOW is required. Usage: make worktree-send BRANCH=<name> WINDOW=<window> CMD="<command>")
endif
ifndef CMD
	$(error CMD is required. Usage: make worktree-send BRANCH=<name> WINDOW=<window> CMD="<command>")
endif
	@SESSION="$(BRANCH)"; \
	WINDOW_NAME="$(WINDOW)"; \
	COMMAND="$(CMD)"; \
	\
	if ! tmux has-session -t "$$SESSION" 2>/dev/null; then \
		echo "Error: Tmux session '$$SESSION' does not exist"; \
		echo ""; \
		echo "Available worktree sessions:"; \
		$(MAKE) worktree-sessions 2>/dev/null || echo "  (none)"; \
		exit 1; \
	fi; \
	\
	if ! tmux list-windows -t "$$SESSION" 2>/dev/null | grep -q "^[0-9]*: $$WINDOW_NAME"; then \
		echo "Error: Window '$$WINDOW_NAME' does not exist in session '$$SESSION'"; \
		echo ""; \
		echo "Available windows in '$$SESSION':"; \
		tmux list-windows -t "$$SESSION" 2>/dev/null | awk '{print "  " $$2}' | sed 's/\*$$//'; \
		exit 1; \
	fi; \
	\
	echo "Sending to $$SESSION:$$WINDOW_NAME: $$COMMAND"; \
	tmux send-keys -t "$$SESSION:$$WINDOW_NAME" "$$COMMAND" C-m
```

**Key Design Decisions**:
- Three required parameters: BRANCH (session name), WINDOW (window name), CMD (command to execute)
- Validation order: Session existence → Window existence → Execute command
- Error for missing session: Lists all worktree sessions via `worktree-sessions`
- Error for missing window: Lists all windows in the target session
- Window validation: Uses `tmux list-windows` output format (e.g., `0: editor*`)
- Command execution: Appends `C-m` (Enter key) to execute command automatically
- Target syntax: `session:window` format for tmux

**Example Usage**:
```bash
# Send test command to shell window
make worktree-send BRANCH=swift_fix_00 WINDOW=shell CMD="make test"

# Send git command to git window
make worktree-send BRANCH=feature-auth WINDOW=git CMD="git status"

# Send editor command
make worktree-send BRANCH=swift_fix_00 WINDOW=editor CMD=":q"
```

### Success Criteria:

#### Automated Verification:
- [ ] Parameter validation: Call without BRANCH, verify error message
- [ ] Parameter validation: Call without WINDOW, verify error message
- [ ] Parameter validation: Call without CMD, verify error message
- [ ] Session validation: Send to non-existent session, verify error and session list shown
- [ ] Window validation: Send to non-existent window, verify error and window list shown
- [ ] Command execution: Send `echo test`, verify command executes in target window

#### Manual Verification:
- [ ] Command executes in correct window of correct session
- [ ] Error messages are clear and actionable
- [ ] Window list in error shows all available windows correctly
- [ ] Session list in error shows all worktree sessions
- [ ] Complex commands work (pipes, quotes, multiple arguments)
- [ ] Commands execute immediately (auto-Enter with C-m)

---

## Phase 2: Add worktree-read Command

### Overview
Implement `worktree-read` to capture and display visible pane content from worktree session windows. Uses same validation pattern as `worktree-send`.

### Changes Required:

#### 1. Add worktree-read Target
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: After `worktree-send` target

```makefile
# Read visible pane content from worktree's tmux session window
# Usage: make worktree-read BRANCH=<name> WINDOW=<window>
worktree-read:
ifndef BRANCH
	$(error BRANCH is required. Usage: make worktree-read BRANCH=<name> WINDOW=<window>)
endif
ifndef WINDOW
	$(error WINDOW is required. Usage: make worktree-read BRANCH=<name> WINDOW=<window>)
endif
	@SESSION="$(BRANCH)"; \
	WINDOW_NAME="$(WINDOW)"; \
	\
	if ! tmux has-session -t "$$SESSION" 2>/dev/null; then \
		echo "Error: Tmux session '$$SESSION' does not exist"; \
		echo ""; \
		echo "Available worktree sessions:"; \
		$(MAKE) worktree-sessions 2>/dev/null || echo "  (none)"; \
		exit 1; \
	fi; \
	\
	if ! tmux list-windows -t "$$SESSION" 2>/dev/null | grep -q "^[0-9]*: $$WINDOW_NAME"; then \
		echo "Error: Window '$$WINDOW_NAME' does not exist in session '$$SESSION'"; \
		echo ""; \
		echo "Available windows in '$$SESSION':"; \
		tmux list-windows -t "$$SESSION" 2>/dev/null | awk '{print "  " $$2}' | sed 's/\*$$//'; \
		exit 1; \
	fi; \
	\
	echo "Reading from $$SESSION:$$WINDOW_NAME:"; \
	echo "----------------------------------------"; \
	tmux capture-pane -t "$$SESSION:$$WINDOW_NAME" -p
```

**Key Design Decisions**:
- Two required parameters: BRANCH (session name), WINDOW (window name)
- Same validation pattern as `worktree-send` for consistency
- Uses `tmux capture-pane -p` flag to print directly to stdout
- Captures visible pane only (not scrollback history)
- Adds separator line for readability
- No output filtering or processing (raw pane content)

**Example Usage**:
```bash
# Read shell window output
make worktree-read BRANCH=swift_fix_00 WINDOW=shell

# Read git window output
make worktree-read BRANCH=feature-auth WINDOW=git

# Read editor window (shows nvim UI)
make worktree-read BRANCH=swift_fix_00 WINDOW=editor
```

### Success Criteria:

#### Automated Verification:
- [ ] Parameter validation: Call without BRANCH, verify error message
- [ ] Parameter validation: Call without WINDOW, verify error message
- [ ] Session validation: Read from non-existent session, verify error and session list
- [ ] Window validation: Read from non-existent window, verify error and window list
- [ ] Content capture: Send command, then read, verify output captured

#### Manual Verification:
- [ ] Captured content matches visible pane when attached
- [ ] Only visible content captured (not full scrollback)
- [ ] Output is readable and properly formatted
- [ ] Error messages match `worktree-send` style (consistent UX)
- [ ] Works with different window types (editor, shell, git)
- [ ] Separator line improves readability

---

## Phase 3: Update Help Documentation and Propagate

### Overview
Update help text and .PHONY declarations to document the new cross-session commands, then propagate to all projects.

### Changes Required:

#### 1. Update .PHONY Declaration
**File**: `/home/thomas/.config/overlord/makefiles/base.mk`
**Location**: Near top of file

```makefile
.PHONY: help worktree-new worktree-list worktree-remove worktree-setup \
        worktree-attach worktree-sessions worktree-send worktree-read \
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
	@echo "Examples:"
	@echo "  make worktree-new                        Create worktree with auto-generated name"
	@echo "  make worktree-new BRANCH=fix-auth        Create worktree named 'fix-auth'"
	@echo "  make worktree-send BRANCH=fix-auth WINDOW=shell CMD=\"make test\""
	@echo "  make worktree-read BRANCH=fix-auth WINDOW=shell"
	@echo "  make worktree-attach BRANCH=fix-auth     Attach to 'fix-auth' session"
	@echo "  make worktree-remove BRANCH=fix-auth     Remove worktree and session"
	@echo ""
```

**Key Design Decisions**:
- New section: "Cross-Session Communication" groups the two new commands
- Multi-line formatting for long command signatures
- Examples show realistic workflow (send test command, read output)
- Maintains existing organization and style from Plan 1

#### 3. Propagate to All Projects
**Command**: Run `overlord sync --force`

```bash
cd /home/thomas/bin/overlord
overlord sync --force
```

**Expected Behavior**:
- Overwrites all project Makefiles with updated `base.mk + language.mk`
- All projects gain `worktree-send` and `worktree-read` commands
- Help text updated across all projects

### Success Criteria:

#### Automated Verification:
- [ ] .PHONY includes new targets: `grep worktree-send base.mk`
- [ ] Help text includes new section: `make help | grep "Cross-Session Communication"`
- [ ] Help shows examples: `make help | grep worktree-send`
- [ ] Sync completes: `overlord sync --force` exits with code 0
- [ ] Sample project updated: `grep worktree-send ~/Work/Python/active/*/Makefile`

#### Manual Verification:
- [ ] Help text is well-organized and readable
- [ ] Examples are practical and clear
- [ ] All registered projects received updates
- [ ] No syntax errors in any generated Makefiles

---

## Testing Strategy

### Unit Tests (per Command):

**Phase 1 (worktree-send)**:
```bash
cd ~/Work/Python/active/test-project

# Test parameter validation
! make worktree-send 2>&1 | grep -q "BRANCH is required"
! make worktree-send BRANCH=test 2>&1 | grep -q "WINDOW is required"
! make worktree-send BRANCH=test WINDOW=shell 2>&1 | grep -q "CMD is required"

# Test session validation
! make worktree-send BRANCH=nonexistent WINDOW=shell CMD="echo test" 2>&1 | grep -q "does not exist"

# Test window validation
make worktree-new BRANCH=test
! make worktree-send BRANCH=test WINDOW=nonexistent CMD="echo test" 2>&1 | grep -q "does not exist"

# Test command execution
make worktree-send BRANCH=test WINDOW=shell CMD="echo 'hello from main'"
# Verify command executed by reading output
make worktree-read BRANCH=test WINDOW=shell | grep -q "hello from main"
```

**Phase 2 (worktree-read)**:
```bash
# Test parameter validation
! make worktree-read 2>&1 | grep -q "BRANCH is required"
! make worktree-read BRANCH=test 2>&1 | grep -q "WINDOW is required"

# Test session validation
! make worktree-read BRANCH=nonexistent WINDOW=shell 2>&1 | grep -q "does not exist"

# Test window validation
! make worktree-read BRANCH=test WINDOW=nonexistent 2>&1 | grep -q "does not exist"

# Test content capture
make worktree-send BRANCH=test WINDOW=shell CMD="echo 'test output 123'"
sleep 1
OUTPUT=$(make worktree-read BRANCH=test WINDOW=shell)
echo "$OUTPUT" | grep -q "test output 123"
```

### Integration Tests:

**Full Cross-Session Workflow**:
```bash
cd ~/Work/Python/active/test-project

# 1. Create worktree
make worktree-new BRANCH=integration-test

# 2. Send command to run tests
make worktree-send BRANCH=integration-test WINDOW=shell CMD="make test"

# 3. Wait for tests to complete
sleep 5

# 4. Read test output
make worktree-read BRANCH=integration-test WINDOW=shell

# 5. Send git status command
make worktree-send BRANCH=integration-test WINDOW=git CMD="git status"

# 6. Read git output
make worktree-read BRANCH=integration-test WINDOW=git

# 7. Verify editor window exists
make worktree-read BRANCH=integration-test WINDOW=editor

# 8. Cleanup
make worktree-remove BRANCH=integration-test
```

**Multi-Worktree Coordination**:
```bash
# Create multiple worktrees
make worktree-new BRANCH=feature-a
make worktree-new BRANCH=feature-b

# Send different commands to each
make worktree-send BRANCH=feature-a WINDOW=shell CMD="echo 'Working on feature A'"
make worktree-send BRANCH=feature-b WINDOW=shell CMD="echo 'Working on feature B'"

# Read from both
make worktree-read BRANCH=feature-a WINDOW=shell
make worktree-read BRANCH=feature-b WINDOW=shell

# Cleanup
make worktree-remove BRANCH=feature-a
make worktree-remove BRANCH=feature-b
```

### Manual Testing Steps:

1. **Test Send Command Execution**:
   - Create worktree: `make worktree-new BRANCH=test`
   - Send command: `make worktree-send BRANCH=test WINDOW=shell CMD="ls -la"`
   - Attach to verify: `make worktree-attach BRANCH=test`
   - Check shell window shows `ls -la` output
   - Detach and continue from main repo

2. **Test Read Output Capture**:
   - Send command that produces output: `make worktree-send BRANCH=test WINDOW=shell CMD="echo 'Line 1' && echo 'Line 2' && echo 'Line 3'"`
   - Read output: `make worktree-read BRANCH=test WINDOW=shell`
   - Verify all three lines captured
   - Verify output matches what's visible in pane

3. **Test Window Targeting**:
   - Send to editor: `make worktree-send BRANCH=test WINDOW=editor CMD=":q"`
   - Send to shell: `make worktree-send BRANCH=test WINDOW=shell CMD="pwd"`
   - Send to git: `make worktree-send BRANCH=test WINDOW=git CMD="git log --oneline -5"`
   - Read from each window to verify correct targeting

4. **Test Error Messages**:
   - Try non-existent session: `make worktree-send BRANCH=fake WINDOW=shell CMD="test"`
   - Verify error shows list of available sessions
   - Try non-existent window: `make worktree-send BRANCH=test WINDOW=fake CMD="test"`
   - Verify error shows list of available windows

5. **Test Complex Commands**:
   - Pipes: `make worktree-send BRANCH=test WINDOW=shell CMD="ls -la | grep test"`
   - Quotes: `make worktree-send BRANCH=test WINDOW=shell CMD="echo \"hello world\""`
   - Environment: `make worktree-send BRANCH=test WINDOW=shell CMD="TEST=value && echo \$TEST"`
   - Background: `make worktree-send BRANCH=test WINDOW=shell CMD="sleep 10 &"`

6. **Test Real Development Workflow**:
   - Create worktree for feature development
   - From main repo, send: `make worktree-send BRANCH=feature WINDOW=shell CMD="make install"`
   - Read to verify install completed: `make worktree-read BRANCH=feature WINDOW=shell`
   - Send test command: `make worktree-send BRANCH=feature WINDOW=shell CMD="make test"`
   - Monitor test output: `make worktree-read BRANCH=feature WINDOW=shell`
   - Continue working in main repo while tests run in background

## Performance Considerations

- **Command Latency**: Tmux cross-session commands execute near-instantly (<50ms typical)
- **Window Validation**: `tmux list-windows` is fast, minimal overhead (~10ms)
- **Content Capture**: Only captures visible pane (not full history), very fast
- **Network**: All commands are local (tmux socket communication)
- **No Polling**: Commands are synchronous, no background processes

## Migration Notes

### For Existing Projects:

1. **Prerequisites**: Plan 1 must be implemented first (provides worktree sessions)
2. **Propagation**: Run `overlord sync --force` after updating `base.mk`
3. **Backward Compatibility**: Purely additive - no breaking changes to existing commands
4. **Tmux Required**: Commands fail gracefully if tmux not available

### Usage Patterns:

**Before (Plan 1 only)**:
- Create worktree, attach to session, run commands manually, detach
- Switch between main repo and worktree sessions frequently
- No remote control of worktree sessions

**After (Plan 2)**:
- Create worktree, send commands from main repo without attaching
- Monitor output without switching sessions
- Coordinate multiple worktrees from single location

### Common Use Cases:

1. **Background Testing**: Send test commands to worktree, continue working in main
2. **Multi-Worktree Development**: Coordinate builds/tests across multiple worktrees
3. **Output Monitoring**: Check status of long-running commands without attaching
4. **Automation**: Script worktree operations from main repository

## References

- Original ticket: `thoughts/tickets/feature_worktree_tmux_integration.md`
- Plan 1: `thoughts/plans/worktree_tmux_core_integration.md`
- Base Makefile: `/home/thomas/.config/overlord/makefiles/base.mk`
- Tmux man page sections: send-keys, capture-pane, list-windows

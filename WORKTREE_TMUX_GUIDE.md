# Worktree & Tmux Commands Guide

This guide explains how to use the git worktree and tmux integration commands from your Makefile.

## Overview

The Makefile provides a complete workflow for creating isolated development branches with dedicated tmux sessions. Each worktree gets its own directory, git branch, and tmux session with multiple windows (editor, shell, git).

## Quick Start

```bash
# Create a worktree with auto-generated name
make worktree-new

# Create a worktree with a specific name
make worktree-new BRANCH=fix-auth

# List all worktrees
make worktree-list

# Attach to a worktree's session
make worktree-attach BRANCH=fix-auth

# Remove a worktree and its session
make worktree-remove BRANCH=fix-auth
```

## Core Commands

### 1. Creating Worktrees

#### Auto-Generated Names
```bash
make worktree-new
```

When you omit the `BRANCH` parameter, the system auto-generates unique names using two word lists:
- **Adjectives**: swift, bright, clever, smooth, quick, clean, sharp, neat, cool, fast, bold, calm, clear, crisp, fresh, light, prime, ready, smart, steady
- **Nouns**: fix, task, work, dev, patch, branch, code, build, test, run, flow, sync, push, pull, draft, spike, probe, check, scan, forge

Example output: `swift_fix_00`, `bright_task_01`, `clever_work_02`

This generates 400 unique combinations (20 adjectives × 20 nouns) before needing collision detection.

#### Custom Branch Names
```bash
make worktree-new BRANCH=fix-auth
```

Creates a worktree with your specified branch name. If the branch already exists, it creates a worktree from that branch. Otherwise, it creates both the branch and worktree.

#### What Happens During Creation

1. **Branch handling**: Creates new branch if needed, or uses existing one
2. **.gitignore copying**: Copies root `.gitignore` to the worktree directory
3. **Setup script execution**: Runs `.worktree-setup.sh` if it exists
4. **Tmux session creation**: Initializes a tmux session with 3 windows:
   - `editor` - Opens nvim for editing
   - `shell` - Command shell for running commands
   - `git` - Git operations window (runs `git status` on launch)
5. **Error handling**: Rolls back both branch and worktree on setup failure

#### Output
```
Auto-generated name: swift_fix_00
Creating new branch 'swift_fix_00' and worktree...
Running .worktree-setup.sh...
Creating tmux session: swift_fix_00

Worktree created successfully!
  Branch: swift_fix_00
  Path: .worktrees/swift_fix_00
  Session: swift_fix_00

Attach with: make worktree-attach BRANCH=swift_fix_00
```

### 2. Listing Worktrees

#### List Git Worktrees
```bash
make worktree-list
```

Shows all git worktrees in the repository using `git worktree list`.

Example output:
```
/home/user/project                    abcd1234 [main]
/home/user/project/.worktrees/fix-auth  ef5678 [fix-auth]
/home/user/project/.worktrees/feature  gh9012 [feature]
```

#### List Tmux Sessions
```bash
make worktree-sessions
```

Shows all tmux sessions created for worktrees with window counts and attachment status.

Example output:
```
Tmux sessions for worktrees:

  fix-auth                             3 windows  detached
  feature                              3 windows  attached
  swift_fix_00                          3 windows  detached
```

### 3. Attaching to Worktrees

```bash
make worktree-attach BRANCH=<name>
```

Connects you to a worktree's tmux session. The command intelligently handles two scenarios:

- **Already in tmux**: Uses `tmux switch-client` to switch to the session
- **Outside tmux**: Uses `tmux attach` to attach to the session

Example:
```bash
make worktree-attach BRANCH=fix-auth
```

## Cross-Session Communication

These commands allow you to control and monitor worktrees without being inside their tmux sessions.

### Send Commands to a Worktree

```bash
make worktree-send BRANCH=<name> WINDOW=<window> CMD="<command>"
```

Sends a command to a specific window in a worktree's tmux session.

**Parameters:**
- `BRANCH` - The worktree/branch name (required)
- `WINDOW` - The window name: `editor`, `shell`, or `git` (required)
- `CMD` - The command to execute (required, must be quoted)

**Examples:**

Run tests in the shell window:
```bash
make worktree-send BRANCH=fix-auth WINDOW=shell CMD="make test"
```

Compile code in the shell window:
```bash
make worktree-send BRANCH=feature WINDOW=shell CMD="cargo build"
```

Run git operations in the git window:
```bash
make worktree-send BRANCH=fix-auth WINDOW=git CMD="git log --oneline -5"
```

Switch to a file in the editor:
```bash
make worktree-send BRANCH=fix-auth WINDOW=editor CMD=":e src/main.rs"
```

**Error Handling:**

If the session or window doesn't exist, the command shows helpful error messages:

```
Error: Tmux session 'nonexistent' does not exist

Available worktree sessions:
  fix-auth                             3 windows  detached
  feature                              3 windows  attached
```

Or for window errors:

```
Error: Window 'nonexistent' does not exist in session 'fix-auth'

Available windows in 'fix-auth':
  editor
  shell
  git
```

### Read Window Output

```bash
make worktree-read BRANCH=<name> WINDOW=<window>
```

Displays the visible content of a specific window pane without entering the session.

**Parameters:**
- `BRANCH` - The worktree/branch name (required)
- `WINDOW` - The window name: `editor`, `shell`, or `git` (required)

**Examples:**

Check test results in the shell window:
```bash
make worktree-read BRANCH=fix-auth WINDOW=shell
```

View git status in the git window:
```bash
make worktree-read BRANCH=fix-auth WINDOW=git
```

See editor content:
```bash
make worktree-read BRANCH=fix-auth WINDOW=editor
```

This is useful for checking if a long-running process has completed or reviewing command output without switching sessions.

### 4. Removing Worktrees

```bash
make worktree-remove BRANCH=<name>
```

Safely removes a worktree and its associated tmux session. The removal happens in this order:

1. **Kills tmux session** if it exists
2. **Removes git worktree** if the directory exists
3. **Cleanup complete** - Confirms successful removal

**Example:**
```bash
make worktree-remove BRANCH=fix-auth
```

**Output:**
```
Removing worktree and tmux session: fix-auth
Killing tmux session: fix-auth
Removing worktree: fix-auth
Worktree removed: fix-auth
Cleanup complete: fix-auth
```

### 5. Setup Script Execution

```bash
make worktree-setup
```

Runs the `.worktree-setup.sh` script in the current directory. This is useful for:
- Running custom initialization in a worktree
- Installing dependencies
- Setting up environment variables
- Configuring tools

The script is automatically executed during `worktree-new` if it exists in the project root.

## Tmux Session Structure

Each worktree session has 3 windows by default:

| Window | Name | Initial Command | Purpose |
|--------|------|-----------------|---------|
| 0 | editor | `nvim` | Code editing |
| 1 | shell | - | Running commands and scripts |
| 2 | git | `git status` | Git operations |

### Customizing Session Layout

Create a `.tmux.local` file in your project root to customize the tmux session layout.

**MODE=override** (default):
```bash
# .tmux.local
MODE=override

tmux rename-window -t "${TMUX_SESSION}:base" "work"
tmux send-keys -t "${TMUX_SESSION}:work" "nvim" C-m
tmux new-window -t "${TMUX_SESSION}:" -n "tests" -c "${TMUX_PROJECT_DIR}"
```

**MODE=merge**:
Applies your custom configuration after the default layout is created.

**Available Variables:**
- `TMUX_SESSION` - The session name (e.g., `fix-auth`)
- `TMUX_PROJECT_DIR` - The worktree directory path

## Practical Workflows

### Workflow 1: Parallel Development

```bash
# Terminal 1: Create first feature branch
make worktree-new BRANCH=feature-auth

# Terminal 2: Create second feature branch  
make worktree-new BRANCH=feature-api

# Terminal 1: Attach to first worktree
make worktree-attach BRANCH=feature-auth

# Terminal 2: Attach to second worktree
make worktree-attach BRANCH=feature-api

# You can now work on both features simultaneously in separate sessions
```

### Workflow 2: Remote Testing

```bash
# Create a worktree for testing
make worktree-new BRANCH=test-deploy

# Run tests without entering the session
make worktree-send BRANCH=test-deploy WINDOW=shell CMD="make test"

# Check results
make worktree-read BRANCH=test-deploy WINDOW=shell

# Clean up when done
make worktree-remove BRANCH=test-deploy
```

### Workflow 3: Building Multiple Configurations

```bash
# Create worktrees for different configurations
make worktree-new BRANCH=build-debug
make worktree-new BRANCH=build-release
make worktree-new BRANCH=build-test

# Run builds in parallel
make worktree-send BRANCH=build-debug WINDOW=shell CMD="cargo build"
make worktree-send BRANCH=build-release WINDOW=shell CMD="cargo build --release"
make worktree-send BRANCH=build-test WINDOW=shell CMD="cargo test"

# Monitor progress without entering sessions
make worktree-read BRANCH=build-debug WINDOW=shell
make worktree-read BRANCH=build-release WINDOW=shell
make worktree-read BRANCH=build-test WINDOW=shell
```

### Workflow 4: Interactive Development

```bash
# Create a worktree with a custom name
make worktree-new BRANCH=bugfix-123

# Attach to the session for interactive work
make worktree-attach BRANCH=bugfix-123

# (You're now inside the tmux session)
# Use Ctrl+B + Window number to switch between windows:
#   Ctrl+B + 0 → editor window
#   Ctrl+B + 1 → shell window
#   Ctrl+B + 2 → git window

# When done, you can:
# - Detach from session: Ctrl+B + D
# - Kill session from outside: make worktree-remove BRANCH=bugfix-123
```

## Internal Details

### Auto-Generated Name Generation

The system uses a counter file (`.worktrees/.counter`) to generate deterministic, unique names:

```
Counter → Name Format
0 → swift_fix_00
1 → bright_task_00
20 → swift_patch_01
400 → swift_forge_20 (cycles through combinations)
```

Formula:
- Adjective index: `counter % 20`
- Noun index: `(counter / 20) % 20`
- Counter suffix: counter value

### Worktree Directory Structure

```
.worktrees/
├── .counter              # Counter file for name generation
├── fix-auth/             # Worktree for fix-auth branch
│   └── .tmux.local       # Optional custom tmux config
├── feature/              # Worktree for feature branch
└── swift_fix_00/         # Auto-named worktree
```

### Session Collision Detection

When creating a worktree:
1. Generates initial name from counter
2. Checks if tmux session with that name exists
3. If collision detected, appends counter to name: `swift_fix_00_00`
4. Continues checking until unique name found

## Troubleshooting

### Session Already Exists
```
Note: Tmux session 'fix-auth' already exists
```
The session is still running. Either:
- Attach to it: `make worktree-attach BRANCH=fix-auth`
- Kill it first: `tmux kill-session -t fix-auth`

### Window Not Found
```
Error: Window 'shell' does not exist in session 'fix-auth'
```
The window name is incorrect. Check available windows:
```bash
make worktree-sessions
```

### Worktree Not Found
```
Warning: Worktree directory not found: .worktrees/nonexistent
```
The worktree was already removed or the name is incorrect. List worktrees:
```bash
make worktree-list
```

### Setup Script Failed
```
Error: Setup script failed, rolling back...
```
The `.worktree-setup.sh` script encountered an error. Check:
1. Script permissions: `chmod +x .worktree-setup.sh`
2. Script syntax errors: `bash -n .worktree-setup.sh`
3. Required dependencies installed

## Tips & Best Practices

1. **Use descriptive branch names** for easier management:
   ```bash
   make worktree-new BRANCH=fix-login-redirect
   ```

2. **Let system auto-generate names** when you don't care about naming:
   ```bash
   make worktree-new
   ```

3. **Use cross-session commands** for batch operations:
   ```bash
   make worktree-send BRANCH=build-1 WINDOW=shell CMD="make"
   make worktree-send BRANCH=build-2 WINDOW=shell CMD="make"
   make worktree-read BRANCH=build-1 WINDOW=shell
   make worktree-read BRANCH=build-2 WINDOW=shell
   ```

4. **Create `.worktree-setup.sh`** for automated initialization:
   ```bash
   #!/bin/bash
   set -euo pipefail
   
   # Install dependencies
   make install
   
   # Run initial tests
   make test
   ```

5. **Clean up old worktrees** regularly:
   ```bash
   make worktree-list
   make worktree-remove BRANCH=old-feature
   ```

6. **Use session names for context switching** in tmux:
   ```bash
   # Inside tmux, switch between sessions with Ctrl+B + S
   make worktree-attach BRANCH=another-branch
   ```

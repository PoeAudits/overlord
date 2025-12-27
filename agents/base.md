# [PROJECT NAME] - Project Management System

## Makefile Commands

**Worktree Management:**
```bash
make worktree-new [BRANCH=name]     # Create worktree + tmux session
make worktree-list                  # List worktrees
make worktree-attach BRANCH=name    # Attach to session
make worktree-remove BRANCH=name    # Remove worktree + kill session
make worktree-archive BRANCH=name   # Archive logs from worktree
make worktree-archive-remove BRANCH=name  # Archive logs then remove worktree
make worktree-setup                 # Run .worktree-setup.sh in current directory
```

**Tmux Session Management:**
```bash
make worktree-sessions               # List tmux sessions for all worktrees
```

**Cross-Session Communication:**
```bash
make worktree-send BRANCH=x WINDOW=y CMD="z"   # Send command to worktree session window
make worktree-read BRANCH=x WINDOW=y           # Read visible pane content from worktree
```

**Current Session Utilities (run from within tmux):**
```bash
make tmux-send WINDOW=x CMD="y"     # Send command to current session window
make tmux-read WINDOW=x             # Read visible pane content from window
make tmux-list                      # List all windows in current session
```

---
type: feature
priority: high
created: 2025-12-16T00:00:00Z
status: archived
tags: [config, tmux, template, prerequisite]
keywords: [base.tmux, tmux template, generic workspace, TMUX_SESSION, TMUX_PROJECT_DIR]
patterns: [tmux layout, window creation, MODE variable]
---

# FEATURE-005: Add `base.tmux` Template for Non-Language-Specific Projects

## Description
Create a generic `base.tmux` template in `~/.config/overlord/tmux/` for projects that don't have a specific language (Python/TypeScript/Solidity). This template provides the common workspace layout without language-specific options.

## Context
The existing language-specific templates (`python.tmux`, `typescript.tmux`, `solidity.tmux`) all share a common structure: editor window, shell window, and git window. When `overlord add` or `overlord init` are used with `--base` or when no language is detected, they need a template to copy. This ticket is a **prerequisite** for the full functionality of `overlord add` and `overlord init`.

## Requirements

### Functional Requirements
- Create `~/.config/overlord/tmux/base.tmux` template
- Include the common workspace layout:
  - Editor window (nvim)
  - Shell window
  - Git window (with `git status`)
- Use same structure as language-specific templates:
  - Shebang: `#!/usr/bin/env bash`
  - MODE variable (default: override)
  - `set -euo pipefail`
  - Use `$TMUX_SESSION` and `$TMUX_PROJECT_DIR` variables
- No language-specific commented options
- Include generic commented options that might be useful:
  - Logs window
  - Additional shell windows
  - htop/monitoring window

### Non-Functional Requirements
- Follow exact same format as existing templates
- Make executable (`chmod +x`)
- Keep it minimal but extensible

## Current State
- `~/.config/overlord/tmux/` contains:
  - `python.tmux`
  - `typescript.tmux`
  - `solidity.tmux`
- No `base.tmux` exists
- `overlord add` and `overlord init` will need this for base configuration fallback

## Desired State
- `~/.config/overlord/tmux/base.tmux` exists with generic workspace layout
- `overlord add` and `overlord init` can use it when no language is detected/specified

## Research Context

### Keywords to Search
- `TMUX_SESSION` - Session name variable
- `TMUX_PROJECT_DIR` - Project root variable
- `copy_tmux_template` - Function that copies templates
- `.tmux.local` - Destination filename

### Patterns to Investigate
- Common structure in `python.tmux` lines 1-19 (shared across all templates)
- MODE variable usage (line 4)
- Window creation pattern: `tmux new-window -t "$S:" -n <name> -c "$ROOT"`
- Window selection at end: `tmux select-window -t "${S}:editor"`

### Key Decisions Made
- Template provides editor + shell + git (the universal trio)
- No language-specific options (that's the point of base)
- Include generic commented options for common use cases
- Same MODE=override default as other templates

## Success Criteria

### Automated Verification
- [x] File exists at `~/.config/overlord/tmux/base.tmux`
- [x] File is executable
- [x] File contains required variables (`TMUX_SESSION`, `TMUX_PROJECT_DIR`)
- [x] File follows same structure as other templates

### Manual Verification
- [ ] Can be sourced without errors
- [ ] When used with `overlord open`, creates expected windows
- [ ] Editor, shell, and git windows function correctly

## Related Information
- Location: `~/.config/overlord/tmux/base.tmux`
- Reference templates: `python.tmux`, `typescript.tmux`, `solidity.tmux`
- Blocker for: `feature_overlord_add.md`, `feature_overlord_init.md`

## Notes
- This is a simple ticket but critical as a dependency
- Could be implemented as part of `overlord add` or `overlord init` tickets
- Alternatively, could be added to `overlord-migrate-init` or a setup script

## Implementation

The template should look like this:

```bash
#!/usr/bin/env bash
# Base project tmux layout (no language-specific options)
# MODE can be "override" (default) or "merge"
MODE=override
set -euo pipefail

S="$TMUX_SESSION"
ROOT="$TMUX_PROJECT_DIR"

# Rename the starter window (created by overlord-open)
tmux rename-window -t "${S}:base" "editor"
tmux send-keys -t "${S}:editor" "nvim" C-m

# Shell window
tmux new-window -t "$S:" -n shell -c "$ROOT"

# Git window
tmux new-window -t "$S:" -n git -c "$ROOT"
tmux send-keys -t "${S}:git" "git status" C-m

# --- Generic options ---
# Additional shell window
# tmux new-window -t "$S:" -n shell2 -c "$ROOT"

# Logs window
# tmux new-window -t "$S:" -n logs -c "$ROOT"
# tmux send-keys -t "${S}:logs" "tail -f /var/log/syslog" C-m

# System monitoring
# tmux new-window -t "$S:" -n htop -c "$ROOT"
# tmux send-keys -t "${S}:htop" "htop" C-m

# Docker logs (if applicable)
# tmux new-window -t "$S:" -n docker -c "$ROOT"
# tmux send-keys -t "${S}:docker" "docker compose logs -f" C-m

tmux select-window -t "${S}:editor"
```

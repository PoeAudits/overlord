---
type: feature
priority: medium
created: 2025-12-19T00:00:00Z
status: archived
tags: [overlord, list, interactive, ui]
keywords: [overlord list, interactive editing, project states, tmux ui]
patterns: [interactive selection, state management, registry updates]
---

# FEATURE: Interactive project state editor for overlord list

## Description
Add an interactive `--edit` / `-e` flag to `overlord list` that displays all projects in the registry with an interactive UI allowing users to navigate, toggle project states, and apply changes in bulk.

## Context
Currently, moving projects between states (active, lib, archive) requires individual `overlord mv` commands. An interactive editor would streamline bulk state management and provide visual feedback of all changes before applying them.

## Requirements
- Add `--edit` / `-e` flag to `overlord list` command
- Display all projects in format matching current `overlord list --all` output
- Support navigation with arrow keys (↑/↓) AND vim keybindings (j/k)
- Support state toggling with Enter key OR vim keybindings (h/l)
  - h: previous state (archive → lib → active → archive)
  - l: next state (active → lib → archive → active)
  - Enter: same as l (next state)
- Show confirmation prompt at bottom with pending changes summary
- Apply all changes atomically to registry on confirmation
- Exit immediately after applying changes
- Exit without changes on cancel/escape

## Current State
`overlord list` displays projects but requires manual `overlord mv` commands for state changes.

## Desired State
Users can launch an interactive session with `overlord list --edit`, see all projects, modify multiple states, review changes, and apply them all at once.

## Research Context

### Keywords to Search
- overlord list - Current list implementation
- overlord mv - State transition logic
- registry updates - Atomic registry modifications
- interactive selection - UI patterns for terminal navigation

### Patterns to Investigate
- Current registry update patterns in overlord-mv
- How overlord-list formats output
- Terminal UI interaction patterns (used in other scripts)
- Confirmation/prompt patterns in existing commands

## Success Criteria

### Automated Verification
- [ ] `overlord list --edit` launches without errors
- [ ] `overlord list -e` works as alias
- [ ] Registry updates are atomic (all or nothing)
- [ ] Invalid state transitions are prevented

### Manual Verification
- [ ] Arrow keys navigate up/down through project list
- [ ] j/k keys navigate up/down through project list
- [ ] h/l keys cycle through states in correct direction
- [ ] Enter key cycles to next state
- [ ] Confirmation shows all pending changes
- [ ] Changes apply correctly to registry after confirmation
- [ ] Exit occurs immediately after applying changes
- [ ] Escape/cancel exits without modifying registry
- [ ] Display format matches `overlord list --all` style

## Out of Scope
- Changes to non-interactive `overlord list` output
- New sorting or filtering options in interactive mode
- Display customization beyond current list format
- Keyboard shortcuts beyond navigation (j/k/↑/↓) and toggle (h/l/Enter)
- Mouse support

## Notes
- Use existing registry update patterns from overlord-mv for atomicity
- Consider terminal size and scrolling for large project lists
- Ensure clear visual indication of current selection
- Display state change clearly in confirmation (e.g., "project-name: active → lib")

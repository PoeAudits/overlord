---
type: feature
priority: medium
status: open
created: 2025-12-26
---
# Update overlord thoughts directory structure

## What
When `overlord init` creates a new project, it should initialize the thoughts directory with the full standard structure: `tickets/`, `research/`, `plans/`, `proposals/`, `specs/`, `logs/`, `reviews/`, and `archive/` — matching the OpenCode thoughts directory guide.

## Why
Projects need to support all three workflows (Quick Flow, AI Development Flow, Spec Flow) with proper organizational structure from initialization. Current setup only creates partial directories.

## Success Criteria
- [ ] `overlord init` creates all 8 subdirectories: `tickets/`, `research/`, `plans/`, `proposals/`, `specs/`, `logs/`, `reviews/`, `archive/`
- [ ] Existing behavior preserved (directory creation is additive, non-destructive)
- [ ] `overlord sync` propagates missing directories to existing projects (additive, non-destructive)
- [ ] All directories are created empty on first `overlord init`

## Notes
- Migration through `overlord sync --all` command (non-destructive)
- No changes to existing projects until explicit `overlord sync`
- Aligns with OpenCode thoughts directory structure guide

---
status: accepted
---
# Overlord v2 is a from-scratch rewrite; GitHub replaces the local project registry

Projects live on GitHub and local clones are disposable worktrees created by T3 Code, so the registry of categories, languages, paths and statuses, together with tmux Workspaces, `.tmux.local`, language Makefiles, `opencode.jsonc`, archive and backup, has no role. The new code replaces the tree on `PoeAudits/overlord` without carrying old code forward, because the old design encoded a terminal-and-tmux workflow the user no longer has. Overlord keeps three jobs: project Setup, Harness Sync, and machine startup for T3 Code.

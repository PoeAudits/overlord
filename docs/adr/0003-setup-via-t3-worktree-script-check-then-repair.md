---
status: accepted
---
# Setup runs from T3 Code's worktree-create script, checks first, and repairs only what it owns

T3 Code runs the first project script flagged `runOnWorktreeCreate` after creating a worktree for a new thread, with the worktree as working directory and `T3CODE_PROJECT_ROOT` / `T3CODE_WORKTREE_PATH` in the environment. Overlord registers `overlord setup` as a `defaultProjectScripts` entry in `~/.t3/userdata/settings.json` with `async: false`, so setup finishes before the agent starts and applies to every project, including ones added earlier. A `t3.json` in the repo is not executed by T3 (it is only importable from the UI), so it is not used. so new or incomplete projects are set up before the agent starts and without the user typing commands. Setup first checks the project, then creates missing files and refreshes only its own marked blocks (header/footer-delimited, versioned) when they are out of date. It never edits text outside those blocks. We rejected Harness session-start hooks as the setup trigger because they depend on the model acting and differ per Harness; hooks are used only for Sync.

## Consequences

- A project's committed files are the contract; a fresh clone works in every Harness even without Overlord.
- Project-specific rules must be written outside Overlord's marked blocks, and the templates leave an explicit area for them.
- T3 runs only one worktree-create script, and a per-project scripts override replaces the global list. Sync therefore reports any project whose override lacks the Overlord entry, and project-specific post-clone commands (installs, env files) must be chained behind `overlord setup` rather than added as a second script.
- Setup must exit zero after repairs and non-zero only on real failure, because with `async: false` a non-zero exit blocks the thread from starting.
- T3 also fires the script for threads in the main checkout (project root passed as the worktree path), which is harmless because setup is idempotent.

---
status: accepted
---
# Setup runs only when the user invokes it; T3 Code never triggers it

This supersedes ADR 0003's trigger. Sync used to register `overlord setup` as a T3 Code `defaultProjectScripts` entry with `runOnWorktreeCreate: true`, so T3 ran Setup every time it created a worktree for a thread. Once Setup could create a private GitHub repository and refuse to continue on a Secret in history (ADR 0006), that made opening a thread able to create a repository or block the thread. Setup is now an Overlord command the user runs deliberately, per Project (`overlord setup`, or the project-setup skill); Sync registers nothing in T3, and removes the `overlord-setup` entry earlier Syncs added to the global `defaultProjectScripts`, leaving every other entry and setting alone. The rest of ADR 0003 stands: Setup checks first, creates what is missing, refreshes only its own Managed Blocks, and never edits text outside them.

## Consequences

- A new T3 worktree of a Project that is not set up stays that way until the user runs Setup; committed files remain the contract, so a Project set up once is set up in every clone.
- Sync removes the entry only from the global list. An `overlord-setup` entry a user copied into a Project's own scripts override is theirs, and Sync leaves it.
- Doctor reports an `overlord-setup` entry still in the global list as drift; it no longer checks Project scripts overrides.

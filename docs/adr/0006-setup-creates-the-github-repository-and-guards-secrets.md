---
status: accepted
---
# Setup creates a Project's missing GitHub repository, but never pushes a Secret

ADR 0005 makes GitHub the Project's truth, yet only `overlord new` created a repository, so directories set up in place stayed local or were pushed by hand, and agents committed `.env` files because nothing ignored them. Setup now guarantees an origin: when a Project has none, it runs `git init` if needed, creates the private repository `<githubOwner>/<directory name>` with `gh repo create --source --remote origin`, and pushes every branch and tag. An existing origin is never touched, whoever owns it. A name that already exists on GitHub is used only when it is empty, since attaching to a repository with content would mix two histories; otherwise the item fails. Before creating or pushing anything, Setup lists every path that any branch's or tag's history has tracked and refuses, pushing nothing, if one matches the Secret patterns (`templates.Secrets`); a pushed Secret cannot be recalled, and rewriting history is the Project's call. The check is by path only; content scanning was rejected as slow and noisy. The same patterns head a `.gitignore` Managed Block that `new` and Setup write, and Setup lists, without untracking, tracked files that block ignores.

## Consequences

- Setup runs only when the user invokes it (ADR 0007), so a GitHub repository is created only by a deliberate `overlord setup` or `overlord new`, never by opening a T3 thread.
- A Secret in history, a non-empty repository with the same name, or a failed push is a real failure: Setup exits 1 until the history is cleaned or the origin is added by hand. A gh failure that says nothing about the repository (offline, not logged in) only warns.
- A failed push after the origin was created is not retried, since Setup then finds an origin; the message says to push by hand.
- The `.gitignore` block goes at the top of an existing file, so a Project's own lines, such as `!.vscode/launch.json`, can override it.

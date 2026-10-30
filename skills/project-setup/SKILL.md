---
name: project-setup
description: Check and repair the files Overlord guarantees in a Project (AGENTS.md block, coding standards, GLOSSARY.md, ADRs, triage labels) with `overlord setup`, then offer to commit. Use when the user asks to set up, check or refresh a project, or when those files are missing or out of date.
---

# Project setup

Setup checks one Project checkout against the items Overlord guarantees, creates what is missing, refreshes Overlord's own Managed Blocks when their Template changed, and leaves every other byte alone. T3 Code already runs `overlord setup` on every new worktree; this skill runs the same thing on demand and shows the user what it did.

Run every command in the checkout you are working in. In a T3 thread that is the working directory; pass a path (`overlord setup <path>`) only to set up a different checkout.

## Steps

1. **Check.** Run `overlord setup --check`. It changes nothing and exits 0. Done when you have the plan: one line per item, read with [the plan](#the-plan) below.
2. **Show the plan.** Tell the user which items are `missing` or `stale`, and name every `conflict` and `warning` with its reason. When every line is `ok`, say the Project is already set up and stop here.
   Items missing in a T3 worktree mean T3 did not run Setup when it created it: run `overlord doctor` (read-only) and pass on any `scripts override without overlord-setup, so Setup does not run there` finding for this Project.
3. **Apply.** Run `overlord setup`. Done when it exits 0 and prints `Setup applied to <path>`. A non-zero exit is a real failure (a Template failed to render or a path could not be written): report the `overlord: ...` line from stderr and stop.
4. **Report.** Summarise the `created` and `refreshed` lines. Repeat each `conflict` and what the user would have to move or delete for Setup to own that path; Setup never resolves a conflict itself. Repeat each `warning`; Setup still succeeded.
5. **Offer to commit.** Run `git status --short` and show the changed files. Ask the user whether to commit them; on yes, `git add` exactly those files and commit with the message `Overlord setup`. Done when the user has answered and, on yes, `git status` is clean for those files.

## The plan

Both runs print a heading, then one line per item: the status word, the path, and a reason in parentheses for conflicts, warnings and labels.

```
Setup plan for /home/me/projects/sample
  missing    AGENTS.md#agents
  stale      docs/coding-standards.md#lang-go
  ok         GLOSSARY.md
  conflict   .claude/skills (real directory with content; left untouched)
  missing    triage labels (needs-info, wontfix)
```

A `#name` suffix is a Managed Block inside that file. `triage labels` is the five GitHub labels on the `origin` repo, not a file.

| `--check` | after `overlord setup` | Meaning |
| --- | --- | --- |
| `missing` | `created` | Absent; Setup writes it (or the block, or the listed labels). |
| `stale` | `refreshed` | A Managed Block differs from its Template, a directory's `.gitkeep` is wrong, a symlink points elsewhere, or a generated agent file is out of date. Setup replaces the block wholesale, so edits made inside it are lost; text outside the block is kept. |
| `ok` | `ok` | Already as guaranteed. |
| `conflict` | `conflict` | Something Overlord does not own sits at the path (a real directory where the symlink goes, a hand-written agent file, a block with no end marker). Left untouched. |
| `warning` | `warning` | Setup could not check or create the triage labels: no GitHub `origin`, or `gh` failed. Setup carries on and exits 0. |

`overlord setup --check --json` prints the same plan as JSON, `{"root": ..., "items": [{"path", "block", "strategy", "status", "detail", "missingLabels"}]}`, with `block`, `detail` and `missingLabels` present only when set. The JSON always uses the `--check` words, even without `--check`.

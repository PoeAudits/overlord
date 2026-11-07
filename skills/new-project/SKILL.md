---
name: new-project
description: Create a brand-new Project with `overlord new`, a GitHub repo that arrives cloned, set up, committed and registered in T3 Code. Use when the user asks to create, start or make a new project or repository.
---

# New project

`overlord new <name>` turns one sentence ("create a new project called X") into a working Project. Run it from any directory.

## Steps

1. **Settle the inputs.** Take the name from the user: a single directory name, used as the GitHub repo name and the T3 project title. Take the language, one of `go`, `typescript`, `python` or `rust`; it is required, so ask if the user did not say. The repo is private unless the user asks for public. Add a description only if the user gave one.
2. **Run it.**

   ```sh
   overlord new <name> --lang <go|typescript|python|rust> [--description "<text>"] [--public]
   ```

   `<projectsRoot>` is `projectsRoot` and `<owner>` is `githubOwner` in `~/.config/overlord/config.yaml` (defaults `~/Overlord/projects` and `PoeAudits`). It runs these steps in order and stops at the first failure:

   1. Refuses if `<projectsRoot>/<name>` already exists, or `--lang` is missing or unknown, before touching GitHub.
   2. `gh repo create <owner>/<name> --private` (or `--public`, plus `--description`).
   3. `gh repo clone <owner>/<name> <projectsRoot>/<name>`.
   4. Writes the language's marker file in the clone: `go.mod` (module `github.com/<owner>/<name>`, the Go version Overlord was built with), `package.json`, `pyproject.toml`, or `Cargo.toml` with an empty `src/lib.rs`.
   5. Setup in the clone, printing the plan as `overlord setup` does (`Setup applied to ...`, then `created` / `ok` / `warning` lines; see the `project-setup` skill for each word). The marker makes Setup add the language's coding standards and the CI workflow. `gh label list` is retried a few times over about six seconds while GitHub finishes creating the repo. If any item is `failed`, or the triage labels line is a `warning`, it stops here, before committing.
   6. `git add -A`, `git commit -m "Initial project setup"`, `git push -u origin HEAD`.
   7. `~/.t3/bin/t3 project add --title <name> <projectsRoot>/<name>`.

   Done when it exits 0 and its last line is `Created <owner>/<name> at <path> and registered it with T3 Code`.
3. **Tell the user.** The repo `<owner>/<name>` exists on GitHub with the first commit pushed, and `<name>` is now a T3 Code project: open a new thread in it to start work. Its first commit already holds the language's marker file, coding standards and CI workflow.

## When it fails

A failure prints one line to stderr, `overlord: <command> failed: <first line of its output, else its exit status and error output>`, `overlord: setup in <path> could not set up every item; see the failed lines above` (the plan above it has a `failed` line, with its error, for each item), `overlord: setup in <path> could not set up the triage labels: <gh's error>` (the plan above it ends with the matching `warning` line), `overlord: unknown language "<lang>"; use go, typescript, python or rust`, or `overlord: <path> already exists; choose another name or remove it`. Report it, then go by the failed command, because the earlier steps have already happened and rerunning `overlord new` would stop at the existing repo or directory:

- **Refused, or `gh repo create` failed**: nothing was created. Fix the cause (another name, `gh auth login`) and rerun.
- **A later step failed**: the GitHub repo exists. Fix the cause and finish the remaining steps above by hand, in order, in `<projectsRoot>/<name>` (run `overlord setup` there for step 5; the marker file from step 4 is already in the clone).

## What Setup guarantees

Every item is committed in the first commit, so a fresh clone works in every Harness without Overlord:

- `AGENTS.md` with Overlord's Managed Block pointing agents at the glossary, ADRs, coding standards and Project skills. Project rules go outside the block.
- `docs/coding-standards.md`: a general block, one block per detected language, and a `## Project-specific standards` section for the Project's own rules.
- `GLOSSARY.md`, and `docs/adr/` with a starter ADR (lint and tests must pass before merge).
- `docs/agents/issue-tracker.md`, `triage-labels.md` and `domain.md`, so the issue and triage skills work without their own setup run.
- `.agents/skills/` for Project skills, with `.claude/skills` linking to it.
- `.gitignore` with Overlord's Managed Block at the top: the Secret patterns, common dependency and cache directories, and OS and editor files, then a block for the `--lang` language when it has one.
- `.github/workflows/ci.yml` running lint and tests for the `--lang` language.
- The five triage labels on the GitHub repo (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`).

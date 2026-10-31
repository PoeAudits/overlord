# Overlord Project Guide

Overlord is a small Go CLI with three jobs: Setup a Project (check it for the
files Overlord guarantees and repair only what Overlord owns), Sync the machine
(apply skills, Roles and global instructions from `~/.agents` to every Harness)
and Start the machine (Sync, then bring up the services T3 Code needs). The
spec is GitHub issue #2. Read `GLOSSARY.md` and `docs/adr/` before naming
things or changing decided behaviour.

## Boundaries

- All external effects go through `system.System`: `Home` (every path under
  the user's home is derived from it), `Getenv`, `Workdir`, `Runner` (external
  programs such as `gh`, `git`, `t3`, run or launched detached), `Health`
  (HTTP health checks), `Sleep` and `Stdout`/`Stderr`. Never call
  `os.UserHomeDir`, `os.Getenv`, `os.Getwd` or `os/exec` outside
  `internal/system`.
- Tests drive commands through `cmd.Execute(sys, args)` with a temporary
  `Home`, a temporary Project directory, a fake `Runner` and, for `start`, a
  fake `Health`. Assert on the
  resulting files, the output and the recorded commands, never on internal
  calls. Never touch the real home directory, real Harness config or real
  services in tests.
- Setup never edits text outside a Managed Block, and `--check` never writes.
- What Setup guarantees is data in `internal/templates/manifest.yaml`; adding a
  guaranteed file is a manifest and Template change, not new code.

## Architecture

- `cmd/overlord/main.go`: process entry point only; builds `system.OS()` and
  calls `cmd.Execute`.
- `internal/system`: the injected seams (`System`, `Runner`, `ExecRunner`,
  `HealthChecker`, `HTTPHealth`).
- `internal/config`: Overlord's own config, `~/.config/overlord/config.yaml`.
  `Load(home)` returns it with paths absolute (defaults when the file is
  absent, never writes); `WriteDefault(home)` writes the defaults once.
- `internal/cmd`: thin Cobra wiring. Commands: `setup`, `sync`, `start`,
  `doctor`, `new-project`.
- `internal/templates`: embedded Templates and the Setup manifest
  (`Manifest()` returns the guaranteed items and their strategy).
- `internal/project`: Setup for one Project, and `Create` for `new-project`.
  `Check(ctx, runner, root, projectRoot)` returns a `Plan` with a `missing`,
  `stale`, `ok`, `conflict` or `warning` status per manifest item; languages
  are detected in `projectRoot`, files are written under `root`.
  `Apply` tries every item and marks each one it cannot write `failed`, with
  the error as its detail; `setup` prints the whole plan, then exits 1 when
  `Plan.HasFailures`, and `Create` stops before committing.
  `setup --check` exits 1 when `Plan.NeedsWork` (anything but `ok` or
  `warning`); `sync --check` likewise exits 1 when Sync would change anything
  or finds a conflict, but not for a `shadowed` skill.
- `internal/catalog`: reads `~/.agents` (skills, Roles, global instructions).
- `internal/harness`: one adapter per Harness (paths, agent file format, model
  config writer, hook installer).
- `internal/jsonedit`: in-place JSON edits that keep every other byte, shared
  by `internal/harness` and `internal/t3`.
- `internal/machine`: Sync and Start.
- `internal/t3`: edits T3 Code's settings.
- `skills/`: the `new-project` and `project-setup` skills, laid out like
  `~/.agents/skills`; the test checks their frontmatter and that every
  `overlord <command>` they name exists.

Packages not yet present are created by the ticket that needs them.

## Validation

Run before every commit:

```sh
go test ./...
go vet ./...
gofmt -l .        # must print nothing
go build ./...
git diff --check
```

`make check` runs all five. `make build` writes the ignored `overlord` binary.

## Agent skills

### Issue tracker

Issues are tracked as GitHub Issues on `PoeAudits/overlord` via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default triage vocabulary: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

# Overlord v2 Sync Simplification Implementation Checklist

> Created: 2026-02-07
> Source plan: `~/.config/opencode/dev/sync-simplification-plan.md`
> Status: Ready for execution

## Goal

Implement a Syncthing-first sync architecture where project files and registry artifacts are synced via Syncthing, git is removed from operational sync flows, and registry writes are protected by explicit lock ownership.

## Guardrails

- Keep command surface minimal.
- Do not reintroduce git push/pull in sync-critical command paths.
- Any new CLI command must be added to `Makefile` shortcuts.
- Preserve idempotent, non-destructive behavior.

## Phase 0 - Preflight and cleanup

- [ ] Confirm current baseline behavior of:
  - [ ] `overlord activate <name>`
  - [ ] `overlord deactivate <name>`
  - [ ] `overlord sync`
- [ ] Add migration note in this folder if historical context is needed.

## Phase 1 - Remove git coupling from sync paths

### Code changes

- [ ] Remove `commitAndPushRegistry()` invocation from `internal/cmd/activate.go`.
- [ ] Remove `commitAndPushRegistry()` invocation from `internal/cmd/deactivate.go`.
- [ ] Audit other registry-mutating sync paths and remove auto git commit/push behavior.
- [ ] Keep registry `Load/Save` behavior intact (atomic write + validation).

### Validation

- [ ] `activate/deactivate` still update registry fields correctly.
- [ ] No git push/pull is triggered by sync commands.
- [ ] No SSH password prompts occur from sync commands.

## Phase 2 - Registry lock model (single writer)

### Data model

- [ ] Add lock artifact file support (target path: `~/.config/overlord/registry.lock.yaml`).
- [ ] Define lock schema fields:
  - [ ] `owner_machine`
  - [ ] `updated_at`
  - [ ] `note` (optional)
- [ ] Add validation and read/write helpers for lock artifact.

### Command enforcement

- [ ] Enforce lock ownership for all registry-mutating commands:
  - [ ] `activate`
  - [ ] `deactivate`
  - [ ] `new`
  - [ ] `add`
  - [ ] any additional command that writes `registry.yaml`
- [ ] Implement denied-write error message with:
  - [ ] current lock owner
  - [ ] local machine name
  - [ ] exact transfer command to run

### Lock operations

- [ ] Implement lock visibility command (for example: `overlord sync lock show`).
- [ ] Implement explicit transfer command (for example: `overlord sync lock transfer <machine>`).
- [ ] Ensure transfer updates lock artifact and prints confirmation.
- [ ] Update `Makefile` with short command targets for lock operations.

### Validation

- [ ] Local owner can write registry.
- [ ] Non-owner receives actionable failure.
- [ ] Transfer changes owner and unlocks writes on target machine.

## Phase 3 - Bulk activation/deactivation

### Command behavior

- [ ] Add bulk activate support:
  - [ ] `overlord activate --all`
  - [ ] `overlord activate <name1> <name2> ...`
- [ ] Add matching bulk deactivate behavior.
- [ ] Keep existing single-project behavior unchanged.

### UX and safety

- [ ] Multi-project output is concise and readable.
- [ ] Per-project failures are clearly reported.
- [ ] Command returns non-zero on hard failures while showing partial progress.

### Validation

- [ ] Bulk activate updates registry and Syncthing state as expected.
- [ ] Bulk deactivate updates registry and Syncthing state as expected.
- [ ] Lock ownership checks are applied consistently in bulk mode.

## Phase 4 - Bootstrap + reconcile integration

### Bootstrap behavior

- [ ] Update bootstrap flow to require Syncthing seed for:
  - [ ] `registry.yaml`
  - [ ] `registry.lock.yaml`
- [ ] Add wait/timeout behavior for initial artifact arrival.
- [ ] Keep bootstrap idempotent.

### Reconcile/provision behavior

- [ ] Ensure active projects are provisioned on first run.
- [ ] Ensure expected Syncthing folders/devices exist.
- [ ] Never auto-delete remote project directories in reconcile path.

### Validation

- [ ] Fresh machine reaches usable state without git pull/push.
- [ ] Re-running bootstrap has no destructive side effects.

## Phase 5 - Doctor and verification

- [ ] Implement one doctor-style verification command (final command name to match chosen CLI naming).
- [ ] Doctor checks include:
  - [ ] machine config validity
  - [ ] Syncthing API connectivity
  - [ ] registry presence/readability
  - [ ] lock artifact validity/owner visibility
  - [ ] folder/path consistency for sync-active projects
- [ ] Ensure doctor output is concise with clear remediation hints.

## Tests and quality gates

- [ ] Add/adjust unit tests for lock model and command guards.
- [ ] Add/adjust command tests for bulk activate/deactivate behavior.
- [ ] Add bootstrap-related tests where feasible.
- [ ] Run full test suite:
  - [ ] `go test ./...`
- [ ] Run local smoke checks for sync commands in a safe environment.

## Documentation updates (required before close)

### Overlord repo docs

- [ ] Update `README.md` in `overlord-v2`:
  - [ ] remove git-based registry sync wording
  - [ ] add lock model and transfer commands
  - [ ] add bulk activate/deactivate usage
- [ ] Update `AGENTS.md` in `overlord-v2`:
  - [ ] remove outdated GitOps sync assumptions
  - [ ] document lock artifact and write-guard pattern
  - [ ] document any new command files/modules

### System docs in `~/.config/opencode/dev/`

- [ ] Update `dev/overview.md`:
  - [ ] reflect Syncthing-first registry + project sync
  - [ ] include lock ownership/transfer workflow
  - [ ] refresh quick-reference commands/status
- [ ] Update `dev/overlord.md`:
  - [ ] remove GitOps from sync-critical path description
  - [ ] add lock model details and denied-write behavior
  - [ ] document bulk activate/deactivate and doctor command
- [ ] Update `dev/dotfiles.md`:
  - [ ] describe Syncthing-seeded bootstrap for registry + lock artifacts
  - [ ] clarify chezmoi-managed vs Syncthing-synced runtime files
  - [ ] remove instructions implying git pull/push for registry sync
- [ ] Update `dev/plan.md` with a superseded note for old git-based registry sync assumptions.

## Completion checklist

- [ ] All phases complete.
- [ ] No sync command triggers git push/pull.
- [ ] Lock ownership model works end-to-end.
- [ ] Fresh machine bootstrap verified.
- [ ] Docs updated and consistent.
- [ ] Final rollout notes written (migration caveats + rollback steps).

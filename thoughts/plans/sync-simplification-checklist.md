# Overlord v2 Sync Simplification Implementation Checklist

> Created: 2026-02-07
> Last updated: 2026-02-07
> Source plan: `~/.config/opencode/dev/sync-simplification-plan.md`
> Status: Ready for execution

## Goal

Implement a Syncthing-first sync architecture where project files and registry artifacts are synced via Syncthing across multiple machines (main, vps, runner, aux). Git is removed from operational sync flows, registry writes are protected by explicit lock ownership, and non-sensitive peer info is shared via a synced peer registry.

## Guardrails

- Keep command surface minimal.
- Do not reintroduce git push/pull in sync-critical command paths.
- Any new CLI command must be added to `Makefile` shortcuts.
- Preserve idempotent, non-destructive behavior.
- All Syncthing communication over Tailscale only.
- Dependencies excluded from all synced folders (`.stglobalignore`).

## Machine roles

| Role | Chezmoi Mode | Description |
|------|-------------|-------------|
| **main** | desktop | Primary workstation, all projects, source of truth, default lock owner |
| **vps** | server | Headless remote, resource-constrained, active/targeted projects |
| **runner** | desktop | Always-on with display, script execution, selective projects |
| **aux** | desktop | Mobile primary, similar to main, active/targeted projects |

## Implementation order

Desktop (main) first, then rebuild VPS to validate bootstrap. Laptop and NUC added after VPS validates the flow.

---

## Phase 0 — Preflight and cleanup

- [x] Confirm current baseline behavior of:
  - [x] `overlord activate <name>`
  - [x] `overlord deactivate <name>`
  - [x] `overlord sync`
- [x] Add migration note in this folder if historical context is needed.

## Phase A — Remove git coupling from sync paths

### Code changes

- [x] Remove `commitAndPushRegistry()` invocation from `internal/cmd/activate.go`.
- [x] Remove `commitAndPushRegistry()` invocation from `internal/cmd/deactivate.go`.
- [x] Audit other registry-mutating sync paths and remove auto git commit/push behavior.
- [x] Keep registry `Load/Save` behavior intact (atomic write + validation).

### Migration

- [x] Remove `.git/` directory from `~/.config/overlord/` on desktop.
- [x] Delete stale plan: `~/Overlord/core/tools/overlord-v2/thoughts/plans/sync-extension.md`.

### Validation

- [x] `activate/deactivate` still update registry fields correctly.
- [x] No git push/pull is triggered by sync commands.
- [ ] No SSH password prompts occur from sync commands.

## Phase B — Peer registry and machine config split

### Data model

- [x] Define `peers.yaml` schema:
  - [x] `peers.<name>.role` (main/vps/runner/aux)
  - [x] `peers.<name>.syncthing.device_id`
  - [x] `peers.<name>.syncthing.address`
  - [x] `peers.<name>.base_dir`
- [x] Add read/write helpers for `peers.yaml` (atomic write, validation).
- [x] Add peer types/structs (Go code in `internal/machine/` or new `internal/peers/` package).

### Machine config refactor

- [x] Refactor `machine.yaml` to contain only local-sensitive fields:
  - [x] `name`
  - [x] `role`
  - [x] `syncthing.api_url`
  - [x] `syncthing.api_key`
- [x] Remove peer entries from `machine.yaml` struct.
- [ ] Update `internal/machine/config.go`:
  - [x] `MachineConfig` struct: add `Role` field, remove `Peers` from `SyncthingConfig`.
  - [x] Add `LoadPeers()` function for `peers.yaml`.
  - [x] Add merged view: machine config + peer info for a given machine name.
- [x] Update all callers that access peer info to use new `peers.yaml` path.

### Migration

- [x] Create initial `peers.yaml` from existing `machine.yaml` peer entries on desktop.
- [x] Verify overlord commands still work after the split.

### Validation

- [x] `peers.yaml` loads and validates correctly.
- [x] `machine.yaml` loads without peer entries.
- [x] Commands that need peer info (activate, deactivate, sync) resolve peers from `peers.yaml`.

## Phase C — Registry lock model (single writer)

### Data model

- [x] Add lock artifact file support (path: `~/.config/overlord/registry.lock.yaml`).
- [x] Define lock schema fields:
  - [x] `owner` (machine name, matches key in `peers.yaml`)
  - [x] `updated_at` (ISO 8601)
  - [x] `note` (optional)
- [x] Add validation and read/write helpers for lock artifact.

### Command enforcement

- [x] Enforce lock ownership for all registry-mutating commands:
  - [x] `activate`
  - [x] `deactivate`
  - [x] `new`
  - [x] `add`
  - [x] `rm`
  - [x] `archive` / `unarchive`
  - [x] any additional command that writes `registry.yaml`
- [x] Implement denied-write error message with:
  - [x] current lock owner name
  - [x] local machine name
  - [x] exact transfer command to run
  - [x] exact claim command if owner might be offline

### Lock operations

- [x] Implement `overlord sync lock show`:
  - [x] Display owner, timestamp, note.
  - [x] Indicate if local machine is the owner.
- [x] Implement `overlord sync lock transfer <machine>`:
  - [x] Validate target exists in `peers.yaml`.
  - [x] Update lock artifact with new owner and timestamp.
  - [x] Print confirmation and next-step reminder.
- [x] Implement `overlord sync lock claim`:
  - [x] Overwrite lock with local machine as owner.
  - [x] Print warning about potential conflict with old owner.
  - [x] Print confirmation.
- [x] Update `Makefile` with short command targets for lock operations.

### Validation

- [ ] Local owner can write registry.
- [ ] Non-owner receives actionable failure with transfer/claim guidance.
- [ ] Transfer changes owner and unlocks writes on target machine.
- [ ] Force-claim overwrites lock locally; old owner sees denial on next write.

## Phase D — `~/.config/overlord/` as Syncthing folder

### Setup

- [x] Add `~/.config/overlord/` as a Syncthing shared folder via REST API.
- [x] Create `.stignore` in `~/.config/overlord/` excluding `machine.yaml`.
- [x] Share folder with all known peers from `peers.yaml`.
- [x] Ensure folder ID is deterministic (e.g., `overlord-config`).

### Integration

- [x] Add helper function to programmatically add the overlord config folder to Syncthing.
- [x] Use same Syncthing client patterns as project folder management.
- [x] Ensure this runs during `overlord setup --init`.

### Validation

- [ ] `registry.yaml` propagates between paired machines.
- [ ] `registry.lock.yaml` propagates between paired machines.
- [ ] `peers.yaml` propagates between paired machines.
- [ ] `machine.yaml` is NOT synced.

## Phase E — Bulk activation/deactivation

### Command behavior

- [x] Add bulk activate support:
  - [x] `overlord activate --all`
  - [x] `overlord activate <name1> <name2> ...`
- [x] Add matching bulk deactivate behavior.
- [x] Keep existing single-project behavior unchanged.

### UX and safety

- [x] Multi-project output is concise and readable.
- [x] Per-project failures are clearly reported.
- [x] Command returns non-zero on hard failures while showing partial progress.
- [x] Lock ownership checks applied once at start, not per-project.

### Validation

- [ ] Bulk activate updates registry and Syncthing state as expected.
- [ ] Bulk deactivate updates registry and Syncthing state as expected.
- [ ] Lock ownership is enforced consistently in bulk mode.

## Phase F — Bootstrap and reconcile integration

### Install script changes

- [x] Update `install.sh` to prompt for Overlord machine name and role.
- [x] Update `install.sh` to prompt for one peer device ID (or blank for later).
- [x] Generate initial `machine.yaml` with local Syncthing API credentials during bootstrap.
- [x] Add `~/.config/overlord/` as Syncthing shared folder during bootstrap.
- [x] Create `.stignore` for overlord config folder during bootstrap.

### Overlord setup command

- [x] Implement `overlord setup --init` for standalone first-time configuration:
  - [x] Interactive prompts for name, role, peer device ID.
  - [x] Generate `machine.yaml`.
  - [x] Configure Syncthing folder for overlord config.
  - [x] Wait for synced artifacts (registry, peers, lock) with timeout.
  - [x] Run reconcile for active projects.
- [x] Ensure `--init` is idempotent (safe to re-run).

### Reconcile behavior

- [x] Ensure active projects are provisioned on first run.
- [x] Ensure expected Syncthing folders/devices exist.
- [x] Never auto-delete remote project directories in reconcile path.

### Chezmoi mode updates

- [x] Confirm chezmoi `mode` prompt works for: `desktop`, `server`.
- [x] Add overlord role prompt to chezmoi or install script (separate from mode).

### Validation

- [ ] Fresh machine reaches usable state without git pull/push.
- [ ] Re-running bootstrap has no destructive side effects.
- [ ] Wait/timeout for Syncthing seed works correctly.

## Phase G — Doctor and verification

- [x] Implement `overlord doctor` verification command.
- [x] Doctor checks include:
  - [x] Machine config validity (name, role present)
  - [x] Syncthing API connectivity (local)
  - [x] Registry presence and readability
  - [x] Lock artifact validity and owner visibility
  - [x] Peer registry loaded, all peers listed
  - [x] Syncthing peers reachable (device connected status)
  - [x] Folder/path consistency for sync-active projects
  - [x] `.stignore` present in overlord config folder
  - [x] Overlord config folder is a Syncthing shared folder
- [x] Concise output with clear remediation hints.
- [x] Update `Makefile` with doctor command target.

## Phase H — Disaster recovery backup

- [x] Implement `overlord backup` command.
- [x] Initialize backup git repo if not configured (one-time setup).
- [x] Commit `registry.yaml` + `peers.yaml` with timestamp message.
- [x] One-way push to private GitHub remote.
- [x] Never pull from remote.
- [x] Update `Makefile` with backup command target.

---

## Tests and quality gates

- [x] Add/adjust unit tests for peer registry load/save/validate.
- [x] Add/adjust unit tests for lock model and command guards.
- [x] Add/adjust command tests for bulk activate/deactivate behavior.
- [ ] Add bootstrap-related tests where feasible.
- [x] Run full test suite: `go test ./...`
- [ ] Run local smoke checks for sync commands in a safe environment.
- [ ] Validate VPS rebuild end-to-end (primary integration test).

## Documentation updates (required before close)

### Overlord repo docs

- [x] Update `README.md` in `overlord-v2`:
  - [x] Remove git-based registry sync wording.
  - [x] Add lock model and transfer/claim commands.
  - [x] Add bulk activate/deactivate usage.
  - [x] Add peer registry description.
  - [x] Add doctor and backup commands.
- [x] Update `AGENTS.md` in `overlord-v2`:
  - [x] Remove outdated GitOps sync assumptions.
  - [x] Document lock artifact and write-guard pattern.
  - [x] Document peer registry and machine config split.
  - [x] Document any new command files/modules.

### System docs in `~/.config/opencode/dev/`

- [x] Update `dev/overview.md`:
  - [x] 4-machine topology with roles.
  - [x] Syncthing-first registry + project sync.
  - [x] Lock ownership/transfer workflow.
  - [x] Updated quick-reference commands.
- [x] Update `dev/overlord.md`:
  - [x] Remove GitOps from sync-critical path description.
  - [x] Add peer registry (`peers.yaml`) schema.
  - [x] Add lock model section (artifact, commands, denied-write, force-claim).
  - [x] Document bulk activate/deactivate and doctor/backup commands.
  - [x] Update machine config schema (split from peer info).
- [x] Update `dev/dotfiles.md`:
  - [x] Syncthing-seeded bootstrap for registry + lock + peers.
  - [x] Chezmoi mode vs overlord role distinction.
  - [x] Install script changes for Overlord setup prompts.
  - [x] Clarify chezmoi-managed vs Syncthing-synced runtime files.
- [x] Update `dev/plan.md`:
  - [x] Superseded note for git-based registry sync assumptions.
  - [x] Cross-link to sync simplification plan.
- [x] Update `dev/tools.md`:
  - [x] Add lock, doctor, backup commands to Overlord section.
  - [x] Add `peers.yaml` and `registry.lock.yaml` to key paths.

### Stale cleanup

- [x] Delete `~/Overlord/core/tools/overlord-v2/thoughts/plans/sync-extension.md`.

## Completion checklist

- [x] All phases (A through H) complete.
- [x] No sync command triggers git push/pull.
- [ ] Lock ownership model works end-to-end.
- [ ] Peer registry shared via Syncthing.
- [ ] Fresh VPS rebuild verified end-to-end.
- [x] Docs updated and consistent.
- [x] Final rollout notes written (migration caveats + rollback steps).

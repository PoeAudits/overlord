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

- [ ] Confirm current baseline behavior of:
  - [ ] `overlord activate <name>`
  - [ ] `overlord deactivate <name>`
  - [ ] `overlord sync`
- [ ] Add migration note in this folder if historical context is needed.

## Phase A — Remove git coupling from sync paths

### Code changes

- [ ] Remove `commitAndPushRegistry()` invocation from `internal/cmd/activate.go`.
- [ ] Remove `commitAndPushRegistry()` invocation from `internal/cmd/deactivate.go`.
- [ ] Audit other registry-mutating sync paths and remove auto git commit/push behavior.
- [ ] Keep registry `Load/Save` behavior intact (atomic write + validation).

### Migration

- [ ] Remove `.git/` directory from `~/.config/overlord/` on desktop.
- [ ] Delete stale plan: `~/Overlord/core/tools/overlord-v2/thoughts/plans/sync-extension.md`.

### Validation

- [ ] `activate/deactivate` still update registry fields correctly.
- [ ] No git push/pull is triggered by sync commands.
- [ ] No SSH password prompts occur from sync commands.

## Phase B — Peer registry and machine config split

### Data model

- [ ] Define `peers.yaml` schema:
  - [ ] `peers.<name>.role` (main/vps/runner/aux)
  - [ ] `peers.<name>.syncthing.device_id`
  - [ ] `peers.<name>.syncthing.address`
  - [ ] `peers.<name>.base_dir`
- [ ] Add read/write helpers for `peers.yaml` (atomic write, validation).
- [ ] Add peer types/structs (Go code in `internal/machine/` or new `internal/peers/` package).

### Machine config refactor

- [ ] Refactor `machine.yaml` to contain only local-sensitive fields:
  - [ ] `name`
  - [ ] `role`
  - [ ] `syncthing.api_url`
  - [ ] `syncthing.api_key`
- [ ] Remove peer entries from `machine.yaml` struct.
- [ ] Update `internal/machine/config.go`:
  - [ ] `MachineConfig` struct: add `Role` field, remove `Peers` from `SyncthingConfig`.
  - [ ] Add `LoadPeers()` function for `peers.yaml`.
  - [ ] Add merged view: machine config + peer info for a given machine name.
- [ ] Update all callers that access peer info to use new `peers.yaml` path.

### Migration

- [ ] Create initial `peers.yaml` from existing `machine.yaml` peer entries on desktop.
- [ ] Verify overlord commands still work after the split.

### Validation

- [ ] `peers.yaml` loads and validates correctly.
- [ ] `machine.yaml` loads without peer entries.
- [ ] Commands that need peer info (activate, deactivate, sync) resolve peers from `peers.yaml`.

## Phase C — Registry lock model (single writer)

### Data model

- [ ] Add lock artifact file support (path: `~/.config/overlord/registry.lock.yaml`).
- [ ] Define lock schema fields:
  - [ ] `owner` (machine name, matches key in `peers.yaml`)
  - [ ] `updated_at` (ISO 8601)
  - [ ] `note` (optional)
- [ ] Add validation and read/write helpers for lock artifact.

### Command enforcement

- [ ] Enforce lock ownership for all registry-mutating commands:
  - [ ] `activate`
  - [ ] `deactivate`
  - [ ] `new`
  - [ ] `add`
  - [ ] `rm`
  - [ ] `archive` / `unarchive`
  - [ ] any additional command that writes `registry.yaml`
- [ ] Implement denied-write error message with:
  - [ ] current lock owner name
  - [ ] local machine name
  - [ ] exact transfer command to run
  - [ ] exact claim command if owner might be offline

### Lock operations

- [ ] Implement `overlord sync lock show`:
  - [ ] Display owner, timestamp, note.
  - [ ] Indicate if local machine is the owner.
- [ ] Implement `overlord sync lock transfer <machine>`:
  - [ ] Validate target exists in `peers.yaml`.
  - [ ] Update lock artifact with new owner and timestamp.
  - [ ] Print confirmation and next-step reminder.
- [ ] Implement `overlord sync lock claim`:
  - [ ] Overwrite lock with local machine as owner.
  - [ ] Print warning about potential conflict with old owner.
  - [ ] Print confirmation.
- [ ] Update `Makefile` with short command targets for lock operations.

### Validation

- [ ] Local owner can write registry.
- [ ] Non-owner receives actionable failure with transfer/claim guidance.
- [ ] Transfer changes owner and unlocks writes on target machine.
- [ ] Force-claim overwrites lock locally; old owner sees denial on next write.

## Phase D — `~/.config/overlord/` as Syncthing folder

### Setup

- [ ] Add `~/.config/overlord/` as a Syncthing shared folder via REST API.
- [ ] Create `.stignore` in `~/.config/overlord/` excluding `machine.yaml`.
- [ ] Share folder with all known peers from `peers.yaml`.
- [ ] Ensure folder ID is deterministic (e.g., `overlord-config`).

### Integration

- [ ] Add helper function to programmatically add the overlord config folder to Syncthing.
- [ ] Use same Syncthing client patterns as project folder management.
- [ ] Ensure this runs during `overlord setup --init`.

### Validation

- [ ] `registry.yaml` propagates between paired machines.
- [ ] `registry.lock.yaml` propagates between paired machines.
- [ ] `peers.yaml` propagates between paired machines.
- [ ] `machine.yaml` is NOT synced.

## Phase E — Bulk activation/deactivation

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
- [ ] Lock ownership checks applied once at start, not per-project.

### Validation

- [ ] Bulk activate updates registry and Syncthing state as expected.
- [ ] Bulk deactivate updates registry and Syncthing state as expected.
- [ ] Lock ownership is enforced consistently in bulk mode.

## Phase F — Bootstrap and reconcile integration

### Install script changes

- [ ] Update `install.sh` to prompt for Overlord machine name and role.
- [ ] Update `install.sh` to prompt for one peer device ID (or blank for later).
- [ ] Generate initial `machine.yaml` with local Syncthing API credentials during bootstrap.
- [ ] Add `~/.config/overlord/` as Syncthing shared folder during bootstrap.
- [ ] Create `.stignore` for overlord config folder during bootstrap.

### Overlord setup command

- [ ] Implement `overlord setup --init` for standalone first-time configuration:
  - [ ] Interactive prompts for name, role, peer device ID.
  - [ ] Generate `machine.yaml`.
  - [ ] Configure Syncthing folder for overlord config.
  - [ ] Wait for synced artifacts (registry, peers, lock) with timeout.
  - [ ] Run reconcile for active projects.
- [ ] Ensure `--init` is idempotent (safe to re-run).

### Reconcile behavior

- [ ] Ensure active projects are provisioned on first run.
- [ ] Ensure expected Syncthing folders/devices exist.
- [ ] Never auto-delete remote project directories in reconcile path.

### Chezmoi mode updates

- [ ] Confirm chezmoi `mode` prompt works for: `desktop`, `server`.
- [ ] Add overlord role prompt to chezmoi or install script (separate from mode).

### Validation

- [ ] Fresh machine reaches usable state without git pull/push.
- [ ] Re-running bootstrap has no destructive side effects.
- [ ] Wait/timeout for Syncthing seed works correctly.

## Phase G — Doctor and verification

- [ ] Implement `overlord doctor` verification command.
- [ ] Doctor checks include:
  - [ ] Machine config validity (name, role present)
  - [ ] Syncthing API connectivity (local)
  - [ ] Registry presence and readability
  - [ ] Lock artifact validity and owner visibility
  - [ ] Peer registry loaded, all peers listed
  - [ ] Syncthing peers reachable (device connected status)
  - [ ] Folder/path consistency for sync-active projects
  - [ ] `.stignore` present in overlord config folder
  - [ ] Overlord config folder is a Syncthing shared folder
- [ ] Concise output with clear remediation hints.
- [ ] Update `Makefile` with doctor command target.

## Phase H — Disaster recovery backup

- [ ] Implement `overlord backup` command.
- [ ] Initialize backup git repo if not configured (one-time setup).
- [ ] Commit `registry.yaml` + `peers.yaml` with timestamp message.
- [ ] One-way push to private GitHub remote.
- [ ] Never pull from remote.
- [ ] Update `Makefile` with backup command target.

---

## Tests and quality gates

- [ ] Add/adjust unit tests for peer registry load/save/validate.
- [ ] Add/adjust unit tests for lock model and command guards.
- [ ] Add/adjust command tests for bulk activate/deactivate behavior.
- [ ] Add bootstrap-related tests where feasible.
- [ ] Run full test suite: `go test ./...`
- [ ] Run local smoke checks for sync commands in a safe environment.
- [ ] Validate VPS rebuild end-to-end (primary integration test).

## Documentation updates (required before close)

### Overlord repo docs

- [ ] Update `README.md` in `overlord-v2`:
  - [ ] Remove git-based registry sync wording.
  - [ ] Add lock model and transfer/claim commands.
  - [ ] Add bulk activate/deactivate usage.
  - [ ] Add peer registry description.
  - [ ] Add doctor and backup commands.
- [ ] Update `AGENTS.md` in `overlord-v2`:
  - [ ] Remove outdated GitOps sync assumptions.
  - [ ] Document lock artifact and write-guard pattern.
  - [ ] Document peer registry and machine config split.
  - [ ] Document any new command files/modules.

### System docs in `~/.config/opencode/dev/`

- [ ] Update `dev/overview.md`:
  - [ ] 4-machine topology with roles.
  - [ ] Syncthing-first registry + project sync.
  - [ ] Lock ownership/transfer workflow.
  - [ ] Updated quick-reference commands.
- [ ] Update `dev/overlord.md`:
  - [ ] Remove GitOps from sync-critical path description.
  - [ ] Add peer registry (`peers.yaml`) schema.
  - [ ] Add lock model section (artifact, commands, denied-write, force-claim).
  - [ ] Document bulk activate/deactivate and doctor/backup commands.
  - [ ] Update machine config schema (split from peer info).
- [ ] Update `dev/dotfiles.md`:
  - [ ] Syncthing-seeded bootstrap for registry + lock + peers.
  - [ ] Chezmoi mode vs overlord role distinction.
  - [ ] Install script changes for Overlord setup prompts.
  - [ ] Clarify chezmoi-managed vs Syncthing-synced runtime files.
- [ ] Update `dev/plan.md`:
  - [ ] Superseded note for git-based registry sync assumptions.
  - [ ] Cross-link to sync simplification plan.
- [ ] Update `dev/tools.md`:
  - [ ] Add lock, doctor, backup commands to Overlord section.
  - [ ] Add `peers.yaml` and `registry.lock.yaml` to key paths.

### Stale cleanup

- [ ] Delete `~/Overlord/core/tools/overlord-v2/thoughts/plans/sync-extension.md`.

## Completion checklist

- [ ] All phases (A through H) complete.
- [ ] No sync command triggers git push/pull.
- [ ] Lock ownership model works end-to-end.
- [ ] Peer registry shared via Syncthing.
- [ ] Fresh VPS rebuild verified end-to-end.
- [ ] Docs updated and consistent.
- [ ] Final rollout notes written (migration caveats + rollback steps).

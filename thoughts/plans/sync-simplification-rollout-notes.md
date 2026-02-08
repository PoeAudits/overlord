# Sync Simplification Rollout Notes

Date: 2026-02-08
Plan: `thoughts/plans/sync-simplification-checklist.md`

## What rolled out

- Syncthing-first sync flow for project folders and Overlord registry artifacts.
- Registry write lock model (`registry.lock.yaml`) with `show`, `transfer`, `claim` commands.
- Shared peer registry (`peers.yaml`) split from local-sensitive `machine.yaml`.
- Bulk sync lifecycle commands (`activate --all`, `activate <n1> <n2> ...`, matching deactivate).
- Bootstrap and reconcile path in `overlord setup --init`.
- Health diagnostics (`overlord doctor`) and one-way DR snapshots (`overlord backup`).

## Migration actions applied (desktop)

- Removed legacy git metadata from `~/.config/overlord/`.
- Migrated legacy peer entries from `machine.yaml` into `peers.yaml`.
- Normalized machine/peer role values to `main|vps|runner|aux`.
- Initialized lock owner with `overlord sync lock claim`.

## Known caveats

- Syncthing API auth must be valid in local `machine.yaml` (`api_url`, `api_key`) or `doctor` reports failures.
- Multi-machine lock and propagation behavior still requires explicit real-world validation before final production sign-off.
- `install.sh` now invokes `overlord setup --init`; this requires `overlord` to be installed and available in `PATH`.

## Rollback strategy

1. Restore desktop config artifacts from migration backups:
   - `~/.local/share/overlord/migration-backups/migration-*/machine.yaml.pre-migration`
   - `~/.local/share/overlord/migration-backups/migration-*/peers.yaml.pre-migration` (if present)
2. Restore legacy git metadata if needed:
   - Extract `~/.local/share/overlord/migration-backups/overlord-config-git-*.tar.gz` into `~/.config/overlord/`
3. Revert code/docs to previous commit in the Overlord repo if command behavior regression appears.
4. Re-run sanity checks:
   - `overlord sync lock show`
   - `overlord doctor`
   - `go test ./...`

## Post-rollout operator checklist

- Run lock ownership handoff test between desktop and VPS.
- Verify `registry.yaml`, `peers.yaml`, `registry.lock.yaml` Syncthing propagation.
- Confirm `machine.yaml` remains local-only.
- Execute VPS rebuild validation playbook once and archive outcomes.

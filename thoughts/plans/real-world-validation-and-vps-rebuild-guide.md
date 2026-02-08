# Real-World Validation and VPS Rebuild Guide

Date: 2026-02-08
Scope: validate Syncthing-first Overlord architecture in production-like conditions.

## Preconditions

- Desktop has latest Overlord changes and valid `~/.config/overlord/{registry.yaml,peers.yaml,registry.lock.yaml}`.
- Desktop local config has valid Syncthing API credentials in `machine.yaml`.
- VPS is reachable over Tailscale SSH.
- Backup snapshots are available (`overlord backup`).

---

## Part A — Desktop validation before VPS wipe

1. Confirm lock owner and local health:
   - `overlord sync lock show`
   - `overlord doctor`
2. Run command sanity:
   - `overlord activate <known-project>`
   - `overlord sync <known-project>`
   - `overlord deactivate <known-project>`
3. Validate no git sync coupling:
   - Confirm no sync command performs git pull/push.
4. Take DR snapshot:
   - `overlord backup`

Pass criteria:
- Commands succeed without SSH password prompts.
- Doctor failures (if any) are expected and understood.

---

## Part B — Wipe and recreate VPS

1. In Hetzner Cloud Console:
   - Power off existing VPS.
   - Delete server.
   - Create fresh VPS (Ubuntu 24.04, same size/profile).
2. Reattach operational metadata:
   - Assign expected hostname.
   - Add SSH key(s).
3. Install baseline access:
   - SSH into new VPS.
   - Install and connect Tailscale.

Pass criteria:
- New VPS reachable as `ssh vps` (or MagicDNS name).

---

## Part C — Rebootstrap VPS (chezmoi + Overlord)

1. Run bootstrap:
   - `curl -fsSL https://raw.githubusercontent.com/PoeAudits/dotfiles/main/install.sh | bash -s -- --mode server`
2. During prompts:
   - Provide machine role `vps`.
   - Provide optional peer device ID (desktop) if requested.
3. Script will run `overlord setup --init`:
   - confirm machine config creation
   - confirm `overlord-config` Syncthing folder setup
   - confirm lock/registry/peers seed wait behavior
   - confirm reconcile phase executes

Pass criteria:
- `~/.config/overlord/registry.yaml` and `peers.yaml` present on VPS.
- `machine.yaml` present only locally on VPS.

---

## Part D — Multi-machine lock tests

From desktop:
1. `overlord sync lock show` (desktop should own lock initially)
2. `overlord sync lock transfer vps`

From VPS:
3. `overlord sync lock show` (owner should now be `vps`)
4. Run a registry-mutating command (`overlord activate <project>`)

From desktop:
5. Run registry-mutating command and confirm denied-write guidance
6. `overlord sync lock claim` (simulate owner offline)

Pass criteria:
- Ownership transfer and denial behavior are correct on both machines.

---

## Part E — Propagation and folder consistency

1. On lock owner machine, activate 1-2 projects:
   - `overlord activate <p1> <p2>`
2. On both machines, verify:
   - `overlord sync`
   - `overlord doctor`
3. Confirm artifacts propagate:
   - `registry.yaml`
   - `peers.yaml`
   - `registry.lock.yaml`
4. Confirm exclusions:
   - `machine.yaml` does not appear/overwrite across peers.

Pass criteria:
- Syncthing project folders and status align with registry `sync.status`.

---

## Part F — Recovery confidence checks

1. Run `overlord backup` from lock owner.
2. Confirm latest commit appears in backup remote repo.
3. Capture final verification notes:
   - lock owner transitions
   - doctor output snapshots
   - unresolved warnings/issues

---

## Failure handling playbook

- **403 from Syncthing API**: verify `machine.yaml` `api_url`/`api_key`, test in Syncthing UI.
- **Lock conflicts**: use `sync lock transfer` if owner online, `sync lock claim` if offline.
- **Missing artifacts after bootstrap**: rerun `overlord setup --init`, inspect Syncthing folder sharing and peers.
- **Unexpected registry behavior**: restore from migration backup and rerun stepwise tests.

---

## Sign-off template

- Date/time:
- Operator:
- Desktop doctor status:
- VPS doctor status:
- Lock transfer test: PASS/FAIL
- Force-claim test: PASS/FAIL
- Artifact propagation test: PASS/FAIL
- `machine.yaml` exclusion test: PASS/FAIL
- Bulk activate/deactivate test: PASS/FAIL
- DR backup push test: PASS/FAIL
- Notes / follow-ups:

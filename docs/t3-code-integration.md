# T3 Code Integration Notes

This records how T3 Code was integrated into Overlord before removal, so the integration can be recreated later if needed.

## Goal

Overlord made T3 Code the phone-facing interface for the local OpenCode server. The intended path was:

1. `overlord start` starts the local OpenCode server.
2. `overlord start` starts the T3 Code server as a separate service.
3. T3 Code is configured to use the local OpenCode server as its provider.
4. Tailscale Serve exposes the T3 Code HTTP server to the Tailnet.
5. `overlord start pair` prints a phone-ready `/pair#token=...` URL and terminal QR code.

OpenCode remained the model/session backend. T3 Code supplied the browser/phone UI, session cookie auth, and pairing UX.

## Overlord Integration Points

### Stack Settings

File: `internal/stack/settings.go`

The integration added a `t3code` service name and settings:

- `host`, default `127.0.0.1`
- `port`, default `3773`
- `workdir`, default `/home/thomas/Overlord/core/tools/code`
- `base_dir`, default `~/.local/share/t3code`
- `command`, optional command override

The exported helpers were:

- `T3CodeWorkDir(cfg)`: expanded T3 Code repository path for shelling out to the T3 CLI.
- `T3CodeBaseDir(cfg)`: expanded T3 Code state directory for auth/session storage.

The Tailscale default target was changed to `T3CodeURL(cfg)`, which made T3 Code the remote Tailnet service instead of the OpenCode server.

### Endpoint Resolution

File: `internal/stack/endpoints.go`

The integration used:

- `T3CodeURL(cfg)`: local URL, `http://<host>:<port>`.
- `RemoteT3CodeURL(cfg)`: resolves the current Tailscale HTTPS URL by reading `tailscale serve status --json` and matching the configured local T3 URL.

The existing generic helper `remoteURLForTarget(localURL)` performed the Tailscale Serve status parsing.

### Service Startup

File: `internal/stack/service.go`

`DefaultServices` included the `t3code` service after `opencode` and before `frontend`/`tailscale`.

The T3 service command did three things:

1. Verified `apps/server/dist/bin.mjs` exists in the T3 repository.
2. Seeded `~/.local/share/t3code/userdata/settings.json` with the local OpenCode server URL at `providers.opencode.serverUrl`.
3. Ran the T3 server:

```sh
node apps/server/dist/bin.mjs serve --host 127.0.0.1 --port 3773 --base-dir ~/.local/share/t3code
```

The service set `T3CODE_TAILSCALE_SERVE=false` so T3 did not try to configure Tailscale itself. Overlord owned Tailscale Serve.

The T3 health check used:

```text
/.well-known/t3/environment
```

It accepted 2xx and 3xx responses because protected T3 routes may redirect while still proving the server is alive.

### Tailscale Pane Pairing Output

File: `internal/stack/service.go`

The Tailscale service command configured Serve and then printed the pairing block:

```sh
tailscale serve --bg 'http://127.0.0.1:3773'
overlord start pair
exec ${SHELL:-sh}
```

The command retried `overlord start url --remote` briefly before printing pairing output, giving Tailscale Serve time to publish the HTTPS route.

Keeping the pane open with `exec ${SHELL:-sh}` made the QR visible in tmux after startup.

### Pairing Command

File: `internal/cmd/start_pair.go`

The integration added:

```sh
overlord start pair
```

The command:

1. Loaded the stack config.
2. Resolved the remote Tailscale URL with `stack.RemoteT3CodeURL(cfg)`.
3. Fell back to the local T3 URL if remote resolution failed.
4. Ran the T3 Code CLI from the configured T3 repo:

```sh
node apps/server/dist/bin.mjs auth pairing create \
  --base-dir <t3 base dir> \
  --base-url <remote or local url> \
  --headless-output
```

`NODE_NO_WARNINGS=1` was set for this subprocess so Node experimental SQLite warnings did not appear before the QR code.

Overlord did not generate tokens or QR codes itself. It delegated those details to T3 Code.

## T3 Code Changes

Repository: `/home/thomas/Overlord/core/tools/code`

### CLI Pairing Output

File: `apps/server/src/cli/auth.ts`

The `auth pairing create` command gained:

- `--base-url`: existing public URL used to build a `/pair#token=...` link.
- `--qr`: prints a terminal QR code for the pairing URL.
- `--headless-output`: prints the same ready-to-connect output used by headless server startup.

The implementation reused T3 helpers from `apps/server/src/startupAccess.ts`:

- `buildPairingUrl`
- `formatHeadlessServeOutput`
- `renderTerminalQrCode`

This kept token formatting and QR rendering inside T3 Code instead of duplicating that policy in Overlord.

The command quieted logs for JSON, QR, and headless output so auth logs would not corrupt machine-readable output or QR displays.

### Pairing Semantics

T3 Code owned pairing and auth:

- Pairing token TTL was short-lived, defaulting to T3's existing setting.
- The QR encoded the full URL: `https://<tailnet-host>/pair#token=<token>`.
- The web `/pair` route read the token from the URL hash, stripped it from the address bar, called the auth bootstrap endpoint, and then relied on the normal T3 session cookie.

No tokenless Tailnet auth was added. Tailscale only transported the protected T3 server.

### OpenCode Adapter Fix

File: `apps/server/src/provider/Layers/OpenCodeAdapter.ts`

Separately from Overlord startup, the T3 OpenCode adapter was fixed for OpenCode delta-only streams. OpenCode 1.15 emitted `message.part.delta` events followed by `session.status: idle`, without a final `message.updated` or `message.part.updated`. T3 previously lost assistant text in that case.

The fix buffered assistant text deltas and synthesized pending assistant text when the session became idle, before completing the turn. The regression test was added in `apps/server/src/provider/Layers/OpenCodeAdapter.test.ts`.

## Standalone Launcher

File: `/home/thomas/Overlord/core/tools/t3-launcher/t3`

The standalone `t3` command remained separate from Overlord. It was updated to avoid competing with the Overlord-owned server:

- It checked the T3 health endpoint before doing build or tmux work.
- If a healthy T3 server already existed, it printed `status: already-running`.
- If tmux session `StartServer` existed, it printed `owner: likely-overlord`.
- It printed the local URL, detected Tailscale remote URL, and instructed the user to run `overlord start pair` for a phone-ready QR.

This prevented a duplicate `T3Sidecar` session when Overlord was already running T3.

## Tests And Validation

Overlord tests covered:

- `overlord start pair` CLI argument construction and fallback behavior.
- T3 service command construction.
- Tailscale service command invoking `overlord start pair`.
- Exported path helpers for T3 work/base directories.

T3 Code tests covered:

- Headless pairing output with QR code.
- Existing startup access formatting.
- OpenCode adapter delta-only completion behavior.

Validation commands used:

```sh
go test ./...
bun run --cwd apps/server test src/bin.test.ts src/startupAccess.test.ts src/cliAuthFormat.test.ts
bun run --cwd apps/server test src/provider/Layers/OpenCodeAdapter.test.ts
bun run --cwd apps/server typecheck
bun run --cwd apps/server build
bash -n /home/thomas/Overlord/core/tools/t3-launcher/t3
```

Manual checks used:

```sh
overlord start pair
```

## Removal Checklist

To remove the integration from Overlord while preserving OpenCode server support:

1. Remove `internal/cmd/start_pair.go` and its tests.
2. Remove `t3code` settings and exported T3 path helpers from `internal/stack/settings.go`.
3. Remove `T3CodeURL` and `RemoteT3CodeURL` unless still needed elsewhere.
4. Remove `t3CodeService` from `internal/stack/service.go` and from `DefaultServices`.
5. Change the Tailscale default target back to `OpenCodeURL(cfg)` or to the replacement desktop integration target.
6. Remove T3-specific Tailscale pane pairing output.
7. Update service tests to reflect the new service list and Tailscale target.
8. Remove or archive the standalone `t3` launcher if it is no longer wanted.
9. Leave the T3 Code repository changes in their own repo unless intentionally reverting that repo too.

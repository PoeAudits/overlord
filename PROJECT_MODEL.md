# Overlord Project Model

## Purpose and lifecycle

Overlord is a retained, frequently used single-host Go CLI. Its dependable core
maintains the local Project catalog, creates Projects, resolves their paths, and
lists them for human and AI use. It also contains older Workspace/tmux,
workstation-startup, archive, and backup capabilities accumulated early in the
system's development. Those capabilities are usable but are not automatically
the right long-term boundaries. Its current supported behavior is the source,
tests, [README](README.md), and [agent guide](AGENTS.md), not historical plans.

Overlord supplies host infrastructure to Alice; it does **not** own Alice
coordination truth, workflow policy, Efforts, Work Items, Runs, Conversations,
or the Coordination Journal. Live CLI commands can mutate the registry, tmux
sessions, files, and managed services. Treat operational commands as live
actions, not read-only inspection.

The current operating contract is one computer, one filesystem, one human
operator, and effectively one mutating Overlord process at a time. Concurrent
read-only path/list operations are normal; multi-writer and multi-machine
coordination are not current requirements.

## Ownership and boundaries

Overlord owns:

- Registry Project identity, category/language/status validation, name/alias
  resolution, and Project path lookup/creation.
- Workspace placement and the tmux sessions it creates for registered Projects.
- The `StartServer` tmux session, enabled local stack-service windows, targeted
  restart, health/status inspection, and stack stop behavior.
- Registry-tagged source backups and archive/unarchive moves.

A **Project** is a logical registry entry. A **Workspace** is a particular
checkout or worktree opened for a Run; they are not interchangeable. Overlord
does not own a Project's source, git remote/history, Alice's state, OpenCode's
state, Phoenix data, or speech-outbox contents. The `speech-outbox` service is
only a stack adapter, even though it serves Alice-adjacent functionality.

## Current architecture

- `cmd/overlord`: process entry point.
- `internal/cmd`: Cobra CLI orchestration, lifecycle hooks, and user-facing
  failures.
- `internal/registry`: version-2 YAML model, validation, resolution, and
  atomic persistence.
- `internal/fileutil`: path expansion, atomic writes, and directory moves.
- `internal/tmux`: the testable tmux command adapter used for Workspaces and
  the stack.
- `internal/stack`: stack YAML, service adapters, service lifecycle, restart,
  stop, and endpoint helpers.
- `internal/templates`: embedded files used by `overlord new`.

The registry and tmux command runner are existing seams: registry mutation must
flow through load/save, and external processes should stay behind the injected
executor/adapter seams so fixture tests do not touch real state.

Other active entrypoints include `init`, which initializes a Project in the
current directory; `detect`, which reports detected Project metadata; and
`code`, which ensures the configured stack and attaches the current directory
to OpenCode. These are operational commands rather than coordination APIs, and
their exact flags and side effects remain defined by CLI help and source.

## Registry contract and persistence

The runtime registry source of truth is `~/.config/overlord/registry.yaml`.
It is YAML with `version: 2`, `settings.base_dir`, and a map of named Projects.
Each Project requires `path`, `category`, `lang`, a `YYYY-MM-DD` `created`
date, non-empty `description`, and `status.state` (`active` or `archived`).
`last_opened`, when present, is RFC3339. Names and aliases resolve exactly
before fuzzy matching; no match and equally scored ambiguity are errors.

Valid categories and their relative creation paths are:

| Category | Path under `base_dir` |
| --- | --- |
| `contracts` | `projects/contracts/` |
| `web` | `projects/web/` |
| `services` | `projects/services/` |
| `ml` | `projects/ml/` |
| `libs` | `projects/libs/` |
| `cli` | `projects/cli/` |
| `core-agents` | `core/agents/` |
| `core-tools` | `core/tools/` |
| `core-evals` | `core/evals/` |
| `sandbox` | `sandbox/` |
| `config` | `config/` |

Languages are `python`, `typescript`, `go`, `solidity`, and `base`. Relative
Project paths resolve below the expanded `settings.base_dir`; absolute Project
paths are retained as absolute. `~` and `~/...` expand, but `~user/...` is
rejected. Nested registered paths resolve to the most-specific containing
Project. `new` creates at the category path; `add` stores a relative path only
when the directory is under `base_dir`.

Loads return a default empty registry if the file is absent, but malformed or
invalid YAML fails. Saves validate first, make parent directories, preserve the
previous registry as `.bak` when present, then atomically replace the file.
Consequently, callers must handle a failed write; archive/unarchive can report
that a completed directory move and the registry are out of sync if persistence
then fails. `open` also updates `last_opened`, warning rather than preventing
the Workspace operation when that save fails.

## Workspace and tmux contract

`overlord open <name>` (and `overlord <name>`) resolves an active Project,
checks its directory, creates or reuses a tmux session named after the registry
Project, then attaches or switches the current tmux client. A newly created
session runs an optional project-local `.tmux.local` with `TMUX_SESSION` and
`TMUX_PROJECT_DIR`; its failure is a warning. Missing tmux, a missing Project
directory, a pre-open hook failure, or an unavailable session is surfaced with
a corrective error. Archived Projects cannot be opened.

Session names are therefore shared namespace inputs: conflicting manual tmux
sessions can be reused by name. Overlord creates and manages only the sessions
it addresses; it does not establish a general worktree manager or own every
tmux session on the host.

## Optional local stack

`~/.config/overlord/stack.yaml` is separate from the registry. Missing stack
configuration yields an in-memory version-1 default; commands that save it use
an atomic write and `.bak` replacement. Enabled default adapters start in this
order: `opencode`, `speech-outbox`, `frontend`,
`tailscale`, `phoenix`, and `ollama`. They run in named windows of the
`StartServer` tmux session. Separately, `overlord start` launches the installed
T3 Code (Nightly), ChatGPT, and OpenCode Desktop applications through their
desktop entries using `gtk-launch`. Launch failures are warnings; desktop apps
are not stack services and are not closed by `overlord start stop`.

`overlord start` creates the stack before opening configured Project Workspaces;
`overlord up` / `overlord start up` only ensures the stack. Starting an existing
`StartServer` is idempotent and reports enabled services as skipped rather than
recreating them. `overlord start restart <service...>` requires that session and
recreates only named, enabled service windows; unknown or disabled names fail.
`overlord start stop` kills `StartServer`. It does not reset Tailscale Serve:
existing routes persist because they can belong to unrelated workflows.

Health checks are short local HTTP checks where applicable and `tailscale serve
status` for Tailscale. `start status` is observational but can contact configured
local endpoints and invoke Tailscale. Service availability, package
compatibility, ports, commands, and configured working/data directories remain
host-dependent.

Phoenix is an optional local service adapter, not the Phoenix Project or its
data owner. Speech outbox runs `alice speech-outbox serve` with its directory
configured by `speech-outbox.settings.directory` (default
`~/.config/alice`); Overlord neither reads nor interprets that directory's
coordination data. OpenCode server ownership and application state remain
external.

The stack and tmux mechanisms are early practical implementations rather than
accepted future architecture. The owner relies on `overlord start` for initial
workstation setup, but it does not currently provide a satisfactory way to
reload an active OpenCode configuration consistently. A redesign must decide
whether startup, configuration reload, service lifecycle, and Workspace/tmux
control remain Overlord modules or become narrower owner adapters elsewhere.

## Backup and archive boundaries

`overlord backup` creates a timestamped zip in the home directory from Projects
tagged `backup`; `overlord backup --dry-run` plans without writing a zip. Git
Projects include tracked plus non-ignored untracked files; fallback traversal
filters generated/cache directories. `.env*` is always excluded, but a backup
is not a secrets-management, retention, restore, or off-host replication
system.

Backup is an occasionally used and valued portable export: the owner wants one
command that collects important local source/configuration for transfer to an
external drive. Its possible future role in moving or reproducing the broader
workstation/AOS setup is unresolved. Do not silently broaden the present
selective Project zip into machine restore or configuration migration without a
new manifest, inclusion, secret, verification, and restoration contract.

`archive` moves a Project directory to `archive/<name>` beneath `base_dir` and
marks it archived; `unarchive` moves it to the chosen category and marks it
active. Both affect files and registry state, and can partially complete before
a registry-save failure. `rm` normally removes only the registry entry; `rm
--force` and `clear` can delete directories after confirmation. These are live,
potentially destructive operations, not cleanup APIs for Alice.

## Setup and supported use

Go 1.25.5 or newer is required. tmux is required for Workspace and stack
operations; individual enabled services require their own installed binaries
and configured dependencies. From this Project root:

```sh
make build
./overlord --help
./overlord start --help
./overlord start restart --help
```

For a new host, `overlord setup` creates the config/base/category directories,
initial registry, and managed `oc` shim; it may interactively initialize or
clone a registry git repository. It is mutating and should not be used merely
to validate installation. Configure enabled stack services in `stack.yaml`
before calling `start`; do not assume the defaults match a host.

## Validation

Run these from the Project root:

```sh
make test       # unit and fixture/fake-executor integration coverage
make vet        # Go static analysis
make build      # compiles cmd/overlord to ignored ./overlord
go mod verify   # verifies module dependencies
git diff --check # detects whitespace errors in the working tree
```

For command/interface validation without reading or mutating the live registry,

```sh
go run ./cmd/overlord --help
go run ./cmd/overlord start --help
go run ./cmd/overlord start restart --help
go run ./cmd/overlord backup --help
```

Tests must use temporary homes, fixture registries, and fake command executors.
Do not use `list`, `path`, `info`, `status`, `start`, `open`, setup, backup, or
archive commands as supposedly safe validation against a real host: even
commands that appear read-only can load private configuration or inspect live
processes, and several commands mutate state.

## Generated outputs, privacy, and limitations

`overlord`, `overlord-linux`, `.benchmark/`, and `.opencode/` are ignored,
reproducible/local outputs and must not be committed. Registry and stack YAML,
their `.bak` files, local paths, service settings, backup zips, and application
data are host state, not repository artifacts. They may expose Project names,
paths, service configuration, or user data; do not embed them in documentation
or tests.

Known limitations: source tests use fixtures rather than live services; fuzzy
resolution has a recorded narrow nondeterministic no-candidate coverage gap;
backup is selective rather than a complete disaster-recovery system; and
archive/unarchive cannot make filesystem and registry changes transactional as
one operation. The local stack assumes host tools and does not prove external
service health beyond its checks.

## Relationships and documentation authority

Overlord is infrastructure used by Wayfinder's Alice coordinator, but Alice's
coordination truth belongs outside this Project. Wayfinder supplies the retained
Project vocabulary and coordination context; it is not a registry backend.
OpenCode is a managed optional service/client dependency. Phoenix is an optional
telemetry/UI service adapter. `speech-outbox` is an Alice-adjacent managed
process, not an ownership transfer of Alice state.

[README](README.md) is the primary user-facing command guide and
[AGENTS.md](AGENTS.md) is the repository execution boundary. Source and tests
are authoritative for current behavior; this model is optional-load orientation
and subordinate to them. The [readiness record](../../../config/wayfinder/efforts/production-readiness/issues/assets/013-ready-overlord/readiness-record.md)
is historical validation evidence, not mutable runtime state.

### Future direction (not current behavior)

Treat the current implementation as overhaul input, not a shape to preserve
through incremental hardening. Preserve the high-value Project catalog/create/
path/list behavior while designing deeper boundaries for:

- Project identity, metadata quality, discovery, creation, and validation.
- Workspace selection and lifecycle without assuming tmux names are the model.
- Workstation startup and reliable OpenCode configuration reload.
- Verified portable backup and, only if justified, workstation setup migration.
- Narrow machine-readable interfaces for future callers without transferring
  authorization, scheduling, or coordination truth into Overlord.

Archive/unarchive is currently low-use and should not drive the redesign. A
simple validation command may be useful, but it is not the product direction by
itself. No external coordination API, multi-machine ownership model,
transaction protocol, or Alice-state adapter exists today; future work must not
reinterpret local registry entries, tmux sessions, or stack configuration as
Alice truth.

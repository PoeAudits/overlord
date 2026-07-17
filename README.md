# Overlord

Overlord is a Go CLI for maintaining a versioned registry of development
Projects, resolving their paths, and opening their Workspaces in tmux. It can
also manage an optional local startup stack for OpenCode and related services.

Overlord does **not** coordinate Alice Efforts, Work Items, Runs, or
Conversations. It supplies Project/Workspace discovery and local process
infrastructure that a coordinator may use.

## Build and validate

Requires Go 1.25.5 or newer. Tmux is required only for workspace and startup
stack operations.

```sh
make test
make vet
make fmt
make build
```

Install the generated binary with `make install`.

Generated binaries (`overlord`, `overlord-linux`), benchmark reports
(`.benchmark/`), and local OpenCode runtime dependencies/state (`.opencode/`)
are intentionally ignored and are not source artifacts.

## Registry and Projects

The registry is stored at `~/.config/overlord/registry.yaml`. It is versioned
YAML with a base directory and named Project entries:

```yaml
version: 2
settings:
  base_dir: ~/Overlord
projects:
  example:
    path: projects/web/example
    category: web
    lang: typescript
    created: "2026-07-11"
    description: Example web Project
    aliases: [ex]
    tags: [demo]
    status:
      state: active
```

Relative Project paths resolve beneath `settings.base_dir`; absolute paths are
also supported. Registry saves validate data and use atomic replacement with a
backup. Tests and integrations must use temporary homes and fixture registries,
never the live registry.

Valid categories are `contracts`, `web`, `services`, `ml`, `libs`, `cli`,
`core-agents`, `core-tools`, `core-evals`, `sandbox`, and `config`. Valid
languages are `python`, `typescript`, `go`, `solidity`, and `base`.

Common commands:

```sh
overlord list
overlord path project-a project-b
overlord switch example
overlord info example
overlord add /path/to/project --category=web --ts
overlord new example --category=web --ts
overlord open example
overlord archive example
overlord unarchive example
overlord backup --dry-run
```

`overlord <name>` is shorthand for `overlord open <name>`. `open` resolves the
Project, creates or reuses its tmux Workspace session, sources `.tmux.local`
when present, and attaches or switches the current tmux client.

`overlord switch <name>` resolves a Project name, alias, or fuzzy match and
opens it in the current tmux session. It selects an existing window named for
the Project or creates one rooted at the Project directory:

```sh
overlord switch agentbench
```

Outside tmux, use `overlord open <name>` to open a Project Workspace.

## Startup stack

`overlord start` (alias `overlord up`) manages enabled services in the
`StartServer` tmux session. Supported service adapters currently include
OpenCode, OpenCode Desktop, speech outbox, a frontend, Tailscale Serve,
Phoenix, and Ollama. Configuration lives in
`~/.config/overlord/stack.yaml`; missing configuration receives safe defaults.

```sh
overlord start
overlord start status
overlord start list
overlord start restart opencode phoenix
overlord start stop
overlord start url
overlord start url --remote
overlord code
```

Startup preflight may skip an incompatible OpenCode Desktop package while
leaving the rest of the stack available. Targeted restart recreates only named,
enabled service windows. `overlord code` ensures the stack is available and
attaches the current directory to the configured OpenCode server.

T3 Code integration was removed. Historical T3 plans and instructions are not
current startup-stack documentation.

## Development layout

```text
cmd/overlord/       process entry point
internal/cmd/       CLI commands
internal/registry/  registry model, resolution, persistence
internal/stack/     managed startup stack
internal/tmux/      tmux adapter
internal/templates/ embedded Project templates
```

See `AGENTS.md` for repository boundaries and validation policy.

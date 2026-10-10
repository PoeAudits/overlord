# Overlord Project Guide

Overlord is a Go CLI that owns the local Project registry, resolves Project paths,
and opens Project Workspaces in tmux. It also manages the optional local startup
stack used by `overlord start` and `overlord code`.

## Boundaries

- `~/.config/overlord/registry.yaml` is the runtime source of truth. Never use the
  live registry in tests; use a temporary `HOME` and fixture registry.
- A Project is a logical registry entry. A Workspace is the checkout or worktree
  opened for a Run. Do not use these terms interchangeably.
- Overlord may create and manage tmux sessions for Project Workspaces and the
  startup stack. Tests must use fake command executors, not real tmux sessions.
- Overlord does not own Alice coordination, Efforts, Work Items, Runs,
  Conversations, or the Coordination Journal. The optional `speech-outbox`
  process is only a managed stack service.

## Architecture

- `cmd/overlord/main.go`: process entry point only.
- `internal/cmd`: Cobra commands and CLI orchestration.
- `internal/registry`: versioned registry model, validation, resolution, and
  atomic persistence.
- `internal/tmux`: tmux adapter used by commands and stack management.
- `internal/stack`: startup configuration, service adapters, lifecycle,
  restart, status, and stop behavior.
- `internal/templates`: embedded files for newly initialized Projects.

Keep registry mutation behind the registry load/save path, preserve atomic
writes, and keep external process calls behind the existing executor seams so
they remain fixture-testable.

## Validation

Run:

```sh
make test
make vet
make fmt
make build
git diff --check
```

`make build` creates the ignored `overlord` binary. Benchmark reports under
`.benchmark/`, local OpenCode runtime state under `.opencode/`, and cross-built
`overlord-linux` binaries are generated outputs and must not be committed.

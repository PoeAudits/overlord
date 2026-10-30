<!-- overlord:begin general v1 -->
## General

- Keep a function whole unless a part is reused or names a real concept. Do not extract single-use helpers.
- Inline a value that is used once. Prefer early returns to `else`.
- When a function has several validation branches, let the main function read as the happy path and put the supporting details in small helpers below it.
- Comment non-obvious constraints and surprising behaviour, not what the code already says.
- Validate input and handle external errors at the boundaries (CLI, config, network, external APIs). Trust internal types inside them.
- Do not add complexity for a speculative edge case without agreeing it first.
- Tests drive the public interface and assert observable behaviour. Never duplicate the implementation's logic in a test.
- Lint and tests pass before every merge.

Overlord manages this block and replaces it whenever its Template changes. Write Project rules under **Project-specific standards** below.
<!-- overlord:end general -->

<!-- overlord:begin lang-go v1 -->
## Go

- `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass.
- Return errors; do not panic. Wrap with `fmt.Errorf("...: %w", err)` when adding context.
- Accept interfaces and return concrete types. Define an interface where it is consumed.
- Pass `context.Context` as the first parameter to anything that blocks or does I/O.
- Prefer table-driven tests with `t.Run` subtests and golden files for rendered output.
<!-- overlord:end lang-go -->

<!-- overlord:begin lang-typescript v1 -->
## TypeScript

- Enable `strict` in `tsconfig.json`. Do not use `any`; use `unknown` and narrow it.
- Parse external data at the boundary into typed values.
- Model alternatives as discriminated unions and handle every variant.
- Prefer `const` and pure functions; avoid classes unless they hold state.
- The `lint` and `test` scripts in `package.json` pass.
<!-- overlord:end lang-typescript -->

<!-- overlord:begin lang-python v1 -->
## Python

- Manage the environment with `uv`; `ruff check` and `pytest` pass.
- Type-annotate public functions. Use dataclasses or typed models instead of loose dicts.
- Raise specific exceptions; never use a bare `except`.
- Keep modules importable without side effects.
<!-- overlord:end lang-python -->

<!-- overlord:begin lang-rust v1 -->
## Rust

- `cargo fmt --check`, `cargo clippy -- -D warnings` and `cargo test` pass.
- Return `Result` and propagate with `?`. Do not `unwrap` or `expect` outside tests and proven invariants.
- Use enums to make illegal states unrepresentable, and match exhaustively.
- Prefer borrowing to cloning unless ownership is needed.
<!-- overlord:end lang-rust -->

## Project-specific standards

Add rules for this Project here. Overlord never edits this section.

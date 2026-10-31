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

## Project-specific standards

Add rules for this Project here. Overlord never edits this section.

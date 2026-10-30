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

<!-- overlord:begin lang-typescript v1 -->
## TypeScript

- Enable `strict` in `tsconfig.json`. Do not use `any`; use `unknown` and narrow it.
- Parse external data at the boundary into typed values.
- Model alternatives as discriminated unions and handle every variant.
- Prefer `const` and pure functions; avoid classes unless they hold state.
- The `lint` and `test` scripts in `package.json` pass.
<!-- overlord:end lang-typescript -->

## Project-specific standards

Add rules for this Project here. Overlord never edits this section.

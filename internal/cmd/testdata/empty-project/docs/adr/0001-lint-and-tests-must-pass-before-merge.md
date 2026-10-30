---
status: accepted
---
# Lint and tests must pass before merge

A change merges only when the Project's lint and tests pass, locally and in CI. A failing check is fixed or the change is reverted; it is never merged around. Every agent and reviewer works from the same baseline, and the main branch stays releasable.

## Consequences

- CI runs lint and tests for every language the Project uses.
- Disabling or skipping a check needs a new ADR that says why.

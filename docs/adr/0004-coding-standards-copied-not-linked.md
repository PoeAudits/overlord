---
status: accepted
---
# Coding standards are copied into each project, not linked to the global preset

`docs/coding-standards.md` is written by Setup as a general section plus one section per detected language (Go, TypeScript, Python, Rust), each inside a versioned marked block, with a free area for project rules. A symlink to the global preset would keep one real copy, but the repo must be self-contained so the `code-review` Standards axis and CI work for anyone cloning it. This is the one deliberate exception to the "one real copy" rule; Setup refreshes the blocks when the preset version changes.

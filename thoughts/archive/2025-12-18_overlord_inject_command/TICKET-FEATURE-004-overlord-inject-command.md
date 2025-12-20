---
type: feature
priority: medium
created: 2025-12-18T00:00:00Z
status: archived
tags: [overlord, dependency-management, local-dependencies, python, typescript]
keywords: [overlord inject, path-based dependencies, local file dependencies, uv, pnpm, bun, pyproject.toml, package.json]
patterns: [dependency injection, registry lookup, language detection, configuration file modification, error handling]
---

# FEATURE-004: overlord inject command for local project dependencies

## Description
Add a new `overlord inject` command that allows developers to add registered Overlord projects as path-based local dependencies to their current project. This enables the use of shared libraries within a monorepo-like setup where code can be edited directly and have changes reflected immediately without publishing to package registries.

The command adds path-based dependency references to the target project's package/dependency configuration file (e.g., `package.json` for TypeScript, `pyproject.toml` for Python) so that imported code from the injected project can be properly resolved.

## Context
Developers often need to reuse code across multiple projects in the Overlord registry. Currently, they must either:
1. Publish libraries to remote registries (npm, PyPI)
2. Manually manage path-based references
3. Copy code between projects

The `overlord inject` command streamlines this by automating the dependency linking process, enabling local library development workflows.

## Requirements

### Functional Requirements
- **Command**: `overlord inject <project-name-or-alias>` - runs from current working directory
- **Lookup**: Resolve project name/alias from Overlord registry to get absolute path
- **Language Detection**: Auto-detect target project language (Python or TypeScript) from config files in current directory
- **Language Validation**: Error if injected project language doesn't match target project language
- **Dependency Addition**: Add path-based dependency reference to target project's config file
  - Python: Add to `pyproject.toml` 
  - TypeScript: Add to `package.json`
- **Idempotency**: If dependency already exists, do nothing (no error, no modification)
- **Alias Support**: Support both project names and aliases equally
- **Status Agnostic**: Work with projects in any status (active, lib, archive)
- **No Auto-Sync**: Command should not automatically trigger `uv sync`, `pnpm install`, `bun install`, etc.

### Non-Functional Requirements
- **Language Support**: Python and TypeScript only (Solidity deferred as future enhancement)
- **Path Format**: Use absolute paths for dependency references
- **Error Handling**: Clear error messages for:
  - Project not found in registry
  - Language mismatch between target and injected project
  - No valid config file found in current directory
- **User-Friendly**: Minimize command options, auto-detect where possible
- **No Dependency Resolution**: Do not attempt to resolve transitive dependencies - user responsible for managing those separately

## Current State
No dependency injection mechanism exists. Users cannot easily link local Overlord projects as path-based dependencies.

## Desired State
Users can run `overlord inject <project>` from any project directory, and the path-based dependency is automatically added to the appropriate config file. Users then run their language-specific sync commands (`uv sync`, `pnpm install`, `bun install`) and can import code from the injected project.

## Research Context

### Keywords to Search
- `overlord inject` - new command dispatcher entry point
- `registry.json` - project lookup and metadata retrieval
- `pyproject.toml` - Python dependency configuration format
- `package.json` - TypeScript dependency configuration format
- `detect_language` - existing language detection logic in common.sh
- `path-based dependencies` - how uv, pnpm, bun handle file: protocol or path references
- `jq` - JSON manipulation for registry queries and file updates

### Patterns to Investigate
- **Command Structure**: How existing overlord subcommands (overlord-add, overlord-list, overlord-open) handle argument parsing and registry queries
- **Language Detection**: Existing `detect_language` function in `lib/common.sh` - understand current implementation
- **Config File Modification**: How to safely modify `pyproject.toml` (TOML parsing) and `package.json` (JSON parsing)
- **Error Handling**: Existing error patterns in overlord scripts for consistency
- **Alias Resolution**: How other commands (overlord-open, overlord-info) resolve project names/aliases from registry

### Key Decisions Made
- **Scope**: Python and TypeScript only; Solidity deferred
- **Target Project Registration**: NOT required in registry (only injected project needs registry entry)
- **Path Type**: Absolute paths (standard practice, avoids relativity issues if projects moved)
- **Dependency Format**: Native language/tool format (not a wrapper layer)
- **No Auto-Sync**: User runs sync commands separately for control and clarity
- **Idempotent**: Re-running inject on same project is safe and does nothing
- **Language Validation**: Strict matching - no cross-language injection

## Success Criteria

### Automated Verification
- [ ] Command parser correctly handles `overlord inject <name>` syntax
- [ ] Registry lookup works for both project names and aliases
- [ ] Language detection correctly identifies Python vs TypeScript in target directory
- [ ] Language mismatch validation prevents cross-language injection
- [ ] `pyproject.toml` modified correctly with path dependency for Python projects
- [ ] `package.json` modified correctly with path dependency for TypeScript projects
- [ ] Idempotency: running inject twice on same project doesn't duplicate entries
- [ ] No error when target directory has no existing config file (if applicable)
- [ ] Proper error messages for missing projects and language mismatches

### Manual Verification
- [ ] Run `overlord inject <python-lib>` from Python project, verify `pyproject.toml` updated correctly
- [ ] Run `overlord inject <ts-lib>` from TypeScript project, verify `package.json` updated correctly
- [ ] Attempt inject with project not in registry - verify clear error message
- [ ] Attempt inject with mismatched languages - verify language mismatch error
- [ ] Run `uv sync` or `pnpm install` after inject - verify dependency resolves
- [ ] Manually import code from injected project - verify it works
- [ ] Run inject again on same project - verify no duplicate entries or errors
- [ ] Test with both project names and aliases - verify both work equally

## Related Information
- Related to worktree integration and tmux setup (monorepo development workflow)
- Complements existing `overlord new`, `overlord add`, `overlord list` functionality
- Built on existing registry infrastructure

## Notes
- Research should focus on how uv, pnpm, and bun handle local path-based dependencies (correct syntax and format)
- Need to understand TOML and JSON manipulation in bash (consider existing tools/patterns)
- Error messages should follow existing Overlord command style (see lib/common.sh logging functions)
- Consider whether absolute paths need any escaping or special handling for different OSes

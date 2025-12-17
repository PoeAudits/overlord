---
type: feature
priority: high
created: 2025-12-16T00:00:00Z
status: archived
tags: [cli, registry, project-management]
keywords: [overlord-add, register_project, registry.json, add existing, language detection]
patterns: [argument parsing, jq registry manipulation, file detection, tmux template copying]
---

# FEATURE-001: Add `overlord add` Command to Register Existing Directories

## Description
Create a new `overlord add` command that allows users to register existing project directories in the Overlord registry without creating new projects. This enables managing projects that weren't created with `overlord new`, such as configuration directories, cloned repositories, or legacy projects.

## Context
Currently, only projects created with `overlord new` are tracked in the registry. Users need to manage existing directories (like `~/bin/overlord` itself) through Overlord's workspace system. This command bridges that gap by adding existing directories to the registry with appropriate configuration files.

## Requirements

### Functional Requirements
- Create new script `overlord-add` in `~/bin/overlord/`
- Command syntax: `overlord add <name> <path> [options]`
- Both `<name>` and `<path>` are required arguments
- Path can be `.` for current directory, absolute path, or `~` expansion
- Auto-detect language from project files if no language flag provided:
  - `pyproject.toml` or `setup.py` -> Python
  - `package.json` -> TypeScript
  - `foundry.toml` -> Solidity
  - None detected -> base configuration (no language-specific additions)
- Support explicit language flags: `--py`/`--python`, `--ts`/`--typescript`, `--sol`/`--solidity`
- Default status is `active`, with `--lib` flag for library status
- Support `--alias` flag to set aliases during add (e.g., `--alias ol --alias ovlrd`)
- Check if project name exists in registry:
  - Warn and exit if exists
  - `--force` flag to overwrite existing entry
- Create `.tmux.local` if it doesn't exist (use language-specific or base template)
- Create `Makefile` if it doesn't exist (combine base.mk + language-specific or just base.mk)
- Register project in `registry.json` with name, lang, status, path, created date, aliases
- Update main `overlord` dispatcher to route `add` subcommand
- Running `overlord add` with no arguments shows help (not add current directory)

### Non-Functional Requirements
- Follow existing code patterns from `overlord-new`
- Use same color scheme and logging functions
- Maintain consistency with other subcommands
- Support `--help` flag

## Current State
- No way to add existing directories to registry
- Only `overlord new` can create registry entries
- Existing `register_project()` function in `overlord-new` can be referenced

## Desired State
- `overlord add myproject /path/to/project --py` registers existing Python project
- `overlord add overlord ~/bin/overlord` registers the overlord repo itself with auto-detected base config
- `overlord add mylib . --ts --lib --alias ml` registers current dir as TypeScript library with alias

## Research Context

### Keywords to Search
- `register_project` - Existing function to register in registry.json
- `copy_tmux_template` - Function to copy tmux templates
- `generate_makefile` - Function to generate Makefiles
- `parse_args` - Argument parsing patterns
- `OVERLORD_REGISTRY` - Registry file path variable

### Patterns to Investigate
- Argument parsing in `overlord-new` lines 55-115
- Registry manipulation with jq in `overlord-new` lines 283-309
- Template copying in `overlord-new` lines 228-240
- Makefile generation in `overlord-new` lines 252-280
- Language detection by checking file existence

### Key Decisions Made
- Name and path are both required (no defaults)
- Path requires absolute path or `.` or `~` (no relative paths like `../foo`)
- Auto-detection falls back to base config, not error
- Does NOT run `overlord open` after adding (unlike `overlord new`)
- Creates config files only if they don't exist (no overwrite by default)

## Success Criteria

### Automated Verification
- [ ] `overlord add --help` shows usage
- [ ] `overlord add` with no args shows help
- [ ] `overlord add testproj /tmp/testproj --py` creates registry entry
- [ ] `overlord add testproj /tmp/testproj` without flag auto-detects or uses base
- [ ] `overlord add existing .` warns if name exists without --force
- [ ] `overlord add existing . --force` overwrites entry
- [ ] `.tmux.local` created if missing
- [ ] `Makefile` created if missing

### Manual Verification
- [ ] Can add `~/bin/overlord` to registry as base project
- [ ] Can open added project with `overlord open`
- [ ] Added project appears in `overlord list`
- [ ] Aliases work for lookup

## Related Information
- Reference: `overlord-new` for code patterns
- Reference: `~/.config/overlord/tmux/` for templates
- Reference: `~/.config/overlord/makefiles/` for Makefile templates
- Depends on: base.tmux template may need to be created if it doesn't exist

## Notes
- Consider creating `base.tmux` template if it doesn't exist (may be separate ticket)
- The `~` expansion should use bash's built-in expansion or explicit `$HOME` substitution

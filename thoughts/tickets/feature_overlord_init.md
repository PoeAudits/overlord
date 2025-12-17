---
type: feature
priority: high
created: 2025-12-16T00:00:00Z
status: implemented
tags: [cli, registry, initialization, project-setup]
keywords: [overlord-init, initialize, setup, makefile, tmux.local, git init, base.tmux]
patterns: [language detection, template copying, git initialization, registry registration]
---

# FEATURE-003: Add `overlord init` Command to Initialize Existing Repositories

## Description
Create a new `overlord init` command that initializes an existing directory with Overlord configuration files (Makefile, .tmux.local) and registers it in the registry. This replaces the functionality of the legacy `ws init` command, bringing existing repositories into the Overlord ecosystem.

## Context
Users have existing repositories (cloned from GitHub, legacy projects, etc.) that need to be set up with Overlord's workspace configuration. Unlike `overlord add` which primarily registers a project, `overlord init` focuses on setting up the configuration files and optionally initializing git, similar to what `overlord new` does but for existing directories.

## Requirements

### Functional Requirements
- Create new script `overlord-init` in `~/bin/overlord/`
- Command syntax: `overlord init [path] [options]`
- Default to current directory if no path specified
- Auto-detect language from project files if no language flag:
  - `pyproject.toml` or `setup.py` -> Python
  - `package.json` -> TypeScript  
  - `foundry.toml` -> Solidity
  - None detected -> base configuration
- Support explicit language flags: `--py`/`--python`, `--ts`/`--typescript`, `--sol`/`--solidity`
- Support `--base` flag to force base configuration (no language-specific additions)
- Default status is `active`, with `--lib` flag for library status
- Support `--name` flag to override directory name for registry entry
- Support `--alias` flag to set aliases (can be used multiple times)
- Initialize git repository if not present (with `--no-git` flag to skip)
- Create `.tmux.local` from template:
  - Use language-specific template if language detected/specified
  - Use `base.tmux` template for base configuration
  - Skip if file exists (unless `--force`)
- Create `Makefile` from templates:
  - Combine `base.mk` + language-specific `.mk` if language detected/specified
  - Use only `base.mk` for base configuration
  - Skip if file exists (unless `--force`)
- Register project in `registry.json`
- If project already in registry: warn and skip (do not update)
- `--force` flag to overwrite existing `.tmux.local` and `Makefile`
- Update main `overlord` dispatcher to route `init` subcommand

### Non-Functional Requirements
- Follow existing code patterns from `overlord-new`
- Use same color scheme and logging functions
- Reuse existing functions where possible (template copying, makefile generation)
- Support `--help` flag

## Current State
- No way to initialize existing directories with Overlord config
- `overlord new` creates new directories but can't work with existing ones
- `overlord add` registers but doesn't set up config files comprehensively
- May need to create `base.tmux` template if it doesn't exist

## Desired State
- `overlord init` in a Python project auto-detects and sets up Python config
- `overlord init --base` sets up minimal config without language specifics
- `overlord init --ts --name myproj --alias mp` initializes with custom name/alias
- `overlord init /path/to/project --sol --lib` initializes Solidity library

## Research Context

### Keywords to Search
- `init_python`, `init_typescript`, `init_solidity` - Language init functions (for reference, NOT to call)
- `init_git` - Git initialization function
- `copy_tmux_template` - Template copying function
- `generate_makefile` - Makefile generation function
- `register_project` - Registry registration function
- `base.mk` - Base makefile template

### Patterns to Investigate
- Git initialization in `overlord-new` lines 212-225
- Template copying in `overlord-new` lines 228-240
- Makefile generation in `overlord-new` lines 252-280
- Registry registration in `overlord-new` lines 283-309
- Language detection by file existence checks

### Key Decisions Made
- Works on current directory by default (unlike `overlord add` which requires explicit path)
- Does NOT run language-specific initialization (no `uv init`, `pnpm init`, `forge init`)
- Only sets up Overlord config files (Makefile, .tmux.local)
- Initializes git if not present (can skip with --no-git)
- Skips existing files unless --force
- Skips if already in registry (no update, just warn)
- Registers in registry as part of init

### Dependencies
- May require creating `base.tmux` template in `~/.config/overlord/tmux/`
- This could be a sub-task or separate ticket

## Success Criteria

### Automated Verification
- [ ] `overlord init --help` shows usage
- [ ] `overlord init` in empty dir creates base config
- [ ] `overlord init` in Python project creates Python config
- [ ] `overlord init --base` forces base config even if language detected
- [ ] `overlord init --no-git` skips git initialization
- [ ] `overlord init --force` overwrites existing files
- [ ] `overlord init` on registered project warns and skips
- [ ] `.tmux.local` created from appropriate template
- [ ] `Makefile` created with appropriate content
- [ ] Project registered in registry

### Manual Verification
- [ ] Can clone external repo and run `overlord init`
- [ ] Can open initialized project with `overlord open`
- [ ] Makefile `make help` works
- [ ] tmux workspace opens correctly

## Related Information
- Reference: `overlord-new` for code patterns
- Reference: `~/.config/overlord/tmux/` for templates
- Reference: `~/.config/overlord/makefiles/` for Makefile templates
- Related: `overlord add` (register-focused) vs `overlord init` (setup-focused)
- Dependency: `base.tmux` template needs to exist

## Notes
- Key difference from `overlord add`: 
  - `add` = register existing project, create config if missing
  - `init` = full setup (git, config files, register), designed for fresh/cloned repos
- Key difference from `overlord new`:
  - `new` = create directory, run language init (uv/pnpm/forge), setup config
  - `init` = existing directory, NO language init, setup config only
- Consider whether `base.tmux` template creation should be part of this ticket or separate

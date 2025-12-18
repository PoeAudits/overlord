---
type: feature
priority: high
created: 2025-12-17T00:00:00Z
status: archived
tags: [configuration, setup, deployment, portability]
keywords: [configuration consolidation, single repository, setup script, PATH installation]
patterns: [overlord-* scripts, registry initialization, environment variables, symlinks]
---

# FEATURE: Consolidate Overlord configuration into single git repository

## Description
Restructure Overlord so all configuration (templates, makefiles, scripts) lives in a single directory that can be cloned and used across multiple computers. Currently, the `overlord-*` command scripts are scattered in `~/bin/overlord/` alongside the main `overlord` dispatcher, making it difficult to version-control and deploy as a single repository.

## Context
For Overlord to be portable and version-controllable across computers, all non-user-specific files should live in one git repository. The main `overlord` command needs to be in a PATH-accessible location (like `/usr/local/bin/`), but all configuration, templates, makefiles, and helper scripts should be centralized and easily cloned.

## Requirements
- Move all `overlord-*` command scripts from `~/bin/overlord/` to the main configuration directory
- Keep only the main `overlord` dispatcher in a PATH-accessible location (final state: `/usr/local/bin/overlord`)
- Create a setup script that:
  - Installs the `overlord` command to `/usr/local/bin/`
  - Initializes the registry file in the configuration directory
  - Sets up any required environment configuration
  - Can be run post-clone to fully prepare the system on a new computer
- Ensure all scripts can reliably locate the configuration directory
- Registry lives in the configuration directory (not `~/.config/`)

## Current State
```
~/bin/overlord/
├── overlord (main dispatcher)
├── overlord-add
├── overlord-config
├── overlord-edit
├── overlord-info
├── overlord-init
├── overlord-list
├── overlord-mv
├── overlord-new
├── overlord-open
├── overlord-rm
├── overlord-sync
└── ... (other files)

~/.config/overlord/
├── registry.json
├── templates/
│   └── opencode-*.jsonc
├── tmux/
│   ├── base.tmux
│   ├── python.tmux
│   ├── typescript.tmux
│   └── solidity.tmux
└── makefiles/
    ├── base.mk
    ├── python.mk
    ├── typescript.mk
    └── solidity.mk
```

## Desired State
After running setup script on a fresh clone:

```
/path/to/cloned/overlord-repo/
├── overlord (dispatcher - source)
├── overlord-add
├── overlord-config
├── overlord-edit
├── overlord-info
├── overlord-init
├── overlord-list
├── overlord-mv
├── overlord-new
├── overlord-open
├── overlord-rm
├── overlord-sync
├── setup.sh (setup script)
├── registry.json (created by setup)
├── templates/
│   └── opencode-*.jsonc
├── tmux/
│   ├── base.tmux
│   ├── python.tmux
│   ├── typescript.tmux
│   └── solidity.tmux
└── makefiles/
    ├── base.mk
    ├── python.mk
    ├── typescript.mk
    └── solidity.mk

/usr/local/bin/overlord (symlink or copy of dispatcher)
```

## Research Context

### Keywords to Search
- overlord-* scripts - Where they're currently located and their dependencies
- registry.json - Current location and structure
- OVERLORD_CONFIG - How it's currently used in scripts
- overlord dispatcher - Main command logic and how it locates scripts

### Patterns to Investigate
- Configuration path resolution - How scripts locate templates and makefiles
- Script execution - How overlord dispatcher finds and runs overlord-* scripts
- Environment variables - Current OVERLORD_* env var usage

## Success Criteria

### Automated Verification
- [ ] All `overlord-*` scripts are in configuration directory
- [ ] Main `overlord` command is at `/usr/local/bin/overlord`
- [ ] Registry file exists at configuration directory path after setup

### Manual Verification
- [ ] Clone repo to a new location
- [ ] Run setup script successfully
- [ ] `overlord` command works from any directory
- [ ] `overlord list` returns correct registry state
- [ ] Creating new projects works and stores in registry

## Out of Scope
- Migrating existing user installations automatically
- Changes to core overlord command logic
- Creating new commands or features beyond setup script

## Notes
- Setup script should be idempotent where possible
- Consider how to handle existing `/usr/local/bin/overlord` if it exists
- Registry initialization should create an empty projects object
- May need to update OVERLORD_CONFIG env var or make it optional with fallback logic

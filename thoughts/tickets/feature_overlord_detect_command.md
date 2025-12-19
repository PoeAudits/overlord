---
type: feature
priority: medium
created: 2025-01-18T10:00:00Z
status: implemented
tags: [cli, registry, detection]
keywords: [overlord detect, unregistered projects, work directory scanning]
patterns: [registry.json lookup, directory traversal]
---

# FEATURE: Create overlord detect command for unregistered projects

## Description
Add an `overlord-detect` command that scans the OVERLORD_WORK_DIR for projects that exist on disk but are not registered in the Overlord registry.

## Context
Users may have projects in their Work directory that were created manually or lost from the registry. This command helps identify orphaned projects that need to be registered.

## Requirements
- Scan Python/, Typescript/, Solidity/ subdirectories in OVERLORD_WORK_DIR
- Compare found projects against registry.json entries
- List only projects not currently registered
- Print project paths to console
- No auto-add functionality

## Current State
No command exists to find unregistered projects. Users must manually check registry vs filesystem.

## Desired State
`overlord detect` command lists all filesystem projects not in registry.

## Research Context

### Keywords to Search
- OVERLORD_WORK_DIR - Environment variable definition
- registry.json - Registry file format and location
- overlord-list - Existing registry lookup logic

### Patterns to Investigate
- directory traversal - How to scan subdirectories safely
- registry comparison - How to compare filesystem vs registry
- project validation - What constitutes a valid project directory

## Success Criteria

### Automated Verification
- [ ] Script exists at overlord-detect
- [ ] Script is executable from overlord dispatcher

### Manual Verification
- [ ] `overlord detect` shows unregistered projects
- [ ] `overlord detect` shows nothing when all projects registered
- [ ] Projects are correctly identified as registered vs unregistered

## Out of Scope
- JSON output format
- Auto-add functionality
- Project type detection
- Integration with other commands

## Notes
The command should rely on directory structure rather than file-based detection since projects should be organized under their respective language folders.
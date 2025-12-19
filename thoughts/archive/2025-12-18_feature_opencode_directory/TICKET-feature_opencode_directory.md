---
type: feature
priority: medium
created: 2025-12-18
status: archived
tags: [opencode, configuration, directory-structure]
keywords: [opencode.jsonc, .opencode, directory, projects, overlord-init, overlord-sync]
patterns: [configuration file placement, project initialization]
---

# FEATURE-XXX: Change opencode.jsonc to use .opencode directory

## Description
Change the existing behavior from placing opencode.jsonc directly in the project root to placing it inside a .opencode directory within the project. This improves consistency and avoids cluttering the project root with configuration files.

## Context
Having opencode.jsonc in the project root can "muck things up" by cluttering the directory. Moving it to a dedicated .opencode directory keeps related configuration organized together.

## Requirements
- Move opencode.jsonc into .opencode/ directory for new projects
- Update relevant Overlord commands to handle the new location
- Ensure backward compatibility or migration path for existing projects

### Functional Requirements
- opencode.jsonc is created in .opencode/ instead of project root
- Overlord commands (init, sync) recognize and work with the new structure
- Only opencode.jsonc moves; thoughts/ and other files remain unchanged

### Non-Functional Requirements
- Simple change with minimal impact on existing functionality
- Maintain consistency across all project types

## Current State
opencode.jsonc is placed directly in the project root directory.

## Desired State
opencode.jsonc is placed inside a .opencode/ directory in the project root.

## Research Context

### Keywords to Search
- opencode.jsonc - The configuration file being moved
- .opencode - The new directory structure
- directory - Project directory management
- projects - Overlord project handling
- overlord-init - Command that creates the file
- overlord-sync - Command that propagates the file

### Patterns to Investigate
- configuration file placement - How config files are currently placed
- project initialization - Where and how files are created during init

### Key Decisions Made
- Only opencode.jsonc moves to .opencode/; thoughts/ remains in root
- Keep the change simple and focused

## Success Criteria

### Automated Verification
- [ ] overlord-init creates .opencode/opencode.jsonc instead of opencode.jsonc in root
- [ ] overlord-sync propagates to .opencode/ directory
- [ ] Existing projects continue to work (backward compatibility)

### Manual Verification
- [ ] New projects have clean root directory without opencode.jsonc
- [ ] .opencode/ directory contains the configuration file
- [ ] Overlord commands function correctly with new structure

## Related Information
None

## Notes
This is a simple organizational change to improve project structure consistency.
---
type: debt
priority: medium
created: 2025-12-18
status: archived
tags: [consistency, refactoring, templates, configuration]
keywords: [configuration variables, template usage, function duplication, error handling]
patterns: [bash scripts, template files, helper functions]
---

# DEBT-XXX: Resolve Codebase Consistency Issues

## Description
Resolve inconsistencies in configuration variable access, template usage, function duplication, and error handling across the Overlord codebase to improve maintainability and prevent potential bugs.

## Context
Codebase analysis revealed multiple inconsistencies in how scripts handle configuration variables, use templates, duplicate functions, and handle errors. These inconsistencies could lead to maintenance difficulties and unexpected behavior when extending the system.

## Requirements
- Standardize configuration variable definitions and exports across all scripts
- Implement consistent template usage patterns for makefiles and opencode templates
- Eliminate duplicated helper functions by creating shared library
- Standardize error handling for missing templates
- Ensure consistent path resolution for template directories

## Current State
Inconsistent patterns including:
- Variable definition order varies between scripts
- Template generation logic differs between overlord-new, overlord-add, and overlord-sync
- Identical functions duplicated across multiple scripts
- Different error handling strategies for missing templates

## Desired State
Consistent codebase with:
- Unified configuration variable handling
- Standardized template usage patterns
- Shared helper functions
- Uniform error handling
- Consistent path construction

## Research Context

### Keywords to Search
- configuration variables - How variables are defined and exported
- template usage - Patterns for copying and generating templates
- function duplication - Identical functions across scripts
- error handling - Different approaches to missing templates

### Patterns to Investigate
- bash script configuration - Variable definition patterns
- template file handling - Path construction and copying logic
- helper function usage - Duplicated functions like get_lang_dir, detect_language

## Success Criteria

### Automated Verification
- [ ] All scripts pass basic syntax checks
- [ ] Template generation works consistently across all commands

### Manual Verification
- [ ] Configuration variables defined consistently in all scripts
- [ ] No duplicated functions remain
- [ ] Template usage follows same pattern everywhere
- [ ] Error handling for missing templates is uniform

## Out of Scope
- Changes to template content itself
- New features or functionality
- Performance optimizations

## Notes
Key inconsistencies to address:
1. OVERLORD_BIN definition/export missing in some scripts
2. WORK_DIR variable inconsistently defined
3. Makefile generation logic varies between scripts
4. Opencode template functions duplicated
5. Different fallback strategies for missing templates
6. Helper functions like get_lang_dir, get_status_dir, etc. duplicated
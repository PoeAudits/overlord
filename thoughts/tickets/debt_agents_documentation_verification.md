---
type: debt
priority: high
created: 2025-12-18T00:00:00Z
created_by: thomas
status: researched
tags: [documentation, verification, agents, codebase-sync]
keywords: [AGENTS.md, command scripts, flags, options, environment variables, registry, templates, documentation-accuracy]
patterns: [command existence, flag validation, environment variable usage, bidirectional verification, schema matching]
---

# DEBT-001: Verify AGENTS.md Documentation Accuracy Against Current Codebase

## Description

Verify that the AGENTS.md documentation file is accurate, complete, and up-to-date with the current state of the overlord codebase. This is a bidirectional verification ensuring that:
1. All documented features, commands, and flags actually exist in the code
2. All implemented features in the code are documented in AGENTS.md
3. Environment variables are used consistently as documented
4. Registry schema and template structures match documentation

This is primarily an **existence verification** (not a functionality test) to ensure AI agents have accurate information when analyzing the codebase.

## Context

Recent changes have been made to the overlord codebase. The documentation needs verification to ensure it accurately reflects the current implementation state, preventing agents from receiving incorrect or outdated information during code analysis and research phases.

## Requirements

### Functional Requirements

- **Command Script Verification**: Verify all documented overlord subcommands have corresponding executable scripts
  - Check that `overlord-new`, `overlord-add`, `overlord-init`, `overlord-list`, `overlord-mv`, `overlord-rm`, `overlord-detect`, `overlord-open`, `overlord-info`, `overlord-config`, `overlord-edit`, `overlord-sync`, `overlord-uninstall` exist
  - Verify command dispatcher (`overlord` main script) routes to correct subcommands

- **Flag and Option Verification**: For each command, verify documented flags/options are accepted
  - Language flags (`--py`, `--python`, `--ts`, `--typescript`, `--sol`, `--solidity`, `--base`)
  - Status flags (`--active`, `--lib`, `--archive`, `--all`)
  - Action flags (`--force`, `--dry-run`, `--fuzzy`, `--json`, `--no-git`, `--no-open`)
  - Option flags (e.g., `--import`, `--alias`, `--name`)

- **Argument Verification**: Confirm documented arguments are required/optional as stated
  - Required arguments (e.g., project `name`, target `status`)
  - Optional arguments (e.g., project `path`, custom `name`)

- **Feature Implementation**: Verify documented features exist in code
  - Language auto-detection mechanism
  - Alias support for projects
  - Status category system (active, lib, archive)
  - Registry schema and project metadata structure
  - Template system (opencode.jsonc, .tmux.local, Makefile)
  - Fuzzy search integration (fzf)
  - Worktree auto-naming with counter system
  - Cross-session communication (worktree-send, worktree-read, tmux-send, tmux-read)

- **Environment Variable Verification**: Check all documented environment variables
  - `OVERLORD_BASE_DIR` - usage, default value consistency
  - `OVERLORD_CONFIG` - usage, default value consistency
  - `OVERLORD_BIN` - usage, default value consistency
  - `OVERLORD_STRICT` - usage, default value consistency
  - `EDITOR` - usage, default value consistency
  - Verify no additional undocumented environment variables exist

- **File and Template Verification**: Confirm all documented template files exist
  - Templates directory: `templates/opencode-*.jsonc` (python, typescript, solidity, base)
  - Tmux templates: `tmux/base.tmux`, `python.tmux`, `typescript.tmux`, `solidity.tmux`
  - Makefile templates: `makefiles/base.mk`, `python.mk`, `typescript.mk`, `solidity.mk`

- **Bidirectional Completeness**: Identify gaps in both directions
  - **Code → Docs**: Commands, flags, or features in code that aren't documented
  - **Docs → Code**: Documented commands, flags, or features that don't exist in code
  - **Partial Implementation**: Features documented but only partially implemented

### Non-Functional Requirements

- Verification focuses on **existence**, not functionality or correctness of behavior
- Output should be a structured markdown document with categorized findings
- Findings should be actionable and suitable for follow-up fixes
- Use specialized agents (codebase-pattern-finder, codebase-locator, codebase-analyzer) for code analysis

## Current State

AGENTS.md exists at `/home/thomas/bin/overlord/AGENTS.md` and documents the overlord system. Recent code changes may have introduced discrepancies between documentation and implementation.

## Desired State

- All documented commands, flags, and options verified to exist in code
- All code-level commands, flags, and options verified to be in documentation (or identified as gaps)
- Environment variable usage verified for consistency with documentation
- All template and schema references verified
- Comprehensive report generated documenting all discrepancies, organized by priority

## Research Context

### Keywords to Search

- `overlord` - Main dispatcher script and all subcommands
- `overlord-new` - Project creation command
- `overlord-add` - Project registration command
- `overlord-init` - Project initialization command
- `overlord-list` - Project listing command
- `overlord-mv` - Project status movement command
- `overlord-rm` - Project removal command
- `overlord-detect` - Unregistered project detection
- `overlord-open` - Workspace opening command
- `overlord-info` - Project information display
- `overlord-config` - Registry configuration management
- `overlord-edit` - Script editing command
- `overlord-sync` - File synchronization command
- `overlord-uninstall` - System removal command
- `registry.json` - Project registry structure
- `OVERLORD_*` - Environment variable definitions
- Language flags (`--py`, `--ts`, `--sol`) - Flag implementations
- Status flags (`--active`, `--lib`, `--archive`) - Status handling

### Patterns to Investigate

- **Command routing pattern**: How main `overlord` dispatcher routes to subcommands
- **Flag parsing pattern**: How flags and options are validated in scripts
- **Language detection pattern**: How language is auto-detected from file presence
- **Alias handling pattern**: How project aliases are stored and retrieved
- **Environment variable usage pattern**: How env vars are referenced and defaulted
- **Template loading pattern**: How templates are selected and applied
- **Registry schema pattern**: Project data structure and validation
- **Feature implementation pattern**: How features like fuzzy search, auto-naming, cross-session communication work

### Key Decisions Already Made

- Verification is **existence-only**, not functionality testing
- Bidirectional check: code↔docs must be synchronized
- Output format: structured markdown with categories (Critical, High, Medium, Low)
- Not updating documentation yet; identifying discrepancies for follow-up
- Focus only on command references, excluding Makefile/tmux integration details (they're out of scope)
- Environment variables must be checked for consistent usage across codebase

## Success Criteria

### Automated Verification

- [ ] All documented overlord subcommands have corresponding executable scripts
- [ ] All documented flags are referenced in script argument parsing
- [ ] All documented arguments are present in script implementations
- [ ] All template files referenced in documentation exist on disk
- [ ] Environment variables documented are used consistently in code
- [ ] Registry schema fields match documented structure

### Manual Verification

- [ ] Codebase-analyzer confirms all command features documented are implemented
- [ ] Codebase-pattern-finder identifies any commands/flags in code but missing from docs
- [ ] Pattern investigations confirm feature implementations align with documentation
- [ ] Review generated verification report for completeness and accuracy

## Output Specification

Generate a markdown file at: `thoughts/archive/<date>_agents_documentation_verification_report.md`

**Report Structure:**

```markdown
# AGENTS.md Documentation Verification Report

Generated: [ISO timestamp]
Verification Scope: Command references, flags, options, environment variables, templates

## Executive Summary
- Total discrepancies found: X
- Critical issues: X
- High issues: X
- Medium issues: X
- Low issues: X

## Critical Issues

### [Issue Type]: [Brief Description]
- **Location**: Code or Docs
- **Finding**: Specific detail
- **Impact**: Why this matters
- **Recommendation**: How to fix

## High Priority Issues

[Same format as critical]

## Medium Priority Issues

[Same format as critical]

## Low Priority Issues

[Same format as critical]

## Code → Docs Gaps (Implemented but not documented)

### Command/Feature: [name]
- **Location**: [file path and relevant lines]
- **Description**: [what exists in code]
- **Documentation Status**: [not mentioned in AGENTS.md]

### Environment Variable: [name]
- **Usage**: [where used in code]
- **Documentation Status**: [not documented or incomplete]

## Docs → Code Gaps (Documented but not implemented)

### Command/Flag: [name]
- **Documentation**: [what AGENTS.md says]
- **Code Status**: [doesn't exist or partially implemented]

## Environment Variable Consistency

### Variable: [OVERLORD_*]
- **Documented Default**: [what AGENTS.md states]
- **Actual Default**: [what code uses]
- **Usage Locations**: [files where used]
- **Consistency Status**: [consistent/inconsistent]

## Template and Schema Verification

### Template: [name]
- **Documented Location**: [path in docs]
- **Actual Location**: [file path]
- **Status**: [exists/missing]

## Recommendations for Follow-up

1. [Prioritized fix recommendation]
2. [Prioritized fix recommendation]
...
```

## Related Information

- AGENTS.md location: `/home/thomas/bin/overlord/AGENTS.md`
- Overlord scripts location: `/home/thomas/bin/overlord/overlord*`
- Registry reference: `/home/thomas/bin/overlord/registry.json` (example)
- Templates location: `/home/thomas/bin/overlord/templates/`
- Makefiles location: `/home/thomas/bin/overlord/makefiles/`
- Previous documentation notes: `/home/thomas/bin/overlord/thoughts/`

## Notes

- This verification ensures agents have accurate information about the overlord system
- Focus on existence verification; functionality testing is out of scope
- Bidirectional verification is critical to catch both missing documentation and unimplemented features
- Report findings without updating documentation; updates will be handled in separate tickets based on priority
- Use specialized research agents to maximize accuracy and coverage

---
type: feature
priority: low
created: 2025-12-16T00:00:00Z
status: archived
tags: [cli, list, filtering, enhancement]
keywords: [overlord-list, filter, language, python, typescript, solidity, FILTER_LANG]
patterns: [argument parsing, jq filtering, multiple conditions]
---

# FEATURE-004: Enhance `overlord list` with Combined Language and Status Filtering

## Description
Enhance the existing `overlord list` command to support combining language filters (`--py`, `--ts`, `--sol`) with status filters (`--active`, `--lib`, `--archive`, `--all`). Additionally, support multiple language flags to show projects from multiple languages.

## Context
The current `overlord list` implementation already has language filtering (`--py`, `--ts`, `--sol`) but the user wants to be able to combine these with status filters more intuitively. For example, `overlord list --py --all` should show all Python projects across all statuses, and `overlord list --py --ts` should show both Python and TypeScript projects.

## Requirements

### Functional Requirements
- Maintain existing language flags: `--py`/`--python`, `--ts`/`--typescript`, `--sol`/`--solidity`
- Maintain existing status flags: `--active`, `--lib`, `--archive`, `--all`
- Support combining language + status filters:
  - `overlord list --py` = Python active projects (default status)
  - `overlord list --py --all` = All Python projects (any status)
  - `overlord list --py --archive` = Only archived Python projects
  - `overlord list --py --lib` = Only Python libraries
- Support multiple language flags:
  - `overlord list --py --ts` = Python AND TypeScript active projects
  - `overlord list --py --sol --all` = All Python AND Solidity projects
- No output format change when filtering by language (keep LANG column)
- Update help text to reflect enhanced filtering capabilities
- Update main `overlord` dispatcher help if needed

### Non-Functional Requirements
- Maintain backward compatibility with existing usage
- Keep same output format and colors
- Efficient jq filtering for multiple conditions

## Current State
Looking at `overlord-list` (lines 44-86), the current implementation:
- Has `FILTER_LANG` variable (single language)
- Has `FILTER_STATUS` variable (single status, defaults to "active")
- Builds jq filter with conditions joined by "and"
- Already supports `--py`, `--ts`, `--sol` flags
- Already supports `--active`, `--lib`, `--archive`, `--all` flags

The current behavior already supports `--py --archive` combinations. The main enhancement needed is:
1. Support multiple language flags (OR logic between languages)
2. Ensure documentation/help reflects this capability

## Desired State
- `overlord list --py --ts --all` shows all Python and TypeScript projects
- `overlord list --sol --archive` shows archived Solidity projects only
- Help text clearly documents filtering combinations

## Research Context

### Keywords to Search
- `FILTER_LANG` - Language filter variable
- `FILTER_STATUS` - Status filter variable
- `jq_filter` - jq query construction
- `conditions` - Filter conditions array

### Patterns to Investigate
- Argument parsing in `overlord-list` lines 48-86
- jq filter construction in `overlord-list` lines 124-139
- Current language flag handling (single value assignment)

### Key Decisions Made
- No change to output format (keep LANG column visible)
- Multiple languages use OR logic (show Python OR TypeScript)
- Language + status use AND logic (show Python AND archived)
- No `--base` filter for non-language projects
- Backward compatible with existing usage

## Success Criteria

### Automated Verification
- [ ] `overlord list --py` shows only Python active projects
- [ ] `overlord list --py --all` shows all Python projects
- [ ] `overlord list --py --archive` shows only archived Python projects
- [ ] `overlord list --py --ts` shows Python and TypeScript active projects
- [ ] `overlord list --py --ts --sol --all` shows all projects of all three languages
- [ ] `overlord list --help` documents filtering options clearly

### Manual Verification
- [ ] Output format unchanged
- [ ] Colors still work correctly
- [ ] No regression in existing usage patterns

## Related Information
- File: `~/bin/overlord/overlord-list`
- Current implementation already has most infrastructure
- Main change: FILTER_LANG from single value to array/multiple values

## Notes
- Implementation approach: Change `FILTER_LANG` from single string to array
- jq filter needs to use `or` for multiple languages: `.value.lang == "python" or .value.lang == "typescript"`
- Current code at line 131-133 builds single language condition
- Need to modify to build multiple language conditions with OR

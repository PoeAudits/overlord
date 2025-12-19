---
type: feature
priority: high
created: 2025-12-18T10:30:00Z
status: archived
tags: [cli, command-dispatch, user-experience]
keywords: [command dispatch, project lookup, overlord main script, fuzzy search flag]
patterns: [case statement, jq project queries, argument parsing]
---

# FEATURE-004: Enable overlord <repo_name> shortcut syntax

## Description
Allow users to run `overlord <repo_name>` as a shortcut for `overlord open <repo_name>`, with enhanced error handling and optional fuzzy search capability.

## Context
This improves the user experience by reducing typing for the most common operation (opening projects) while maintaining backward compatibility and providing helpful error messages.

## Requirements

### Core Functionality
- `overlord <repo_name>` should open the specified project (same as `overlord open <repo_name>`)
- If no project name matches exactly, show warning that project wasn't found
- After warning, display help information as if no argument was given
- Include message suggesting `overlord list` to see available projects
- Additional arguments should pass through to `overlord-open`

### Fuzzy Search Enhancement
- Add optional fuzzy search flag: `overlord <repo_name> --fuzzy` (or `-f`)
- When fuzzy flag is used, perform fuzzy search for the repo name
- Without fuzzy flag, require exact project name match
- `overlord open` should continue to always support fuzzy search (unchanged)

### Priority and Compatibility
- Subcommands take priority over project names (e.g., `overlord new` still creates new project)
- Maintain full backward compatibility
- All existing functionality should remain unchanged

## Current State
```bash
overlord                    # Lists active projects
overlord open myproject     # Opens myproject with fuzzy fallback
overlord myproject          # Error: Unknown command
```

## Desired State
```bash
overlord                    # Lists active projects (unchanged)
overlord open myproject     # Opens myproject with fuzzy fallback (unchanged)
overlord myproject          # Opens myproject if exact match exists
overlord myproj --fuzzy     # Fuzzy searches for "myproj"
overlord nonexistent        # Warns project not found, shows help, suggests overlord list
```

## Research Context

### Keywords to Search
- command dispatch - Main overlord script case statement
- project lookup - jq queries for project finding
- fuzzy search - fzf integration patterns
- argument parsing - How arguments are processed

### Patterns to Investigate
- case statement modification - Main script dispatch logic
- jq project queries - Existing project lookup patterns in overlord-open
- error handling - How errors and warnings are displayed
- help display - Usage function implementation

## Success Criteria

### Automated Verification
- [ ] All existing tests pass (backward compatibility)
- [ ] New test cases cover exact match, fuzzy flag, and error scenarios

### Manual Verification
- [ ] `overlord <valid_project>` opens exact match project
- [ ] `overlord <invalid_project>` shows warning + help + list suggestion
- [ ] `overlord <partial> --fuzzy` performs fuzzy search
- [ ] `overlord open <partial>` still works with fuzzy search (unchanged)
- [ ] `overlord new` still creates new project (subcommand priority)
- [ ] `overlord myproject extra_args` passes extra_args to overlord-open
- [ ] `overlord` with no args still lists projects (unchanged)

## Out of Scope
- Modifying `overlord open` behavior (should remain unchanged)
- Changing subcommand priority logic
- Modifying project registry structure
- Changes to help text beyond what's necessary

## Implementation Notes

### File to Modify
- `/home/thomas/bin/overlord/overlord` (main dispatcher script)

### Key Changes Required
1. **Add project lookup function** - Replicate `find_project_exact()` logic from overlord-open
2. **Modify case statement** - Add fallback before generic error case
3. **Add fuzzy flag handling** - Parse `--fuzzy`/`-f` flag and dispatch accordingly
4. **Update usage function** - Document new syntax and fuzzy flag
5. **Enhanced error messaging** - Show warning + help + list suggestion for invalid projects

### Error Message Flow
```
overlord nonexistent
→ Warning: Project 'nonexistent' not found
→ (shows help information)
→ Tip: Use 'overlord list' to see available projects
```

### Argument Priority
1. Help/version flags (`--help`, `--version`)
2. Subcommands (`new`, `add`, `open`, etc.)
3. Project name + optional fuzzy flag
4. Error handling

### Fuzzy Flag Implementation
- Parse `--fuzzy` or `-f` as second argument
- If fuzzy flag present, dispatch to `overlord-open` with both arguments
- `overlord myproj --fuzzy` → `exec overlord-open myproj --fuzzy` (if overlord-open supports it)
- Or implement fuzzy logic directly in main script
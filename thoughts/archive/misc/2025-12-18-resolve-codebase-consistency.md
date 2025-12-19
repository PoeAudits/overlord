# Readable Summary
Resolve Codebase Consistency

<analysis>
The conversation centered on addressing technical debt in the Overlord project management system. I began by reading the debt ticket and existing research, which identified inconsistencies in configuration handling, function duplication, and error patterns across multiple bash scripts.

I conducted a comprehensive analysis of all 12 scripts in the repository, mapping out specific line numbers for duplicated logging functions, directory helpers, and template generation logic. I found that variable exports were inconsistent (breaking nested script calls) and that the project root path (`WORK_DIR`) was hardcoded to `$HOME/Work` in several places.

I drafted an implementation plan which went through one iteration based on user feedback. The final decisions were:
1.  **Rename to BASE**: Replace all `WORK` naming conventions with `BASE` (e.g., `OVERLORD_BASE_DIR`).
2.  **Shared Library**: Extract duplicated functions into `lib/common.sh`.
3.  **Configurable Root**: Allow the project root to be set via environment variable or a `settings` object in `registry.json`.
4.  **Template Fallbacks**: Standardize falling back to `base.tmux` and `base.mk` when language-specific templates are missing.
5.  **Standardized Errors**: Use warnings for non-destructive project setup but fatal errors for batch sync operations if critical files are missing.

The plan is documented in `thoughts/plans/resolve_codebase_consistency.md` and the ticket `thoughts/tickets/debt_consistency_issues.md` has been moved to "planned" status. The next step is to begin Phase 1: Foundation & Shared Library.
</analysis>

<plan>
# Session Handoff Plan

## 1. Primary Request and Intent
The goal is to implement the "Codebase Consistency & Refactoring Implementation Plan" to resolve technical debt in the Overlord scripts. This involves centralizing duplicated logic, standardizing path resolution (moving from `WORK` to `BASE` terminology), and ensuring consistent error/fallback behavior across the system.

## 2. Key Technical Concepts
- **Bash Scripting**: Heavily used across all commands; requires `set -euo pipefail`.
- **JSON Registry**: `registry.json` stores project metadata; we are adding a `settings` object.
- **Environment Inheritance**: Standardizing exports (`OVERLORD_CONFIG`, `OVERLORD_BIN`, `OVERLORD_BASE_DIR`) to support nested script execution.
- **Template Fallbacks**: Logic to use `base.tmux` or `base.mk` when language-specific templates are unavailable.

## 3. Files and Code Sections
### thoughts/plans/resolve_codebase_consistency.md
- **Why important**: This is the source of truth for the refactoring work. It contains the 4-phase implementation strategy and the exact line numbers of all duplicated code to be removed.
- **Code snippet**:
```markdown
## Phase 1: Foundation & Shared Library
### Changes Required:
#### 1. Create Shared Library
**File**: `lib/common.sh`
#### 2. Initialize Settings in Registry
**File**: `registry.json`
```

### overlord
- **Why important**: The main dispatcher. It must resolve `OVERLORD_BASE_DIR` (formerly `WORK_DIR`) from the registry or environment and export it for all subcommands.

### overlord-new, overlord-init, overlord-add, overlord-sync
- **Why important**: These scripts contain the bulk of the duplicated template and helper logic identified in the plan. They will be refactored to source `lib/common.sh`.

## 4. Problem Solving
- **Duplication**: Identified over 11 duplicated functions across 8+ scripts.
- **Hardcoding**: Resolved to replace `$HOME/Work` with a configurable `OVERLORD_BASE_DIR`.
- **Template Logic**: Standardized on a "warn and fallback to base" approach for most commands, while `overlord sync` will error if the critical `base.mk` is missing.

## 5. Next Step
Proceed with **Phase 1: Foundation & Shared Library**. This involves:
1. Creating the `lib/` directory and the `lib/common.sh` file.
2. Moving the logging functions and basic directory helpers (`get_lang_dir`, `get_status_dir`) into `lib/common.sh`.
3. Adding the `settings` object to `registry.json` to store `base_dir`.
</plan>

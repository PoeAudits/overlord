# Codebase Consistency & Refactoring Implementation Plan

## Overview

This plan addresses significant inconsistencies in configuration handling, function duplication, and error patterns across the Overlord codebase. By consolidating shared logic into a common library and standardizing environment variables, we will improve the system's maintainability and robustness. A key focus is replacing the hardcoded `WORK` naming convention with a more flexible `BASE` directory structure.

## Current State Analysis

- **Hardcoded Paths**: `WORK_DIR` is hardcoded to `$HOME/Work` in multiple scripts (`overlord-new`, `overlord-mv`). This should be transitioned to `OVERLORD_BASE_DIR`.
- **Function Duplication**: Logging, language detection, and template handling functions are duplicated across 8+ scripts, leading to over 200 lines of redundant code.
- **Inconsistent Fallbacks**: `overlord-add` implements template fallbacks to `base`, while `overlord-new` and `overlord-init` do not.
- **Fragmented Error Handling**: `overlord-sync` treats missing templates as fatal errors, whereas other scripts only issue warnings.
- **Broken Inheritance**: Variable exports are inconsistent, which can break nested subcommand calls.

### Key Discoveries (Repeated Code Locations):
- **Logging functions** (`log_info`, `log_success`, etc.):
  - `overlord-add:46-49`
  - `overlord-init:54-57`
  - `overlord-mv:40-43`
  - `overlord-new:43-46`
  - `overlord-open:42-45`
  - `overlord-rm:50-53`
  - `overlord-sync:46-49`
- **`get_lang_dir`**:
  - `overlord-init:69-75`
  - `overlord-mv:59-69`
  - `overlord-new:126-132`
- **`get_status_dir`**:
  - `overlord-init:77-83`
  - `overlord-mv:46-56`
  - `overlord-new:134-140`
- **`detect_language`**:
  - `overlord-add:52-63`
  - `overlord-init:94-106`
- **`copy_tmux_template`**:
  - `overlord-add:72-97`
  - `overlord-init:131-143`
  - `overlord-new:229-241`
- **`generate_makefile`**:
  - `overlord-add:110-154`
  - `overlord-init:146-174`
  - `overlord-new:253-281`
  - `overlord-sync:98-120`
- **`copy_opencode_template`**:
  - `overlord-init:192-203`
  - `overlord-new:284-295`

## Desired End State

A unified, DRY codebase where:
- Shared logic resides in `lib/common.sh`.
- `OVERLORD_BASE_DIR` is the standard variable for the project root directory.
- Configuration is manageable via environment variables or a settings object in `registry.json`.
- Template handling is consistent with reliable fallbacks to `base` versions.

## What We're NOT Doing

- Testing `detect_language` against various directory structures in this phase.
- Testing complex tmux settings or worktree integrations.
- Modifying the content of existing templates.
- Adding new language support.

## Implementation Approach

1. **Extract & Centralize**: Move all duplicated helper functions (identified in Key Discoveries) to `lib/common.sh`.
2. **Rename to BASE**: Replace all `WORK` naming conventions (variables and directory references) with `BASE`.
3. **Standardize Scripts**: Refactor each subcommand to source the common library and use standardized `OVERLORD_BASE_DIR` resolution.
4. **Unify Logic**: Apply consistent fallback and error handling strategies across all template-related operations.

---

## Phase 1: Foundation & Shared Library

### Overview
Create the centralized library and prepare the registry for configurable settings.

### Changes Required:

#### 1. Create Shared Library
**File**: `lib/common.sh`
**Changes**: Extract logging, language detection, and template helper functions from the locations identified above.

#### 2. Initialize Settings in Registry
**File**: `registry.json`
**Changes**: Add an optional `settings` key to store global configuration.

```json
{
  "settings": {
    "base_dir": "/home/thomas/Work"
  },
  "projects": { ... }
}
```

---

## Phase 2: Variable & Export Standardization (Transition to BASE)

### Overview
Standardize how environment variables are resolved and shared, replacing `WORK` with `BASE`.

### Changes Required:

#### 1. Update Main Dispatcher
**File**: `overlord`
**Changes**: Resolve `OVERLORD_BASE_DIR` from registry or environment and export it.

```bash
# Resolve BASE_DIR with priority: ENV > Registry > Default
OVERLORD_BASE_DIR="${OVERLORD_BASE_DIR:-$(jq -r '.settings.base_dir // empty' "$OVERLORD_REGISTRY" || echo "$HOME/Work")}"
export OVERLORD_CONFIG OVERLORD_BIN OVERLORD_BASE_DIR
```

#### 2. Update Subcommands
**Files**: All `overlord-*` scripts
**Changes**: Replace `WORK_DIR` / `OVERLORD_WORK_DIR` with `OVERLORD_BASE_DIR` and source `lib/common.sh`.

---

## Phase 3: Unifying Template Logic & Error Handling

### Overview
Apply the "fallback to base" and "warn vs error" policies consistently.

### Changes Required:

#### 1. Standardize Makefile Generation
**File**: `lib/common.sh` (function `generate_makefile`)
**Changes**: Ensure it falls back to `base.mk` if language-specific `.mk` is missing.

#### 2. Unified Error Handling
**Policy**: 
- Warning + Fallback for project initialization (non-destructive).
- Error + Exit for batch operations like `overlord sync` if critical files (`base.mk`) are missing.

---

## Phase 4: Script Refactoring & Cleanup

### Overview
Systematically remove duplicated code from all scripts and verify functionality.

### Success Criteria:

#### Automated Verification:
- [x] All `overlord` commands execute without syntax errors.
- [x] `grep` shows no duplicate definitions of `log_info`, `get_lang_dir`, etc. in subcommand files.
- [x] No occurrences of `WORK_DIR` or `OVERLORD_WORK_DIR` remain in the scripts.

#### Manual Verification:
- [x] `overlord list` displays projects correctly.
- [x] `overlord new` creates a project in the designated `OVERLORD_BASE_DIR`.
- [x] `overlord sync` processes projects without error.

---

## Testing Strategy

### Unit Tests:
- Verify `get_lang_dir` and `get_status_dir` mappings in `lib/common.sh`.

### Integration Tests:
1. Set `OVERLORD_BASE_DIR` to a temporary directory.
2. Run `overlord new test-proj --py`.
3. Verify project is created in the temporary directory.
4. Verify `overlord list` shows the new project.

## References

- Original ticket: `thoughts/tickets/debt_consistency_issues.md`
- Research document: `thoughts/research/2025-12-18_codebase_consistency_issues.md`
- Similar implementation: `overlord-add:89-95` (template fallback pattern)

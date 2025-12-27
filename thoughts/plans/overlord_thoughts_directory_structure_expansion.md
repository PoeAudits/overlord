# Overlord Thoughts Directory Structure Expansion

## Overview

Expand the thoughts directory structure created by `overlord init` and `overlord sync` to include all 8 OpenCode-standard subdirectories plus the overlord-specific `handoffs/` directory. This enables projects to support all three OpenCode workflows (Quick Flow, AI Development Flow, and Spec Flow) from initialization.

## Current State Analysis

**Current directories created** (lib/common.sh:142):
```bash
for subdir in tickets plans logs research handoffs; do
```

Creates 5 subdirectories:
- `tickets/` - Task definitions (bug, feature, debt)
- `plans/` - Implementation plans with phases
- `logs/` - Execution logs (auto-generated)
- `research/` - Codebase and documentation research
- `handoffs/` - Custom overlord directory

**Missing directories from OpenCode specification:**
- `proposals/` - Spec-driven change proposals (required for Spec Flow)
- `specs/` - Canonical system specifications (required for Spec Flow)
- `reviews/` - Validation reports (optional but recommended)
- `archive/` - Completed work organized by date

**Where the function is called:**
- `overlord init` (overlord-init:227) - Creates thoughts structure during project initialization
- `overlord sync` (overlord-sync:308) - Adds missing directories to existing projects (always additive)

**Function characteristics:**
- Already designed as additive (only creates missing directories)
- Never overwrites existing directories
- Logs success only if new directories were created
- Called after Makefile and opencode.jsonc setup

## Desired End State

After running `overlord init` or `overlord sync`, the `thoughts/` directory should contain all 9 subdirectories organized by workflow phase:

```
thoughts/
├── tickets/        # Task definitions (bugs, features, debt)
├── research/       # Codebase and documentation research
├── plans/          # Implementation plans with phases
├── proposals/      # Spec-driven change proposals (Spec Flow only)
├── specs/          # Canonical system specifications (Spec Flow only)
├── logs/           # Execution records (auto-generated)
├── reviews/        # Validation reports (optional)
├── archive/        # Completed work organized by date
└── handoffs/       # Overlord-specific: team handoff documentation
```

**Verification:**
- `overlord init` creates all 9 subdirectories on first initialization
- `overlord sync` creates missing subdirectories on existing projects
- No existing directories are modified or deleted
- Projects can immediately use all three OpenCode workflows

## What We're NOT Doing

- Migrating existing `handoffs/` usage (backward compatible)
- Creating initial files in any subdirectory (structure only)
- Changing how `overlord init` or `overlord sync` are called
- Modifying registry.json or project metadata
- Changing the additive/non-destructive behavior of either command

## Implementation Approach

Single-function update to `lib/common.sh:create_thoughts_dirs()`. Change the loop from 5 subdirectories to 9 subdirectories. This single change propagates to both `overlord init` and `overlord sync` automatically.

The function already implements:
- Existence checking (only creates if missing)
- Non-destructive behavior (never overwrites)
- Success/no-op logging
- Directory creation with proper parent handling

No changes needed to calling code or function signature.

## Phase 1: Update create_thoughts_dirs() Function

### Overview

Update the subdirectory loop in `lib/common.sh` to include all 9 directories: tickets, research, plans, proposals, specs, logs, reviews, archive, and handoffs.

### Changes Required:

#### 1. lib/common.sh - Update create_thoughts_dirs() function

**File**: `lib/common.sh` (lines 136-152)

**Current code:**
```bash
# Create thoughts directory structure (always additive)
create_thoughts_dirs() {
  local dir="$1"
  local thoughts_dir="$dir/thoughts"
  local created=false
  
  for subdir in tickets plans logs research handoffs; do
    if [[ ! -d "$thoughts_dir/$subdir" ]]; then
      mkdir -p "$thoughts_dir/$subdir"
      created=true
    fi
  done
  
  if [[ "$created" == true ]]; then
    log_success "Created thoughts/ directory structure"
  fi
}
```

**Changes:**
- Line 142: Change subdirectory list from `tickets plans logs research handoffs` to `tickets research plans proposals specs logs reviews archive handoffs`

**Updated code:**
```bash
# Create thoughts directory structure (always additive)
create_thoughts_dirs() {
  local dir="$1"
  local thoughts_dir="$dir/thoughts"
  local created=false
  
  for subdir in tickets research plans proposals specs logs reviews archive handoffs; do
    if [[ ! -d "$thoughts_dir/$subdir" ]]; then
      mkdir -p "$thoughts_dir/$subdir"
      created=true
    fi
  done
  
  if [[ "$created" == true ]]; then
    log_success "Created thoughts/ directory structure"
  fi
}
```

**Rationale for ordering:**
- Grouped by workflow phase for readability: Intake (tickets, research), Planning (plans, proposals, specs), Execution (logs, reviews), and Completion (archive)
- Maintained `handoffs/` at end (custom to overlord)

### Success Criteria:

#### Automated Verification:

- [x] Run `overlord init test-proj-1 --base` and verify all 9 directories exist:
  - `test-proj-1/thoughts/{tickets,research,plans,proposals,specs,logs,reviews,archive,handoffs}/` all exist
  - Command output shows "Created thoughts/ directory structure"

- [x] Run `overlord init test-proj-2 --py` and verify all 9 directories exist:
  - Repeat for TypeScript: `overlord init test-proj-3 --ts`
  - Repeat for Solidity: `overlord init test-proj-4 --sol`

- [ ] Test sync on existing project without thoughts structure:
  - Create temporary project directory without thoughts
  - Run `overlord sync <proj> --dry-run` and verify it would create thoughts structure
  - Run `overlord sync <proj>` and verify all 9 directories created

- [ ] Test sync with partial thoughts structure (backward compatibility):
  - Create project with only 5 old directories (tickets, plans, logs, research, handoffs)
  - Run `overlord sync <proj>` and verify only 4 new directories created (proposals, specs, reviews, archive)
  - Original 5 directories unchanged

- [ ] Test sync with `--all` flag:
  - Run `overlord sync --all` and verify all projects get complete structure
  - No errors or warnings in output

#### Manual Verification:

- [ ] Initialize a new project and confirm directory tree matches OpenCode specification
- [ ] Run `overlord init` on a pre-existing directory with git repo and verify no data loss
- [ ] Verify that `overlord sync --all` completes successfully with no warnings for all projects
- [ ] Check that existing handoffs files are untouched after sync
- [ ] Verify the structure works correctly with OpenCode workflows (ability to create tickets, plans, proposals, etc.)

---

## Testing Strategy

### Unit Tests:
- Verify single directory creation (isolated invocation)
- Verify all 9 directories created in sequence
- Verify idempotency (running twice on same project creates no duplicates)
- Verify with various path formats (relative, absolute, with ~)

### Integration Tests:
- `overlord init` with each language (base, python, typescript, solidity)
- `overlord init` with status flags (default active, --lib for library)
- `overlord sync` on single project
- `overlord sync --all` with language filters
- `overlord sync --dry-run` shows correct structure

### Manual Testing Steps:
1. Create test directory and run `overlord init` - verify all 9 subdirectories exist
2. Run `overlord init` on directory with existing thoughts/ - verify no changes
3. Run `overlord sync` on existing project without complete structure - verify only missing directories created
4. Delete one subdirectory manually, run `overlord sync` - verify it's recreated
5. Run `overlord sync --all --dry-run` - verify output shows proposed changes
6. Create a ticket/plan/proposal in new project - verify workflows function correctly

## Migration Notes

**For existing projects:**
- No action required from users
- First `overlord sync --all` will automatically add missing directories to all projects
- Completely non-destructive (never modifies or deletes existing directories)
- Handoffs directory preserved as-is

**For new projects:**
- `overlord init` automatically creates complete 9-directory structure
- Ready to use all OpenCode workflows immediately

## Deviations from Plan

### Phase 1: Update create_thoughts_dirs() Function
- **Original Plan**: Expected `overlord sync` to call `create_thoughts_dirs()` unconditionally since the function is additive.
- **Actual Implementation**: Found that `overlord-sync` had a condition preventing `create_thoughts_dirs()` from being called when `thoughts/tickets` already existed, which broke backward compatibility for projects with partial thoughts structures.
- **Reason for Deviation**: The condition `if [[ ! -d "$thoughts_path/tickets" ]]` on line 307 prevented adding missing subdirectories to existing partial thoughts structures.
- **Impact Assessment**: Fixed by removing the condition, allowing `create_thoughts_dirs()` to always run its additive logic. This ensures projects with old 5-directory structures get the 4 new directories when running `overlord sync`.
- **Date/Time**: 2025-12-26T18:30:00Z

## References

- Original ticket: `thoughts/tickets/feature_overlord_thoughts_directory_structure.md`
- OpenCode specification: `/home/thomas/.config/opencode/AGENTS.md` (Thoughts Directory Structure section)
- Implementation location: `/home/thomas/.config/overlord/lib/common.sh` lines 136-152
- Called by: `overlord init` (line 227) and `overlord sync` (line 308)

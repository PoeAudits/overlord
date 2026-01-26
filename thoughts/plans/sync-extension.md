# Orchestration Plan: Overlord v2 Multi-Machine Sync Extension

## Overview

**Objective:** Extend the existing Overlord v2 Go CLI with multi-machine file synchronization capabilities, enabling projects to be tagged as active/inactive for sync and transferred between storage (desktop) and working-set machines (VPS, laptop) via explicit commands.

**Phases:** 4 phases, 12 total tasks

**Estimated Complexity:** Complex

**Key Skills Required:**
- `go-coding-guidelines` - All Go implementation tasks
- `go-testing` - Unit tests for new modules
- `go-backend-development` - CLI commands and subprocess handling

---

## Phase 1: Foundation ⏳

**Status:** ⏳ Pending
**Goal:** Extend registry schema and create machine configuration system

**Dependencies:** None

**Parallel Execution:** Tasks 1.1, 1.2, and 1.3 can run in parallel (independent modules)

### Task 1.1: Extend Settings with SyncSettings ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Add a new `SyncSettings` struct to the registry types and integrate it into the existing `Settings` struct. This will hold global sync defaults like exclusion patterns.

**Context:**
From the planning brief:
- Global exclusion patterns for sync (node_modules, .venv, __pycache__, etc.)
- Must be backward compatible (new field is optional with `omitempty`)
- Existing registries without sync settings should still load correctly

**References:**
- Existing types: `/home/thomas/Overlord/core/tools/overlord-v2/internal/registry/types.go`
- Follow the existing validation pattern (Settings.Validate())

**Success Criteria:**
- [ ] `SyncSettings` struct created with `DefaultExclude []string` field
- [ ] `Settings` struct extended with `Sync SyncSettings` field (yaml tag with omitempty)
- [ ] `SyncSettings.Validate()` method implemented (can be permissive)
- [ ] `Settings.Validate()` updated to call `SyncSettings.Validate()` if present
- [ ] Existing tests still pass (backward compatibility)
- [ ] New tests for SyncSettings validation

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- Do not modify existing field names or types
- New fields must have `omitempty` YAML tags for backward compatibility
- Follow existing validation patterns in types.go

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 1.2: Extend Project with ProjectSync ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Add sync-related fields to the Project struct, including sync status (active/inactive) and per-project exclusion patterns.

**Context:**
From the planning brief:
- `SyncStatus` type with values: active, inactive
- `ProjectSync` struct with Status and Exclude fields
- Sync status is orthogonal to archive status (project can be active+synced, active+not-synced, or archived)
- Per-project excludes are optional, global defaults apply when empty

**References:**
- Existing Project struct: `/home/thomas/Overlord/core/tools/overlord-v2/internal/registry/types.go`
- Follow the existing State enum pattern for SyncStatus

**Success Criteria:**
- [ ] `SyncStatus` type created with `SyncActive` and `SyncInactive` constants
- [ ] `SyncStatus.IsValid()` method implemented
- [ ] `ProjectSync` struct created with `Status SyncStatus` and `Exclude []string` fields
- [ ] `Project` struct extended with `Sync ProjectSync` field (yaml tag with omitempty)
- [ ] `ProjectSync.Validate()` method implemented
- [ ] `Project.Validate()` updated to validate Sync if present
- [ ] New tests for ProjectSync and SyncStatus validation
- [ ] Existing tests still pass

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- SyncStatus defaults to empty (treated as inactive for backward compatibility)
- Do not modify existing State enum (archive status is separate)
- Follow the existing enum pattern (type, constants, IsValid method)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 1.3: Create Machine Configuration Module ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Create a new module for loading machine-specific configuration from `~/.config/overlord/machine.yaml`. This file is local to each machine and not synced.

**Context:**
From the planning brief:
- Machine config contains: name, role (storage/working-set), storage_host
- Config file location: `~/.config/overlord/machine.yaml`
- This file is NOT synced (local to each machine)
- Role determines sync behavior:
  - storage: has all projects, receives sync from working-set machines
  - working-set: only has active projects, syncs to/from storage

**References:**
- Existing store pattern: `/home/thomas/Overlord/core/tools/overlord-v2/internal/registry/store.go`
- Use same expandPath utility for ~ expansion

**Success Criteria:**
- [ ] New package `internal/machine/` created
- [ ] `Role` type with `RoleStorage` and `RoleWorkingSet` constants
- [ ] `MachineConfig` struct with Name, Role, StorageHost fields
- [ ] `Load()` function to read machine.yaml
- [ ] Default config returned if file doesn't exist (name=hostname, role=storage)
- [ ] `Validate()` method for MachineConfig
- [ ] Unit tests for load and validation

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- Do not create a Save function (machine.yaml is manually created)
- StorageHost only required when role is working-set
- Use os.Hostname() as default name if not specified

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 2: Sync Infrastructure ⏳

**Status:** ⏳ Pending
**Goal:** Build the core sync mechanisms (rsync wrapper, git operations, conflict detection)

**Dependencies:** Phase 1 (needs SyncSettings for exclusion patterns)

**Parallel Execution:** Tasks 2.1 and 2.2 can run in parallel; Task 2.3 depends on 2.1

### Task 2.1: Create rsync Wrapper Module ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Create a module that wraps rsync for bidirectional file synchronization between machines over SSH.

**Context:**
From the planning brief:
- Use rsync over SSH (Tailscale provides network)
- Support exclusion patterns (passed via --exclude flags)
- Bidirectional sync: can push (local→remote) or pull (remote→local)
- Progress indication for large transfers
- Resumable transfers (rsync handles this natively)

**References:**
- rsync command patterns: `rsync -avz --progress --exclude=... src/ host:dest/`
- Existing subprocess patterns in the codebase (if any)

**Success Criteria:**
- [ ] New package `internal/sync/` created
- [ ] `RsyncOptions` struct with Source, Dest, Excludes, DryRun, Delete fields
- [ ] `Push(opts RsyncOptions)` function for local→remote sync
- [ ] `Pull(opts RsyncOptions)` function for remote→local sync
- [ ] Proper handling of SSH host format (user@host:path)
- [ ] Exclusion patterns correctly passed to rsync
- [ ] Error handling for rsync failures (non-zero exit codes)
- [ ] Capture and return rsync output for progress/debugging
- [ ] Unit tests with mocked exec.Command

**Required Skills:**
- `go-coding-guidelines`
- `go-backend-development`

**Constraints:**
- Do not reinvent rsync - use the actual rsync binary
- Assume rsync is installed on both machines
- Use exec.Command with proper argument escaping
- Handle paths with spaces correctly

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 2.2: Create Git Operations Module ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Create a module for git operations needed to sync the registry: add, commit, push, and pull.

**Context:**
From the planning brief:
- Registry lives in a git repo (dotfiles)
- activate/deactivate commands auto-commit and push registry changes
- sync command pulls registry before syncing files
- Git operations should be automatic and not require user intervention

**References:**
- Git command patterns: `git -C <repo-path> add/commit/push/pull`
- Registry path from machine config or default

**Success Criteria:**
- [ ] New package `internal/gitops/` created
- [ ] `GitOps` struct with RepoPath field
- [ ] `Add(files ...string)` method
- [ ] `Commit(message string)` method
- [ ] `Push()` method
- [ ] `Pull()` method
- [ ] `HasChanges()` method to check if there are uncommitted changes
- [ ] Proper error handling for git failures
- [ ] Unit tests with mocked exec.Command

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- Use `git -C <path>` to operate on specific repo
- Do not use interactive git commands (no -i flags)
- Commit messages should be prefixed with "overlord: "
- Handle case where repo has no remote configured (warn but don't fail on push)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 2.3: Create Conflict Detection Module ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Create a module to detect file conflicts between local and remote versions before sync.

**Context:**
From the planning brief:
- Conflicts occur when files are modified on both sides since last sync
- Should warn about conflicts, not auto-resolve
- User must manually resolve conflicts
- Use mtime comparison or file checksums

**References:**
- rsync dry-run output can show what would change
- Task 2.1 rsync wrapper (depends on this for dry-run capability)

**Success Criteria:**
- [ ] `Conflict` struct with Path, LocalMtime, RemoteMtime fields
- [ ] `DetectConflicts(localPath, remoteHost, remotePath string)` function
- [ ] Uses rsync dry-run to identify files that differ
- [ ] Distinguishes between: local-only, remote-only, both-modified
- [ ] Returns list of conflicts for user review
- [ ] Unit tests with sample scenarios

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- Do not auto-resolve conflicts
- For MVP, detecting "both sides have changes" is sufficient
- Can use simpler heuristics initially (e.g., if file exists on both and differs, it's a conflict)

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 3: Commands ⏳

**Status:** ⏳ Pending
**Goal:** Implement the activate, deactivate, and sync commands

**Dependencies:** Phase 1, Phase 2

**Parallel Execution:** Task 3.1 can start first; 3.2 can run in parallel once 3.1 structure is established; 3.3 depends on 3.1 and 3.2

### Task 3.1: Implement Activate Command ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Implement the `overlord activate <project>` command that marks a project for sync and commits/pushes the registry change.

**Context:**
From the planning brief:
- Sets project's sync.status to "active"
- Auto-commits and pushes registry to git
- Uses existing fuzzy matching for project resolution
- Works with project aliases

**References:**
- Existing command patterns: `/home/thomas/Overlord/core/tools/overlord-v2/internal/cmd/`
- Registry resolution: `internal/registry/resolve.go`
- Git operations: Task 2.2 module

**Success Criteria:**
- [ ] New file `internal/cmd/activate.go` created
- [ ] `overlord activate <project>` command registered with Cobra
- [ ] Uses existing registry.Resolve() for fuzzy matching
- [ ] Updates project's Sync.Status to SyncActive
- [ ] Saves registry (existing atomic write)
- [ ] Calls gitops.Add(), gitops.Commit(), gitops.Push()
- [ ] Clear success/error messages
- [ ] Handles case where project is already active (no-op with message)
- [ ] Handles case where project is archived (error)

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Follow existing command patterns (look at archive.go as reference)
- Use existing error message styling
- Git push failure should warn but not fail the command (registry is still updated locally)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 3.2: Implement Sync Command ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Implement the `overlord sync [project]` command that synchronizes files between machines based on sync status and machine role.

**Context:**
From the planning brief:
- Pulls registry from git first
- On working-set machine: pulls active projects from storage, pushes local changes back
- On storage machine: receives changes from working-set machines
- Respects exclusion patterns (global + per-project)
- Supports --dry-run flag
- Can sync all active projects or a specific project

**References:**
- Machine config: Task 1.3 module
- rsync wrapper: Task 2.1 module
- Conflict detection: Task 2.3 module
- Registry types: Phase 1 tasks

**Success Criteria:**
- [ ] New file `internal/cmd/sync.go` created
- [ ] `overlord sync` command registered with Cobra
- [ ] `overlord sync <project>` variant for single project
- [ ] `--dry-run` flag to preview sync without executing
- [ ] Pulls git registry before sync operations
- [ ] Loads machine config to determine role
- [ ] For working-set role:
  - [ ] Pulls missing active projects from storage
  - [ ] Syncs existing active projects bidirectionally
- [ ] For storage role:
  - [ ] Accepts incoming sync (passive, or explicit pull from working-set)
- [ ] Merges global + per-project exclusion patterns
- [ ] Checks for conflicts before sync, warns if found
- [ ] Progress indication during sync
- [ ] Clear summary of what was synced

**Required Skills:**
- `go-coding-guidelines`
- `go-backend-development`

**Constraints:**
- Conflict detection is advisory - warn but allow user to proceed
- If storage host is unreachable, fail with clear error
- Large syncs should show progress (rsync --progress)

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 3.3: Implement Deactivate Command ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Implement the `overlord deactivate <project>` command that syncs final changes, marks project inactive, and removes from working-set machine.

**Context:**
From the planning brief:
- First syncs any local changes back to storage
- Sets project's sync.status to "inactive"
- Removes project directory from working-set machine (if on working-set)
- Auto-commits and pushes registry to git
- On storage machine, just updates status (no directory removal)

**References:**
- Activate command: Task 3.1 (similar structure)
- Sync functionality: Task 3.2
- rm command pattern: `/home/thomas/Overlord/core/tools/overlord-v2/internal/cmd/rm.go`

**Success Criteria:**
- [ ] New file `internal/cmd/deactivate.go` created
- [ ] `overlord deactivate <project>` command registered with Cobra
- [ ] Uses existing registry.Resolve() for fuzzy matching
- [ ] Syncs project to storage before deactivating (if on working-set)
- [ ] Updates project's Sync.Status to SyncInactive
- [ ] Saves registry
- [ ] Calls gitops.Add(), gitops.Commit(), gitops.Push()
- [ ] On working-set: removes project directory after successful sync
- [ ] Confirmation prompt before directory removal
- [ ] `--force` flag to skip confirmation
- [ ] Handles case where project is already inactive (no-op with message)
- [ ] Handles sync failure gracefully (don't remove if sync failed)

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- NEVER remove directory if sync to storage failed
- Confirmation required for directory removal (unless --force)
- On storage machine, only update status (no directory removal)
- Archived projects cannot be deactivated (must unarchive first)

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Phase 4: Integration & Polish ⏳

**Status:** ⏳ Pending
**Goal:** Integration testing, documentation, and final polish

**Dependencies:** Phase 3

**Parallel Execution:** Tasks 4.1 and 4.2 can run in parallel after Phase 3; 4.3 depends on 4.1 and 4.2

### Task 4.1: Integration Testing ⏳

**Status:** ⏳ Pending
**Subagent:** executor

**Task Description:**
Create integration tests that verify the full sync workflow works end-to-end.

**Context:**
- Need to test activate → sync → deactivate workflow
- Test conflict detection and warning
- Test exclusion patterns are respected
- Test machine role behavior differences

**References:**
- Existing test patterns in the codebase
- All Phase 3 command implementations

**Success Criteria:**
- [ ] Integration test file `internal/cmd/sync_integration_test.go`
- [ ] Test: activate project updates registry and triggers git
- [ ] Test: sync pulls/pushes files correctly (mock rsync or use temp dirs)
- [ ] Test: deactivate syncs, updates registry, removes directory
- [ ] Test: exclusion patterns prevent sync of node_modules, .venv
- [ ] Test: conflict detection warns when files differ
- [ ] Test: dry-run shows changes without executing
- [ ] All tests pass

**Required Skills:**
- `go-coding-guidelines`
- `go-testing`

**Constraints:**
- Integration tests can use mock commands or temp directories
- Don't require actual SSH/remote machine for tests
- Tests should be runnable in CI environment

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 4.2: Update Documentation ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Update README.md and AGENTS.md to document the new sync commands and configuration.

**Context:**
- Need to document: activate, deactivate, sync commands
- Need to document: machine.yaml configuration
- Need to document: sync settings in registry
- Need to document: workflow for multi-machine usage

**References:**
- Existing README.md: `/home/thomas/Overlord/core/tools/overlord-v2/README.md`
- Existing AGENTS.md: `/home/thomas/Overlord/core/tools/overlord-v2/AGENTS.md`

**Success Criteria:**
- [ ] README.md updated with:
  - [ ] New commands section (activate, deactivate, sync)
  - [ ] Machine configuration section
  - [ ] Multi-machine workflow example
  - [ ] Sync settings in registry section
- [ ] AGENTS.md updated with:
  - [ ] New sync-related types and patterns
  - [ ] New command files in directory structure
  - [ ] New modules (machine, sync, gitops)
  - [ ] Common tasks: adding new exclusion patterns

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Follow existing documentation style
- Include practical examples
- Document both CLI usage and registry configuration

**Completion Notes:** _(filled by orchestrator after completion)_

---

### Task 4.3: CLI Polish and Help Text ⏳

**Status:** ⏳ Pending
**Subagent:** worker

**Task Description:**
Polish the new commands with consistent styling, helpful error messages, and complete help text.

**Context:**
- New commands should match the style of existing commands
- Help text should include examples
- Error messages should be actionable

**References:**
- Existing command help text patterns in `/home/thomas/Overlord/core/tools/overlord-v2/internal/cmd/`
- Lipgloss styling used in list.go

**Success Criteria:**
- [ ] All new commands have descriptive `--help` output
- [ ] Examples included in help text for complex commands
- [ ] Error messages include suggested fixes
- [ ] Success messages are clear and styled consistently
- [ ] `overlord --help` updated to show new commands
- [ ] Consistent color scheme with existing commands

**Required Skills:**
- `go-coding-guidelines`

**Constraints:**
- Match existing styling (Lipgloss patterns)
- Don't over-style - keep it readable
- Ensure colors work on both light and dark terminals

**Completion Notes:** _(filled by orchestrator after completion)_

---

## Execution Notes

### Parallelization Opportunities

**Phase 1:**
- Tasks 1.1, 1.2, 1.3 can all run in parallel (independent modules)

**Phase 2:**
- Tasks 2.1 and 2.2 can run in parallel
- Task 2.3 depends on 2.1 (uses rsync dry-run)

**Phase 3:**
- Task 3.1 should start first (establishes pattern)
- Task 3.2 can run in parallel with 3.1 once structure is clear
- Task 3.3 depends on both 3.1 and 3.2 (uses sync functionality)

**Phase 4:**
- Tasks 4.1 and 4.2 can run in parallel
- Task 4.3 can run after 4.1 (may surface UX issues)

### Critical Path

```
1.1 ─┬─> 2.1 ─┬─> 2.3 ─┬─> 3.2 ─┬─> 3.3 ─> 4.1 ─> 4.3
1.2 ─┤        │        │        │
1.3 ─┤        │        │        │
     │   2.2 ─┴────────┴─> 3.1 ─┤
     │                          │
     └──────────────────────────┘
```

Longest path: 1.1 → 2.1 → 2.3 → 3.2 → 3.3 → 4.1 → 4.3

### Risk Points

| Task | Risk | Mitigation |
|------|------|------------|
| 2.1 rsync wrapper | Subprocess handling complexity | Start simple, add features incrementally |
| 2.3 Conflict detection | May be complex to get right | MVP: just detect differences, refine later |
| 3.2 Sync command | Most complex task, many code paths | Break into helper functions, test thoroughly |
| 3.3 Deactivate | Directory removal is destructive | Require confirmation, sync-first safety |

### Review Checkpoints

- After Phase 1 completion (schema extensions solid before building on them)
- After Task 2.1 completion (rsync wrapper is critical infrastructure)
- After Phase 3 completion (all commands working)
- Final review after Phase 4

### Documentation Checkpoints

- After Phase 1 review passes (document new registry fields)
- After Phase 3 review passes (document new commands)
- After Phase 4 review passes (final documentation polish)

---

## Task Summary Table

| Phase | Task | Name | Subagent | Dependencies | Parallel? | Status |
|-------|------|------|----------|--------------|-----------|--------|
| 1 | 1.1 | Extend Settings with SyncSettings | worker | none | yes (with 1.2, 1.3) | ⏳ |
| 1 | 1.2 | Extend Project with ProjectSync | worker | none | yes (with 1.1, 1.3) | ⏳ |
| 1 | 1.3 | Create Machine Configuration Module | worker | none | yes (with 1.1, 1.2) | ⏳ |
| 2 | 2.1 | Create rsync Wrapper Module | executor | Phase 1 | yes (with 2.2) | ⏳ |
| 2 | 2.2 | Create Git Operations Module | worker | Phase 1 | yes (with 2.1) | ⏳ |
| 2 | 2.3 | Create Conflict Detection Module | worker | 2.1 | no | ⏳ |
| 3 | 3.1 | Implement Activate Command | executor | Phase 1, 2.2 | partial | ⏳ |
| 3 | 3.2 | Implement Sync Command | executor | Phase 1, Phase 2 | partial | ⏳ |
| 3 | 3.3 | Implement Deactivate Command | executor | 3.1, 3.2 | no | ⏳ |
| 4 | 4.1 | Integration Testing | executor | Phase 3 | yes (with 4.2) | ⏳ |
| 4 | 4.2 | Update Documentation | worker | Phase 3 | yes (with 4.1) | ⏳ |
| 4 | 4.3 | CLI Polish and Help Text | worker | 4.1, 4.2 | no | ⏳ |

---

## Execution Status

_This section is updated by the orchestrator during execution._

**Last Updated:** not started
**Current Phase:** -
**Current Task:** -

### Progress
- Phases complete: 0 of 4
- Tasks complete: 0 of 12

### Divergences from Plan
_(none yet)_

### Handoff History
_(none)_

---

## Appendix: Key Decisions Reference

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Registry sync | Git (auto commit/push) | Leverages existing dotfiles, explicit history |
| File sync | rsync over SSH | Battle-tested, delta transfers, exclusion patterns |
| Sync trigger | Manual command | User controls when sync happens |
| Conflict handling | Warn, don't resolve | User decides how to handle |
| Machine roles | Configurable per-machine | Flexibility for laptop |
| New sync status | Separate from archive state | Orthogonal concerns |

## Appendix: File Locations

**New Files to Create:**
```
internal/
├── machine/
│   ├── config.go          # Task 1.3
│   └── config_test.go
├── sync/
│   ├── rsync.go           # Task 2.1
│   ├── rsync_test.go
│   ├── conflict.go        # Task 2.3
│   └── conflict_test.go
├── gitops/
│   ├── git.go             # Task 2.2
│   └── git_test.go
└── cmd/
    ├── activate.go        # Task 3.1
    ├── deactivate.go      # Task 3.3
    ├── sync.go            # Task 3.2
    └── sync_integration_test.go  # Task 4.1
```

**Files to Modify:**
```
internal/registry/types.go     # Tasks 1.1, 1.2
internal/registry/types_test.go
README.md                       # Task 4.2
AGENTS.md                       # Task 4.2
```

## Appendix: Schema Changes

**Settings Extension (Task 1.1):**
```go
type SyncSettings struct {
    DefaultExclude []string `yaml:"default_exclude,omitempty"`
}

type Settings struct {
    BaseDir     string       `yaml:"base_dir"`
    ThoughtsDir string       `yaml:"thoughts_dir"`
    Sync        SyncSettings `yaml:"sync,omitempty"`
}
```

**Project Extension (Task 1.2):**
```go
type SyncStatus string

const (
    SyncActive   SyncStatus = "active"
    SyncInactive SyncStatus = "inactive"
)

type ProjectSync struct {
    Status  SyncStatus `yaml:"status,omitempty"`
    Exclude []string   `yaml:"exclude,omitempty"`
}

type Project struct {
    // ... existing fields ...
    Sync ProjectSync `yaml:"sync,omitempty"`
}
```

**Machine Config (Task 1.3):**
```go
type Role string

const (
    RoleStorage    Role = "storage"
    RoleWorkingSet Role = "working-set"
)

type MachineConfig struct {
    Name        string `yaml:"name"`
    Role        Role   `yaml:"role"`
    StorageHost string `yaml:"storage_host,omitempty"`
}
```

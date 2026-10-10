# Overlord Improvement Audit

## Status and Scope

This document is a proposal-only synthesis of one repository exploration and
three independent read-only audits:

- architecture improvement, using the `improve-codebase-architecture` command
  fallback and the `codebase-design` skill;
- standards, correctness, and code-quality review using the `code-review`
  skill;
- deep-module and seam review using the `codebase-design` skill.

No recommendation in this report has been implemented. No tests, builds,
formatters, package managers, generators, or artifact-producing commands were
run as part of the audit.

The audited repository is:

`/home/thomas/Overlord/core/tools/overlord`

The tracked working tree was clean during the audits. The visible
`/home/thomas/Overlord/core/tools/overlord/scripts/ta` file was untracked and is
not treated as authoritative product architecture. Generated binaries,
`.benchmark/`, `.opencode/`, and host runtime state were excluded.

## Executive Summary

Overlord has a strong central purpose: it is a single-host project catalog and
workspace launcher whose registry owns project identity and whose adapters
connect that catalog to filesystems, tmux, Git, and a local service stack. The
thin process entrypoint, versioned registry, tmux runner seam, bounded
checkpoint format, embedded templates, and candid project model are all worth
preserving.

The highest-risk weakness is that path resolution currently also grants
destructive authority. A malformed or manually edited registry can direct
`rm --force` or `clear` at a privileged, shared, overlapping, or unrelated
path. The second systemic weakness is that destructive filesystem operations
often happen before registry persistence. Save failure can therefore leave the
registry and filesystem irreconcilably different.

The highest-priority improvement set is:

1. Introduce a canonical project-path classification and destructive-operation
   policy before any further command refactoring.
2. Put removal and relocation behind one invariant-owning project-transition
   module with explicit planning, commit, partial-success, and recovery
   semantics.
3. Fix stack persistence so a failed save cannot remove the active
   configuration.
4. Stop using host-global `tailscale serve reset`; reconcile only routes owned
   by Overlord.
5. Separate structured process data from explicitly trusted shell programs.
6. Deepen registry validation to own global identity, alias, path, and state
   invariants.
7. Make stack and checkpoint results describe observed state rather than tmux
   topology alone.
8. Move application workflows below Cobra incrementally, beginning with the
   destructive commands.

These changes should precede broad decomposition of large command files. The
correct target is a few deeper modules, not a proliferation of thin wrappers.

## System Model

The maintained system has five principal flows:

```text
project catalog
  registry.yaml -> resolve/list/info/path/add/new/rm/archive

workspace launch
  project identity -> tmux session -> optional trusted .tmux.local hook

local stack
  stack.yaml -> service specifications -> StartServer tmux topology

checkpointing
  tmux topology -> workspace-checkpoints.yaml -> missing-session restoration

portable export
  project selection -> Git/filesystem discovery -> filtered ZIP archive
```

Primary state lives outside the repository:

- `~/.config/overlord/registry.yaml`
- `~/.config/overlord/stack.yaml`
- `~/.config/overlord/workspace-checkpoints.yaml`
- configured project and archive directories
- tmux sessions and service processes
- selective backup ZIP files

The intended operating model is one host, one user, and effectively one
mutating process. This report does not propose distributed coordination or
multi-machine synchronization.

## Findings and Proposals

### 1. Critical: Path Resolution Grants Unbounded Destructive Authority

**Evidence**

- Registry validation requires only a non-empty project path:
  `/home/thomas/Overlord/core/tools/overlord/internal/registry/types.go:148-194`.
- Absolute paths remain absolute during resolution:
  `/home/thomas/Overlord/core/tools/overlord/internal/fileutil/fileutil.go:152-169`.
- `rm --force` passes the resolved path to recursive deletion:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/rm.go:94-147`.
- `clear` does the same for each selected project:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/clear.go:110-156`.
- External absolute projects are an intentional feature:
  `/home/thomas/Overlord/core/tools/overlord/PROJECT_MODEL.md:90-95`.

**Problem**

Catalog lookup and permission to mutate are currently the same interface. A
hand-edited, migrated, or corrupt registry can identify `/`, `$HOME`, the base
directory, a category root, another project, or an ancestor shared by multiple
projects. Confirmation displays the path but does not enforce a hard boundary.
Name or alias ambiguity can further target the wrong project.

**Proposal**

Add one `ProjectPathPolicy` or equivalent module that canonicalizes and
classifies paths before destructive effects. It should distinguish:

- a managed project beneath the configured base directory;
- a deliberately registered external project;
- a privileged root, home, base, category, or archive directory;
- a path overlapping another registered project;
- a symlink, symlink escape, missing path, inaccessible path, or wrong kind.

Removal should require a validated mutation capability, not a raw string.
External projects may remain supported, but destructive use should require a
stronger explicit confirmation that displays canonical path and classification.

**Benefit**

This creates a safety invariant independent of registry correctness and user
attentiveness.

**Risk and cost**

Physical canonicalization is difficult for missing paths and intentional
symlinked project trees. The policy must be specified rather than assuming all
external or symlinked projects are invalid.

### 2. Critical: Project Transitions Can Diverge Filesystem and Registry State

**Evidence**

- `rm --force` deletes before lifecycle dispatch, registry mutation, and save:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/rm.go:94-147`.
- `clear` performs multiple deletions before one final save:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/clear.go:110-165`.
- Archive moves directories before the batch registry save:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/archive.go:138-231`.
- Unarchive likewise moves before persistence:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/unarchive.go:205-267`.
- The limitation is acknowledged in
  `/home/thomas/Overlord/core/tools/overlord/PROJECT_MODEL.md:176-181,238-243`.

**Problem**

File-level atomic YAML replacement does not make a multi-resource workflow
atomic. Registry-save failure after deletion is irreversible. Save failure
after movement leaves the project at a path the registry does not describe.
Batch commands can partially succeed but expose only coarse command-level
outcomes.

**Proposal**

Create a deep project-transition module used by `rm --force`, `clear`, archive,
and unarchive. Its narrow interface should own:

1. target resolution and canonical path authorization;
2. complete collision and overlap preflight;
3. an immutable transition plan;
4. all vetoing checks before irreversible effects;
5. filesystem application and receipts;
6. registry commit;
7. compensation where possible;
8. durable recovery information where compensation is impossible;
9. per-target outcomes for batch operations.

Deletion should use quarantine or trash semantics before registry commit if
the product accepts that tradeoff. Otherwise, it must return an explicit
irreversible partial-state result and recovery instructions. It should not
pretend to provide a transaction that cannot be rolled back.

**Benefit**

One module owns the highest-risk invariants, and all commands and tests cross
the same transition seam.

**Risk and cost**

High. Cross-filesystem movement and failed compensation remain possible.
Implement this first for `rm --force` and `clear`; archive/unarchive are
documented as low-use and should not drive the initial design.

### 3. Critical: Stack Save Can Remove the Active Configuration

**Evidence**

- Stack save renames `stack.yaml` to `.bak` before replacement:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/store.go:66-75`.
- Replacement can fail in the atomic-write helper:
  `/home/thomas/Overlord/core/tools/overlord/internal/fileutil/fileutil.go:137-149`.
- Registry save preserves the active file until replacement:
  `/home/thomas/Overlord/core/tools/overlord/internal/registry/store.go:87-97`.

**Problem**

An attempted update can leave the primary stack configuration absent and only
the backup available. This contradicts the stated atomic-write contract.

**Proposal**

Unify registry, stack, and checkpoint persistence behind one durable-document
implementation that:

- validates and serializes before touching the active file;
- writes a uniquely named sibling temporary file;
- applies private file permissions;
- flushes and closes the temporary file;
- atomically installs the replacement;
- updates the backup without first removing the active file;
- synchronizes the parent directory where crash durability is required;
- reports backup and commit outcomes separately.

**Benefit**

Consistent and testable source-of-truth persistence across all YAML state.

**Risk and cost**

Existing `.bak` expectations need an explicit compatibility decision. File and
directory synchronization have platform-specific behavior.

### 4. High: Stack Stop Deletes Unrelated Tailscale Serve Routes

**Evidence**

- Stack stop always runs `tailscale serve reset`:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/stop.go:16-45`.
- This can happen regardless of session existence or route ownership.

**Problem**

`tailscale serve reset` has host-global scope. Stopping Overlord can remove
routes created and owned by other applications or by the user.

**Proposal**

Introduce an ownership-aware route manager. Record or discover the exact
routes Overlord created and remove only those routes. Provide a dry-run plan
and never issue a global reset without a separate explicit confirmation.

**Benefit**

Aligns destructive authority with Overlord's actual ownership boundary.

**Risk and cost**

Existing routes may lack ownership metadata. Migration must preserve unknown
host state, and Tailscale's structured interfaces may vary by version.

### 5. High: Structured Settings and Trusted Shell Programs Share One Interface

**Evidence**

- A service exposes command and environment values as strings:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/service.go:18-38`.
- Environment assignments and command text are joined into one shell string:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/session.go:150-159`.
- Configured hosts, targets, models, directories, binaries, and Phoenix command
  values are interpolated in
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/service.go:86-310`.
- Project `.tmux.local` files execute through Bash:
  `/home/thomas/Overlord/core/tools/overlord/internal/tmux/tmux.go:158-181`.

**Problem**

Plain data and executable shell text are indistinguishable. Spaces, quotes, and
metacharacters can break ordinary configuration or change command execution.
Some shell execution is intentional, but the trust model is implicit.

**Proposal**

Use a typed process specification for ordinary services:

```text
ProcessSpec { program, args, environment, directory, stdio }
```

Keep an explicit `TrustedShellProgram` variant for features that genuinely
require shell syntax, such as owner-authored hooks or a deliberately free-form
Phoenix command. Centralize POSIX quoting only where tmux forces a shell
transport. Validate data fields before interpolation and document that
`stack.yaml` and `.tmux.local` have different trust levels.

**Benefit**

Safer execution, clearer tests, and less quoting knowledge distributed among
service adapters.

**Risk and cost**

Some commands use loops, pipelines, or shell operators. The migration must
preserve an explicit shell escape hatch rather than forcing all behavior into
argv form.

### 6. High: External Process Policy Is Fragmented

**Evidence**

- Tmux has an injectable command runner:
  `/home/thomas/Overlord/core/tools/overlord/internal/tmux/tmux.go:19-48`.
- Stack defines a different command seam:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/command.go:3-6`.
- Setup invokes Git directly:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/setup.go:190-261`.
- Backup invokes Git directly:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/backup.go:423-445`.
- OpenCode attach constructs an executable directly:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/oc.go:61-67`.
- Repository guidance says process calls belong behind executor seams:
  `/home/thomas/Overlord/core/tools/overlord/AGENTS.md:30-32`.

**Problem**

Production and tests do not consistently cross the same interface. Timeout,
environment, working-directory, redaction, and error policies cannot be
enforced centrally.

**Proposal**

Add one low-level process runner with structured inputs, then keep narrow Git,
tmux, service, interactive attach, and trusted-hook adapters above it. Do not
expose the generic runner directly to every Cobra command.

**Benefit**

Deterministic failure tests and one location for process policy without losing
domain-specific interfaces.

**Risk and cost**

Interactive attachment has materially different stdio and terminal behavior.
It should remain a distinct adapter method rather than forcing a universal
runner shape.

### 7. High: Registry Validation Does Not Own Global Identity Invariants

**Evidence**

- Validation checks projects independently:
  `/home/thomas/Overlord/core/tools/overlord/internal/registry/types.go:148-243`.
- Exact alias lookup returns the first Go map match:
  `/home/thomas/Overlord/core/tools/overlord/internal/registry/types.go:251-260`.
- Resolution builds fuzzy candidates from map iteration:
  `/home/thomas/Overlord/core/tools/overlord/internal/registry/resolve.go:35-115`.
- Commands mutate `reg.Projects` directly, including
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/rm.go:129-142` and
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/archive.go:202-207`.

**Problem**

The registry can contain duplicate aliases, name/alias collisions, duplicate
normalized paths, invalid map keys, or incoherent state/path combinations.
Alias resolution can become nondeterministic, including for destructive
commands.

**Proposal**

Deepen the registry module to own:

- canonical project key and alias grammar;
- global name and alias uniqueness;
- deterministic indexes and resolution order;
- normalized path uniqueness and overlap diagnostics;
- active/archive state and path coherence;
- legal mutation methods or validated next-state construction.

Existing invalid registries should receive actionable migration diagnostics
rather than unexplained hard failure.

**Benefit**

One invariant boundary protects every caller and makes destructive targeting
deterministic.

**Risk and cost**

Hand-edited registries may already violate stricter rules. A report/repair path
is needed before enforcement.

### 8. High: Cross-Filesystem Moves Do Not Match Rename Semantics

**Evidence**

- Move falls back from rename to recursive copy and deletion:
  `/home/thomas/Overlord/core/tools/overlord/internal/fileutil/fileutil.go:38-61`.
- Recursive copy writes directly into the final destination:
  `/home/thomas/Overlord/core/tools/overlord/internal/fileutil/fileutil.go:64-134`.

**Problem**

The fallback has no explicit symlink or special-file policy, can merge into an
existing destination, can leave a partial tree, and can change symlinks into
copied content. Removing the source after an incomplete semantic copy risks
loss or disclosure.

**Proposal**

Deepen `MoveDirectory` rather than wrapping it:

1. require an absent reserved destination;
2. inspect with `Lstat`;
3. explicitly preserve or reject symlinks and special files;
4. copy into a unique sibling staging directory;
5. validate completion and required metadata;
6. rename staging into place;
7. remove source only after successful installation;
8. return a receipt describing applied and cleanup states.

**Benefit**

Archive and unarchive gain one trustworthy movement contract.

**Risk and cost**

Cross-platform metadata and special-file behavior must be deliberately scoped.

### 9. High: Setup Can Hard-Reset Existing Configuration

**Evidence**

- Setup initializes or reuses Git in the configuration directory:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/setup.go:190-226`.
- It fetches and tries `git reset --hard` against guessed remote branches:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/setup.go:229-261`.

**Problem**

Local tracked configuration can be overwritten by remote content. Fetch
failure may be downgraded to a warning, so requested synchronization can fail
while setup appears successful. The remote default branch is guessed.

**Proposal**

Treat remote import as a separate explicit operation. Fetch or clone into a
staging directory, validate the resulting configuration, show a diff, and
require confirmation before import or merge. Discover the remote default
branch instead of guessing. Do not hard-reset an existing configuration tree.

**Benefit**

Protects local source-of-truth state and gives setup a truthful result.

**Risk and cost**

Conflict handling requires deliberate UX and a decision about local versus
remote authority.

### 10. Medium-High: Stack Lifecycle Models Topology, Not Service State

**Evidence**

- Existing `StartServer` causes enabled services to be skipped:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/session.go:39-61`.
- Startup records success after window creation:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/session.go:113-136`.
- Restart kills the old window before creating a replacement:
  `/home/thomas/Overlord/core/tools/overlord/internal/stack/restart.go:30-68`.
- Status relies heavily on session/window and broad health probes:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/start_status.go:36-87`.

**Problem**

Session existence is treated as idempotent success even when windows are
missing, configuration changed, or processes died. Restart can leave a service
absent and obscure partial completion. Status does not consistently prove
process ownership or readiness.

**Proposal**

Model desired and observed states separately:

```text
disabled -> planned -> starting -> running -> healthy | unhealthy | failed
```

Reconcile desired services with owned windows/processes. Return a per-service
result for start and restart, including unchanged, created, replaced, degraded,
failed, and unknown ownership. Use bounded readiness probes and preserve
manual/unowned state unless explicitly adopted.

**Benefit**

Idempotence becomes convergence, and automation can distinguish partial
success from a healthy stack.

**Risk and cost**

Process ownership is difficult for daemonizing tools and manually customized
tmux sessions. Reconciliation must remain conservative.

### 11. Medium-High: Workspace and Checkpoint Results Are Not Truthful Enough

**Evidence**

- Project name is used as tmux session identity:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/open.go:607-637`.
- Existing same-named sessions are reused without ownership metadata:
  `/home/thomas/Overlord/core/tools/overlord/internal/tmux/tmux.go:184-255`.
- Checkpoint restoration treats existing sessions as available without
  reconciling topology:
  `/home/thomas/Overlord/core/tools/overlord/internal/tmux/checkpoint.go:86-119`.
- Most window, pane, layout, and selection failures become warnings:
  `/home/thomas/Overlord/core/tools/overlord/internal/tmux/checkpoint.go:121-182`.

**Problem**

Display name, project identity, tmux target, and ownership are overloaded.
Overlord can reuse unrelated sessions. Checkpoint restore returns availability,
not whether topology was restored, skipped, or partially applied.

**Proposal**

Place a `WorkspaceManager` above the raw tmux adapter. Give owned workspaces a
stable encoded identity and metadata separate from display names. Checkpoint
restoration should validate first and return per-session outcomes:

- existing and unchanged;
- created;
- partially restored;
- failed;
- unmanaged collision.

Name restore modes honestly, for example `missing-only`, `reconcile`, and
`replace-owned`. Preserve the topology-only checkpoint boundary.

**Benefit**

Safer tmux ownership and accurate restore/attach behavior without pretending to
restore process memory or pane content.

**Risk and cost**

Existing sessions need an adoption or compatibility path. Reconciliation can
disturb intentional manual changes.

### 12. Medium: The Global Lifecycle Bus Does Not Currently Earn Its Cost

**Evidence**

- The event inventory and phase inference live in
  `/home/thomas/Overlord/core/tools/overlord/internal/lifecycle/events.go:8-58`.
- The mutable global bus lives in
  `/home/thomas/Overlord/core/tools/overlord/internal/lifecycle/bus.go:8-81`.
- No maintained production subscriber was found; subscriptions appear in
  tests.
- Hook ordering is inconsistent, including removal after deletion in
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/rm.go:94-147`.

**Problem**

Every workflow pays for global mutable state, inferred pre/post semantics,
panic recovery, and reset machinery without a demonstrated production adapter.
The package is internal, so it is not a stable external extension mechanism.

**Proposal**

Apply the deletion test after searching external deployment wiring. If no real
subscriber exists, remove the bus. If hooks are required, inject a narrow,
explicitly phased dispatcher into the specific workflow:

```text
Event { action, phase, resource, correlation_id }
```

Classify handlers as vetoing preconditions, observational post-events, or
compensating actions. Never infer failure policy from an event-name suffix.

**Benefit**

Either removes hypothetical complexity or replaces it with a stable local
contract.

**Risk and cost**

Out-of-repository subscribers must be ruled out before deletion.

### 13. Medium: Cobra Owns Too Much Application Policy

**Evidence**

- Root and child commands are package globals registered through `init`:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/root.go:14-20,108-114`.
- Helpers hard-wire default paths and global lifecycle state:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/helpers.go:18-100`.
- Tests replace package-global functions, for example
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/archive_workflow_test.go:14-23`.
- Large command files such as `open.go` and `backup.go` combine policy,
  orchestration, process calls, persistence, and presentation.

**Problem**

Cobra is the application layer rather than a thin adapter. Tests align with
mutable implementation hooks instead of operation interfaces, and multiple
independent roots cannot be safely constructed in one process.

**Proposal**

Introduce `NewRoot(...) *cobra.Command`, retaining `Execute()` as a thin
compatibility wrapper. Move operation policy behind focused modules such as
project transitions, workspace management, stack control, and backup creation.
Inject only operation-specific dependencies rather than one universal service
container. Route output through Cobra writers.

**Benefit**

Invocation-local state, better black-box tests, and smaller command handlers.

**Risk and cost**

High migration churn. Apply incrementally after characterizing the destructive
workflows, not as a repository-wide rewrite.

### 14. Medium: Backup Is Selective but Not Confidential or Verifiable Enough

**Evidence**

- Backup creation, discovery, filtering, and ZIP output are combined in
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/backup.go`.
- The archive is created with default `os.Create` permissions:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/backup.go:129-159`.
- Exclusions are incomplete secret protection:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/backup.go:604-620`.
- The narrow product boundary is documented in
  `/home/thomas/Overlord/core/tools/overlord/PROJECT_MODEL.md:160-174`.

**Problem**

Archives can contain private metadata or credentials not named `.env`, may be
locally readable depending on umask, and provide no manifest proving what was
included. Git and filesystem fallback modes have different completeness
semantics.

**Proposal**

Extract a cohesive backup module with deterministic planning, shared filtering,
Git and filesystem source adapters, atomic finalization, and an archive
manifest. Create output as `0600`. Record source roots, selection mode, included
entries, exclusions, counts, and optional checksums. Provide verification, but
do not expand this feature into full workstation restore, secret management,
encryption, retention, or off-host replication without a separate product
decision.

**Benefit**

The existing selective-export promise becomes auditable and safer without
broadening scope.

**Risk and cost**

Manifest and filename changes may affect external scripts. Secret detection
can produce false positives.

### 15. Medium: Scaffolding Has No Single Manifest or Completeness Contract

**Evidence**

- Template inventory and rendering live in
  `/home/thomas/Overlord/core/tools/overlord/internal/templates/templates.go:40-171`.
- `new` writes a separate hard-coded set:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/new.go:313-342`.
- `init` independently checks and writes templates:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/init.go:132-176`.
- Template map iteration makes write order incidental.
- Generated Makefiles can assume ecosystem files the scaffold does not create.

**Problem**

Selection, ordering, modes, overwrite behavior, cleanup, and capability claims
are duplicated. A partially failed initialization leaves earlier files in
place. “Full setup” can produce a project whose advertised commands do not run.

**Proposal**

Add a manifest-backed scaffold module:

```text
Plan(project, policy) -> ordered file intents
Render(intent) -> bytes
Apply(plan) -> apply report
```

Keep `new` ownership/rollback and `init` preserve-existing behavior as explicit
policies. Describe each language bundle as metadata-only or runnable, with
required prerequisites and generated capabilities.

**Benefit**

One deep interface supports `new`, `init`, dry-run, validation, and tests.

**Risk and cost**

Generating complete language ecosystems may overreach. Correcting the promise
and adding capability metadata is the low-risk first step.

### 16. Medium: Installation, Deployment, CLI, and Documentation Contracts Drift

**Evidence**

- The Makefile advertises removed command families:
  `/home/thomas/Overlord/core/tools/overlord/Makefile:42-57,296-378`.
- Local install removes the old binary before copying:
  `/home/thomas/Overlord/core/tools/overlord/Makefile:155-162`.
- Remote deployment writes to a hard-coded root-owned path without checksum or
  rollback:
  `/home/thomas/Overlord/core/tools/overlord/Makefile:164-175`.
- Help promises usage exit code `2`, but all errors become `1`:
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/root.go:71-105`.
- README documents checkpoint index `0`, while the command requires a
  one-based positive index:
  `/home/thomas/Overlord/core/tools/overlord/README.md:99-105` and
  `/home/thomas/Overlord/core/tools/overlord/internal/cmd/start_checkpoint.go:56-64`.

**Problem**

Users and automation cannot rely on the command catalog, exit taxonomy,
installation safety, or documented examples. Remote deployment may be
owner-specific tooling rather than an Overlord responsibility.

**Proposal**

- Remove stale Make wrappers after checking external callers.
- Add a command-manifest consistency check across Cobra, README examples, and
  Make wrappers.
- Define typed error categories and a central exit-code contract.
- Route all output through command writers and define a stable machine mode.
- Install via temporary sibling plus atomic rename.
- Either relocate VPS deployment to owner-specific tooling or add versioned
  upload, checksum verification, atomic promotion, health verification, and
  rollback.

**Benefit**

Truthful operational interfaces and safer release transitions.

**Risk and cost**

Correct exit codes and removed wrappers may break callers that depend on
current accidental behavior.

## Modules and Decisions to Preserve

The following are strong foundations and should be deepened rather than
discarded:

1. The thin process entrypoint at
   `/home/thomas/Overlord/core/tools/overlord/cmd/overlord/main.go`.
2. The registry's role as the sole catalog source of truth.
3. Validation before registry persistence in
   `/home/thomas/Overlord/core/tools/overlord/internal/registry/store.go`.
4. Nested-project specificity in registry path lookup.
5. The `fileutil` seam, after hardening movement and durability semantics.
6. The raw tmux `CommandRunner` adapter and fake runner.
7. Topology-only checkpoints, which avoid pretending to capture process state
   or terminal contents.
8. Bounded checkpoint retention.
9. Embedded template assets and localized rendering.
10. Initial stack-session rollback when later window creation fails.
11. New-project cleanup on registration/save failure.
12. Default non-destructive `rm` behavior and full-word confirmation.
13. Selective tagged backup as a narrow portable export, not a machine restore
   system.
14. The documented separation between Overlord project/workspace ownership and
   Alice/OpenCode coordination state.

## Deletion-Test Candidates

Before removal, search for external callers or deployment wiring:

- the global lifecycle bus and unfired lifecycle events;
- stale Make wrappers for sync, environment, lock, doctor, and worktree flows;
- unused template inspection helpers and unused `TemplateData` fields;
- duplicate path-expansion implementations;
- unreachable startup health result states;
- the root VPS deployment recipe if deployment is not a maintained Overlord
  responsibility;
- archive/unarchive if their documented low use no longer justifies their
  complexity budget.

The visible untracked
`/home/thomas/Overlord/core/tools/overlord/scripts/ta` must not drive the
committed Workspace design until its intended status is clarified.

## Required Tests

### Immediate safety tests

- Destructive targets equal to `/`, home, base, category, archive, and shared
  ancestor directories.
- Overlapping registered paths and symlink aliases.
- External project deletion authorization and confirmation.
- Registry-save failure after deletion, move, or each batch item.
- Recovery after partially applied project transitions.
- Stack-save failure at backup, temporary write, close, sync, and rename.
- Preservation of unrelated Tailscale Serve routes during stop.
- Shell metacharacters and spaces in every configurable stack field.
- Cross-device moves with symlinks, dangling links, existing destinations,
  special files, partial copies, and source-removal failure.

### Registry and persistence tests

- Duplicate aliases, name/alias collisions, malformed map keys, duplicate
  normalized paths, and deterministic resolution.
- Unique temporary files, private modes, backup rotation, and interrupted
  writes.
- Concurrent invocation detection if the single-writer contract is enforced
  through locking or revision checks.

### Stack and workspace tests

- Existing stack session with missing windows, changed enablement, or dead
  processes.
- Restart replacement failure and partial multi-service completion.
- Owned and unmanaged tmux session collisions.
- Exact tmux target behavior and same-name window directory mismatch.
- Checkpoint validation, partial restoration, existing-session drift, and
  structured restore outcomes.
- Attach failure and `LastOpened` persistence only after the selected success
  point.

### Setup, backup, scaffold, and CLI tests

- Existing nonempty setup directory, remote conflict, fetch failure, and
  unexpected default branch.
- Backup output mode, archive collision, write-failure cleanup, manifest,
  Git/fallback parity, and secret-oriented fixtures.
- Deterministic scaffold plans, conflict handling, partial-write failure, and
  generated command capability checks.
- Black-box CLI exit codes, stdout/stderr separation, JSON purity, unhealthy
  status, and partial-success outcomes.
- Command parity between Cobra registrations, README examples, and Make
  wrappers.

## Documentation Required

Document these decisions before implementing the larger refactors:

1. Which path classes may be registered and which may be destructively
   mutated.
2. The symlink policy for registration, lookup, archive, and deletion.
3. Batch command partial-success and recovery semantics.
4. Whether deletion uses quarantine or is immediately irreversible.
5. Whether `stack.yaml` is data, trusted executable configuration, or a mix of
   typed fields and explicit trusted shell programs.
6. The ownership and cleanup model for Tailscale routes and tmux sessions.
7. The meaning of checkpoint restore: missing-only, reconcile, or replace.
8. The CLI exit taxonomy and machine-readable output compatibility promise.
9. Whether generated projects are metadata-only or runnable scaffolds.
10. Backup inclusion, privacy, verification, and restore limitations.
11. Whether VPS deployment and `scripts/ta` are product surfaces.

## Staged Improvement Plan

### Stage 0: Freeze Contracts

1. Clarify the untracked `scripts/ta` status.
2. Search external consumers of lifecycle events, stale Make targets, backup
   filenames, archive/unarchive, and deployment.
3. Decide path classification, symlink, deletion, partial-success, shell-trust,
   tmux ownership, and exit-code contracts.
4. Correct the checkpoint index and other immediately false documentation.

### Stage 1: Close Immediate Safety Gaps

1. Add destructive-path authorization and overlap detection.
2. Fix stack-save ordering and durable write behavior.
3. Replace global Tailscale reset with ownership-scoped cleanup.
4. Harden cross-device move staging and destination collision handling.
5. Make local binary installation atomic.
6. Add the failure-injection tests listed above before restructuring commands.

### Stage 2: Deepen Project State Ownership

1. Introduce the project-transition module for `rm --force` and `clear`.
2. Add explicit transition receipts and repair-needed outcomes.
3. Strengthen registry identity, alias, path, and state invariants.
4. Move archive/unarchive behind the same seam only if those features remain.

### Stage 3: Standardize Effects

1. Introduce the durable-document store for registry, stack, and checkpoints.
2. Introduce structured process specifications and the shared low-level runner.
3. Preserve narrow Git, tmux, service, attach, and trusted-hook adapters.
4. Add private state-file and backup archive permissions.

### Stage 4: Make Workspace and Stack State Truthful

1. Put a Workspace manager above raw tmux.
2. Add ownership metadata and exact target handling.
3. Return structured checkpoint restore outcomes.
4. Model desired versus observed stack state and reconcile existing sessions.
5. Return per-service start/restart results and bounded readiness evidence.

### Stage 5: Move Workflows Below Cobra

1. Add `NewRoot(...)` while retaining a thin compatibility `Execute()`.
2. Move destructive, workspace, stack, and backup policy behind operation
   modules.
3. Replace package-global test hooks with operation-specific dependencies.
4. Route output through command writers and centralize typed errors.

### Stage 6: Consolidate Secondary Subsystems

1. Add the manifest-backed scaffold planner and capability descriptions.
2. Extract backup planning, filtering, and atomic finalization.
3. Add command/documentation parity tooling.
4. Remove or relocate unsafe/stale install and deployment interfaces.

### Stage 7: Delete Unearned Interfaces

After consumer confirmation:

- remove or replace the global lifecycle bus;
- remove stale Make wrappers and dead constants/helpers;
- decide whether archive/unarchive still justify their cost;
- keep owner-specific tmux or VPS utilities outside the core product unless
  they are deliberately adopted.

## Explicit Deferrals

This report does not propose:

- multi-machine registry synchronization;
- a distributed transaction protocol;
- transferring Alice coordination, authorization, efforts, runs, or journal
  ownership into Overlord;
- broad backup encryption, retention, off-host replication, or full machine
  restore;
- replacing all concrete functions with interfaces;
- removing trusted shell customization solely on principle;
- redesigning OpenCode, Phoenix, Tailscale, Ollama, speech-outbox, or tmux
  beyond Overlord's ownership seams;
- making low-use archive/unarchive the organizing architecture;
- treating the untracked `scripts/ta` file as committed product behavior.

## Final Recommendation

Begin with hard destructive-path authorization, stack-save safety, and scoped
Tailscale cleanup. Then characterize transition failures and introduce the
project-transition module. Those changes reduce the largest data-loss and
host-interference risks while establishing the deeper seams needed for later
Cobra, stack, workspace, backup, and scaffolding improvements.

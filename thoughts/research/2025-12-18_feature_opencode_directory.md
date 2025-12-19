---
date: 2025-12-18T12:41:57-08:00
git_commit: 02f1cff96df9524bbb34645bc045dc836fee050d
branch: dev
repository: overlord
topic: "feature_opencode_directory"
tags: [research, codebase, opencode, configuration, directory-structure]
last_updated: 2025-12-18
---

## Ticket Synopsis
Feature request to change opencode.jsonc placement from project root to a dedicated .opencode directory to improve project structure consistency and avoid cluttering the project root with configuration files.

## Summary
The change requires modifications to three Overlord scripts (overlord-init, overlord-new, overlord-sync) to create .opencode/ directory and place opencode.jsonc inside it instead of project root. Current implementation places all configuration files in project root except thoughts/ which creates subdirectories. No migration path needed per ticket requirements - only affects new project creation.

## Detailed Findings

### Configuration File Placement Patterns
All Overlord-managed configuration files are currently placed in project root:
- `.tmux.local` - Tmux workspace configuration (overlord-init:137-138)
- `Makefile` - Build commands (overlord-init:167-171) 
- `opencode.jsonc` - AI assistant instructions (overlord-init:198)
- `thoughts/` - Development artifacts directory with 5 subdirectories (overlord-init:212-213)

The thoughts/ directory is the only configuration that creates subdirectories, following an additive pattern where existing content is never removed.

### Overlord-Init Opencode Creation Logic
The `copy_opencode_template()` function (overlord-init:192-203) copies language-specific templates from `~/bin/overlord/templates/opencode-{lang}.jsonc` to project root. Conditional logic checks for existing file and respects `--force` flag (overlord-init:361-366).

**Required changes:**
- Add `mkdir -p "$dir/.opencode"` before copying
- Change copy destination from `$dir/opencode.jsonc` to `$dir/.opencode/opencode.jsonc`
- Update existence check from `$PROJECT_PATH/opencode.jsonc` to `$PROJECT_PATH/.opencode/opencode.jsonc`

### Overlord-New Opencode Creation Logic
Identical `copy_opencode_template()` function (overlord-new:284-295) used during new project creation. Same changes required as overlord-init.

### Overlord-Sync Opencode Propagation Logic
The `generate_opencode()` function (overlord-sync:123-132) reads templates and redirects output to project root. Conditional logic creates/overwrites based on file existence and `--force` flag (overlord-sync:185-193).

**Required changes:**
- Change `opencode_path` variable from `"$path/opencode.jsonc"` to `"$path/.opencode/opencode.jsonc"`
- Add `mkdir -p "$path/.opencode"` before output redirection
- Dry-run logic automatically updates since it uses the same variable

### Template Structure
Four language-specific templates exist:
- `opencode-base.jsonc` - Empty instructions for non-language projects
- `opencode-python.jsonc` - References CODING.md and PYTHON_STYLEGUIDE.md
- `opencode-typescript.jsonc` - References CODING.md and TYPESCRIPT_STYLEGUIDE.md  
- `opencode-solidity.jsonc` - References CODING.md and SOLIDITY_STYLEGUIDE.md

Templates remain unchanged; only destination directory changes.

## Code References
- `overlord-init:192-203` - copy_opencode_template() function that needs .opencode/ directory creation
- `overlord-init:361-366` - Existence check that needs path update to .opencode/opencode.jsonc
- `overlord-new:284-295` - Identical copy_opencode_template() function requiring same changes
- `overlord-sync:156` - opencode_path variable that needs .opencode/ path update
- `overlord-sync:186-193` - Sync logic that needs mkdir -p before file creation
- `templates/opencode-*.jsonc` - Template files that remain in current location

## Architecture Insights
Configuration file placement follows a consistent root-level pattern except for thoughts/ directory. The change introduces a new pattern of dedicated configuration subdirectories, similar to how .git/, .vscode/, or .github/ organize project metadata. This improves separation of concerns between project content and project management configuration.

## Historical Context (from thoughts/)
Initial opencode.jsonc implementation (2025-12-17) placed files in project root alongside other configuration. The current feature ticket (2025-12-18) recognizes this causes "clutter" and proposes .opencode/ directory to improve organization. Similar to how thoughts/ directory was introduced for development artifacts, .opencode/ will contain AI assistant configuration separately from project files.

## Related Research
None identified in thoughts/research/ directory.

## Open Questions
- Migration strategy for existing projects with opencode.jsonc in root (ticket mentions "backward compatibility or migration path")
- Whether .opencode directory should be added to .gitignore automatically
- Impact on any external tools that might read opencode.jsonc from project root</content>
<parameter name="filePath">thoughts/research/2025-12-18_feature_opencode_directory.md
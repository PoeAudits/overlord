# Add `overlord add` Command - Implementation Plan

## Overview
Create a new `overlord add` command that registers existing project directories in the Overlord registry without creating new projects. This enables managing pre-existing directories (configuration repos, cloned projects, legacy codebases) through Overlord's workspace system.

## Current State
Only projects created with `overlord new` are tracked in the registry. The `overlord-new` script (379 lines) provides all necessary functions for registry manipulation, template copying, and Makefile generation that can be reused.

Key reusable functions from `overlord-new`:
- `register_project()` - lines 283-309
- `copy_tmux_template()` - lines 228-240  
- `generate_makefile()` - lines 252-280
- Logging functions - lines 42-45
- Argument parsing pattern - lines 55-115

## Changes Required

### 1. Create `overlord-add` Script
**File**: `~/bin/overlord/overlord-add` (new file)

**What to change:**
Create new executable script based on `overlord-new` structure with the following modifications:

1. **Header and constants** (adapt from `overlord-new:1-16`):
   - Same shebang, `set -euo pipefail`, config paths, colors

2. **Usage function** (different from `overlord-new:18-40`):
```bash
usage() {
  cat <<'EOF'
Usage:
  overlord add <name> <path> [options]

Arguments:
  name                 Project name for registry
  path                 Directory path (., absolute, or ~/)

Language flags (optional, auto-detected if omitted):
  --py, --python       Force Python configuration
  --ts, --typescript   Force TypeScript configuration
  --sol, --solidity    Force Solidity configuration

Options:
  --lib                Register as library (default: active)
  --alias <name>       Add alias (can be used multiple times)
  --force              Overwrite existing registry entry
  --help               Show this help

Examples:
  overlord add myproject /path/to/existing/project --py
  overlord add overlord ~/bin/overlord
  overlord add mylib . --ts --lib --alias ml
EOF
}
```

3. **Argument parsing** (adapt from `overlord-new:55-115`):
   - Parse `<name>` and `<path>` as required positional args
   - Support `--py|--ts|--sol` as optional (not required like in `overlord-new`)
   - Add `--alias` flag that accumulates into array
   - Path validation: reject relative paths except `.`, expand `~`
   - No args shows help (not error)

4. **Language auto-detection function** (new):
```bash
detect_language() {
  local dir="$1"
  if [[ -f "$dir/pyproject.toml" ]] || [[ -f "$dir/setup.py" ]]; then
    echo "python"
  elif [[ -f "$dir/package.json" ]]; then
    echo "typescript"
  elif [[ -f "$dir/foundry.toml" ]]; then
    echo "solidity"
  else
    echo "base"
  fi
}
```

5. **Registry existence check** (new):
```bash
check_registry_exists() {
  local name="$1"
  jq -e --arg name "$name" '.projects[$name]' "$OVERLORD_REGISTRY" >/dev/null 2>&1
}
```

6. **Modified `copy_tmux_template()`** (adapt from `overlord-new:228-240`):
   - Accept "base" as language parameter
   - Only copy if `.tmux.local` doesn't exist
   - Fall back to `base.tmux` if language template missing

7. **Modified `generate_makefile()`** (adapt from `overlord-new:252-280`):
   - Accept "base" as language parameter  
   - Only generate if `Makefile` doesn't exist
   - For "base" language, use only `base.mk` (no language-specific file)

8. **Modified `register_project()`** (adapt from `overlord-new:283-309`):
   - Accept aliases array parameter
   - Include aliases in jq command

9. **Main function**:
   - Validate path exists and is a directory
   - Expand `~` to `$HOME`
   - Convert relative `.` to absolute path
   - Check registry for existing name (exit if exists and no `--force`)
   - Auto-detect language if not specified
   - Copy `.tmux.local` if missing
   - Generate `Makefile` if missing
   - Register in registry with aliases
   - Success message (do NOT open workspace)

**Why:**
This approach maximizes code reuse from `overlord-new` while adapting the workflow for existing directories instead of project creation.

### 2. Update Dispatcher to Route `add` Command
**File**: `~/bin/overlord/overlord`

**What to change:**
Add `add` to the case statement at line 74:

```bash
new|add|list|mv|open|info|sync|config|edit)
```

**Why:**
Makes `overlord add` route to the new `overlord-add` script, following the existing subcommand pattern.

### 3. Make Script Executable
**Command**: `chmod +x ~/bin/overlord/overlord-add`

**Why:**
Required for the script to be executable by the dispatcher.

---

## Out of Scope
- Creating `base.tmux` template if missing (noted in ticket as potential separate work)
- Modifications to existing commands
- Changes to registry structure
- Validation of project contents

## Success Criteria

### Automated Verification
- [x] Script is executable: `test -x ~/bin/overlord/overlord-add`
- [x] Help works: `overlord add --help` (shows usage)
- [x] No args shows help: `overlord add` (exits 0 with help text)
- [x] Both args required: `overlord add test` fails with error
- [x] Path validation: `overlord add test ../relative` fails with error
- [x] Auto-detection: Create test dir with `pyproject.toml`, run `overlord add testpy /path/to/test`, verify `lang: "python"` in registry
- [x] Explicit language: `overlord add testts /path/to/test --ts`, verify `lang: "typescript"`
- [x] Base fallback: Add dir with no project files, verify `lang: "base"`
- [x] Alias support: `overlord add test /path --alias t1 --alias t2`, verify aliases in registry
- [x] Duplicate check: Add project twice without `--force`, second attempt fails
- [x] Force overwrite: `overlord add existing /path --force` succeeds
- [x] Dispatcher routes: `overlord add --help` works (routes correctly)

### Manual Verification
- [x] Add `~/bin/overlord` itself: `overlord add overlord ~/bin/overlord`
- [x] Verify appears in: `overlord list`
- [x] Verify can open: `overlord open overlord`
- [x] Check `.tmux.local` created if missing
- [x] Check `Makefile` created if missing
- [x] Check existing `.tmux.local` not overwritten
- [x] Check existing `Makefile` not overwritten
- [x] Verify aliases work: `overlord add myproj /path --alias mp`, then `overlord open mp`

## References
- Ticket: `thoughts/tickets/feature_overlord_add.md`
- Template: `overlord-new` (all patterns)
- Registry: `~/.config/overlord/registry.json`
- Templates: `~/.config/overlord/tmux/*.tmux`
- Makefiles: `~/.config/overlord/makefiles/*.mk`

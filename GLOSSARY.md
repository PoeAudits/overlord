# Overlord

Overlord is the single-host CLI that owns the lifecycle of the user's coding projects and keeps every coding agent (Harness) on the machine configured from one source.

## Language

**Harness**:
A coding agent that reads skills, agent definitions and instruction files from the filesystem and that Overlord configures. Currently Claude Code, Codex and OpenCode.
_Avoid_: Vendor, tool, provider, editor

**Agents directory**:
The git checkout, `~/.agents` by default (`agentsDir` in Overlord's config), that holds the skills, Roles and global `AGENTS.md` Sync applies.
_Avoid_: Catalog (in anything the user reads)

**Model**:
A specific model identifier a Harness is told to use for a Role.

**Project**:
A git repository on GitHub that the user works on. A local clone is disposable; the repository is the Project.
_Avoid_: Repo (when meaning the Project as a whole), registry entry

**Role**:
A named job an agent performs (main, explorer, restricted). A Role maps to one Model per Harness and one shared prompt.
_Avoid_: Agent type, subagent name

**Setup**:
Checking one Project for the items Overlord guarantees and creating or refreshing only what is missing or out of date: whole files, Managed Blocks, the GitHub repository and the triage labels. In an existing file it never touches text outside a Managed Block.
_Avoid_: Init, scaffold, bootstrap

**Plan**:
What Setup reports: one row per guaranteed item, with its status.

**Managed Block**:
A header/footer-delimited, versioned region of a committed file that Overlord owns and may replace wholesale. Everything outside it belongs to the Project.
_Avoid_: Template section, generated part

**Template**:
A file or Managed Block body embedded in Overlord that Setup writes into Projects. Templates are the only content Overlord embeds; skills and Roles live in the Agents directory.

**Secret**:
A file that holds a credential, recognised by its path alone (`.env`, `*.pem`, `id_rsa*` and the rest of `templates.Secrets`). The `.gitignore` block ignores Secrets, and Setup never pushes a history that tracks one.
_Avoid_: Sensitive file, credential file

**Sync**:
The idempotent act of applying the canonical global config (skills, instructions, Roles) to every Harness on this machine.

**Start**:
Sync, then bring up the services in Overlord's config and the T3 Code app.

**Doctor**:
A read-only report of drift between the Agents directory, every Harness, T3 Code and the Projects under `projectsRoot`.

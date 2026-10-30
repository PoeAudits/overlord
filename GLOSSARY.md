# Overlord

Overlord is the single-host CLI that owns the lifecycle of the user's coding projects and keeps the AI coding tools on the machine configured from one source.

## Language

**Harness**:
A coding-agent tool that reads skills, agent definitions and instruction files from the filesystem and that Overlord configures. Currently Claude Code, Codex and OpenCode.
_Avoid_: Vendor, tool, provider, editor

**Provider**:
The company whose models a Harness calls (Anthropic, OpenAI). A Harness is not tied to one Provider.

**Model**:
A specific model identifier a Harness is told to use for a Role.

**Project**:
A git repository on GitHub that the user works on. A local clone is disposable; the repository is the Project.
_Avoid_: Repo (when meaning the Project as a whole), registry entry

**Role**:
A named job an agent performs (main, explorer, restricted). A Role maps to one Model per Harness and one shared prompt.
_Avoid_: Agent type, subagent name

**Setup**:
Checking one Project for the files Overlord guarantees and creating or refreshing only what is missing or out of date. Never touches text outside a Managed Block.
_Avoid_: Init, scaffold, bootstrap

**Managed Block**:
A header/footer-delimited, versioned region of a committed file that Overlord owns and may replace wholesale. Everything outside it belongs to the Project.
_Avoid_: Template section, generated part

**Template**:
A file or Managed Block body embedded in Overlord that Setup writes into Projects. Templates are the only content Overlord embeds; skills and Roles live in `~/.agents`.

**Sync**:
The idempotent act of applying the canonical global config (skills, instructions, Roles) to every Harness on this machine.

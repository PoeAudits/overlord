---
status: accepted
---
# `~/.agents` is the single source for skills, Roles and global instructions; Overlord holds only project templates

Skills, Role definitions, prompt bodies and the global AGENTS.md are edited far more often than Overlord's code, so they live in their own git repo cloned to `~/.agents`, the path Codex already reads natively. Overlord embeds only the project templates (AGENTS.md block, coding standards, docs layout, CI workflow) that it applies during Setup. We rejected one combined "workstation" repo because the user's tools already have their own repos, and rejected embedding skills in Overlord because a skill edit must never require a rebuild.

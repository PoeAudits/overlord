package harness

import "path/filepath"

// GlobalAgentDir is where h reads global agent files.
func GlobalAgentDir(h Harness, home string) string {
	return map[Harness]string{
		Claude:   filepath.Join(home, ".claude", "agents"),
		Codex:    filepath.Join(home, ".codex", "agents"),
		OpenCode: filepath.Join(home, ".config", "opencode", "agents"),
	}[h]
}

// ProjectAgentDir is where h reads a Project's agent files, relative to the
// Project root and slash-separated.
func ProjectAgentDir(h Harness) string {
	return map[Harness]string{
		Claude:   ".claude/agents",
		Codex:    ".codex/agents",
		OpenCode: ".opencode/agents",
	}[h]
}

// InstructionPaths are where each Harness reads global instructions.
func InstructionPaths(home string) []string {
	return []string{
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".codex", "AGENTS.md"),
		filepath.Join(home, ".config", "opencode", "AGENTS.md"),
	}
}

// SkillsDir is where Sync links skills; every Harness reads it.
func SkillsDir(home string) string {
	return filepath.Join(home, ".claude", "skills")
}

// OpenCodeSkillsDir is OpenCode's own skills directory. OpenCode reads
// SkillsDir too, so anything here is a stale copy.
func OpenCodeSkillsDir(home string) string {
	return filepath.Join(home, ".config", "opencode", "skills")
}

// ClaudeSettingsPath is Claude Code's settings.json: main Model and hooks.
func ClaudeSettingsPath(home string) string {
	return filepath.Join(home, ".claude", "settings.json")
}

// CodexConfigPath is Codex's config.toml: main Model.
func CodexConfigPath(home string) string {
	return filepath.Join(home, ".codex", "config.toml")
}

// CodexHooksPath is Codex's hooks.json.
func CodexHooksPath(home string) string {
	return filepath.Join(home, ".codex", "hooks.json")
}

// OpenCodePluginPath is where OpenCodePlugin is installed.
func OpenCodePluginPath(home string) string {
	return filepath.Join(home, ".config", "opencode", "plugins", "overlord-sync.js")
}

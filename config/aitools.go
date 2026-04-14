package config

import "path"

// AITool defines a supported AI coding assistant.
// To add a new tool, add an entry to the aiTools slice below.
// NOTE: also update view/help/tiki.md which lists tool names in prose.
type AITool struct {
	Key          string // config identifier: "claude", "gemini", "codex", "opencode"
	DisplayName  string // human-readable label for UI: "Claude Code"
	Command      string // CLI binary name
	PromptFlag   string // flag preceding the prompt arg, or "" for positional
	SkillDir     string // relative base dir for skills: ".claude/skills"
	SettingsFile string // relative path to local settings file, or "" if none
}

// aiTools is the single source of truth for all supported AI tools.
var aiTools = []AITool{
	{
		Key:          "claude",
		DisplayName:  "Claude Code",
		Command:      "claude",
		PromptFlag:   "--append-system-prompt",
		SkillDir:     ".claude/skills",
		SettingsFile: ".claude/settings.local.json",
	},
	{
		Key:         "gemini",
		DisplayName: "Gemini CLI",
		Command:     "gemini",
		PromptFlag:  "-i",
		SkillDir:    ".gemini/skills",
	},
	{
		Key:         "codex",
		DisplayName: "OpenAI Codex",
		Command:     "codex",
		PromptFlag:  "",
		SkillDir:    ".codex/skills",
	},
	{
		Key:         "opencode",
		DisplayName: "OpenCode",
		Command:     "opencode",
		PromptFlag:  "--prompt",
		SkillDir:    ".opencode/skill",
	},
}

// PromptArgs returns CLI arguments to pass a prompt string to this tool.
// If PromptFlag is set, returns [flag, prompt]; otherwise returns [prompt].
func (t AITool) PromptArgs(prompt string) []string {
	if t.PromptFlag != "" {
		return []string{t.PromptFlag, prompt}
	}
	return []string{prompt}
}

// SkillPath returns the relative file path for a skill (e.g. "tiki" → ".claude/skills/tiki/SKILL.md").
func (t AITool) SkillPath(skill string) string {
	return path.Join(t.SkillDir, skill, "SKILL.md")
}

// AITools returns all supported AI tools.
func AITools() []AITool {
	return aiTools
}

// LookupAITool finds a tool by its config key. Returns false if not found.
func LookupAITool(key string) (AITool, bool) {
	for _, t := range aiTools {
		if t.Key == key {
			return t, true
		}
	}
	return AITool{}, false
}

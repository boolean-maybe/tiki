package config

// AITool defines a supported AI coding assistant.
// To add a new tool, add an entry to the aiTools slice below.
// NOTE: the action palette (press Ctrl+A) surfaces available actions; update docs if tool names change.
type AITool struct {
	Key        string // config identifier: "claude", "gemini", "codex", "opencode"
	Command    string // CLI binary name
	PromptFlag string // flag preceding the prompt arg, or "" for positional
}

// aiTools is the single source of truth for all supported AI tools.
var aiTools = []AITool{
	{
		Key:        "claude",
		Command:    "claude",
		PromptFlag: "--append-system-prompt",
	},
	{
		Key:        "gemini",
		Command:    "gemini",
		PromptFlag: "-i",
	},
	{
		Key:        "codex",
		Command:    "codex",
		PromptFlag: "",
	},
	{
		Key:        "opencode",
		Command:    "opencode",
		PromptFlag: "--prompt",
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

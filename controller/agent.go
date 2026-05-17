package controller

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
)

// tikiContextPrompt builds the shared prompt instructing the AI agent about the current tiki task.
func tikiContextPrompt(tikiFilePath string) string {
	return fmt.Sprintf(
		"Read the tiki task at %s first. "+
			"This is the task the user is currently viewing. "+
			"When the user asks to modify something without specifying a file, "+
			"they mean this tiki task. "+
			"After reading, chat with the user about it.",
		tikiFilePath,
	)
}

// resolveAgentCommand maps a logical agent name to the actual command and arguments.
// tikiFilePath is the path to the current tiki task file being viewed.
func resolveAgentCommand(agent string, tikiFilePath string) (name string, args []string) {
	tool, ok := config.LookupAITool(agent)
	if !ok {
		// unknown agent: treat name as the command, no context injection
		return agent, nil
	}
	prompt := tikiContextPrompt(tikiFilePath)
	return tool.Command, tool.PromptArgs(prompt)
}

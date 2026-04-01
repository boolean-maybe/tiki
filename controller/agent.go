package controller

import (
	"fmt"

	"github.com/boolean-maybe/tiki/config"
)

// taskContextPrompt builds the shared prompt instructing the AI agent about the current tiki task.
func taskContextPrompt(taskFilePath string) string {
	return fmt.Sprintf(
		"Read the tiki task at %s first. "+
			"This is the task the user is currently viewing. "+
			"When the user asks to modify something without specifying a file, "+
			"they mean this tiki task. "+
			"After reading, chat with the user about it.",
		taskFilePath,
	)
}

// resolveAgentCommand maps a logical agent name to the actual command and arguments.
// taskFilePath is the path to the current tiki task file being viewed.
func resolveAgentCommand(agent string, taskFilePath string) (name string, args []string) {
	tool, ok := config.LookupAITool(agent)
	if !ok {
		// unknown agent: treat name as the command, no context injection
		return agent, nil
	}
	prompt := taskContextPrompt(taskFilePath)
	return tool.Command, tool.PromptArgs(prompt)
}

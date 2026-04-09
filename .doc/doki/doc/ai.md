# AI collaboration

## AI skills

`tiki` adds optional [agent skills](https://agentskills.io/home) to the repo upon initialization
If installed you can:

- work with [Claude Code](https://code.claude.com), [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Codex](https://openai.com/codex), [Opencode](https://opencode.ai) by simply mentioning `tiki` or `doki` in your prompts
- create, find, modify and delete tikis using AI
- create tikis/dokis directly from Markdown files
- Refer to tikis or dokis when implementing with AI-assisted development - `implement tiki xxxxxxx`
- Keep a history of prompts/plans by saving prompts or plans with your repo

## Chat with AI

If you [configured](config.md) an AI agent these features will be enabled:

- open current tiki in an AI agent such as [Claude Code](https://code.claude.com) and have it read it. You can then chat or edit it without having to find it first


## Configuration

in your `config.yaml`:

```yaml
# AI agent integration
ai:
  agent: claude              # AI tool for chat: "claude", "gemini", "codex", "opencode"
                             # Enables AI collaboration features
                             # Omit or leave empty to disable
```
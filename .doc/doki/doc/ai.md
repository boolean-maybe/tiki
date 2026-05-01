# AI collaboration

## AI skills

`tiki` adds optional [agent skills](https://agentskills.io/home) to the repo upon initialization.
If installed you can:

- work with [Claude Code](https://code.claude.com),
  [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Codex](https://openai.com/codex), and
  [Opencode](https://opencode.ai) by mentioning documents or ids in your prompts
- create, find, modify, and delete documents using AI
- create documents directly from Markdown files
- reference documents by id when implementing with AI-assisted development — `implement ABC123`
- keep a history of prompts/plans by saving them as documents alongside your repo

## Chat with AI

If you [configured](config.md) an AI agent these features are enabled:

- open the current document in an AI agent such as [Claude Code](https://code.claude.com) and have it read the
  file. You can then chat or edit the document without needing to find it first.

## Configuration

In your `config.yaml`:

```yaml
# AI agent integration
ai:
  agent: claude              # AI tool for chat: "claude", "gemini", "codex", "opencode"
                             # Enables AI collaboration features
                             # Omit or leave empty to disable
```

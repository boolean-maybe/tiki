# AI collaboration

## AI skills

`tiki` ships optional [agent skills](https://agentskills.io/home) that you copy into your AI tool
manually — there is no in-app installer. Once installed you can:

- work with [Claude Code](https://code.claude.com),
  [Gemini CLI](https://github.com/google-gemini/gemini-cli), [Codex](https://openai.com/codex), and
  [Opencode](https://opencode.ai) by mentioning documents or ids in your prompts
- create, find, modify, and delete documents using AI
- create documents directly from Markdown files
- reference documents by id when implementing with AI-assisted development — `implement ABC123`
- keep a history of prompts/plans by saving them as documents alongside your repo

### Installing a skill

The skill files ship in the tiki repository under `ai/skills/`:

- `ai/skills/tiki/SKILL.md` — document create/find/update/delete
- `ai/skills/cli/SKILL.md` — driving the `tiki` CLI

Copy the directory containing the relevant `SKILL.md` into your tool's skills location, for example:

| Tool | Target location |
|---|---|
| Claude Code | `.claude/skills/tiki/SKILL.md` (project) or `~/.claude/skills/tiki/SKILL.md` (global) |
| Gemini CLI | the skills directory configured for `gemini` |
| Codex | the skills directory configured for `codex` |
| Opencode | the skills directory configured for `opencode` |

Consult each tool's own documentation for its exact skills path; the `SKILL.md` content itself is
tool-agnostic.

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

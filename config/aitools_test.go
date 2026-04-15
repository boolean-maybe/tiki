package config

import (
	"testing"
)

func TestAITools_ReturnsAllTools(t *testing.T) {
	tools := AITools()
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	keys := make(map[string]bool)
	for _, tool := range tools {
		keys[tool.Key] = true
	}
	for _, expected := range []string{"claude", "gemini", "codex", "opencode"} {
		if !keys[expected] {
			t.Errorf("missing tool key %q", expected)
		}
	}
}

func TestLookupAITool_Found(t *testing.T) {
	tool, ok := LookupAITool("claude")
	if !ok {
		t.Fatal("expected to find claude")
		return
	}
	if tool.Command != "claude" {
		t.Errorf("expected command 'claude', got %q", tool.Command)
	}
	if tool.DisplayName != "Claude Code" {
		t.Errorf("expected display name 'Claude Code', got %q", tool.DisplayName)
	}
}

func TestLookupAITool_NotFound(t *testing.T) {
	_, ok := LookupAITool("unknown")
	if ok {
		t.Error("expected false for unknown tool")
	}
}

func TestAITool_PromptArgs_WithFlag(t *testing.T) {
	tool := AITool{PromptFlag: "--append-system-prompt"}
	args := tool.PromptArgs("hello")
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "--append-system-prompt" {
		t.Errorf("expected flag '--append-system-prompt', got %q", args[0])
	}
	if args[1] != "hello" {
		t.Errorf("expected prompt 'hello', got %q", args[1])
	}
}

func TestAITool_PromptArgs_Positional(t *testing.T) {
	tool := AITool{PromptFlag: ""}
	args := tool.PromptArgs("hello")
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	if args[0] != "hello" {
		t.Errorf("expected prompt 'hello', got %q", args[0])
	}
}

func TestAITool_SkillPath(t *testing.T) {
	tool := AITool{SkillDir: ".claude/skills"}
	path := tool.SkillPath("tiki")
	if path != ".claude/skills/tiki/SKILL.md" {
		t.Errorf("expected '.claude/skills/tiki/SKILL.md', got %q", path)
	}
}

func TestAITool_SkillPath_SingularDir(t *testing.T) {
	tool := AITool{SkillDir: ".opencode/skill"}
	path := tool.SkillPath("doki")
	if path != ".opencode/skill/doki/SKILL.md" {
		t.Errorf("expected '.opencode/skill/doki/SKILL.md', got %q", path)
	}
}

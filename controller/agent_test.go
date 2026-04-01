package controller

import (
	"strings"
	"testing"
)

const testTaskPath = "/tmp/tiki-abc123.md"

func TestResolveAgentCommand_Claude(t *testing.T) {
	name, args := resolveAgentCommand("claude", testTaskPath)
	if name != "claude" {
		t.Errorf("expected name 'claude', got %q", name)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "--append-system-prompt" {
		t.Errorf("expected first arg '--append-system-prompt', got %q", args[0])
	}
	if !strings.Contains(args[1], testTaskPath) {
		t.Errorf("expected prompt to contain task file path, got %q", args[1])
	}
}

func TestResolveAgentCommand_Gemini(t *testing.T) {
	name, args := resolveAgentCommand("gemini", testTaskPath)
	if name != "gemini" {
		t.Errorf("expected name 'gemini', got %q", name)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "-i" {
		t.Errorf("expected first arg '-i', got %q", args[0])
	}
	if !strings.Contains(args[1], testTaskPath) {
		t.Errorf("expected prompt to contain task file path, got %q", args[1])
	}
}

func TestResolveAgentCommand_Codex(t *testing.T) {
	name, args := resolveAgentCommand("codex", testTaskPath)
	if name != "codex" {
		t.Errorf("expected name 'codex', got %q", name)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d: %v", len(args), args)
	}
	if !strings.Contains(args[0], testTaskPath) {
		t.Errorf("expected prompt to contain task file path, got %q", args[0])
	}
}

func TestResolveAgentCommand_OpenCode(t *testing.T) {
	name, args := resolveAgentCommand("opencode", testTaskPath)
	if name != "opencode" {
		t.Errorf("expected name 'opencode', got %q", name)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "--prompt" {
		t.Errorf("expected first arg '--prompt', got %q", args[0])
	}
	if !strings.Contains(args[1], testTaskPath) {
		t.Errorf("expected prompt to contain task file path, got %q", args[1])
	}
}

func TestResolveAgentCommand_Unknown(t *testing.T) {
	name, args := resolveAgentCommand("myagent", testTaskPath)
	if name != "myagent" {
		t.Errorf("expected name 'myagent', got %q", name)
	}
	if args != nil {
		t.Errorf("expected nil args for unknown agent, got %v", args)
	}
}

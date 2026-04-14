package service

import (
	"context"
	"strings"
	"testing"
)

func TestRunShellCommand_NoArgs(t *testing.T) {
	output, err := RunShellCommand(context.Background(), "echo hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "hello" {
		t.Errorf("output = %q, want %q", got, "hello")
	}
}

func TestRunShellCommand_PositionalArgs(t *testing.T) {
	output, err := RunShellCommand(context.Background(), "echo $1 $2", "world", "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "world foo" {
		t.Errorf("output = %q, want %q", got, "world foo")
	}
}

func TestRunShellCommand_PositionalArgsWithSpaces(t *testing.T) {
	output, err := RunShellCommand(context.Background(), "echo \"$1\"", "hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "hello world" {
		t.Errorf("output = %q, want %q", got, "hello world")
	}
}

func TestRunShellCommand_DollarZeroIsPlaceholder(t *testing.T) {
	// $0 should be "_" (the placeholder), not the first real arg
	output, err := RunShellCommand(context.Background(), "echo $0", "real-arg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(output)); got != "_" {
		t.Errorf("$0 = %q, want %q", got, "_")
	}
}

func TestExecutePipeCommand_Success(t *testing.T) {
	err := ExecutePipeCommand(context.Background(), "echo $1", []string{"hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecutePipeCommand_Failure(t *testing.T) {
	err := ExecutePipeCommand(context.Background(), "exit 1", nil)
	if err == nil {
		t.Fatal("expected error for failing command")
	}
}

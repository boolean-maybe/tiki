package task

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

func setupStatusTestRegistry(t *testing.T) {
	t.Helper()
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})
	t.Cleanup(func() { config.ClearStatusRegistry() })
}

func TestParseStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	tests := []struct {
		name       string
		input      string
		wantStatus Status
		wantOK     bool
	}{
		{"valid status", "done", "done", true},
		{"empty input returns default", "", "backlog", true},
		{"normalized input", "In-Progress", "inProgress", true},
		{"unknown status", "nonexistent", "backlog", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseStatus(tt.input)
			if ok != tt.wantOK {
				t.Errorf("ParseStatus(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.wantStatus {
				t.Errorf("ParseStatus(%q) = %q, want %q", tt.input, got, tt.wantStatus)
			}
		})
	}
}

func TestNormalizeStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := NormalizeStatus("DONE"); got != "done" {
		t.Errorf("NormalizeStatus(%q) = %q, want %q", "DONE", got, "done")
	}
	if got := NormalizeStatus("unknown"); got != "backlog" {
		t.Errorf("NormalizeStatus(%q) = %q, want %q (default)", "unknown", got, "backlog")
	}
}

func TestMapStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := MapStatus("ready"); got != "ready" {
		t.Errorf("MapStatus(%q) = %q, want %q", "ready", got, "ready")
	}
}

func TestStatusToString(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := StatusToString("done"); got != "done" {
		t.Errorf("StatusToString(%q) = %q, want %q", "done", got, "done")
	}
	if got := StatusToString("nonexistent"); got != "backlog" {
		t.Errorf("StatusToString(%q) = %q, want default", "nonexistent", got)
	}
}

func TestStatusEmoji(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := StatusEmoji("done"); got != "✅" {
		t.Errorf("StatusEmoji(%q) = %q, want %q", "done", got, "✅")
	}
	if got := StatusEmoji("nonexistent"); got != "" {
		t.Errorf("StatusEmoji(%q) = %q, want empty", "nonexistent", got)
	}
}

func TestStatusLabel(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := StatusLabel("inProgress"); got != "In Progress" {
		t.Errorf("StatusLabel(%q) = %q, want %q", "inProgress", got, "In Progress")
	}
	if got := StatusLabel("nonexistent"); got != "nonexistent" {
		t.Errorf("StatusLabel(%q) = %q, want raw key", "nonexistent", got)
	}
}

func TestStatusDisplay(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := StatusDisplay("done"); got != "Done ✅" {
		t.Errorf("StatusDisplay(%q) = %q, want %q", "done", got, "Done ✅")
	}
}

func TestStatusDisplay_NoEmoji(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "plain", Label: "Plain", Default: true},
	})
	t.Cleanup(func() { config.ClearStatusRegistry() })

	if got := StatusDisplay("plain"); got != "Plain" {
		t.Errorf("StatusDisplay(%q) = %q, want %q (no emoji)", "plain", got, "Plain")
	}
}

func TestDefaultStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := DefaultStatus(); got != "backlog" {
		t.Errorf("DefaultStatus() = %q, want %q", got, "backlog")
	}
}

func TestDoneStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	if got := DoneStatus(); got != "done" {
		t.Errorf("DoneStatus() = %q, want %q", got, "done")
	}
}

func TestAllStatuses(t *testing.T) {
	setupStatusTestRegistry(t)

	all := AllStatuses()
	expected := []Status{"backlog", "ready", "inProgress", "review", "done"}
	if len(all) != len(expected) {
		t.Fatalf("AllStatuses() returned %d, want %d", len(all), len(expected))
	}
	for i, s := range all {
		if s != expected[i] {
			t.Errorf("AllStatuses()[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestIsActiveStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	if IsActiveStatus("backlog") {
		t.Error("expected backlog to not be active")
	}
	if !IsActiveStatus("ready") {
		t.Error("expected ready to be active")
	}
	if !IsActiveStatus("inProgress") {
		t.Error("expected in_progress to be active")
	}
	if IsActiveStatus("done") {
		t.Error("expected done to not be active")
	}
}

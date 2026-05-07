package task

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/workflow"
)

func setupStatusTestRegistry(t *testing.T) {
	t.Helper()
	config.ResetWorkflowFieldsForTest(defaultTestWorkflowFields())
	t.Cleanup(func() { config.ResetWorkflowFieldsForTest(defaultTestWorkflowFields()) })
}

func defaultTestWorkflowFields() []workflow.FieldDef {
	return []workflow.FieldDef{
		{
			Name: "status",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
				{Value: "ready", Label: "Ready", Emoji: "📋"},
				{Value: "inProgress", Label: "In Progress", Emoji: "⚙️"},
				{Value: "review", Label: "Review", Emoji: "👀"},
				{Value: "done", Label: "Done", Emoji: "✅"},
			},
		},
		{
			Name: "type",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "story", Label: "Story", Emoji: "🌀", Default: true},
				{Value: "bug", Label: "Bug", Emoji: "💥"},
				{Value: "spike", Label: "Spike", Emoji: "🔍"},
				{Value: "epic", Label: "Epic", Emoji: "🗂️"},
			},
		},
		{Name: "priority", Type: workflow.TypeInt, DefaultValue: 3},
		{Name: "points", Type: workflow.TypeInt, DefaultValue: 1},
		{Name: "tags", Type: workflow.TypeListString, DefaultValue: []string{"idea"}},
		{Name: "dependsOn", Type: workflow.TypeListRef},
		{Name: "due", Type: workflow.TypeDate},
		{Name: "recurrence", Type: workflow.TypeRecurrence},
		{Name: "assignee", Type: workflow.TypeString},
	}
}

func TestParseStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	tests := []struct {
		name       string
		input      string
		wantStatus string
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
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{{
		Name: "status",
		Type: workflow.TypeEnum,
		EnumValues: []workflow.EnumValue{
			{Value: "plain", Label: "Plain", Default: true},
			{Value: "finished", Label: "Finished"},
		},
	}})
	t.Cleanup(func() { config.ResetWorkflowFieldsForTest(defaultTestWorkflowFields()) })

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

func TestAllStatuses(t *testing.T) {
	setupStatusTestRegistry(t)

	all := AllStatuses()
	expected := []string{"backlog", "ready", "inProgress", "review", "done"}
	if len(all) != len(expected) {
		t.Fatalf("AllStatuses() returned %d, want %d", len(all), len(expected))
	}
	for i, s := range all {
		if s != expected[i] {
			t.Errorf("AllStatuses()[%d] = %q, want %q", i, s, expected[i])
		}
	}
}

func TestIsValidStatus(t *testing.T) {
	setupStatusTestRegistry(t)

	for _, valid := range []string{"backlog", "ready", "inProgress", "review", "done"} {
		if !IsValidStatus(valid) {
			t.Errorf("IsValidStatus(%q) = false, want true", valid)
		}
	}
	if IsValidStatus("nonexistent") {
		t.Error("IsValidStatus(nonexistent) should be false")
	}
}

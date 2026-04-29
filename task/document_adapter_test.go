package task

import (
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/document"
)

func TestFromDocument_basicFields(t *testing.T) {
	mtime := time.Now()
	doc := &document.Document{
		ID:          "ABC123",
		Title:       "hello",
		Body:        "  body text\n",
		Path:        "/tmp/x.md",
		LoadedMtime: mtime,
		CreatedAt:   mtime,
		UpdatedAt:   mtime,
	}

	tk := FromDocument(doc)
	if tk.ID != "ABC123" {
		t.Errorf("ID = %q, want ABC123", tk.ID)
	}
	if tk.Title != "hello" {
		t.Errorf("Title = %q, want hello", tk.Title)
	}
	if tk.Description != "body text" {
		t.Errorf("Description = %q, want trimmed body text", tk.Description)
	}
	if tk.FilePath != "/tmp/x.md" {
		t.Errorf("FilePath = %q, want /tmp/x.md", tk.FilePath)
	}
	if !tk.LoadedMtime.Equal(mtime) {
		t.Errorf("LoadedMtime not carried through")
	}
}

func TestFromDocument_absentWorkflowFieldsStayAbsent(t *testing.T) {
	// Presence-aware contract: a plain doc with no workflow fields must NOT
	// come back as a workflow-ish Task with defaults filled in.
	doc := &document.Document{ID: "ABC123", Title: "plain"}
	tk := FromDocument(doc)

	if tk.Priority != 0 {
		t.Errorf("Priority = %d, want 0 (absent)", tk.Priority)
	}
	if tk.Points != 0 {
		t.Errorf("Points = %d, want 0 (absent)", tk.Points)
	}
	if tk.Status != "" {
		t.Errorf("Status = %q, want empty (absent)", tk.Status)
	}
	if tk.Type != "" {
		t.Errorf("Type = %q, want empty (absent)", tk.Type)
	}
	if len(tk.Tags) != 0 {
		t.Errorf("Tags = %v, want empty (absent)", tk.Tags)
	}
}

func TestFromDocument_nilIsNil(t *testing.T) {
	if FromDocument(nil) != nil {
		t.Error("FromDocument(nil) should be nil")
	}
	if ToDocument(nil) != nil {
		t.Error("ToDocument(nil) should be nil")
	}
}

func TestRoundTripIDAndTitle(t *testing.T) {
	tk := &Task{ID: "ABC123", Title: "hi", Description: "body", FilePath: "/p.md"}
	doc := ToDocument(tk)
	back := FromDocument(doc)

	if back.ID != tk.ID || back.Title != tk.Title || back.Description != tk.Description || back.FilePath != tk.FilePath {
		t.Errorf("round-trip lost data: %+v -> %+v", tk, back)
	}
}

func TestCustomFieldsCloned(t *testing.T) {
	doc := &document.Document{
		ID:            "ABC123",
		CustomFields:  map[string]interface{}{"k": "v"},
		UnknownFields: map[string]interface{}{"u": 1},
	}
	tk := FromDocument(doc)

	// mutate doc maps; tk should not see the change
	doc.CustomFields["k"] = "mutated"
	doc.UnknownFields["u"] = 999

	if tk.CustomFields["k"] != "v" {
		t.Errorf("CustomFields not isolated: %v", tk.CustomFields)
	}
	if tk.UnknownFields["u"] != 1 {
		t.Errorf("UnknownFields not isolated: %v", tk.UnknownFields)
	}
}

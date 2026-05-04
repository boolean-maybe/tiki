package tikistore

import (
	"os"
	"strings"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestCreateDocument_PlainDocFileHasNoWorkflowKeys proves the review's
// High finding #1 is closed: a plain tiki created via CreateTiki must be
// serialized WITHOUT workflow keys (status/type/priority/points) in the
// YAML frontmatter. If workflow keys leak into the file, a reload would
// re-promote the doc through hasAnyWorkflowField.
func TestCreateDocument_PlainDocFileHasNoWorkflowKeys(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := tikipkg.New()
	tk.ID = "PLAIN1"
	tk.Title = "plain doc"
	tk.Body = "hello"
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	data, err := os.ReadFile(tmp + "/PLAIN1.md")
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	for _, forbidden := range []string{"status:", "type:", "priority:", "points:"} {
		if strings.Contains(string(data), forbidden) {
			t.Errorf("plain doc file contains %q workflow key; full contents:\n%s",
				forbidden, data)
		}
	}

	// Reloading must keep the doc plain — the classification survives disk.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	loaded := s.GetTiki("PLAIN1")
	if loaded == nil {
		t.Fatal("GetTiki after Reload = nil")
	}
	if hasAnyWorkflowField(loaded) {
		t.Error("plain doc was promoted to workflow on reload — workflow keys persisted to disk")
	}
}

// TestUpdateDocument_DemotesWorkflowDocWhenFrontmatterStripped proves the
// review's High finding #2 is closed: a caller can demote a workflow doc
// back to plain by stripping every workflow field from the tiki and calling
// UpdateTiki.
func TestUpdateDocument_DemotesWorkflowDocWhenFrontmatterStripped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Create as workflow.
	tk := tikipkg.New()
	tk.ID = "DEMO01"
	tk.Title = "will be demoted"
	tk.Set(tikipkg.FieldStatus, "ready")
	tk.Set(tikipkg.FieldPriority, 2)
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki workflow: %v", err)
	}

	// Confirm starting state.
	before := s.GetTiki("DEMO01")
	if before == nil || !hasAnyWorkflowField(before) {
		t.Fatalf("precondition: expected workflow tiki, got %+v", before)
	}

	// Now update with no workflow fields — explicit demotion via a plain tiki.
	plain := tikipkg.New()
	plain.ID = "DEMO01"
	plain.Title = "will be demoted"
	plain.Path = before.Path
	plain.LoadedMtime = before.LoadedMtime
	if err := s.UpdateTiki(plain); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	after := s.GetTiki("DEMO01")
	if after == nil {
		t.Fatal("GetTiki after UpdateTiki = nil")
	}
	if hasAnyWorkflowField(after) {
		t.Error("UpdateTiki failed to demote: tiki still has workflow fields")
	}

	// And it must persist across a reload.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	reloaded := s.GetTiki("DEMO01")
	if reloaded == nil {
		t.Fatal("reloaded doc missing")
	}
	if hasAnyWorkflowField(reloaded) {
		t.Error("demotion did not survive reload — workflow keys still on disk")
	}
}

// TestUpdateTiki_StillPreservesWorkflowFieldsWhenCallerLoadsFirst proves that
// a caller who loads the stored tiki first and modifies it does not lose
// existing workflow fields — UpdateTiki is exact-presence, so the caller
// must carry the fields they want to keep.
func TestUpdateTiki_StillPreservesWorkflowFieldsWhenCallerLoadsFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tmp := t.TempDir()
	s, err := NewTikiStore(tmp)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tk := tikipkg.New()
	tk.ID = "CARRY1"
	tk.Title = "workflow"
	tk.Set(tikipkg.FieldStatus, "ready")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	// Load, clone, and update — workflow fields are carried because the
	// caller took them from the stored tiki.
	stored := s.GetTiki("CARRY1")
	if stored == nil {
		t.Fatal("GetTiki = nil")
	}
	updated := stored.Clone()
	updated.Title = "updated"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}

	if got := s.GetTiki("CARRY1"); got == nil || !hasAnyWorkflowField(got) {
		t.Errorf("UpdateTiki should preserve workflow fields when caller cloned stored tiki; got %+v", got)
	}
}

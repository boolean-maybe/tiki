package taskdetail

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestConfigurableDetailView_RendersConfiguredMetadata verifies that the
// view materialises a tiki for the configured metadata list and returns
// non-nil primitives for title, the configured rows, and description.
func TestConfigurableDetailView_RendersConfiguredMetadata(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI001")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	registry := controller.DetailViewActions()
	cv := NewConfigurableDetailView(
		s,
		tk.ID,
		"Detail",
		[]string{"status", "type", "priority"},
		registry,
		nil, nil,
	)

	if cv.GetViewName() != "Detail" {
		t.Errorf("GetViewName() = %q, want %q", cv.GetViewName(), "Detail")
	}
	if cv.GetSelectedID() != tk.ID {
		t.Errorf("GetSelectedID() = %q, want %q", cv.GetSelectedID(), tk.ID)
	}
	if cv.GetActionRegistry() == nil {
		t.Error("GetActionRegistry returned nil")
	}
	if cv.GetPrimitive() == nil {
		t.Error("GetPrimitive returned nil")
	}
}

// TestConfigurableDetailView_HandlesMissingTiki verifies the placeholder path
// when the tiki id can't be resolved (e.g. stale selection after delete).
func TestConfigurableDetailView_HandlesMissingTiki(t *testing.T) {
	s := store.NewInMemoryStore()
	cv := NewConfigurableDetailView(
		s,
		"TIKI_GONE",
		"Detail",
		[]string{"status"},
		controller.DetailViewActions(),
		nil, nil,
	)
	if cv.GetSelectedID() != "TIKI_GONE" {
		t.Errorf("GetSelectedID() = %q, want %q", cv.GetSelectedID(), "TIKI_GONE")
	}
	// The view should not panic when refresh runs on a missing tiki.
	cv.refresh()
}

// TestConfigurableDetailView_UnknownFieldRendersPlaceholder verifies that
// an unknown semantic-field name renders a placeholder row instead of
// crashing — the parser already rejects this at load, but the view
// shouldn't depend on that.
func TestConfigurableDetailView_UnknownFieldRendersPlaceholder(t *testing.T) {
	tk := newTestViewTiki("TIKI002")
	ctx := FieldRenderContext{Mode: RenderModeView, Colors: config.GetColors()}
	row := renderConfiguredField("not_a_field", tk, ctx)
	if row == nil {
		t.Fatal("expected a placeholder primitive, got nil")
	}
}

// TestFieldRegistry_LookupKnownFields verifies the schema-known fields
// installed by registerBuiltinFields are visible.
func TestFieldRegistry_LookupKnownFields(t *testing.T) {
	for _, name := range []string{
		tikipkg.FieldStatus,
		tikipkg.FieldType,
		tikipkg.FieldPriority,
	} {
		t.Run(name, func(t *testing.T) {
			fd, ok := LookupField(name)
			if !ok {
				t.Fatalf("expected field %q to be registered", name)
			}
			if fd.Name != name {
				t.Errorf("FieldDescriptor.Name = %q, want %q", fd.Name, name)
			}
			if fd.Get == nil {
				t.Errorf("FieldDescriptor.Get for %q is nil", name)
			}
		})
	}
}

// TestTypeRegistry_AllStubsHaveRenderer asserts that every semantic type
// recorded in the registry has a non-nil renderer, so an unsupported
// editor type still produces predictable visual output.
func TestTypeRegistry_AllStubsHaveRenderer(t *testing.T) {
	for _, sem := range []SemanticType{
		SemanticStatus,
		SemanticType_,
		SemanticPriority,
		SemanticText,
		SemanticInteger,
		SemanticBoolean,
		SemanticDate,
		SemanticDateTime,
		SemanticRecurrence,
		SemanticEnum,
		SemanticStringList,
		SemanticTaskIDList,
	} {
		t.Run(string(sem), func(t *testing.T) {
			ui, ok := LookupType(sem)
			if !ok {
				t.Fatalf("type %q not registered", sem)
			}
			if ui.Render == nil {
				t.Errorf("type %q has nil Render", sem)
			}
		})
	}
}

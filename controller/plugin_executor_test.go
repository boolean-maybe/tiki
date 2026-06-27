package controller

import (
	"testing"

	"github.com/boolean-maybe/ruki"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
)

// newTestPluginExecutor builds a PluginExecutor backed by an empty in-memory
// store, mirroring the create-action setup used elsewhere in the controller
// tests. It returns the executor and its store so a test can assert that the
// store size is unchanged after a no-persist operation.
func newTestPluginExecutor(t *testing.T) (*PluginExecutor, store.Store) {
	t.Helper()
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	schema := rukiRuntime.NewSchema()
	pe := NewPluginExecutor(tikiStore, gate, nil, schema, "TestPlugin", nil)
	return pe, tikiStore
}

func TestPluginExecutor_BuildCreateDraft_SeedsTypeWithoutPersisting(t *testing.T) {
	pe, tikiStore := newTestPluginExecutor(t)
	parser := ruki.NewParser(pe.schema)
	stmt, err := parser.ParseAndValidateStatement(`create type="project"`, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse seed: %v", err)
	}

	before := len(tikiStore.GetAllTikis())
	draft, err := pe.BuildCreateDraft(stmt)
	if err != nil {
		t.Fatalf("BuildCreateDraft: %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft tiki")
	}
	got, present, _ := draft.StringField("type")
	if !present || got != "project" {
		t.Fatalf("expected type=project, got %q (present=%v)", got, present)
	}
	if after := len(tikiStore.GetAllTikis()); after != before {
		t.Fatalf("BuildCreateDraft must not persist: store went %d -> %d", before, after)
	}
}

func TestPluginExecutor_BuildCreateDraft_RejectsNonCreate(t *testing.T) {
	pe, _ := newTestPluginExecutor(t)
	parser := ruki.NewParser(pe.schema)
	stmt, err := parser.ParseAndValidateStatement(`update where id = id() set type="project"`, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse update: %v", err)
	}
	if _, err := pe.BuildCreateDraft(stmt); err == nil {
		t.Fatal("expected error for non-create statement")
	}
}

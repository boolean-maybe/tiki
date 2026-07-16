package controller

import (
	"sync"
	"testing"

	"github.com/boolean-maybe/ruki"
	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/service"
	"github.com/boolean-maybe/tiki/store"
)

// progressSpy is a local model.ProgressSink recording the latest state so the
// executor test can assert the indeterminate bar is cleared after a run. Kept
// in the controller package (rather than exported from model) so the prod API
// stays clean; the ProgressSink interface is exported, so any package can
// satisfy it.
type progressSpy struct {
	mu     sync.Mutex
	active bool
}

func (s *progressSpy) SetProgress(int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = true
}

func (s *progressSpy) ClearProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = false
}

func (s *progressSpy) isActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

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
	pe := NewPluginExecutor(tikiStore, gate, nil, nil, schema, "TestPlugin", nil)
	return pe, tikiStore
}

func TestPluginExecutor_Execute_ReportsIndeterminate(t *testing.T) {
	tikiStore := store.NewInMemoryStore()
	gate := service.NewTikiMutationGate()
	gate.SetStore(tikiStore)
	schema := rukiRuntime.NewSchema()

	spy := &progressSpy{}
	hub := model.NewProgressHub(spy, func(fn func()) { fn() })
	defer hub.Stop()

	pe := NewPluginExecutor(tikiStore, gate, nil, hub, schema, "TestPlugin", nil)

	stmt, err := ruki.NewParser(schema).ParseAndValidateStatement(`select`, ruki.ExecutorRuntimePlugin)
	if err != nil {
		t.Fatalf("parse select: %v", err)
	}
	pa := &plugin.PluginAction{Action: stmt, KeyStr: "s", Label: "list"}

	if ok := pe.Execute(pa, ruki.ExecutionInput{}); !ok {
		t.Fatal("expected benign select to succeed")
	}
	hub.DrainForTest()

	// after completion the bar must be cleared (Done deferred inside Execute)
	if spy.isActive() {
		t.Fatal("progress not cleared after Execute completed")
	}
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

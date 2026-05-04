package ruki

import (
	"testing"

	"github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/tiki"
)

// Phase 4: ruki `create` preserves template defaults alongside caller
// overrides. The Tiki that ruki produces carries every schema-known field
// the template had plus whatever the CREATE assignments wrote.
//
// Phase 5's "presence-map seeding" semantics moved to the persistence
// layer (store/tikistore). What the ruki executor guarantees is that
// every Fields entry present on the template survives to the created
// tiki unless overridden; the save path decides sparse vs full.
func TestPhase5_Create_TemplateDefaultsSurvive(t *testing.T) {
	e := newTestExecutor()
	p := newTestParser()

	// Mimic what NewTaskTemplate returns: workflow-capable with typed defaults.
	template := tikiFromTask(&task.Task{
		ID:         "NEWDOC",
		Status:     "backlog",
		Type:       "story",
		Priority:   3,
		Points:     1,
		Tags:       []string{"idea"},
		IsWorkflow: true,
	})

	stmt, err := p.ParseStatement(`create title="x" status="ready"`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	result, err := e.Execute(stmt, nil, ExecutionInput{CreateTemplate: template})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Create == nil || result.Create.Tiki == nil {
		t.Fatal("expected Create result with tiki")
	}
	created := result.Create.Tiki

	if got, _ := created.Get(tiki.FieldStatus); got != "ready" {
		t.Errorf("status = %v, want ready (caller override)", got)
	}
	if got, _ := created.Get(tiki.FieldType); got != "story" {
		t.Errorf("type = %v, want story (template default)", got)
	}
	if got, _ := created.Get(tiki.FieldPriority); got != 3 {
		t.Errorf("priority = %v, want 3 (template default)", got)
	}
	if got, _ := created.Get(tiki.FieldPoints); got != 1 {
		t.Errorf("points = %v, want 1 (template default)", got)
	}
	tags, _ := created.Get(tiki.FieldTags)
	ss, _ := tags.([]string)
	if len(ss) != 1 || ss[0] != "idea" {
		t.Errorf("tags = %v, want [idea] (template default)", tags)
	}
}

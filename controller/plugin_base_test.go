package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestCreateDraftTiki_ReturnsInMemoryDraft pins the helper's contract:
// it produces a draft from the store template without persisting it.
// The dispatcher and the deps editor both depend on the "draft is not
// in the store yet" property.
func TestCreateDraftTiki_ReturnsInMemoryDraft(t *testing.T) {
	s := store.NewInMemoryStore()

	got, err := createDraftTiki(s)
	if err != nil {
		t.Fatalf("createDraftTiki returned error: %v", err)
	}
	if got == nil {
		t.Fatal("draft is nil")
	}
	if got.ID() == "" {
		t.Error("draft has empty ID")
	}
	if persisted := s.GetTiki(got.ID()); persisted != nil {
		t.Error("draft was unexpectedly persisted to store")
	}
}

func TestDefaultTikiSortDoesNotRecognizePriorityFieldName(t *testing.T) {
	config.ResetWorkflowFieldsForTest([]workflow.FieldDef{
		{
			Name: "priority",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "high"},
				{Value: "low"},
			},
		},
	})
	t.Cleanup(teststatuses.Init)

	alpha := tikipkg.New()
	alpha.SetID("AAAAAA")
	alpha.SetTitle("Alpha")
	alpha.Set("priority", "low")
	beta := tikipkg.New()
	beta.SetID("BBBBBB")
	beta.SetTitle("Beta")
	beta.Set("priority", "high")
	tikis := []*tikipkg.Tiki{alpha, beta}

	sortTikisByTitle(tikis)

	if tikis[0] != alpha {
		t.Fatalf("default sort used priority field value; first title = %q, want Alpha", tikis[0].Title())
	}
}

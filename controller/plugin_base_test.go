package controller

import (
	"testing"

	"github.com/boolean-maybe/tiki/store"
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

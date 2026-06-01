package tiki

import (
	"testing"

	"github.com/boolean-maybe/tiki/ruki"
)

func TestDoc_SatisfiesRukiDocument(t *testing.T) {
	var _ ruki.Document = Doc{}
}

func TestWrapUnwrapDoc_RoundTrip(t *testing.T) {
	tk := New()
	tk.SetID("ABC123")

	if got := UnwrapDoc(WrapDoc(tk)); got != tk {
		t.Fatalf("UnwrapDoc(WrapDoc(t)) = %p, want %p", got, tk)
	}
}

func TestUnwrapDoc_Nil(t *testing.T) {
	if got := UnwrapDoc(nil); got != nil {
		t.Fatalf("UnwrapDoc(nil) = %v, want nil", got)
	}
}

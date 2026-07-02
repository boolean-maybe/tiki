package tiki

import (
	"testing"
	"time"
)

func TestTiki_Accessors(t *testing.T) {
	now := time.Now()
	doc := New()
	doc.SetID("K3X9M2")
	doc.SetTitle("hello")
	doc.SetBody("body text")
	doc.SetPath("/repo/.doc/K3X9M2.md")
	doc.SetCreatedAt(now)
	doc.SetUpdatedAt(now)

	if doc.ID() != "K3X9M2" {
		t.Errorf("ID() = %q, want K3X9M2", doc.ID())
	}
	if doc.Title() != "hello" {
		t.Errorf("Title() = %q, want hello", doc.Title())
	}
	if doc.Body() != "body text" {
		t.Errorf("Body() = %q, want body text", doc.Body())
	}
	if doc.Path() != "/repo/.doc/K3X9M2.md" {
		t.Errorf("Path() = %q, want the path", doc.Path())
	}
	if !doc.CreatedAt().Equal(now) {
		t.Errorf("CreatedAt() = %v, want %v", doc.CreatedAt(), now)
	}
	if !doc.UpdatedAt().Equal(now) {
		t.Errorf("UpdatedAt() = %v, want %v", doc.UpdatedAt(), now)
	}
}

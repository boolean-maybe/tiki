package gridlayout

import (
	"reflect"
	"testing"
)

// hiding a field replaces its value cell, its .caption cell, and their
// row-span continuations with EmptyCell — yielding a spec identical to one
// authored with "_" in those positions.
func TestHideFields_ReplacesFieldAndCaptionWithEmptyCells(t *testing.T) {
	// caption + value on row 0; both row-span to row 1 via ^.
	withField, err := ParseGrid([][]string{
		{"status", `<text.label>dependsOn.caption`, "dependsOn"},
		{"status", "^", "^"},
	})
	if err != nil {
		t.Fatalf("parse withField: %v", err)
	}
	// the hand-authored "empty" equivalent: dependsOn caption+value are "_".
	wantEmpty, err := ParseGrid([][]string{
		{"status", "_", "_"},
		{"status", "_", "_"},
	})
	if err != nil {
		t.Fatalf("parse wantEmpty: %v", err)
	}

	got := HideFields(withField, []string{"dependsOn"})

	if !reflect.DeepEqual(got.Cells, wantEmpty.Cells) {
		t.Errorf("Cells mismatch:\n got=%v\nwant=%v", got.Cells, wantEmpty.Cells)
	}
	// status anchor must survive; dependsOn value + caption anchors gone.
	names := got.AnchorNames()
	if len(names) == 0 {
		t.Fatal("expected status anchor to survive")
	}
	for _, n := range names {
		if n == "dependsOn" {
			t.Errorf("dependsOn anchor should have been removed, got anchors: %v", names)
		}
	}
}

// hiding a name that no field anchor uses is a no-op: cells and anchor count
// are unchanged.
func TestHideFields_MissingFieldIsNoOp(t *testing.T) {
	spec, err := ParseGrid([][]string{{"status"}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := HideFields(spec, []string{"nonexistent"})
	if !reflect.DeepEqual(got.Cells, spec.Cells) {
		t.Errorf("Cells mismatch:\n got=%v\nwant=%v", got.Cells, spec.Cells)
	}
	if len(got.Anchors) != len(spec.Anchors) {
		t.Errorf("anchor count changed: %d -> %d", len(spec.Anchors), len(got.Anchors))
	}
}

// hiding with nil names returns a spec whose Cells deep-equal the input's.
func TestHideFields_EmptyNamesReturnsEqual(t *testing.T) {
	spec, err := ParseGrid([][]string{{"status"}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := HideFields(spec, nil)
	if !reflect.DeepEqual(got.Cells, spec.Cells) {
		t.Errorf("Cells mismatch:\n got=%v\nwant=%v", got.Cells, spec.Cells)
	}
}

// a field that appears in two distinct columns (here a caption column and a
// value column sharing the same name) is hidden in both columns.
func TestHideFields_HidesFieldAppearingInTwoColumns(t *testing.T) {
	spec, err := ParseGrid([][]string{{"status.caption", "status"}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := HideFields(spec, []string{"status"})

	for c, cell := range got.Cells[0] {
		if _, ok := cell.(EmptyCell); !ok {
			t.Errorf("col %d not blanked: %#v", c, cell)
		}
	}
	for _, n := range got.AnchorNames() {
		if n == "status" {
			t.Errorf("status anchor should have been removed, got: %v", got.AnchorNames())
		}
	}
}

// hiding does not mutate the input spec.
func TestHideFields_DoesNotMutateInput(t *testing.T) {
	spec, err := ParseGrid([][]string{{"dependsOn"}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	before := len(spec.Anchors)
	_ = HideFields(spec, []string{"dependsOn"})
	if len(spec.Anchors) != before {
		t.Errorf("input spec mutated: anchors %d -> %d", before, len(spec.Anchors))
	}
}

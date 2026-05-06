package taskdetail

import (
	"testing"
)

// fixedHeight returns a GridField whose HeightAt always returns h.
func fixedHeight(name string, h int) GridField {
	return GridField{Name: name, HeightAt: func(int) int { return h }}
}

// widthDependent returns a GridField whose HeightAt branches on width threshold.
func widthDependent(name string, threshold, hSmall, hLarge int) GridField {
	return GridField{Name: name, HeightAt: func(w int) int {
		if w >= threshold {
			return hLarge
		}
		return hSmall
	}}
}

func assertInvariants(t *testing.T, plan GridPlan, expectedNames []string) {
	t.Helper()
	if plan.FixedHeight != rowsPerColumn {
		t.Fatalf("FixedHeight = %d, want %d", plan.FixedHeight, rowsPerColumn)
	}
	var flat []string
	for ci, col := range plan.Columns {
		if col.Width < 1 {
			t.Errorf("column %d: Width = %d, want >= 1", ci, col.Width)
		}
		used := 0
		for fi, f := range col.Fields {
			if f.Row < 0 || f.Row >= rowsPerColumn {
				t.Errorf("column %d field %d (%s): Row = %d, want [0, %d)", ci, fi, f.Name, f.Row, rowsPerColumn)
			}
			if f.H < 1 || f.H > rowsPerColumn {
				t.Errorf("column %d field %d (%s): H = %d, want [1, %d]", ci, fi, f.Name, f.H, rowsPerColumn)
			}
			if f.Row+f.H > rowsPerColumn {
				t.Errorf("column %d field %d (%s): Row+H = %d, want <= %d", ci, fi, f.Name, f.Row+f.H, rowsPerColumn)
			}
			used += f.H
			flat = append(flat, f.Name)
		}
		if used > rowsPerColumn {
			t.Errorf("column %d: total height = %d, want <= %d", ci, used, rowsPerColumn)
		}
	}
	if len(flat) != len(expectedNames) {
		t.Fatalf("flattened names len = %d, want %d (got %v)", len(flat), len(expectedNames), flat)
	}
	for i, name := range expectedNames {
		if flat[i] != name {
			t.Errorf("flattened[%d] = %q, want %q", i, flat[i], name)
		}
	}
}

func TestCalculateGridLayout_EmptyInput(t *testing.T) {
	for _, w := range []int{-5, 0, 1, 50, 200} {
		plan := CalculateGridLayout(w, nil)
		if plan.Columns != nil {
			t.Errorf("width=%d nil: Columns = %v, want nil", w, plan.Columns)
		}
		if plan.FixedHeight != rowsPerColumn {
			t.Errorf("width=%d nil: FixedHeight = %d, want %d", w, plan.FixedHeight, rowsPerColumn)
		}
		plan = CalculateGridLayout(w, []GridField{})
		if plan.Columns != nil {
			t.Errorf("width=%d empty: Columns = %v, want nil", w, plan.Columns)
		}
	}
}

func TestCalculateGridLayout_EightSingleRowFieldsWide(t *testing.T) {
	fields := []GridField{
		fixedHeight("status", 1),
		fixedHeight("type", 1),
		fixedHeight("priority", 1),
		fixedHeight("points", 1),
		fixedHeight("assignee", 1),
		fixedHeight("due", 1),
		fixedHeight("recurrence", 1),
		fixedHeight("tags", 1),
	}
	plan := CalculateGridLayout(120, fields)
	if len(plan.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(plan.Columns))
	}
	for i, col := range plan.Columns {
		if len(col.Fields) != 4 {
			t.Errorf("column %d: len(Fields) = %d, want 4", i, len(col.Fields))
		}
	}
	assertInvariants(t, plan, []string{"status", "type", "priority", "points", "assignee", "due", "recurrence", "tags"})
}

func TestCalculateGridLayout_NarrowBelowMinColumnWidth(t *testing.T) {
	fields := []GridField{
		fixedHeight("status", 1),
		fixedHeight("type", 1),
		fixedHeight("priority", 1),
		fixedHeight("points", 1),
		fixedHeight("assignee", 1),
		fixedHeight("due", 1),
		fixedHeight("recurrence", 1),
		fixedHeight("tags", 1),
	}
	plan := CalculateGridLayout(20, fields)
	if len(plan.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2 (every field reachable)", len(plan.Columns))
	}
	for i, col := range plan.Columns {
		if col.Width < 1 {
			t.Errorf("column %d: Width = %d, want >= 1", i, col.Width)
		}
	}
	assertInvariants(t, plan, []string{"status", "type", "priority", "points", "assignee", "due", "recurrence", "tags"})
}

func TestCalculateGridLayout_TagsWrapTwoRows(t *testing.T) {
	fields := []GridField{
		fixedHeight("status", 1),
		fixedHeight("type", 1),
		fixedHeight("priority", 1),
		fixedHeight("points", 1),
		fixedHeight("tags", 2),
	}
	plan := CalculateGridLayout(120, fields)
	if len(plan.Columns) != 2 {
		t.Fatalf("len(Columns) = %d, want 2", len(plan.Columns))
	}
	if len(plan.Columns[0].Fields) != 4 {
		t.Errorf("col0: len(Fields) = %d, want 4 (single-row group)", len(plan.Columns[0].Fields))
	}
	if len(plan.Columns[1].Fields) != 1 || plan.Columns[1].Fields[0].Name != "tags" {
		t.Errorf("col1 = %+v, want one tags field", plan.Columns[1].Fields)
	}
	if plan.Columns[1].Fields[0].H != 2 {
		t.Errorf("tags H = %d, want 2", plan.Columns[1].Fields[0].H)
	}
	assertInvariants(t, plan, []string{"status", "type", "priority", "points", "tags"})
}

func TestCalculateGridLayout_FieldHeightSevenClampedToFour(t *testing.T) {
	fields := []GridField{
		fixedHeight("dependsOn", 7),
	}
	plan := CalculateGridLayout(80, fields)
	if len(plan.Columns) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(plan.Columns))
	}
	if plan.Columns[0].Fields[0].H != rowsPerColumn {
		t.Errorf("dependsOn H = %d, want %d (clamped)", plan.Columns[0].Fields[0].H, rowsPerColumn)
	}
	assertInvariants(t, plan, []string{"dependsOn"})
}

func TestCalculateGridLayout_OrderPreserved(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	fields := make([]GridField, len(names))
	for i, n := range names {
		fields[i] = fixedHeight(n, 1)
	}
	plan := CalculateGridLayout(200, fields)
	assertInvariants(t, plan, names)
}

func TestCalculateGridLayout_TwoPassConvergence(t *testing.T) {
	// At full width: tag is 1 row. At half width (after splitting into 2 columns): tag is 3 rows.
	// Threshold 80 < 120 so first pass reports H=1, but per-column width is ~60 so pass 2 reports H=3.
	fields := []GridField{
		fixedHeight("a", 1),
		fixedHeight("b", 1),
		fixedHeight("c", 1),
		fixedHeight("d", 1),
		widthDependent("tags", 80, 3, 1),
	}
	plan := CalculateGridLayout(120, fields)
	// Expect the tag's converged height to be 3 (col width ~60 < 80 threshold).
	var tagH int
	for _, col := range plan.Columns {
		for _, f := range col.Fields {
			if f.Name == "tags" {
				tagH = f.H
			}
		}
	}
	if tagH != 3 {
		t.Errorf("tags converged H = %d, want 3", tagH)
	}
	assertInvariants(t, plan, []string{"a", "b", "c", "d", "tags"})
}

func TestCalculateGridLayout_EmptyListsConsumeOneRow(t *testing.T) {
	// Simulates tags=[], dependsOn=[] returning H=1 each.
	fields := []GridField{
		fixedHeight("status", 1),
		fixedHeight("type", 1),
		fixedHeight("priority", 1),
		fixedHeight("tags", 1),
		fixedHeight("dependsOn", 1),
	}
	plan := CalculateGridLayout(120, fields)
	if len(plan.Columns) < 1 {
		t.Fatalf("expected ≥1 column, got %d", len(plan.Columns))
	}
	assertInvariants(t, plan, []string{"status", "type", "priority", "tags", "dependsOn"})
}

func TestCalculateGridLayout_GeometryInvariantsAtPathologicalWidths(t *testing.T) {
	fields := []GridField{
		fixedHeight("a", 1),
		fixedHeight("b", 1),
		fixedHeight("c", 1),
		fixedHeight("d", 1),
		fixedHeight("e", 1),
		fixedHeight("f", 1),
		fixedHeight("g", 1),
		fixedHeight("h", 1),
	}
	for _, w := range []int{0, 1, 2, 3, interColumnGap, interColumnGap - 1, 7, 15} {
		plan := CalculateGridLayout(w, fields)
		assertInvariants(t, plan, []string{"a", "b", "c", "d", "e", "f", "g", "h"})
		if len(plan.Columns) == 0 {
			t.Errorf("width=%d: Columns is empty for non-empty input", w)
		}
	}
}

func TestCalculateGridLayout_SingleColumnForFewFields(t *testing.T) {
	fields := []GridField{
		fixedHeight("a", 1),
		fixedHeight("b", 1),
		fixedHeight("c", 1),
	}
	plan := CalculateGridLayout(120, fields)
	if len(plan.Columns) != 1 {
		t.Fatalf("len(Columns) = %d, want 1", len(plan.Columns))
	}
	if len(plan.Columns[0].Fields) != 3 {
		t.Errorf("col0: len(Fields) = %d, want 3", len(plan.Columns[0].Fields))
	}
	assertInvariants(t, plan, []string{"a", "b", "c"})
}

package tiki

import (
	"testing"
	"time"
)

func TestNewAllocatesFieldsMap(t *testing.T) {
	tk := New()
	if tk.Fields == nil {
		t.Fatal("New() must allocate Fields; got nil")
	}
	// Set after New must not panic.
	tk.Set("k", "v")
	if v, ok := tk.Get("k"); !ok || v != "v" {
		t.Fatalf("Get after Set: got (%v, %v), want (v, true)", v, ok)
	}
}

func TestHasGetRequire(t *testing.T) {
	tk := &Tiki{ID: "ABC123", Fields: map[string]interface{}{"status": "ready"}}

	if !tk.Has("status") {
		t.Error("Has(status) = false, want true")
	}
	if tk.Has("priority") {
		t.Error("Has(priority) = true, want false")
	}

	if v, ok := tk.Get("status"); !ok || v != "ready" {
		t.Errorf("Get(status) = (%v, %v), want (ready, true)", v, ok)
	}
	if _, ok := tk.Get("missing"); ok {
		t.Error("Get(missing) returned present=true, want false")
	}

	if _, err := tk.Require("status"); err != nil {
		t.Errorf("Require(status) errored: %v", err)
	}
	if _, err := tk.Require("missing"); err == nil {
		t.Error("Require(missing) returned nil error; want hard error")
	}
}

func TestHasOnNilTikiAndNilFields(t *testing.T) {
	var tk *Tiki
	if tk.Has("anything") {
		t.Error("Has on nil *Tiki must return false")
	}

	tk2 := &Tiki{ID: "XYZ789"} // Fields is nil
	if tk2.Has("status") {
		t.Error("Has on nil Fields must return false")
	}
	if _, ok := tk2.Get("status"); ok {
		t.Error("Get on nil Fields must return present=false")
	}
}

func TestSetAllocatesFieldsMapLazily(t *testing.T) {
	tk := &Tiki{} // Fields nil
	tk.Set("priority", 3)
	if tk.Fields == nil {
		t.Fatal("Set must allocate Fields when nil")
	}
	if v, ok := tk.Get("priority"); !ok || v != 3 {
		t.Errorf("after Set(priority,3): got (%v, %v), want (3, true)", v, ok)
	}
}

func TestDeleteRemovesField(t *testing.T) {
	tk := &Tiki{Fields: map[string]interface{}{"status": "ready", "priority": 3}}
	tk.Delete("status")
	if tk.Has("status") {
		t.Error("Delete did not remove status")
	}
	if !tk.Has("priority") {
		t.Error("Delete removed unrelated field priority")
	}
	// Deleting an absent field is a no-op.
	tk.Delete("missing")
	// Delete on nil Fields is a no-op.
	tk2 := &Tiki{}
	tk2.Delete("anything")
}

func TestCloneDeepCopiesSliceFields(t *testing.T) {
	original := &Tiki{
		ID:    "ABC123",
		Title: "root",
		Body:  "body text",
		Fields: map[string]interface{}{
			"tags":      []string{"alpha", "beta"},
			"dependsOn": []string{"DEF456"},
			"priority":  3,
		},
	}

	clone := original.Clone()
	if clone == original {
		t.Fatal("Clone returned same pointer")
	}
	if clone.ID != "ABC123" || clone.Title != "root" || clone.Body != "body text" {
		t.Errorf("Clone scalar fields drift: %+v", clone)
	}

	// Mutating clone's tag slice must not affect original.
	cloneTags, ok := clone.Fields["tags"].([]string)
	if !ok {
		t.Fatalf("clone.Fields[tags] is not []string: %T", clone.Fields["tags"])
	}
	cloneTags[0] = "CHANGED"
	origTags, ok := original.Fields["tags"].([]string)
	if !ok {
		t.Fatalf("original.Fields[tags] is not []string: %T", original.Fields["tags"])
	}
	if origTags[0] != "alpha" {
		t.Errorf("clone mutation leaked to original: %v", origTags)
	}
}

func TestClonePreservesAbsentFieldsMap(t *testing.T) {
	// A Tiki with nil Fields should clone to another Tiki with nil Fields,
	// preserving the presence distinction between "no fields" and "empty map".
	tk := &Tiki{ID: "XYZ789"}
	clone := tk.Clone()
	if clone.Fields != nil {
		t.Errorf("Clone turned nil Fields into %v", clone.Fields)
	}
}

func TestCloneOfNilReturnsNil(t *testing.T) {
	var tk *Tiki
	if tk.Clone() != nil {
		t.Error("Clone on nil *Tiki must return nil")
	}
}

func TestIsSchemaKnown(t *testing.T) {
	for _, name := range SchemaKnownFields {
		if !IsSchemaKnown(name) {
			t.Errorf("IsSchemaKnown(%q) = false, want true", name)
		}
	}
	if IsSchemaKnown("customField") {
		t.Error("IsSchemaKnown(customField) = true, want false")
	}
	if IsSchemaKnown("id") || IsSchemaKnown("title") {
		t.Error("id and title are not schema-known fields (they live on the Tiki struct)")
	}
}

func TestStringFieldCoercion(t *testing.T) {
	tk := &Tiki{Fields: map[string]interface{}{"status": "ready", "priority": 3}}

	s, present, coerced := tk.StringField("status")
	if !present || !coerced || s != "ready" {
		t.Errorf("StringField(status) = (%q, %v, %v), want (ready, true, true)", s, present, coerced)
	}

	// Non-string value: present, but does not coerce.
	_, present, coerced = tk.StringField("priority")
	if !present {
		t.Error("StringField on present int must report present=true")
	}
	if coerced {
		t.Error("StringField on int value must report coerced=false")
	}

	// Missing: present=false.
	_, present, _ = tk.StringField("missing")
	if present {
		t.Error("StringField on missing must report present=false")
	}
}

func TestRequireStringErrors(t *testing.T) {
	tk := &Tiki{ID: "ABC123", Fields: map[string]interface{}{"priority": 3}}
	if _, err := tk.RequireString("missing"); err == nil {
		t.Error("RequireString(missing) must error")
	}
	if _, err := tk.RequireString("priority"); err == nil {
		t.Error("RequireString on non-string must error")
	}
}

func TestIntFieldCoercion(t *testing.T) {
	cases := map[string]interface{}{
		"int":     3,
		"int64":   int64(3),
		"float64": float64(3),
		"uint":    uint(3),
	}
	for label, v := range cases {
		tk := &Tiki{Fields: map[string]interface{}{"priority": v}}
		n, present, coerced := tk.IntField("priority")
		if !present || !coerced || n != 3 {
			t.Errorf("IntField(%s) = (%d, %v, %v), want (3, true, true)", label, n, present, coerced)
		}
	}

	// Non-numeric: present, not coerced.
	tk := &Tiki{Fields: map[string]interface{}{"priority": "three"}}
	_, present, coerced := tk.IntField("priority")
	if !present {
		t.Error("IntField on present string must report present=true")
	}
	if coerced {
		t.Error("IntField on string must report coerced=false")
	}
}

func TestStringSliceFieldCoercion(t *testing.T) {
	tk := &Tiki{Fields: map[string]interface{}{
		"tags":      []string{"a", "b"},
		"dependsOn": []interface{}{"X1", "X2"},
		"solo":      "onlyone",
	}}

	ss, _, coerced := tk.StringSliceField("tags")
	if !coerced || len(ss) != 2 || ss[0] != "a" || ss[1] != "b" {
		t.Errorf("StringSliceField(tags) = %v coerced=%v", ss, coerced)
	}

	ss, _, coerced = tk.StringSliceField("dependsOn")
	if !coerced || len(ss) != 2 || ss[0] != "X1" || ss[1] != "X2" {
		t.Errorf("StringSliceField([]interface{}) = %v coerced=%v", ss, coerced)
	}

	// Single scalar string promoted to one-element slice.
	ss, _, coerced = tk.StringSliceField("solo")
	if !coerced || len(ss) != 1 || ss[0] != "onlyone" {
		t.Errorf("StringSliceField(scalar) = %v coerced=%v", ss, coerced)
	}
}

func TestStringSliceFieldReturnsCopy(t *testing.T) {
	orig := []string{"a", "b"}
	tk := &Tiki{Fields: map[string]interface{}{"tags": orig}}
	got, _, _ := tk.StringSliceField("tags")
	got[0] = "CHANGED"
	if orig[0] != "a" {
		t.Errorf("StringSliceField leaked internal slice; original mutated to %v", orig)
	}
}

func TestTimeFieldCoercion(t *testing.T) {
	want := time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	tk := &Tiki{Fields: map[string]interface{}{
		"due":     want,
		"dueStr":  "2026-03-05",
		"dueBad":  "not-a-date",
		"dueKind": 42,
	}}

	got, _, coerced := tk.TimeField("due")
	if !coerced || !got.Equal(want) {
		t.Errorf("TimeField(time.Time) = (%v, %v), want (%v, true)", got, coerced, want)
	}

	got, _, coerced = tk.TimeField("dueStr")
	if !coerced || !got.Equal(want) {
		t.Errorf("TimeField(string) = (%v, %v), want (%v, true)", got, coerced, want)
	}

	_, _, coerced = tk.TimeField("dueBad")
	if coerced {
		t.Error("TimeField(bad string) must report coerced=false")
	}

	_, _, coerced = tk.TimeField("dueKind")
	if coerced {
		t.Error("TimeField(int) must report coerced=false")
	}
}

func TestSchemaKnownFieldsOrderIsStable(t *testing.T) {
	// Downstream YAML serialization relies on this order.
	want := []string{"status", "type", "priority", "points", "tags", "dependsOn", "due", "recurrence", "assignee"}
	if len(SchemaKnownFields) != len(want) {
		t.Fatalf("SchemaKnownFields length = %d, want %d", len(SchemaKnownFields), len(want))
	}
	for i, w := range want {
		if SchemaKnownFields[i] != w {
			t.Errorf("SchemaKnownFields[%d] = %q, want %q", i, SchemaKnownFields[i], w)
		}
	}
}

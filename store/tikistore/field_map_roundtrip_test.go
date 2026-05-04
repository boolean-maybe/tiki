package tikistore

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// TestFieldMapRoundTrip_ExactPresenceAcrossSaveLoad verifies the headline
// Phase 7 invariant: the set of keys present in Fields after a save → reload
// cycle is byte-identical to the set the caller put in. No keys are
// synthesized on save; no keys are synthesized on load.
func TestFieldMapRoundTrip_ExactPresenceAcrossSaveLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	// Seed the dependency target first so dependsOn validation succeeds.
	dep := tikipkg.New()
	dep.ID = "DEP001"
	dep.Title = "dependency target"
	if err := s.CreateTiki(dep); err != nil {
		t.Fatalf("CreateTiki dep: %v", err)
	}

	tk := tikipkg.New()
	tk.ID = "RT0001"
	tk.Title = "round trip"
	// Body is stored without a trailing newline; the persistence layer
	// adds one on write and parsing strips it on load.
	tk.Body = "body content"
	tk.Set(tikipkg.FieldStatus, "backlog")
	tk.Set(tikipkg.FieldPriority, 2)
	tk.Set(tikipkg.FieldDependsOn, []string{"DEP001"})
	tk.Set(tikipkg.FieldTags, []string{"alpha", "beta"})

	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	// Reload from disk to round-trip everything through the file.
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	got := s.GetTiki("RT0001")
	if got == nil {
		t.Fatal("GetTiki post-reload = nil")
	}

	// Identity fields preserved.
	if got.ID != "RT0001" {
		t.Errorf("ID drift: %q", got.ID)
	}
	if got.Title != "round trip" {
		t.Errorf("Title drift: %q", got.Title)
	}
	if got.Body != "body content" {
		t.Errorf("Body drift: %q", got.Body)
	}

	// Field presence set is byte-identical to what the caller wrote — plus
	// the audit-ish "createdBy" key the persistence layer always populates
	// from identity. That key is allowed to appear; the schema-known set
	// must match exactly.
	wantSchemaPresent := []string{
		tikipkg.FieldStatus,
		tikipkg.FieldPriority,
		tikipkg.FieldDependsOn,
		tikipkg.FieldTags,
	}
	for _, k := range wantSchemaPresent {
		if !got.Has(k) {
			t.Errorf("schema-known field %q missing after reload", k)
		}
	}
	for _, k := range tikipkg.SchemaKnownFields {
		shouldBePresent := false
		for _, want := range wantSchemaPresent {
			if want == k {
				shouldBePresent = true
				break
			}
		}
		if !shouldBePresent && got.Has(k) {
			t.Errorf("schema-known field %q gained presence on reload (not set by caller); value=%v",
				k, got.Fields[k])
		}
	}

	// And the values for the keys we set are intact.
	if status, _, _ := got.StringField(tikipkg.FieldStatus); status != "backlog" {
		t.Errorf("status drift: %q", status)
	}
	if pri, _, _ := got.IntField(tikipkg.FieldPriority); pri != 2 {
		t.Errorf("priority drift: %d", pri)
	}
	deps, _, _ := got.StringSliceField(tikipkg.FieldDependsOn)
	sort.Strings(deps)
	if !reflect.DeepEqual(deps, []string{"DEP001"}) {
		t.Errorf("dependsOn drift: %v", deps)
	}
	tags, _, _ := got.StringSliceField(tikipkg.FieldTags)
	sort.Strings(tags)
	if !reflect.DeepEqual(tags, []string{"alpha", "beta"}) {
		t.Errorf("tags drift: %v", tags)
	}
}

// TestFieldMapRoundTrip_UnknownFieldsRoundTripVerbatim verifies the
// unknown-field round-trip story: a frontmatter key that is neither a
// schema-known field nor a registered Custom field passes through load →
// save → load unchanged. This is what lets external tools annotate tiki
// files with their own keys without the store stripping them.
func TestFieldMapRoundTrip_UnknownFieldsRoundTripVerbatim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	path := filepath.Join(dir, "RTUNKN.md")
	src := "---\nid: RTUNKN\ntitle: with unknowns\nstatus: backlog\nexternalRef: \"link-123\"\nexternalScore: 7\n---\nbody\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s, err := NewTikiStore(dir)
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}
	loaded := s.GetTiki("RTUNKN")
	if loaded == nil {
		t.Fatal("GetTiki = nil")
	}
	if v, ok := loaded.Fields["externalRef"]; !ok || v != "link-123" {
		t.Errorf("externalRef did not load: got (%v, %v)", v, ok)
	}

	// Save and reload — unknowns survive.
	updated := loaded.Clone()
	updated.Title = "edited"
	if err := s.UpdateTiki(updated); err != nil {
		t.Fatalf("UpdateTiki: %v", err)
	}
	if err := s.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	got := s.GetTiki("RTUNKN")
	if got == nil {
		t.Fatal("GetTiki post-reload = nil")
	}
	if v, ok := got.Fields["externalRef"]; !ok || v != "link-123" {
		t.Errorf("externalRef did not survive round trip: got (%v, %v)", v, ok)
	}
	if v, ok := got.Fields["externalScore"]; !ok || v != 7 {
		t.Errorf("externalScore did not survive round trip: got (%v, %v)", v, ok)
	}
}

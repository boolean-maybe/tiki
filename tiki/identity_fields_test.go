package tiki

import "testing"

// TestIdentityFieldsAreNotInFieldsMap pins the contract that id, title, and
// body are special — they live as struct fields rather than entries in the
// generic Fields map. Has/Get/Require for those names should never resolve
// against the struct fields; that's by design so frontmatter parsing and
// ruki's `has(field)` operate on the same notion of "in Fields".
func TestIdentityFieldsAreNotInFieldsMap(t *testing.T) {
	tk := &Tiki{
		ID:    "ABC123",
		Title: "hello",
		Body:  "body content",
	}

	for _, name := range []string{"id", "title", "body"} {
		if tk.Has(name) {
			t.Errorf("Has(%q) returned true; identity fields must not be exposed via the Fields map", name)
		}
		if _, ok := tk.Get(name); ok {
			t.Errorf("Get(%q) reported present; identity fields must not be exposed via the Fields map", name)
		}
		if _, err := tk.Require(name); err == nil {
			t.Errorf("Require(%q) returned nil error; identity fields must not be reachable as generic Fields", name)
		}
	}

	// And the actual struct fields are unaffected by the absence of map
	// entries — the Tiki still carries the special values.
	if tk.ID != "ABC123" || tk.Title != "hello" || tk.Body != "body content" {
		t.Errorf("identity fields drifted: %+v", tk)
	}
}

// TestIsIdentityFieldRecognizesAuditAndIdentity exercises the full set of
// names the persistence layer treats as struct/audit metadata rather than
// Fields-map entries.
func TestIsIdentityFieldRecognizesAuditAndIdentity(t *testing.T) {
	identity := []string{
		"id", "title", "description", "body",
		"createdBy", "createdAt", "updatedAt",
		"filepath", "path",
	}
	for _, name := range identity {
		if !IsIdentityField(name) {
			t.Errorf("IsIdentityField(%q) = false, want true", name)
		}
	}

	for _, name := range []string{"status", "type", "priority", "tags", "customField"} {
		if IsIdentityField(name) {
			t.Errorf("IsIdentityField(%q) = true; non-identity name must not be classified as identity", name)
		}
	}
}

// TestSetIdentityNameDoesNotShadowStructField verifies that even if a caller
// puts "id" or "title" into Fields, the special accessors still work. The
// Fields map can carry the entry but it does not promote into the special
// struct fields.
func TestSetIdentityNameDoesNotShadowStructField(t *testing.T) {
	tk := &Tiki{ID: "REAL01", Title: "real title"}
	tk.Set("id", "FAKE99")
	tk.Set("title", "fake title")

	if tk.ID != "REAL01" {
		t.Errorf("Tiki.ID got clobbered by Set(\"id\", ...): %q", tk.ID)
	}
	if tk.Title != "real title" {
		t.Errorf("Tiki.Title got clobbered by Set(\"title\", ...): %q", tk.Title)
	}
	// The Fields-map entry is its own thing — Set is generic about names.
	if v, ok := tk.Get("id"); !ok || v != "FAKE99" {
		t.Errorf("Get(\"id\") after Set: got (%v, %v); want stored entry (FAKE99, true)", v, ok)
	}
}

// TestSetClearsStaleMarker verifies the stale-provenance protocol: an
// explicit Set on a field that was loaded as Unknown (because its value
// failed coercion) clears the marker, so the field round-trips back as
// a normal Custom field on the next save.
func TestSetClearsStaleMarker(t *testing.T) {
	tk := New()
	tk.Set("severity", "garbage-value")
	tk.MarkStaleForPersistence("severity")
	if _, has := tk.StaleKeys()["severity"]; !has {
		t.Fatal("MarkStaleForPersistence did not record severity in StaleKeys")
	}

	// Caller writes a fresh value: Set must clear the stale marker so a
	// follow-up validateTikiCustomFields treats this as a normal Custom
	// field write.
	tk.Set("severity", "high")
	if _, has := tk.StaleKeys()["severity"]; has {
		t.Error("Set must clear the stale marker; the rewrite is treated as a repair")
	}
}

// TestDeleteClearsStaleMarker complements TestSetClearsStaleMarker: a
// Delete must also drop the stale-marker entry so an absent field never
// has lingering provenance.
func TestDeleteClearsStaleMarker(t *testing.T) {
	tk := New()
	tk.Set("severity", "garbage")
	tk.MarkStaleForPersistence("severity")

	tk.Delete("severity")
	if _, has := tk.StaleKeys()["severity"]; has {
		t.Error("Delete must clear the stale marker for the removed field")
	}
}

// TestStaleKeysReturnsCopy verifies the persistence-adapter contract: the
// returned map is a snapshot, not a live reference into Tiki state.
func TestStaleKeysReturnsCopy(t *testing.T) {
	tk := New()
	tk.Set("a", 1)
	tk.MarkStaleForPersistence("a")

	snap := tk.StaleKeys()
	delete(snap, "a")

	if _, has := tk.StaleKeys()["a"]; !has {
		t.Error("mutating the StaleKeys snapshot leaked into the Tiki's stale set")
	}
}

// TestStaleKeysOnNilOrEmpty checks the nil/empty edge cases of StaleKeys.
func TestStaleKeysOnNilOrEmpty(t *testing.T) {
	var nilTk *Tiki
	if nilTk.StaleKeys() != nil {
		t.Error("StaleKeys on nil *Tiki must return nil")
	}

	emptyTk := New()
	if emptyTk.StaleKeys() != nil {
		t.Error("StaleKeys on Tiki with no stale entries must return nil")
	}
}

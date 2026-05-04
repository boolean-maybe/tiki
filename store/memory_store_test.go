package store

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

// newWorkflowTiki builds a workflow tiki from basic task-like fields.
// Tags and dependsOn are normalized (deduped, trimmed) before storage.
func newWorkflowTiki(id, title, status string, tags, dependsOn []string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Title = title
	if status != "" {
		tk.Set(tikipkg.FieldStatus, status)
	}
	if len(tags) > 0 {
		tk.Set(tikipkg.FieldTags, collectionutil.NormalizeStringSet(tags))
	}
	if len(dependsOn) > 0 {
		tk.Set(tikipkg.FieldDependsOn, collectionutil.NormalizeRefSet(dependsOn))
	}
	return tk
}

func TestInMemoryStore_CreateTiki(t *testing.T) {
	tests := []struct {
		name     string
		inputID  string
		expected string
	}{
		{"normalizes ID to uppercase", "abc123", "ABC123"},
		{"trims whitespace from ID", "  DEF456  ", "DEF456"},
		{"already uppercase passthrough", "GHI789", "GHI789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewInMemoryStore()
			tk := tikipkg.New()
			tk.ID = tt.inputID
			tk.Title = "Test"
			tk.Set(tikipkg.FieldType, string(taskpkg.TypeStory))
			tk.Set(tikipkg.FieldStatus, string(taskpkg.DefaultStatus()))

			if err := s.CreateTiki(tk); err != nil {
				t.Fatalf("CreateTiki() error = %v", err)
			}
			if tk.ID != tt.expected {
				t.Errorf("tk.ID = %q, want %q", tk.ID, tt.expected)
			}
			if tk.CreatedAt.IsZero() {
				t.Error("expected non-zero CreatedAt")
			}
			if tk.UpdatedAt.IsZero() {
				t.Error("expected non-zero UpdatedAt")
			}
			got := s.GetTiki(tt.expected)
			if got == nil {
				t.Errorf("GetTiki(%q) returned nil after CreateTiki", tt.expected)
			}
		})
	}
}

func TestInMemoryStore_CreateTikiNormalizesCollections(t *testing.T) {
	s := NewInMemoryStore()
	// need existing tikis that dependsOn references so validation passes.
	if err := s.CreateTiki(&tikipkg.Tiki{ID: "AAA001", Title: "dep1"}); err != nil {
		t.Fatalf("setup dep1: %v", err)
	}
	if err := s.CreateTiki(&tikipkg.Tiki{ID: "BBB002", Title: "dep2"}); err != nil {
		t.Fatalf("setup dep2: %v", err)
	}
	tk := newWorkflowTiki("NORM01", "normalize", "",
		[]string{"frontend", "frontend", " backend ", ""},
		[]string{"aaa001", "AAA001", "bbb002"},
	)

	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki() error = %v", err)
	}

	tags, _, _ := tk.StringSliceField(tikipkg.FieldTags)
	if !reflect.DeepEqual(tags, []string{"frontend", "backend"}) {
		t.Errorf("tags = %v, want [frontend backend]", tags)
	}
	deps, _, _ := tk.StringSliceField(tikipkg.FieldDependsOn)
	if !reflect.DeepEqual(deps, []string{"AAA001", "BBB002"}) {
		t.Errorf("dependsOn = %v, want [AAA001 BBB002]", deps)
	}
}

func TestInMemoryStore_UpdateTiki(t *testing.T) {
	t.Run("error when tiki not found", func(t *testing.T) {
		s := NewInMemoryStore()
		tk := &tikipkg.Tiki{ID: "MISSING", Title: "Ghost"}
		err := s.UpdateTiki(tk)
		if err == nil {
			t.Error("expected error for non-existent tiki, got nil")
		}
	})

	t.Run("normalizes ID before update", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "ABC123", Title: "Original"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		updated := &tikipkg.Tiki{ID: "abc123", Title: "Updated"}
		if err := s.UpdateTiki(updated); err != nil {
			t.Fatalf("UpdateTiki() error = %v", err)
		}
		got := s.GetTiki("ABC123")
		if got == nil || got.Title != "Updated" {
			t.Errorf("expected title %q, got %v", "Updated", got)
		}
	})

	t.Run("sets UpdatedAt after update", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "UPD001", Title: "Before"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}
		stored := s.GetTiki("UPD001")
		before := stored.UpdatedAt

		time.Sleep(time.Millisecond)
		updated := stored.Clone()
		updated.Title = "After"
		if err := s.UpdateTiki(updated); err != nil {
			t.Fatalf("UpdateTiki() error = %v", err)
		}
		after := s.GetTiki("UPD001")
		if !after.UpdatedAt.After(before) {
			t.Errorf("expected UpdatedAt to advance after update")
		}
	})

	t.Run("updates tiki fields roundtrip", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "RT0001", Title: "Old Title"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		got := s.GetTiki("RT0001").Clone()
		got.Title = "New Title"
		if err := s.UpdateTiki(got); err != nil {
			t.Fatalf("UpdateTiki() error = %v", err)
		}

		reloaded := s.GetTiki("RT0001")
		if reloaded.Title != "New Title" {
			t.Errorf("title = %q, want %q", reloaded.Title, "New Title")
		}
	})

	// UpdateTiki uses exact-presence semantics: fields absent in the incoming
	// tiki are removed. Callers that want to preserve existing fields must load
	// the stored tiki and clone it before updating.
	t.Run("exact-presence: absent fields are removed", func(t *testing.T) {
		s := NewInMemoryStore()
		wf := tikipkg.New()
		wf.ID = "WFMEM1"
		wf.Title = "workflow"
		wf.Set("status", "ready")
		if err := s.CreateTiki(wf); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		// Update with a tiki that carries no schema-known fields. Exact-
		// presence semantics drop the previously-stored status.
		bare := &tikipkg.Tiki{ID: "WFMEM1", Title: "updated"}
		if err := s.UpdateTiki(bare); err != nil {
			t.Fatalf("UpdateTiki() error = %v", err)
		}
		if hasAnySchemaFieldMem(s.GetTiki("WFMEM1")) {
			t.Error("schema-known fields should be removed with exact-presence UpdateTiki")
		}
	})
}

func TestInMemoryStore_DeleteTiki(t *testing.T) {
	t.Run("removes existing tiki", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "DEL001", Title: "To Delete"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		s.DeleteTiki("DEL001")
		if s.GetTiki("DEL001") != nil {
			t.Error("expected nil after delete, got tiki")
		}
	})

	t.Run("no panic for non-existent ID", func(t *testing.T) {
		s := NewInMemoryStore()
		s.DeleteTiki("GHOST1") // should not panic
	})

	t.Run("normalizes ID for delete", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "LOWER1", Title: "Task"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		s.DeleteTiki("lower1") // lowercase bare id
		if s.GetTiki("LOWER1") != nil {
			t.Error("expected nil after delete with lowercase ID")
		}
	})
}

func TestInMemoryStore_TikiComment(t *testing.T) {
	t.Run("comment append and retrieve via tiki fields", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "CMT001", Title: "Task"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		tk := s.GetTiki("CMT001").Clone()
		tk.Set("comments", []taskpkg.Comment{{Text: "first comment"}})
		if err := s.UpdateTiki(tk); err != nil {
			t.Fatalf("UpdateTiki() error = %v", err)
		}

		got := s.GetTiki("CMT001")
		if got == nil {
			t.Fatal("GetTiki = nil")
		}
		v, ok := got.Fields["comments"]
		if !ok {
			t.Fatal("comments field missing")
		}
		cs, ok := v.([]taskpkg.Comment)
		if !ok || len(cs) != 1 || cs[0].Text != "first comment" {
			t.Errorf("comments = %v, want [{first comment}]", v)
		}
	})
}

// TestInMemoryStore_Search_FilterIsCallerSupplied verifies the search
// contract: nil filter returns every tiki, while a caller-supplied predicate
// (here, "has any schema-known field") narrows the result set. The store
// itself does not classify tikis — filtering is purely caller-driven.
func TestInMemoryStore_Search_FilterIsCallerSupplied(t *testing.T) {
	s := NewInMemoryStore()
	// one tiki has a schema-known field set.
	withSchema := tikipkg.New()
	withSchema.ID = "WITH01"
	withSchema.Title = "has status"
	withSchema.Set("status", "backlog")
	s.tikis["WITH01"] = withSchema
	// the other has only id+title — no schema-known fields.
	bare := tikipkg.New()
	bare.ID = "BARE01"
	bare.Title = "bare"
	s.tikis["BARE01"] = bare

	// SearchTikis with a presence-of-schema filter matches only the one
	// that has a schema-known field set.
	results := s.SearchTikis("", func(tk *tikipkg.Tiki) bool {
		return hasAnySchemaFieldMem(tk)
	})
	if len(results) != 1 || results[0].ID != "WITH01" {
		t.Errorf("schema-presence filter: got %v, want [WITH01]", results)
	}

	// nil filter returns all tikis regardless of field-map shape.
	all := s.SearchTikis("", nil)
	if len(all) != 2 {
		t.Errorf("nil filter: got %d, want 2", len(all))
	}
}

func TestInMemoryStore_SearchTikis(t *testing.T) {
	buildStore := func(tb testing.TB) *InMemoryStore {
		tb.Helper()
		s := NewInMemoryStore()
		for _, spec := range []struct {
			id, title, body string
			tags            []string
		}{
			{"S00001", "Alpha feature", "desc alpha", []string{"ui", "frontend"}},
			{"S00002", "Beta Bug", "beta description", []string{"backend"}},
			{"S00003", "Gamma chore", "third task", nil},
		} {
			tk := tikipkg.New()
			tk.ID = spec.id
			tk.Title = spec.title
			tk.Body = spec.body
			if len(spec.tags) > 0 {
				tk.Set(tikipkg.FieldTags, spec.tags)
			}
			tk.Set(tikipkg.FieldStatus, "backlog")
			if err := s.CreateTiki(tk); err != nil {
				tb.Fatalf("CreateTiki() error = %v", err)
			}
		}
		return s
	}

	tests := []struct {
		name       string
		query      string
		minResults int
		maxResults int
	}{
		{"empty query returns all", "", 3, 3},
		{"matches ID case-insensitive", "s00001", 1, 1},
		{"matches title case-insensitive", "alpha", 1, 1},
		{"matches body/description", "beta description", 1, 1},
		{"non-matching query returns empty", "zzz-no-match", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := buildStore(t)
			results := s.SearchTikis(tt.query, nil)
			if len(results) < tt.minResults || len(results) > tt.maxResults {
				t.Errorf("SearchTikis(%q) returned %d results, want [%d, %d]", tt.query, len(results), tt.minResults, tt.maxResults)
			}
		})
	}
}

func TestInMemoryStore_Listeners(t *testing.T) {
	t.Run("called after CreateTiki", func(t *testing.T) {
		s := NewInMemoryStore()
		called := 0
		s.AddListener(func() { called++ })
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "LIS001"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after UpdateTiki", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "LIS002", Title: "orig"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		tk := s.GetTiki("LIS002").Clone()
		if err := s.UpdateTiki(tk); err != nil {
			t.Fatalf("UpdateTiki() error = %v", err)
		}
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("called after DeleteTiki", func(t *testing.T) {
		s := NewInMemoryStore()
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "LIS003"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}
		called := 0
		s.AddListener(func() { called++ })
		s.DeleteTiki("LIS003")
		if called != 1 {
			t.Errorf("listener called %d times, want 1", called)
		}
	})

	t.Run("not called after RemoveListener", func(t *testing.T) {
		s := NewInMemoryStore()
		called := 0
		id := s.AddListener(func() { called++ })
		s.RemoveListener(id)
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "LIS005"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}
		if called != 0 {
			t.Errorf("removed listener called %d times, want 0", called)
		}
	})

	t.Run("multiple listeners all notified", func(t *testing.T) {
		s := NewInMemoryStore()
		a, b := 0, 0
		s.AddListener(func() { a++ })
		s.AddListener(func() { b++ })
		if err := s.CreateTiki(&tikipkg.Tiki{ID: "LIS006"}); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}
		if a != 1 || b != 1 {
			t.Errorf("listeners called a=%d b=%d, want both 1", a, b)
		}
	})
}

// TestInMemoryStore_NewTikiTemplate verifies workflow defaults are applied.
func TestInMemoryStore_NewTikiTemplate(t *testing.T) {
	s := NewInMemoryStore()
	tmpl, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate() error = %v", err)
	}
	if len(tmpl.ID) != 6 {
		t.Errorf("ID = %q, want 6-character bare ID", tmpl.ID)
	}
	if p, _, _ := tmpl.IntField(tikipkg.FieldPriority); p != 3 {
		t.Errorf("Priority = %d, want 3", p)
	}
	if pts, _, _ := tmpl.IntField(tikipkg.FieldPoints); pts != 1 {
		t.Errorf("Points = %d, want 1", pts)
	}
	if tags, _, _ := tmpl.StringSliceField(tikipkg.FieldTags); len(tags) != 1 || tags[0] != "idea" {
		t.Errorf("Tags = %v, want [idea]", tags)
	}
	if s, _, _ := tmpl.StringField(tikipkg.FieldStatus); s != string(taskpkg.DefaultStatus()) {
		t.Errorf("Status = %q, want %q", s, taskpkg.DefaultStatus())
	}
	if s, _, _ := tmpl.StringField(tikipkg.FieldType); s != string(taskpkg.DefaultType()) {
		t.Errorf("Type = %q, want %q", s, taskpkg.DefaultType())
	}
}

func TestBuildMemoryFieldDefaults_DedupesCollectionDefaults(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, DefaultValue: []string{"backend", " backend ", "backend"}},
		{Name: "related", Type: workflow.TypeListRef, DefaultValue: []string{"aaa001", "AAA001", "bbb002"}},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	defaults := buildMemoryFieldDefaults()
	labels, ok := defaults["labels"].([]string)
	if !ok {
		t.Fatalf("labels type = %T, want []string", defaults["labels"])
	}
	if !reflect.DeepEqual(labels, []string{"backend"}) {
		t.Errorf("labels = %v, want [backend]", labels)
	}

	related, ok := defaults["related"].([]string)
	if !ok {
		t.Fatalf("related type = %T, want []string", defaults["related"])
	}
	if !reflect.DeepEqual(related, []string{"AAA001", "BBB002"}) {
		t.Errorf("related = %v, want [AAA001 BBB002]", related)
	}
}

func TestInMemoryStore_NewTikiTemplateCollision(t *testing.T) {
	s := NewInMemoryStore()

	// pre-populate store with a tiki that will collide on bare id
	_ = s.CreateTiki(&tikipkg.Tiki{ID: "AAAAAA", Title: "existing"})

	callCount := 0
	s.idGenerator = func() string {
		callCount++
		if callCount == 1 {
			return "AAAAAA" // will collide with existing bare id
		}
		return "BBBBBB" // will succeed
	}

	tmpl, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate() error = %v", err)
	}
	if tmpl.ID != "BBBBBB" {
		t.Errorf("ID = %q, want BBBBBB (should skip collision)", tmpl.ID)
	}
	if callCount != 2 {
		t.Errorf("idGenerator called %d times, want 2 (one collision + one success)", callCount)
	}
}

func TestInMemoryStore_NewTikiTemplateExhaustion(t *testing.T) {
	s := NewInMemoryStore()

	// pre-populate with the only ID the generator will ever produce
	_ = s.CreateTiki(&tikipkg.Tiki{ID: "AAAAAA", Title: "existing"})

	s.idGenerator = func() string { return "AAAAAA" }

	_, err := s.NewTikiTemplate()
	if err == nil {
		t.Fatal("expected error for ID exhaustion, got nil")
	}
	if !strings.Contains(err.Error(), "failed to generate unique task ID") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestInMemoryStore_GetAllTikis(t *testing.T) {
	t.Run("empty store returns empty slice", func(t *testing.T) {
		s := NewInMemoryStore()
		tikis := s.GetAllTikis()
		if len(tikis) != 0 {
			t.Errorf("got %d tikis, want 0", len(tikis))
		}
	})

	t.Run("3 tikis returns len 3", func(t *testing.T) {
		s := NewInMemoryStore()
		for _, id := range []string{"ALL001", "ALL002", "ALL003"} {
			wf := tikipkg.New()
			wf.ID = id
			wf.Set("status", "ready")
			if err := s.CreateTiki(wf); err != nil {
				t.Fatalf("CreateTiki(%s) error = %v", id, err)
			}
		}
		tikis := s.GetAllTikis()
		if len(tikis) != 3 {
			t.Errorf("got %d tikis, want 3", len(tikis))
		}
	})

	t.Run("returns projections not raw pointers", func(t *testing.T) {
		s := NewInMemoryStore()
		orig := tikipkg.New()
		orig.ID = "PTR001"
		orig.Title = "Pointer Task"
		orig.Set("status", "ready")
		if err := s.CreateTiki(orig); err != nil {
			t.Fatalf("CreateTiki() error = %v", err)
		}

		tikis := s.GetAllTikis()
		if len(tikis) != 1 {
			t.Fatalf("got %d tikis, want 1", len(tikis))
		}
		if tikis[0].Title != "Pointer Task" {
			t.Errorf("title = %q, want 'Pointer Task'", tikis[0].Title)
		}
		// Phase 5: GetAllTikis returns pointers to stored state.
		// Mutations on returned tikis DO affect the store (map semantics).
		// Use UpdateTiki to persist changes correctly.
	})
}

func TestSearchTikis_WithQueryAndFilter(t *testing.T) {
	s := NewInMemoryStore()
	for _, spec := range []struct {
		id, title string
		tags      []string
	}{
		{"SRC001", "Bug in parser", []string{"backend"}},
		{"SRC002", "Bug in UI", []string{"frontend"}},
		{"SRC003", "Feature request", []string{"backend"}},
	} {
		tk := newWorkflowTiki(spec.id, spec.title, "backlog", spec.tags, nil)
		if err := s.CreateTiki(tk); err != nil {
			t.Fatalf("failed to create tiki: %v", err)
		}
	}

	// query "Bug" + filter for backend tag
	results := s.SearchTikis("Bug", func(tk *tikipkg.Tiki) bool {
		tags, _, _ := tk.StringSliceField("tags")
		for _, tag := range tags {
			if tag == "backend" {
				return true
			}
		}
		return false
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result (Bug + backend), got %d", len(results))
	}
	if results[0].ID != "SRC001" {
		t.Errorf("expected SRC001, got %s", results[0].ID)
	}
}

func TestSearchTikis_FilterRejectsAll(t *testing.T) {
	s := NewInMemoryStore()
	if err := s.CreateTiki(&tikipkg.Tiki{ID: "REJ001", Title: "Task"}); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}

	results := s.SearchTikis("", func(_ *tikipkg.Tiki) bool {
		return false // reject all
	})
	if len(results) != 0 {
		t.Fatalf("expected 0 results when filter rejects all, got %d", len(results))
	}
}

func TestSearchTikis_MatchesTags(t *testing.T) {
	s := NewInMemoryStore()
	wf := tikipkg.New()
	wf.ID = "TAG001"
	wf.Title = "No match in title"
	wf.Body = "No match in body either"
	wf.Set("tags", []string{"backend"})
	wf.Set("status", "ready")
	if err := s.CreateTiki(wf); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}

	// SearchTikis searches id, title, body, AND tags — a tag-only query
	// must surface the tiki even when neither title nor body matches.
	results := s.SearchTikis("backend", nil)
	if len(results) != 1 || results[0].ID != "TAG001" {
		ids := make([]string, len(results))
		for i, r := range results {
			ids[i] = r.ID
		}
		t.Fatalf("tag-only query: got %v, want [TAG001]", ids)
	}

	// Substring within a tag should also match (case-insensitive).
	if results := s.SearchTikis("BACK", nil); len(results) != 1 || results[0].ID != "TAG001" {
		t.Errorf("case-insensitive tag substring did not match: got %d results", len(results))
	}

	// And a query that does not match the tag, title, or body returns nothing.
	if results := s.SearchTikis("frontend", nil); len(results) != 0 {
		t.Errorf("non-matching query returned results: %v", results)
	}
}

func TestNewTikiTemplate_IDCollision(t *testing.T) {
	s := NewInMemoryStore()
	// pre-populate so the generated bare ID always collides
	if err := s.CreateTiki(&tikipkg.Tiki{ID: "FIXED1", Title: "blocker"}); err != nil {
		t.Fatalf("failed to create tiki: %v", err)
	}

	// set idGenerator to always return the same ID
	s.idGenerator = func() string { return "FIXED1" }

	_, err := s.NewTikiTemplate()
	if err == nil {
		t.Fatal("expected error for ID exhaustion")
	}
}

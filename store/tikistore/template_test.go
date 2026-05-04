package tikistore

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// TestSetAuthorFromIdentityTiki_EmailOnly_NoAngleEchoing reproduces a regression
// from the user()/display-string promotion work: when only identity.email is
// configured, currentUser returned (email, email) so author formatting
// produced `me@example.com <me@example.com>`. Attribution should just be the
// raw email in that case.
func TestSetAuthorFromIdentityTiki_EmailOnly_NoAngleEchoing(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "", "me@example.com")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	tk := tikipkg.New()
	s.setAuthorFromIdentityTiki(tk)

	createdBy, _, _ := tk.StringField("createdBy")
	if createdBy != "me@example.com" {
		t.Errorf("createdBy = %q, want 'me@example.com' (no angle form when only email is configured)", createdBy)
	}
}

func TestSetAuthorFromIdentityTiki_NameAndEmail_UsesAngleForm(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice", "alice@example.com")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	tk := tikipkg.New()
	s.setAuthorFromIdentityTiki(tk)

	createdBy, _, _ := tk.StringField("createdBy")
	if createdBy != "Alice <alice@example.com>" {
		t.Errorf("createdBy = %q, want 'Alice <alice@example.com>'", createdBy)
	}
}

func TestSetAuthorFromIdentityTiki_NameOnly_UsesName(t *testing.T) {
	isolateConfig(t)
	setIdentityEnv(t, "Alice", "")

	s := &TikiStore{identity: newIdentityResolver(nil)}

	tk := tikipkg.New()
	s.setAuthorFromIdentityTiki(tk)

	createdBy, _, _ := tk.StringField("createdBy")
	if createdBy != "Alice" {
		t.Errorf("createdBy = %q, want 'Alice'", createdBy)
	}
}

func TestBuildCustomFieldDefaults_NoCustomFields(t *testing.T) {
	workflow.ClearCustomFields()
	config.MarkRegistriesLoadedForTest()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	if defaults != nil {
		t.Errorf("expected nil map when no custom fields, got %v", defaults)
	}
}

func TestBuildCustomFieldDefaults_WithDefaults(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "severity", Type: workflow.TypeEnum, AllowedValues: []string{"low", "medium", "high"}, DefaultValue: "medium"},
		{Name: "blocked", Type: workflow.TypeBool, DefaultValue: false},
		{Name: "notes", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	if defaults == nil {
		t.Fatal("expected non-nil defaults map")
	}
	if v, ok := defaults["severity"]; !ok || v != "medium" {
		t.Errorf("severity = %v, want \"medium\"", v)
	}
	if v, ok := defaults["blocked"]; !ok || v != false {
		t.Errorf("blocked = %v, want false", v)
	}
	if _, ok := defaults["notes"]; ok {
		t.Error("notes should not have a default")
	}
}

func TestBuildCustomFieldDefaults_SliceDefaultCopied(t *testing.T) {
	config.MarkRegistriesLoadedForTest()
	original := []string{"a", "b", " a ", "b", ""}
	if err := workflow.RegisterCustomFields([]workflow.FieldDef{
		{Name: "labels", Type: workflow.TypeListString, DefaultValue: original},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(func() { workflow.ClearCustomFields() })

	defaults := buildCustomFieldDefaults()
	got, ok := defaults["labels"].([]string)
	if !ok {
		t.Fatal("expected []string default")
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("labels = %v, want [a b]", got)
	}
	// mutating the returned slice should not affect the original
	got[0] = "mutated"
	if original[0] == "mutated" {
		t.Error("returned slice should be a copy, not the original")
	}
}

// TestNewTikiTemplate_WorkflowCaptureWithDefaultStatus verifies that when the
// active workflow declares a `default: true` status, NewTikiTemplate produces
// a workflow-capable tiki with populated defaults.
func TestNewTikiTemplate_WorkflowCaptureWithDefaultStatus(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Default: true},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Default: true},
			{Key: "done", Label: "Done", Done: true},
		})
	})

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tmpl, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate: %v", err)
	}
	if !hasAnyWorkflowField(tmpl) {
		t.Error("no workflow fields, want workflow-capable tiki when default status is configured")
	}
	if status, _, _ := tmpl.StringField(tikipkg.FieldStatus); status != "backlog" {
		t.Errorf("Status = %q, want %q", status, "backlog")
	}
	if priority, _, _ := tmpl.IntField(tikipkg.FieldPriority); priority == 0 {
		t.Error("Priority = 0, want populated workflow default")
	}
}

// TestNewTikiTemplate_PlainCaptureWithoutDefaultStatus verifies the Phase 7
// semantics: a workflow with no `default: true` status signals "capture as
// plain document." NewTikiTemplate returns a bare tiki with no workflow field
// defaults; the piped/ruki capture paths then persist a plain .md with only
// id+title in the frontmatter.
func TestNewTikiTemplate_PlainCaptureWithoutDefaultStatus(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Default: true},
			{Key: "done", Label: "Done", Done: true},
		})
	})

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tmpl, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate: %v", err)
	}
	if hasAnyWorkflowField(tmpl) {
		t.Error("has workflow fields, want plain tiki when no default status configured")
	}
	if status, ok, _ := tmpl.StringField(tikipkg.FieldStatus); ok && status != "" {
		t.Errorf("Status = %q, want empty", status)
	}
	if priority, ok, _ := tmpl.IntField(tikipkg.FieldPriority); ok && priority != 0 {
		t.Errorf("Priority = %d, want 0 (no workflow defaults on plain docs)", priority)
	}
	if points, ok, _ := tmpl.IntField(tikipkg.FieldPoints); ok && points != 0 {
		t.Errorf("Points = %d, want 0 (no workflow defaults on plain docs)", points)
	}
	if tags, ok, _ := tmpl.StringSliceField(tikipkg.FieldTags); ok && len(tags) != 0 {
		t.Errorf("Tags = %v, want empty (no workflow defaults on plain docs)", tags)
	}
	if tmpl.ID == "" {
		t.Error("ID was not populated; plain capture must still generate an id")
	}
}

// TestCreateTiki_HonorsPlainTemplate verifies the end-to-end Phase 7 contract:
// a plain template produced by NewTikiTemplate (when no default status is
// configured) survives CreateTiki without being auto-promoted to a workflow
// item. This is the path piped capture uses.
func TestCreateTiki_HonorsPlainTemplate(t *testing.T) {
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "alpha", Label: "Alpha"},
		{Key: "done", Label: "Done", Done: true},
	})
	t.Cleanup(func() {
		config.ResetStatusRegistry([]workflow.StatusDef{
			{Key: "backlog", Label: "Backlog", Default: true},
			{Key: "done", Label: "Done", Done: true},
		})
	})

	s, err := NewTikiStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTikiStore: %v", err)
	}

	tmpl, err := s.NewTikiTemplate()
	if err != nil {
		t.Fatalf("NewTikiTemplate: %v", err)
	}
	tmpl.Title = "piped note"

	if err := s.CreateTiki(tmpl); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	stored := s.GetTiki(tmpl.ID)
	if stored == nil {
		t.Fatalf("GetTiki returned nil after CreateTiki(%s)", tmpl.ID)
	}
	if hasAnyWorkflowField(stored) {
		t.Error("hasAnyWorkflowField = true, want false — plain capture was promoted to workflow item")
	}
	// Phase 5: GetAllTikis includes plain docs; callers filter with has(status) / hasAnyWorkflowField.
	if got := len(s.GetAllTikis()); got != 1 {
		t.Errorf("GetAllTikis returned %d, want 1 (plain doc is included since Phase 5)", got)
	}
	if all := s.GetAllTikis(); hasAnyWorkflowField(all[0]) {
		t.Error("plain doc from GetAllTikis must not have any workflow fields")
	}
}

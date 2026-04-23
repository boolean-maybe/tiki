package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/workflow"
)

// setupLoadCustomFieldsTest creates temp dirs and configures the path manager
// so LoadCustomFields can discover workflow.yaml files.
func setupLoadCustomFieldsTest(t *testing.T) (cwdDir string) {
	t.Helper()
	workflow.ClearCustomFields()
	t.Cleanup(func() { workflow.ClearCustomFields() })

	userDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", userDir)
	if err := os.MkdirAll(filepath.Join(userDir, "tiki"), 0750); err != nil {
		t.Fatal(err)
	}

	projectDir := t.TempDir()
	docDir := filepath.Join(projectDir, ".doc")
	if err := os.MkdirAll(docDir, 0750); err != nil {
		t.Fatal(err)
	}

	cwdDir = t.TempDir()
	originalDir, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	_ = os.Chdir(cwdDir)

	ResetPathManager()
	pm := mustGetPathManager()
	pm.projectRoot = projectDir

	return cwdDir
}

func TestLoadCustomFields_BasicTypes(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: notes
    type: text
  - name: score
    type: integer
  - name: active
    type: boolean
  - name: startedAt
    type: datetime
  - name: labels
    type: stringList
  - name: related
    type: taskIdList
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name     string
		wantType workflow.ValueType
	}{
		{"notes", workflow.TypeString},
		{"score", workflow.TypeInt},
		{"active", workflow.TypeBool},
		{"startedAt", workflow.TypeTimestamp},
		{"labels", workflow.TypeListString},
		{"related", workflow.TypeListRef},
	}
	for _, c := range checks {
		f, ok := workflow.Field(c.name)
		if !ok {
			t.Errorf("Field(%q) not found", c.name)
			continue
		}
		if f.Type != c.wantType {
			t.Errorf("Field(%q).Type = %v, want %v", c.name, f.Type, c.wantType)
		}
		if !f.Custom {
			t.Errorf("Field(%q).Custom = false, want true", c.name)
		}
	}
}

func TestLoadCustomFields_EnumWithValues(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: severity
    type: enum
    values: [low, medium, high, critical]
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	f, ok := workflow.Field("severity")
	if !ok {
		t.Fatal("severity field not found")
	}
	if f.Type != workflow.TypeEnum {
		t.Errorf("severity.Type = %v, want TypeEnum", f.Type)
	}
	wantVals := []string{"low", "medium", "high", "critical"}
	if len(f.AllowedValues) != len(wantVals) {
		t.Fatalf("severity.AllowedValues length = %d, want %d", len(f.AllowedValues), len(wantVals))
	}
	for i, v := range wantVals {
		if f.AllowedValues[i] != v {
			t.Errorf("AllowedValues[%d] = %q, want %q", i, f.AllowedValues[i], v)
		}
	}
}

func TestLoadCustomFields_BadTypeRejected(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: broken
    type: nosuchtype
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadCustomFields()
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestLoadCustomFields_EnumWithoutValues(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	content := `
fields:
  - name: severity
    type: enum
`
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadCustomFields()
	if err == nil {
		t.Fatal("expected error for enum without values")
	}
}

func TestLoadCustomFields_HighestPriorityFileWins(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)
	pm := mustGetPathManager()

	// write fields in project config (lower priority)
	projectWorkflow := filepath.Join(pm.ProjectConfigDir(), "workflow.yaml")
	if err := os.WriteFile(projectWorkflow, []byte(`
fields:
  - name: score
    type: integer
`), 0644); err != nil {
		t.Fatal(err)
	}

	// write different fields in cwd (highest priority)
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(`
fields:
  - name: notes
    type: text
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// only cwd fields should be loaded
	if _, ok := workflow.Field("notes"); !ok {
		t.Error("expected 'notes' from cwd workflow")
	}
	if _, ok := workflow.Field("score"); ok {
		t.Error("expected 'score' from project workflow to NOT be loaded")
	}
}

func TestCoerceFieldDefault_ValidTypes(t *testing.T) {
	tests := []struct {
		name    string
		vt      workflow.ValueType
		raw     interface{}
		allowed []string
		want    interface{}
	}{
		{"string", workflow.TypeString, "hello", nil, "hello"},
		{"int", workflow.TypeInt, 42, nil, 42},
		{"int from float", workflow.TypeInt, float64(3), nil, 3},
		{"bool", workflow.TypeBool, false, nil, false},
		{"enum valid", workflow.TypeEnum, "medium", []string{"low", "medium", "high"}, "medium"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := coerceFieldDefault(tt.vt, tt.raw, tt.allowed)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCoerceFieldDefault_TimestampParsed(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want time.Time
	}{
		{"RFC3339", "2026-01-01T00:00:00Z", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"date only", "2026-06-15", time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := coerceFieldDefault(workflow.TypeTimestamp, tt.raw, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tv, ok := got.(time.Time)
			if !ok {
				t.Fatalf("expected time.Time, got %T", got)
			}
			if !tv.Equal(tt.want) {
				t.Errorf("got %v, want %v", tv, tt.want)
			}
		})
	}
}

func TestCoerceFieldDefault_TimestampRejectsInvalid(t *testing.T) {
	for _, raw := range []string{"not-a-date", "2026/01/01", "January 1"} {
		t.Run(raw, func(t *testing.T) {
			_, err := coerceFieldDefault(workflow.TypeTimestamp, raw, nil)
			if err == nil {
				t.Fatal("expected error for invalid timestamp")
			}
		})
	}
}

func TestCoerceFieldDefault_TaskIdListNormalized(t *testing.T) {
	raw := []interface{}{" tiki-abc ", "TIKI-DEF", "  ", "tiki-ghi"}
	got, err := coerceFieldDefault(workflow.TypeListRef, raw, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"TIKI-ABC", "TIKI-DEF", "TIKI-GHI"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCoerceFieldDefault_InvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		vt      workflow.ValueType
		raw     interface{}
		allowed []string
	}{
		{"string got int", workflow.TypeString, 42, nil},
		{"int got string", workflow.TypeInt, "hello", nil},
		{"int got fractional", workflow.TypeInt, float64(1.5), nil},
		{"bool got string", workflow.TypeBool, "yes", nil},
		{"enum not in list", workflow.TypeEnum, "unknown", []string{"low", "medium"}},
		{"enum got int", workflow.TypeEnum, 1, []string{"low"}},
		{"timestamp invalid", workflow.TypeTimestamp, "not-a-date", nil},
		{"timestamp wrong type", workflow.TypeTimestamp, 12345, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := coerceFieldDefault(tt.vt, tt.raw, tt.allowed)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestConvertCustomFieldDef_WithDefault(t *testing.T) {
	raw := customFieldYAML{
		Name:    "severity",
		Type:    "enum",
		Values:  []string{"low", "medium", "high"},
		Default: "medium",
	}
	fd, err := convertCustomFieldDef(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fd.DefaultValue != "medium" {
		t.Errorf("DefaultValue = %v, want \"medium\"", fd.DefaultValue)
	}
}

func TestConvertCustomFieldDef_InvalidDefault(t *testing.T) {
	raw := customFieldYAML{
		Name:    "severity",
		Type:    "enum",
		Values:  []string{"low", "medium", "high"},
		Default: "invalid",
	}
	_, err := convertCustomFieldDef(raw)
	if err == nil {
		t.Fatal("expected error for invalid default")
	}
}

func TestLoadCustomFields_MissingFieldsSection(t *testing.T) {
	cwdDir := setupLoadCustomFieldsTest(t)

	// workflow exists but has no fields: section
	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(`
statuses:
  - key: open
    label: Open
    default: true
  - key: done
    label: Done
    done: true
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := LoadCustomFields(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// no custom fields should be registered
	if _, ok := workflow.Field("score"); ok {
		t.Error("expected no custom fields when fields: section is missing")
	}
}

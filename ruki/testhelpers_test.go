package ruki

import (
	"time"

	"github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow/value"
)

// tikiFixture is a test-only fixture struct that mirrors the workflow-field
// shape ruki tests historically used. The runtime model (tiki.Tiki) keeps
// workflow fields in a generic Fields map; tests build fixtures against this
// struct to keep the (ID, Title, Status, Tags, …) literal style readable,
// then route them through tikiFromFixture at the executor boundary.
type tikiFixture struct {
	ID                  string
	Title               string
	Description         string
	Type                string
	Status              string
	Tags                []string
	DependsOn           []string
	Due                 time.Time
	Recurrence          value.Recurrence
	Assignee            string
	Points              int
	CreatedBy           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	FilePath            string
	IsWorkflow          bool
	CustomFields        map[string]interface{}
	WorkflowFrontmatter map[string]interface{}
}

// tikiFromFixture builds a tiki.Tiki from a fixture, applying the same
// presence semantics the old tikiFromTask helper used: a fixture with any
// non-zero workflow value (or IsWorkflow=true) is treated as workflow-
// declaring, with all schema fields explicitly set so formatters and filters
// behave the same as a tiki created via NewTikiTemplate.
func tikiFromFixture(f *tikiFixture) *tiki.Tiki {
	if f == nil {
		return nil
	}
	tk := tiki.New()
	tk.ID = f.ID
	tk.Title = f.Title
	tk.Body = f.Description
	tk.CreatedAt = f.CreatedAt
	tk.UpdatedAt = f.UpdatedAt
	tk.Path = f.FilePath
	if f.CreatedBy != "" {
		tk.Set("createdBy", f.CreatedBy)
	}

	workflow := f.IsWorkflow || hasAnyWorkflowValue(f)
	if workflow {
		// only set fields that are non-zero, mirroring setWorkflowFieldFromTask
		// behavior: absent zero-values must stay absent so has(field) returns false.
		if f.Status != "" {
			tk.Set(tiki.FieldStatus, f.Status)
		}
		if f.Type != "" {
			tk.Set(tiki.FieldType, f.Type)
		}
		if f.Points != 0 {
			tk.Set(tiki.FieldPoints, f.Points)
		}
		if f.Assignee != "" {
			tk.Set(tiki.FieldAssignee, f.Assignee)
		}
		if !f.Due.IsZero() {
			tk.Set(tiki.FieldDue, f.Due)
		}
		if f.Recurrence != "" {
			tk.Set(tiki.FieldRecurrence, string(f.Recurrence))
		}
		if f.Tags != nil {
			tk.Set(tiki.FieldTags, append([]string(nil), f.Tags...))
		}
		if f.DependsOn != nil {
			tk.Set(tiki.FieldDependsOn, append([]string(nil), f.DependsOn...))
		}
		// full-schema presence: initialize list fields to empty slice when explicitly nil
		// so formatters get "present empty" vs absent for their rendering decisions.
		if !tk.Has(tiki.FieldTags) {
			tk.Set(tiki.FieldTags, []string{})
		}
		if !tk.Has(tiki.FieldDependsOn) {
			tk.Set(tiki.FieldDependsOn, []string{})
		}
		if !tk.Has(tiki.FieldAssignee) {
			tk.Set(tiki.FieldAssignee, f.Assignee) // explicitly-set empty string
		}
		if !tk.Has(tiki.FieldDue) {
			tk.Set(tiki.FieldDue, f.Due) // zero time
		}
		if !tk.Has(tiki.FieldRecurrence) {
			tk.Set(tiki.FieldRecurrence, string(f.Recurrence)) // empty string
		}
		if !tk.Has(tiki.FieldPoints) {
			tk.Set(tiki.FieldPoints, f.Points) // zero int
		}
		if !tk.Has(tiki.FieldPriority) {
			tk.Set(tiki.FieldPriority, "") // explicit empty present
		}
	}
	for k, v := range f.CustomFields {
		tk.Set(k, v)
	}
	return tk
}

func hasAnyWorkflowValue(f *tikiFixture) bool {
	if f == nil {
		return false
	}
	return f.Status != "" || f.Type != "" || f.Points != 0 ||
		f.Tags != nil || f.DependsOn != nil || !f.Due.IsZero() ||
		f.Recurrence != "" || f.Assignee != ""
}

// tikisFromFixtures converts a fixture slice to tikis.
func tikisFromFixtures(fixtures []*tikiFixture) []*tiki.Tiki {
	out := make([]*tiki.Tiki, 0, len(fixtures))
	for _, f := range fixtures {
		if tk := tikiFromFixture(f); tk != nil {
			out = append(out, tk)
		}
	}
	return out
}

// testExecAction wraps TriggerExecutor.ExecAction so existing tests continue
// to read fixture-shaped results.
func (te *TriggerExecutor) testExecAction(trig any, tc *TriggerContext, inputs ...ExecutionInput) (*testResult, error) {
	r, err := te.ExecAction(trig, tc, inputs...)
	if err != nil {
		return nil, err
	}
	return wrapResult(r), nil
}

// testExecTimeAction wraps TriggerExecutor.ExecTimeTriggerAction with
// fixture-shaped input/output for legacy fixtures.
func (te *TriggerExecutor) testExecTimeAction(tt any, allFixtures []*tikiFixture, inputs ...ExecutionInput) (*testResult, error) {
	r, err := te.ExecTimeTriggerAction(tt, tikisFromFixtures(allFixtures), inputs...)
	if err != nil {
		return nil, err
	}
	return wrapResult(r), nil
}

// tikisToFixtures unwraps tikis back to tikiFixture for assertion ergonomics.
//
// Tests use local custom schemas (chooseTestSchema, newCustomExecutor's
// registered fields) that are NOT mirrored in the global workflow registry.
// This shim is deliberately permissive: every non-workflow-declared field
// lands in CustomFields so assertion ergonomics stay intact.
func tikisToFixtures(tks []*tiki.Tiki) []*tikiFixture {
	out := make([]*tikiFixture, 0, len(tks))
	for _, tk := range tks {
		if f := tikiToFixtureForTest(tk); f != nil {
			out = append(out, f)
		}
	}
	return out
}

func tikiToFixtureForTest(tk *tiki.Tiki) *tikiFixture {
	if tk == nil {
		return nil
	}
	createdBy := ""
	if s, _, _ := tk.StringField("createdBy"); s != "" {
		createdBy = s
	}
	f := &tikiFixture{
		ID:        tk.ID,
		Title:     tk.Title,
		CreatedBy: createdBy,
		CreatedAt: tk.CreatedAt,
		UpdatedAt: tk.UpdatedAt,
		FilePath:  tk.Path,
	}

	if v, ok, _ := tk.StringField(tiki.FieldStatus); ok {
		f.Status = v
	}
	if v, ok, _ := tk.StringField(tiki.FieldType); ok {
		f.Type = v
	}
	if v, ok, _ := tk.IntField(tiki.FieldPoints); ok {
		f.Points = v
	}
	if v, ok, _ := tk.StringField(tiki.FieldAssignee); ok {
		f.Assignee = v
	}
	if v, ok, _ := tk.TimeField(tiki.FieldDue); ok {
		f.Due = v
	}
	if v, ok, _ := tk.StringField(tiki.FieldRecurrence); ok {
		f.Recurrence = value.Recurrence(v)
	}
	if v, ok, _ := tk.StringSliceField(tiki.FieldTags); ok {
		f.Tags = v
	}
	if v, ok, _ := tk.StringSliceField(tiki.FieldDependsOn); ok {
		f.DependsOn = v
	}
	f.Description = tk.Body

	// IsWorkflow mirrors the old ToTask behavior: true when any of the
	// well-known kanban frontmatter keys is present in the tiki map.
	wellKnown := []string{
		tiki.FieldStatus, tiki.FieldType, tiki.FieldPriority, tiki.FieldPoints,
		tiki.FieldTags, tiki.FieldDependsOn, tiki.FieldDue, tiki.FieldRecurrence,
		tiki.FieldAssignee,
	}
	for _, fn := range wellKnown {
		if tk.Has(fn) {
			f.IsWorkflow = true
			break
		}
	}

	// collect all non-well-known fields into CustomFields for test assertions
	schemaSet := make(map[string]bool, len(wellKnown))
	for _, fn := range wellKnown {
		schemaSet[fn] = true
	}
	for k, v := range tk.Fields {
		if !schemaSet[k] {
			if f.CustomFields == nil {
				f.CustomFields = map[string]interface{}{}
			}
			f.CustomFields[k] = v
		}
	}
	return f
}

// testExec is a thin fixture-shaped wrapper around Executor.Execute: accepts
// []*tikiFixture for fixture compatibility, converts to tikis for the real
// call, and converts result-bearing variants back so existing test
// assertions keep working.
func (e *Executor) testExec(stmt any, fixtures []*tikiFixture, inputs ...ExecutionInput) (*testResult, error) {
	result, err := e.Execute(stmt, tikisFromFixtures(fixtures), inputs...)
	if err != nil {
		return nil, err
	}
	return wrapResult(result), nil
}

// testResult mirrors Result but exposes fixture-shaped fields so existing
// assertions that read Select.Tasks / Update.Updated[*].Status /
// Create.Task continue to compile without per-test rewrites.
type testResult struct {
	Select    *testSelect
	Update    *testUpdate
	Create    *testCreate
	Delete    *testDelete
	Pipe      *PipeResult
	Clipboard *ClipboardResult
	Scalar    *ScalarResult

	// raw carries the underlying tiki-shaped result for tests that want
	// to assert on Tiki.Fields directly.
	raw *Result
}

type testSelect struct {
	Tasks  []*tikiFixture
	Fields []string
}

type testUpdate struct {
	Updated []*tikiFixture
}

type testCreate struct {
	Task *tikiFixture
}

type testDelete struct {
	Deleted []*tikiFixture
}

func wrapResult(r *Result) *testResult {
	if r == nil {
		return nil
	}
	out := &testResult{
		Pipe:      r.Pipe,
		Clipboard: r.Clipboard,
		Scalar:    r.Scalar,
		raw:       r,
	}
	if r.Select != nil {
		out.Select = &testSelect{Tasks: tikisToFixtures(r.Select.Tikis), Fields: r.Select.Fields}
	}
	if r.Update != nil {
		out.Update = &testUpdate{Updated: tikisToFixtures(r.Update.Updated)}
	}
	if r.Create != nil {
		out.Create = &testCreate{Task: tikiToFixtureForTest(r.Create.Tiki)}
	}
	if r.Delete != nil {
		out.Delete = &testDelete{Deleted: tikisToFixtures(r.Delete.Deleted)}
	}
	return out
}

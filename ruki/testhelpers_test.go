package ruki

import (
	"time"

	"github.com/boolean-maybe/tiki/ruki/recurrence"
)

// well-known kanban field names used by the fixture helpers. Kept as test-local
// constants so the in-package ruki tests do not import the tiki package (which
// would create an import cycle now that tiki imports ruki).
const (
	fixtureFieldStatus     = "status"
	fixtureFieldType       = "type"
	fixtureFieldPriority   = "priority"
	fixtureFieldPoints     = "points"
	fixtureFieldTags       = "tags"
	fixtureFieldDependsOn  = "dependsOn"
	fixtureFieldDue        = "due"
	fixtureFieldRecurrence = "recurrence"
	fixtureFieldAssignee   = "assignee"
)

// tikiFixture is a test-only fixture struct that mirrors the workflow-field
// shape ruki tests historically used. The runtime model keeps workflow fields
// in a generic field map; tests build fixtures against this struct to keep the
// (ID, Title, Status, Tags, …) literal style readable, then route them through
// tikiFromFixture at the executor boundary.
type tikiFixture struct {
	ID                  string
	Title               string
	Description         string
	Type                string
	Status              string
	Tags                []string
	DependsOn           []string
	Due                 time.Time
	Recurrence          recurrence.Recurrence
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

// docPriority returns the priority field off d as a string, or "" when absent
// or non-string. Test-only convenience for assertions on the created/updated
// document's priority recurrence.
func docPriority(d Document) string {
	v, ok := d.Get("priority")
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// testDocFactory returns the DocumentFactory the in-package ruki tests pass to
// NewExecutor / NewTriggerExecutor. It mints a blank fakeDoc, the same shape
// tikiFromFixture produces.
func testDocFactory() DocumentFactory {
	return func() Document { return newFakeDoc() }
}

// tikiFromFixture builds a Document from a fixture, applying the same presence
// semantics the old helper used: a fixture with any non-zero workflow value (or
// IsWorkflow=true) is treated as workflow-declaring, with all schema fields
// explicitly set so formatters and filters behave the same as a tiki created
// via NewTikiTemplate.
func tikiFromFixture(f *tikiFixture) Document {
	if f == nil {
		return nil
	}
	tk := newFakeDoc()
	tk.id = f.ID
	tk.title = f.Title
	tk.body = f.Description
	tk.createdAt = f.CreatedAt
	tk.updatedAt = f.UpdatedAt
	tk.path = f.FilePath
	if f.CreatedBy != "" {
		tk.Set("createdBy", f.CreatedBy)
	}

	if f.IsWorkflow || hasAnyWorkflowValue(f) {
		applyWorkflowFields(tk, f)
	}
	for k, v := range f.CustomFields {
		tk.Set(k, v)
	}
	return tk
}

// applyWorkflowFields sets the full schema-presence field set on tk, mirroring
// setWorkflowFieldFromTiki: non-zero values are set directly; absent fields are
// initialized to their explicit-present zero so has(field) stays consistent.
func applyWorkflowFields(tk *fakeDoc, f *tikiFixture) {
	if f.Status != "" {
		tk.Set(fixtureFieldStatus, f.Status)
	}
	if f.Type != "" {
		tk.Set(fixtureFieldType, f.Type)
	}
	if f.Points != 0 {
		tk.Set(fixtureFieldPoints, f.Points)
	}
	if f.Assignee != "" {
		tk.Set(fixtureFieldAssignee, f.Assignee)
	}
	if !f.Due.IsZero() {
		tk.Set(fixtureFieldDue, f.Due)
	}
	if f.Recurrence != "" {
		tk.Set(fixtureFieldRecurrence, string(f.Recurrence))
	}
	if f.Tags != nil {
		tk.Set(fixtureFieldTags, append([]string(nil), f.Tags...))
	}
	if f.DependsOn != nil {
		tk.Set(fixtureFieldDependsOn, append([]string(nil), f.DependsOn...))
	}
	// full-schema presence: initialize list fields to empty slice when
	// explicitly nil so formatters get "present empty" vs absent semantics.
	if !tk.Has(fixtureFieldTags) {
		tk.Set(fixtureFieldTags, []string{})
	}
	if !tk.Has(fixtureFieldDependsOn) {
		tk.Set(fixtureFieldDependsOn, []string{})
	}
	if !tk.Has(fixtureFieldAssignee) {
		tk.Set(fixtureFieldAssignee, f.Assignee) // explicitly-set empty string
	}
	if !tk.Has(fixtureFieldDue) {
		tk.Set(fixtureFieldDue, f.Due) // zero time
	}
	if !tk.Has(fixtureFieldRecurrence) {
		tk.Set(fixtureFieldRecurrence, string(f.Recurrence)) // empty string
	}
	if !tk.Has(fixtureFieldPoints) {
		tk.Set(fixtureFieldPoints, f.Points) // zero int
	}
	if !tk.Has(fixtureFieldPriority) {
		tk.Set(fixtureFieldPriority, "") // explicit empty present
	}
}

func hasAnyWorkflowValue(f *tikiFixture) bool {
	if f == nil {
		return false
	}
	return f.Status != "" || f.Type != "" || f.Points != 0 ||
		f.Tags != nil || f.DependsOn != nil || !f.Due.IsZero() ||
		f.Recurrence != "" || f.Assignee != ""
}

// tikisFromFixtures converts a fixture slice to documents.
func tikisFromFixtures(fixtures []*tikiFixture) []Document {
	out := make([]Document, 0, len(fixtures))
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

// tikisToFixtures unwraps documents back to tikiFixture for assertion
// ergonomics.
//
// Tests use local custom schemas (chooseTestSchema, newCustomExecutor's
// registered fields) that are NOT mirrored in the global workflow registry.
// This shim is deliberately permissive: every non-workflow-declared field
// lands in CustomFields so assertion ergonomics stay intact.
func tikisToFixtures(tks []Document) []*tikiFixture {
	out := make([]*tikiFixture, 0, len(tks))
	for _, tk := range tks {
		if f := tikiToFixtureForTest(tk); f != nil {
			out = append(out, f)
		}
	}
	return out
}

func tikiToFixtureForTest(tk Document) *tikiFixture {
	if tk == nil {
		return nil
	}
	f := &tikiFixture{
		ID:          tk.ID(),
		Title:       tk.Title(),
		CreatedBy:   fixtureString(tk, "createdBy"),
		CreatedAt:   tk.CreatedAt(),
		UpdatedAt:   tk.UpdatedAt(),
		FilePath:    tk.Path(),
		Description: tk.Body(),
	}

	f.Status = fixtureString(tk, fixtureFieldStatus)
	f.Type = fixtureString(tk, fixtureFieldType)
	if v, ok := fixtureInt(tk, fixtureFieldPoints); ok {
		f.Points = v
	}
	f.Assignee = fixtureString(tk, fixtureFieldAssignee)
	if v, ok := fixtureTime(tk, fixtureFieldDue); ok {
		f.Due = v
	}
	f.Recurrence = recurrence.Recurrence(fixtureString(tk, fixtureFieldRecurrence))
	if v, ok := fixtureStringSlice(tk, fixtureFieldTags); ok {
		f.Tags = v
	}
	if v, ok := fixtureStringSlice(tk, fixtureFieldDependsOn); ok {
		f.DependsOn = v
	}

	wellKnown := []string{
		fixtureFieldStatus, fixtureFieldType, fixtureFieldPriority, fixtureFieldPoints,
		fixtureFieldTags, fixtureFieldDependsOn, fixtureFieldDue, fixtureFieldRecurrence,
		fixtureFieldAssignee,
	}
	for _, fn := range wellKnown {
		if tk.Has(fn) {
			f.IsWorkflow = true
			break
		}
	}

	collectCustomFields(tk, f, wellKnown)
	return f
}

// collectCustomFields copies every field not in the well-known kanban set into
// f.CustomFields so test assertions can inspect them. Reads through the
// fixture-only fakeDoc's backing map.
func collectCustomFields(tk Document, f *tikiFixture, wellKnown []string) {
	fd, ok := tk.(*fakeDoc)
	if !ok {
		return
	}
	schemaSet := make(map[string]bool, len(wellKnown))
	for _, fn := range wellKnown {
		schemaSet[fn] = true
	}
	for k, v := range fd.fields {
		if k == "createdBy" || schemaSet[k] {
			continue
		}
		if f.CustomFields == nil {
			f.CustomFields = map[string]interface{}{}
		}
		f.CustomFields[k] = v
	}
}

// fixtureString reads name off tk as a string, returning "" when absent or not
// a string.
func fixtureString(tk Document, name string) string {
	v, ok := tk.Get(name)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func fixtureInt(tk Document, name string) (int, bool) {
	v, ok := tk.Get(name)
	if !ok {
		return 0, false
	}
	n, ok := v.(int)
	return n, ok
}

func fixtureTime(tk Document, name string) (time.Time, bool) {
	v, ok := tk.Get(name)
	if !ok {
		return time.Time{}, false
	}
	t, ok := v.(time.Time)
	return t, ok
}

func fixtureStringSlice(tk Document, name string) ([]string, bool) {
	v, ok := tk.Get(name)
	if !ok {
		return nil, false
	}
	switch ss := v.(type) {
	case []string:
		return ss, true
	case []interface{}:
		out := make([]string, 0, len(ss))
		for _, e := range ss {
			s, _ := e.(string)
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

// testExec is a thin fixture-shaped wrapper around Executor.Execute: accepts
// []*tikiFixture for fixture compatibility, converts to documents for the real
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
// assertions that read Select.Tikis / Update.Updated[*].Status /
// Create.Tiki continue to compile without per-test rewrites.
type testResult struct {
	Select    *testSelect
	Update    *testUpdate
	Create    *testCreate
	Delete    *testDelete
	Pipe      *PipeResult
	Clipboard *ClipboardResult
	Scalar    *ScalarResult

	// raw carries the underlying document-shaped result for tests that want
	// to assert on the field map directly.
	raw *Result
}

type testSelect struct {
	Tikis  []*tikiFixture
	Fields []string
}

type testUpdate struct {
	Updated []*tikiFixture
}

type testCreate struct {
	Tiki *tikiFixture
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
		out.Select = &testSelect{Tikis: tikisToFixtures(r.Select.Tikis), Fields: r.Select.Fields}
	}
	if r.Update != nil {
		out.Update = &testUpdate{Updated: tikisToFixtures(r.Update.Updated)}
	}
	if r.Create != nil {
		out.Create = &testCreate{Tiki: tikiToFixtureForTest(r.Create.Tiki)}
	}
	if r.Delete != nil {
		out.Delete = &testDelete{Deleted: tikisToFixtures(r.Delete.Deleted)}
	}
	return out
}

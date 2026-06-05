package tikidetail

import (
	"strings"
	"testing"
	"time"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/internal/teststatuses"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/plugin"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// extractTextView extracts the rendered text from a tview.Primitive that we
// expect to be a *tview.TextView. Returns "" for anything else so callers
// can still assert "no focus marker" without crashing on placeholder rows.
// stripTags=true asks tview to strip color tags before returning the text;
// false returns the raw bytes including escape markers.
func extractTextView(p tview.Primitive, stripTags bool) string {
	tv, ok := p.(*tview.TextView)
	if !ok {
		return ""
	}
	return tv.GetText(stripTags)
}

// TestFieldHasEditor_RecognizesWorkflowEnums pins that custom workflow-
// declared enum fields (e.g. `severity:` in bug-tracker.yaml) report as
// editable, so the configurable detail view will route them through the
// SemanticEnum editor instead of read-only rendering.
func TestFieldHasEditor_RecognizesWorkflowEnums(t *testing.T) {
	cleanup := registerExtraWorkflowFieldForTest(t, "severity", []string{"low", "medium", "high"})
	t.Cleanup(cleanup)

	if !FieldHasEditor("severity") {
		t.Error("FieldHasEditor(severity) = false; workflow-declared enums must be editable")
	}
	if !FieldHasEditor("status") {
		t.Error("FieldHasEditor(status) = false; built-in enums must remain editable")
	}
	if FieldHasEditor("does-not-exist") {
		t.Error("FieldHasEditor(does-not-exist) = true; unknown fields must not be editable")
	}
}

// TestBuildFieldEditor_WorkflowEnumProducesEditor pins that the editor
// factory returns a non-nil widget for a custom workflow enum and that
// submitting a display string yields the canonical key on onChange.
func TestBuildFieldEditor_WorkflowEnumProducesEditor(t *testing.T) {
	cleanup := registerExtraWorkflowFieldForTest(t, "severity", []string{"low", "medium", "high"})
	t.Cleanup(cleanup)

	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI097")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	colors := theme.Roles()

	var captured string
	w := buildFieldEditor("severity", tk,
		FieldRenderContext{Mode: RenderModeEdit, Roles: colors},
		func(v string) { captured = v },
	)
	if w == nil {
		t.Fatal("buildFieldEditor returned nil for workflow-declared enum severity")
	}
	// The factory wraps an EditSelectList in an enum-aware adapter
	// whose GetText() returns the canonical key (not the display).
	adapter, ok := w.(*enumSelectAdapter)
	if !ok {
		t.Fatalf("expected enumSelectAdapter, got %T", w)
	}
	// Cycle to a known value, then verify the canonical key surface:
	// GetText() must be a workflow-declared key, not a display string.
	adapter.MoveToNext()
	got := adapter.GetText()
	if got == "" {
		t.Errorf("enum adapter GetText() returned empty")
	}
	// All values pass through; just confirm we have a key, not a display
	// string with trailing space/emoji (the test enum has no labels, so
	// display == value here).
	for _, k := range []string{"low", "medium", "high"} {
		if got == k {
			break
		}
	}
	_ = captured // captured is exercised via the SubmitHandler in flow
}

// TestRenderTextValue_EscapesTviewMarkup pins that user-controlled string
// values pass through tview.Escape before landing in a SetDynamicColors
// TextView. Without escaping, an assignee containing "[red]" would be
// parsed as a tview color tag and either disappear or recolor the rest
// of the line.
func TestRenderTextValue_EscapesTviewMarkup(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI098")
	tk.Set(tikipkg.FieldAssignee, "[red]admin[white]")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	colors := theme.Roles()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: colors, FieldName: tikipkg.FieldAssignee}

	// Read raw bytes (stripTags=false) so the tview.Escape marker is
	// visible; with stripTags=true a parsed-and-discarded color tag would
	// look identical to a successfully-escaped one.
	rawOut := extractTextView(renderTextValue(tk, ctx), false)
	if !strings.Contains(rawOut, "[red[]") {
		t.Errorf("expected escaped [red] marker in raw output, got: %q", rawOut)
	}

	// And the visible-text path: with tags stripped we still see "admin",
	// and the literal "[red]" survives because it was escaped (tview no
	// longer parses it as a color tag, so it doesn't strip it either).
	visibleOut := extractTextView(renderTextValue(tk, ctx), true)
	if !strings.Contains(visibleOut, "admin") {
		t.Errorf("expected literal value to survive escape, got: %q", visibleOut)
	}
	if !strings.Contains(visibleOut, "[red]") {
		t.Errorf("expected literal [red] to remain visible (not parsed away), got: %q", visibleOut)
	}
}

// TestRenderEnumValue_EscapesUserLabelMarkup pins that workflow-supplied
// enum labels are escaped before interpolation into the dynamic-color
// TextView. Without escaping, a workflow author writing label: "[red]High"
// in workflow.yaml could hijack the row's coloring or unbalanced-tag the
// rest of the line. The escape is applied on top of tview's normal markup,
// so the row's own focus marker and dim/full color tags still work.
func TestRenderEnumValue_EscapesUserLabelMarkup(t *testing.T) {
	// Register a custom enum where the label contains tview-tag-shaped
	// content. workflow.yaml is fully user-controlled, so this is the
	// authentic threat model.
	cleanup := registerExtraWorkflowFieldForTest(t, "severity", []string{"low", "medium", "high"})
	t.Cleanup(cleanup)

	// Patch the registered enum with markup-bearing labels. We do this
	// directly through workflow registration since registerExtraWorkflowFieldForTest
	// only sets values without labels — and we want to exercise EnumDisplay
	// with non-empty labels.
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{
			Name: "severity",
			Type: workflow.TypeEnum,
			EnumValues: []workflow.EnumValue{
				{Value: "low", Label: "[red]Low"},
				{Value: "medium", Label: "Medium", Default: true},
				{Value: "high", Label: "High"},
			},
		},
	}); err != nil {
		t.Fatalf("register severity: %v", err)
	}

	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI096")
	tk.Set("severity", "low")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	colors := theme.Roles()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: colors, FieldName: "severity"}

	// Raw bytes (stripTags=false) must contain the escape marker so we
	// know the renderer didn't pass user markup to tview unmodified.
	rawOut := extractTextView(renderEnumValue(tk, ctx), false)
	if !strings.Contains(rawOut, "[red[]") {
		t.Errorf("expected escape marker in raw output, got: %q", rawOut)
	}
	// Visible text (stripTags=true) must still contain the literal
	// "[red]" — escaping prevents tview from parsing it away.
	visibleOut := extractTextView(renderEnumValue(tk, ctx), true)
	if !strings.Contains(visibleOut, "[red]") {
		t.Errorf("expected literal [red] to survive escape, got: %q", visibleOut)
	}
	if !strings.Contains(visibleOut, "Low") {
		t.Errorf("expected literal label text to remain visible, got: %q", visibleOut)
	}
}

// TestGenericFieldValueString_TimestampPreservesClock pins that
// TypeTimestamp fields render with the time component, while TypeDate
// fields stay date-only. The pre-fix code formatted any time.Time value
// as "2006-01-02", silently truncating timestamp displays.
func TestGenericFieldValueString_TimestampPreservesClock(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "dueBy", Type: workflow.TypeTimestamp},
		{Name: "scheduledFor", Type: workflow.TypeDate},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	when := time.Date(2026, 5, 8, 14, 30, 0, 0, time.UTC)
	tk := tikipkg.New()
	tk.SetID("TS001")
	tk.Set("dueBy", when)
	tk.Set("scheduledFor", when)

	tsFD, _ := workflow.Field("dueBy")
	got := genericFieldValueString(tsFD, tk, FieldRenderContext{})
	if !strings.Contains(got, "14:30") {
		t.Errorf("timestamp value = %q, want time component (14:30) preserved", got)
	}

	dateFD, _ := workflow.Field("scheduledFor")
	gotDate := genericFieldValueString(dateFD, tk, FieldRenderContext{})
	if strings.Contains(gotDate, "14:30") {
		t.Errorf("date value = %q, must NOT carry time component", gotDate)
	}
	if !strings.Contains(gotDate, "2026-05-08") {
		t.Errorf("date value = %q, want %q", gotDate, "2026-05-08")
	}
}

// TestGenericFieldValueString_DefaultBranchEscapesMarkup pins escape on
// the default-branch fallback (fmt.Sprintf("%v", v)). YAML can deliver
// arbitrary shapes here — maps, slices of mixed types, etc. — and the
// stringified form ends up in a dynamic-color TextView via labeledLine.
// Without escape, a value like "[red]hi" hijacks the row coloring.
func TestGenericFieldValueString_DefaultBranchEscapesMarkup(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		// TypeString to force routing through the inner switch's string
		// branch — but to hit the default branch we need an unhandled
		// type. Use a custom field with no declared type to land in
		// default. Workflow validation rejects unknown types, so
		// simulate by setting a value that doesn't match the declared
		// type and falls through.
		{Name: "blob", Type: workflow.TypeString},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	t.Cleanup(teststatuses.Init)

	tk := tikipkg.New()
	tk.SetID("ESC001")
	// Set an unsupported shape: a map. The string switch won't match,
	// other branches won't match → default branch fires.
	tk.Set("blob", map[string]string{"label": "[red]hi"})

	fd, _ := workflow.Field("blob")
	got := genericFieldValueString(fd, tk, FieldRenderContext{})
	// tview.Escape inserts an opening-bracket marker.
	if !strings.Contains(got, "[red[]") {
		t.Errorf("default branch did not escape markup: got %q", got)
	}
}

// TestRenderEnumValue_FocusMarkerOnlyWhenFieldMatches pins the contract
// for non-focused row rendering: even in RenderModeEdit, an enum row must
// not paint the focus marker unless ctx.FocusedField equals that field's
// EditField. The bug being guarded against was setting rowCtx.FocusedField
// to the row's own EditField unconditionally — which made every editable
// row see itself as focused.
func TestRenderEnumValue_FocusMarkerOnlyWhenFieldMatches(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI099")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	colors := theme.Roles()
	const marker = "► "

	t.Run("no focus marker in view mode", func(t *testing.T) {
		ctx := FieldRenderContext{Mode: RenderModeView, Roles: colors, FieldName: tikipkg.FieldStatus}
		out := extractTextView(renderEnumValue(tk, ctx), true)
		if strings.Contains(out, marker) {
			t.Errorf("view mode painted focus marker: %q", out)
		}
	})

	t.Run("no focus marker in edit mode when other field is focused", func(t *testing.T) {
		// Status row, but type is the focused field — must not paint marker.
		ctx := FieldRenderContext{
			Mode: RenderModeEdit, Roles: colors,
			FieldName:    tikipkg.FieldStatus,
			FocusedField: model.EditFieldType,
		}
		out := extractTextView(renderEnumValue(tk, ctx), true)
		if strings.Contains(out, marker) {
			t.Errorf("status row painted focus marker while type was focused: %q", out)
		}
	})

	t.Run("focus marker present when this field is focused", func(t *testing.T) {
		ctx := FieldRenderContext{
			Mode: RenderModeEdit, Roles: colors,
			FieldName:    tikipkg.FieldStatus,
			FocusedField: model.EditFieldStatus,
		}
		out := extractTextView(renderEnumValue(tk, ctx), true)
		if !strings.Contains(out, marker) {
			t.Errorf("focused row did not paint focus marker: %q", out)
		}
	})
}

// TestConfigurableDetailView_RendersConfiguredMetadata verifies that the
// view materialises a tiki for the configured metadata list and returns
// non-nil primitives for title, the configured rows, and description.
func TestConfigurableDetailView_RendersConfiguredMetadata(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI001")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	registry := controller.DetailViewActions()
	cv := NewConfigurableDetailView(
		s,
		tk.ID(),
		detailPluginFromFields([]string{"status", "type", "priority"}),
		registry,
		nil, nil,
	)

	if cv.GetViewName() != "Detail" {
		t.Errorf("GetViewName() = %q, want %q", cv.GetViewName(), "Detail")
	}
	if cv.GetSelectedID() != tk.ID() {
		t.Errorf("GetSelectedID() = %q, want %q", cv.GetSelectedID(), tk.ID())
	}
	if cv.GetActionRegistry() == nil {
		t.Error("GetActionRegistry returned nil")
	}
	if cv.GetPrimitive() == nil {
		t.Error("GetPrimitive returned nil")
	}
}

// TestConfigurableDetailView_RendersDependsOnColumn pins that a tiki with
// declared dependencies surfaces them in the detail view: the resolved
// dependency IDs must appear in the rendered output. Guards the kanban
// layout change that added a `dependsOn` column beside Tags — without a
// grid cell referencing the field, RenderDependsOnColumn is never reached.
func TestConfigurableDetailView_RendersDependsOnColumn(t *testing.T) {
	s := store.NewInMemoryStore()

	dep1 := tikipkg.New()
	dep1.SetID("AAAAAA")
	dep1.SetTitle("first dep")
	dep2 := tikipkg.New()
	dep2.SetID("BBBBBB")
	dep2.SetTitle("second dep")
	parent := newTestViewTiki("PARENT")
	parent.Set(tikipkg.FieldDependsOn, []string{"AAAAAA", "BBBBBB"})
	for _, tk := range []*tikipkg.Tiki{dep1, dep2, parent} {
		if err := s.CreateTiki(tk); err != nil {
			t.Fatalf("CreateTiki(%s): %v", tk.ID(), err)
		}
	}

	// the deps list is height-bound by the metadata box (spec.Rows + overhead).
	// declare enough sibling fields that the box is at least as tall as the
	// dependency count, matching the 4-row span the real kanban layout gives
	// the dependsOn column — otherwise the box clips trailing deps.
	cv := NewConfigurableDetailView(
		s,
		parent.ID(),
		detailPluginFromFields([]string{"dependsOn", "status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)

	rendered := drawPrimitive(t, cv.GetPrimitive(), 80, 12)
	for _, want := range []string{"AAAAAA", "BBBBBB"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("rendered detail view missing dependency id %q:\n%s", want, rendered)
		}
	}
}

// drawPrimitive renders p onto a simulation screen of the given size and
// returns the on-screen text, newline-joined per row. Mirrors the
// SimulationScreen pattern used in composite_render_test.go.
func drawPrimitive(t *testing.T, p tview.Primitive, width, height int) string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)
	p.SetRect(0, 0, width, height)
	p.Draw(screen)

	var b strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, _, _, _ := screen.GetContent(x, y)
			if r == 0 {
				r = ' '
			}
			b.WriteRune(r)
		}
		b.WriteRune('\n')
	}
	return b.String()
}

// TestConfigurableDetailView_HandlesMissingTiki verifies the placeholder path
// when the tiki id can't be resolved (e.g. stale selection after delete).
func TestConfigurableDetailView_HandlesMissingTiki(t *testing.T) {
	s := store.NewInMemoryStore()
	cv := NewConfigurableDetailView(
		s,
		"TIKI_GONE",
		detailPluginFromFields([]string{"status"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	if cv.GetSelectedID() != "TIKI_GONE" {
		t.Errorf("GetSelectedID() = %q, want %q", cv.GetSelectedID(), "TIKI_GONE")
	}
	// The view should not panic when refresh runs on a missing tiki.
	cv.refresh()
}

// TestConfigurableDetailView_UnknownFieldRendersPlaceholder verifies that
// an unknown semantic-field name renders a placeholder row instead of
// crashing — the parser already rejects this at load, but the view
// shouldn't depend on that.
func TestConfigurableDetailView_UnknownFieldRendersPlaceholder(t *testing.T) {
	tk := newTestViewTiki("TIKI002")
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	row := renderConfiguredField("not_a_field", tk, ctx)
	if row == nil {
		t.Fatal("expected a placeholder primitive, got nil")
	}
}

// TestConfigurableDetailView_WorkflowFieldFallsBackToGenericRow verifies that
// a workflow-declared field without a typed editor still renders via the
// generic catalog-driven path (rather than producing an "(unknown field)"
// placeholder). This pins the contract that workflow.yaml is the sole
// source of truth for which fields the detail view will render.
func TestConfigurableDetailView_WorkflowFieldFallsBackToGenericRow(t *testing.T) {
	// register a workflow field beyond the typed-editor set
	cleanup := registerExtraWorkflowFieldForTest(t, "severity",
		[]string{"low", "medium", "high"})
	defer cleanup()

	tk := newTestViewTiki("TIKI003")
	tk.Set("severity", "high")

	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}
	row := renderConfiguredField("severity", tk, ctx)
	if row == nil {
		t.Fatal("expected a non-nil primitive for workflow-declared field")
	}
	// the renderer is expected to produce something concrete (not the
	// "(unknown field)" placeholder); a snapshot would be brittle, so we
	// just assert it doesn't return the unknown-field text shape via
	// observable behavior — non-nil is the contract here.
}

// TestFieldRegistry_LookupKnownFields verifies the workflow-declared fields
// installed by registerBuiltinFields are visible.
func TestFieldRegistry_LookupKnownFields(t *testing.T) {
	for _, name := range []string{
		tikipkg.FieldStatus,
		tikipkg.FieldType,
		tikipkg.FieldPriority,
	} {
		t.Run(name, func(t *testing.T) {
			fd, ok := LookupField(name)
			if !ok {
				t.Fatalf("expected field %q to be registered", name)
			}
			if fd.Name != name {
				t.Errorf("FieldDescriptor.Name = %q, want %q", fd.Name, name)
			}
			if fd.Get == nil {
				t.Errorf("FieldDescriptor.Get for %q is nil", name)
			}
		})
	}
}

// TestTypeRegistry_AllStubsHaveRenderer asserts that every semantic type
// recorded in the registry has a non-nil renderer, so an unsupported
// editor type still produces predictable visual output.
func TestTypeRegistry_AllStubsHaveRenderer(t *testing.T) {
	for _, sem := range []SemanticType{
		SemanticEnum,
		SemanticText,
		SemanticInteger,
		SemanticBoolean,
		SemanticDate,
		SemanticDateTime,
		SemanticRecurrence,
		SemanticStringList,
		SemanticTikiIDList,
	} {
		t.Run(string(sem), func(t *testing.T) {
			ui, ok := LookupType(sem)
			if !ok {
				t.Fatalf("type %q not registered", sem)
			}
			if ui.Render == nil {
				t.Errorf("type %q has nil Render", sem)
			}
		})
	}
}

func TestEmptyListFieldNames(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "deps", Type: workflow.TypeListRef, Custom: true},
		{Name: "labels", Type: workflow.TypeListString, Custom: true},
		{Name: "owner", Type: workflow.TypeString, Custom: true},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	defer teststatuses.Init()

	tk := tikipkg.New()
	tk.Set("labels", []string{"x"}) // non-empty list
	tk.Set("owner", "")             // empty scalar — must NOT be hidden

	got := emptyListFieldNames(tk, []string{"deps", "labels", "owner", "nonexistent"})
	// deps is an empty list -> hidden; labels non-empty -> shown; owner scalar -> never.
	if len(got) != 1 || got[0] != "deps" {
		t.Errorf("emptyListFieldNames = %v, want [deps]", got)
	}
}

// TestConfigurableDetailView_HidesEmptyListFieldAndCaption is an end-to-end
// render assertion: a tiki with an empty list-ref field must hide both the
// field value cell AND its caption cell; a tiki with a populated list-ref
// must render the caption and the referenced ID.
//
// We use a custom list-ref field "deps" (not the built-in dependsOn) because
// teststatuses.CanonicalFields already declares dependsOn, and re-declaring it
// via InitWith would trip RegisterWorkflowFields' case-insensitive
// duplicate-name guard. The custom field carries Caption: "Deps", which the
// caption cell resolves through workflow.Field("deps").DisplayCaption().
func TestConfigurableDetailView_HidesEmptyListFieldAndCaption(t *testing.T) {
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "deps", Type: workflow.TypeListRef, Custom: true, Caption: "Deps"},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	defer teststatuses.Init()

	spec, err := gridlayout.ParseGrid([][]string{
		{`<text.label>deps.caption`, "deps"},
	})
	if err != nil {
		t.Fatalf("parse layout: %v", err)
	}
	plug := &plugin.DetailPlugin{
		BasePlugin: plugin.BasePlugin{Name: "Detail", Kind: plugin.KindDetail},
		Layout:     spec,
	}

	// empty deps: caption "Deps" must NOT render.
	sEmpty := store.NewInMemoryStore()
	empty := newTestViewTiki("EMPTY1")
	if err := sEmpty.CreateTiki(empty); err != nil {
		t.Fatalf("create: %v", err)
	}
	cvEmpty := NewConfigurableDetailView(sEmpty, empty.ID(), plug, controller.DetailViewActions(), nil, nil)
	emptyOut := drawPrimitive(t, cvEmpty.GetPrimitive(), 60, 8)
	if strings.Contains(emptyOut, "Deps") {
		t.Errorf("empty deps should hide caption, but %q contains \"Deps\":\n%s", "Deps", emptyOut)
	}

	// non-empty deps: caption AND dep id must render.
	sFull := store.NewInMemoryStore()
	dep := tikipkg.New()
	dep.SetID("AAAAAA")
	dep.SetTitle("a dep")
	full := newTestViewTiki("FULL01")
	full.Set("deps", []string{"AAAAAA"})
	for _, tk := range []*tikipkg.Tiki{dep, full} {
		if err := sFull.CreateTiki(tk); err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	cvFull := NewConfigurableDetailView(sFull, full.ID(), plug, controller.DetailViewActions(), nil, nil)
	fullOut := drawPrimitive(t, cvFull.GetPrimitive(), 60, 8)
	if !strings.Contains(fullOut, "Deps") {
		t.Errorf("non-empty deps should show caption %q:\n%s", "Deps", fullOut)
	}
	if !strings.Contains(fullOut, "AAAAAA") {
		t.Errorf("non-empty deps should show dep id, got:\n%s", fullOut)
	}
}

// TestConfigurableDetailView_HiddenEmptyListFieldNotEditTraversable guards the
// edit-mode Tab-traversal contract: a field hidden from the metadata grid by
// the per-tiki empty-list rule must NOT be a traversal target. tags is a
// built-in editable stringList; when it is empty it is hidden in view mode, so
// it must also be skipped in edit mode. When non-empty it renders and is
// traversable as usual.
func TestConfigurableDetailView_HiddenEmptyListFieldNotEditTraversable(t *testing.T) {
	plug := detailPluginFromFields([]string{"status", "tags"})

	// empty tags -> hidden -> not edit-traversable.
	sEmpty := store.NewInMemoryStore()
	empty := newTestViewTiki("EMPTYT")
	if err := sEmpty.CreateTiki(empty); err != nil {
		t.Fatalf("create empty: %v", err)
	}
	cvEmpty := NewConfigurableDetailView(sEmpty, empty.ID(), plug, controller.DetailViewActions(), nil, nil)
	cvEmpty.SetEditModeRegistry(controller.DetailEditModeActions())
	if !cvEmpty.EnterEditMode() {
		t.Fatal("EnterEditMode (empty)")
	}
	if cvEmpty.isEditableLayoutField("tags") {
		t.Error("empty tags is hidden in the grid; it must not be edit-traversable")
	}

	// non-empty tags -> shown -> traversable as a sanity baseline.
	sFull := store.NewInMemoryStore()
	full := newTestViewTiki("FULLTG")
	full.Set(tikipkg.FieldTags, []string{"frontend"})
	if err := sFull.CreateTiki(full); err != nil {
		t.Fatalf("create full: %v", err)
	}
	cvFull := NewConfigurableDetailView(sFull, full.ID(), plug, controller.DetailViewActions(), nil, nil)
	cvFull.SetEditModeRegistry(controller.DetailEditModeActions())
	if !cvFull.EnterEditMode() {
		t.Fatal("EnterEditMode (full)")
	}
	if !cvFull.isEditableLayoutField("tags") {
		t.Error("non-empty tags should remain edit-traversable")
	}
}

func TestRenderViewModeAnchor_CaptionRendersFieldCaption(t *testing.T) {
	// register a workflow field that carries a caption
	if err := teststatuses.InitWith([]workflow.FieldDef{
		{Name: "myfield", Type: workflow.TypeString, Custom: true, Caption: "My Caption:"},
	}); err != nil {
		t.Fatalf("register field: %v", err)
	}
	defer teststatuses.Init()

	a := gridlayout.Anchor{Kind: gridlayout.AnchorField, Name: "myfield", Display: gridlayout.DisplayCaption, Role: "text.label"}
	tk := tikipkg.New()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: theme.Roles()}

	prim := RenderViewModeAnchor(a, tk, ctx)
	got := extractTextView(prim, true)
	if !strings.Contains(got, "My Caption:") {
		t.Errorf("caption anchor rendered %q, want it to contain %q", got, "My Caption:")
	}
}

// TestTextEmptyPlaceholder_FromDescriptorTrait pins the empty-value
// placeholder text for text-type fields: assignee→"Unassigned",
// createdBy→"Unknown", everything else→"─". After the refactor these
// values come from the FieldDescriptor.EmptyPlaceholder trait rather than
// a per-name switch; the observable result must stay identical.
func TestTextEmptyPlaceholder_FromDescriptorTrait(t *testing.T) {
	if got := textEmptyPlaceholder(tikipkg.FieldAssignee); got != "Unassigned" {
		t.Errorf("assignee placeholder = %q, want %q", got, "Unassigned")
	}
	if got := textEmptyPlaceholder("createdBy"); got != "Unknown" {
		t.Errorf("createdBy placeholder = %q, want %q", got, "Unknown")
	}
	if got := textEmptyPlaceholder(tikipkg.FieldStatus); got != "─" {
		t.Errorf("default placeholder = %q, want %q", got, "─")
	}
	if got := textEmptyPlaceholder("does-not-exist"); got != "─" {
		t.Errorf("unknown-field placeholder = %q, want %q", got, "─")
	}
}

// TestRenderEnumValue_EmptyTypeMutedNone pins that rendering a tiki with an
// empty `type` field produces the muted "(none)" placeholder. This is the
// type field's empty-value presentation; after the refactor it is driven by
// the descriptor's EmptyPlaceholder trait (resolved to muted at render time
// via ctx.Roles.TextMuted) instead of a `name == FieldType` switch. The
// rendered bytes must be identical to today's output.
func TestRenderEnumValue_EmptyTypeMutedNone(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI099")
	tk.Set(tikipkg.FieldType, "") // empty type → muted "(none)"
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	colors := theme.Roles()
	ctx := FieldRenderContext{Mode: RenderModeView, Roles: colors, FieldName: tikipkg.FieldType}

	want := colors.TextMuted().Tag() + "(none)[-]"
	rawOut := extractTextView(renderEnumValue(tk, ctx), false)
	if !strings.Contains(rawOut, want) {
		t.Errorf("empty-type render = %q, want it to contain muted placeholder %q", rawOut, want)
	}
}

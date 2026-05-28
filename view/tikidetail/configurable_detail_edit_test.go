package tikidetail

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/controller"
	"github.com/boolean-maybe/tiki/gridlayout"
	"github.com/boolean-maybe/tiki/model"
	"github.com/boolean-maybe/tiki/store"
	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"

	"github.com/rivo/tview"
)

// TestConfigurableDetailView_EnterAndExitEditMode verifies the in-place
// edit-mode toggle: entering flips the flag, exits revert it, and the
// action registry swaps as the controller installed it.
func TestConfigurableDetailView_EnterAndExitEditMode(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI100")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}

	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	editReg := controller.DetailEditModeActions()
	cv.SetEditModeRegistry(editReg)

	if cv.IsEditMode() {
		t.Fatal("view should start in read-only mode")
	}
	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode returned false on a view with implemented editors")
	}
	if !cv.IsEditMode() {
		t.Error("IsEditMode should be true after EnterEditMode")
	}
	if cv.GetActionRegistry() != editReg {
		t.Error("registry should switch to edit-mode registry")
	}
	cv.ExitEditMode()
	if cv.IsEditMode() {
		t.Error("IsEditMode should be false after ExitEditMode")
	}
}

// fakeFlushWidget is a minimal FieldEditorWidget used to verify the
// flush-all-editors contract. Its GetText returns a fixed payload so the
// test can confirm the right value reached the handler.
type fakeFlushWidget struct {
	tview.Primitive
	text string
}

func (f *fakeFlushWidget) GetText() string       { return f.text }
func (f *fakeFlushWidget) CycleValue(_ int) bool { return false }

// TestConfigurableDetailView_FlushFocusedEditor_FlushesAllEditors pins
// the contract that every cached editor — not just the one currently
// holding focus — is flushed before commit. The tags textarea buffers
// input until Ctrl+S; if the user edits tags, tabs to another field,
// then presses Ctrl+S, the cached tags editor must still push its value
// into the edit session.
func TestConfigurableDetailView_FlushFocusedEditor_FlushesAllEditors(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI107")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type", "tags"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	captured := map[string]string{}
	cv.SetEditFieldChangeHandler("status", func(v string) { captured["status"] = v })
	cv.SetEditFieldChangeHandler("tags", func(v string) { captured["tags"] = v })

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}

	// Simulate the cache state after the user touched both fields:
	// tags was edited (typed text buffered), then user tabbed away to
	// status. tags is no longer focused but its widget still holds the
	// pending value. Without the all-editors flush, that value is lost.
	cv.editors["tags"] = &fakeFlushWidget{text: "frontend backend"}
	cv.editors["status"] = &fakeFlushWidget{text: "ready"}
	// Pin focus on status (index 0) — the bug we're guarding against
	// would only flush the editor at this index, dropping the cached
	// "tags" buffer.
	cv.focusedIdx = 0

	cv.FlushFocusedEditor()

	if got := captured["tags"]; got != "frontend backend" {
		t.Errorf("tags handler got %q, want %q (cached value lost?)", got, "frontend backend")
	}
	if got := captured["status"]; got != "ready" {
		t.Errorf("status handler got %q, want %q", got, "ready")
	}
}

// TestBuildFieldPrimitive_FocusOnlyOnFocusedRow pins the orchestration-
// layer contract: only the row at focusedIdx should render with the
// focus marker. Earlier "tests" of this behavior called renderEnumValue
// directly with a hand-built ctx — they couldn't catch a bug in
// buildFieldPrimitive (which constructs the ctx). This test runs the
// orchestrator and inspects the read-only renderer output for each
// non-focused row, ensuring no focus marker leaks.
func TestBuildFieldPrimitive_FocusOnlyOnFocusedRow(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI111")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}
	// EnterEditMode focuses the first editable field (status, idx 0).
	if got := cv.GetFocusedFieldName(); got != "status" {
		t.Fatalf("expected focused field 'status', got %q", got)
	}

	const marker = "► "
	colors := theme.Roles()
	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: colors, Store: s}

	focusedName := cv.GetFocusedFieldName()

	// "type", non-focused. Its read-only render must not paint the focus
	// marker even though the row IS editable in edit mode.
	typePrim := cv.buildFieldPrimitive(gridlayout.Anchor{Name: "type"}, tk, ctx, focusedName)
	typeText := extractTextView(typePrim, false)
	if strings.Contains(typeText, marker) {
		t.Errorf("non-focused 'type' row painted focus marker: %q", typeText)
	}

	// "priority", non-focused. Same expectation.
	priorityPrim := cv.buildFieldPrimitive(gridlayout.Anchor{Name: "priority"}, tk, ctx, focusedName)
	priorityText := extractTextView(priorityPrim, false)
	if strings.Contains(priorityText, marker) {
		t.Errorf("non-focused 'priority' row painted focus marker: %q", priorityText)
	}

	// Sanity: tab to type, then verify type IS focused and status is not.
	if !cv.FocusNextField() {
		t.Fatal("FocusNextField")
	}
	if got := cv.GetFocusedFieldName(); got != "type" {
		t.Fatalf("after Tab, expected 'type' focused, got %q", got)
	}
	focusedName = cv.GetFocusedFieldName()
	statusPrim := cv.buildFieldPrimitive(gridlayout.Anchor{Name: "status"}, tk, ctx, focusedName)
	statusText := extractTextView(statusPrim, false)
	if strings.Contains(statusText, marker) {
		t.Errorf("non-focused 'status' row painted focus marker after Tab: %q", statusText)
	}
}

// TestConfigurableDetailView_FlushEmitsCanonicalKeyForEnums pins the
// data-integrity contract for the SemanticEnum flush path: the value
// passed to the save handler must be the canonical enum key (e.g. "high"),
// not the display string ("High 🔴") shown in the input field. The
// underlying EditSelectList's GetText returns the display; the enum-
// aware adapter is responsible for the reverse-lookup so a flush call
// produces a save-ready value, not a label that the controller would
// then have to re-parse and could fail validation on.
func TestConfigurableDetailView_FlushEmitsCanonicalKeyForEnums(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI110")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	captured := map[string]string{}
	cv.SetEditFieldChangeHandler("status", func(v string) { captured["status"] = v })
	cv.SetEditFieldChangeHandler("priority", func(v string) { captured["priority"] = v })

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}

	// Build real enum editors via the registry — this exercises the
	// enumSelectAdapter that the factory installs. Cycle each editor
	// to a known value, then flush.
	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles(), FieldName: "status"}
	statusEditor := buildFieldEditor("status", tk, ctx, cv.onEditFieldChange["status"])
	if statusEditor == nil {
		t.Fatal("status editor nil")
	}
	cv.editors["status"] = statusEditor
	statusEditor.CycleValue(+1) // step past the default

	ctx.FieldName = "priority"
	priorityEditor := buildFieldEditor("priority", tk, ctx, cv.onEditFieldChange["priority"])
	if priorityEditor == nil {
		t.Fatal("priority editor nil")
	}
	cv.editors["priority"] = priorityEditor
	priorityEditor.CycleValue(+1) // step past the default

	cv.FlushFocusedEditor()

	// newTestViewTiki seeds status="ready" and priority="medium". One
	// CycleValue(+1) advances each one position in declaration order:
	// status [backlog, ready, inProgress, review, done] → "inProgress".
	// priority [high, medium-high, medium, medium-low, low] → "medium-low".
	// The flush must deliver canonical keys, not display strings like
	// "In Progress ⚙️" or "Medium Low 🟢".
	if got := captured["status"]; got != "inProgress" {
		t.Errorf("status flush emitted %q, want canonical key %q", got, "inProgress")
	}
	if got := captured["priority"]; got != "medium-low" {
		t.Errorf("priority flush emitted %q, want canonical key %q", got, "medium-low")
	}
	// And the broader contract: neither value contains a space, which
	// would indicate a display string slipped through.
	for field, val := range captured {
		if strings.ContainsAny(val, " 🔴🟠🟡🟢🔵📥📋⚙️👀✅🌀💥🔍🗂️") {
			t.Errorf("flush of %s emitted display-like %q (canonical keys must not contain emoji/space)", field, val)
		}
	}
}

// TestConfigurableDetailView_FlushOrderRecurrenceLast pins the flush
// ordering contract: due must flush before recurrence so SaveRecurrence's
// Due side effect (auto-computing the next occurrence) isn't overwritten
// by a stale due editor's text. Map iteration is nondeterministic in Go,
// so a naive `for k, v := range editors` would let due flush either
// before or after recurrence, manifesting as a flaky bug in production.
func TestConfigurableDetailView_FlushOrderRecurrenceLast(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI109")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"due", "recurrence"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	// Record the order in which handlers were called.
	order := []string{}
	cv.SetEditFieldChangeHandler("due", func(_ string) { order = append(order, "due") })
	cv.SetEditFieldChangeHandler("recurrence", func(_ string) { order = append(order, "recurrence") })

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}

	// Cache both editors so the flush traverses them.
	cv.editors["due"] = &fakeFlushWidget{text: "2026-05-08"}
	cv.editors["recurrence"] = &fakeFlushWidget{text: "0 0 * * MON"}

	// Run the flush several times: with map iteration, ordering can vary
	// per iteration and the failure is flaky. Repeating amplifies the
	// signal — even one out-of-order pass fails the test.
	for i := 0; i < 20; i++ {
		order = order[:0]
		cv.FlushFocusedEditor()
		if len(order) != 2 {
			t.Fatalf("iter %d: flushed %d handlers, want 2", i, len(order))
		}
		if order[len(order)-1] != "recurrence" {
			t.Fatalf("iter %d: expected recurrence flushed last, got %v", i, order)
		}
	}
}

// TestConfigurableDetailView_TabTraversesCustomEnumField pins that a
// workflow-declared enum field with no static FieldDescriptor still
// participates in Tab traversal and edit mode. The previous fix made
// FieldHasEditor recognize workflow enums, but isEditableLayoutField
// short-circuited on the missing static descriptor — so EnterEditMode
// and Tab skipped severity entirely even though the editor was wired.
func TestConfigurableDetailView_TabTraversesCustomEnumField(t *testing.T) {
	cleanup := registerExtraWorkflowFieldForTest(t, "severity", []string{"low", "medium", "high"})
	t.Cleanup(cleanup)

	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI108")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "severity", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}
	visited := []string{cv.GetFocusedFieldName()}
	for cv.FocusNextField() {
		visited = append(visited, cv.GetFocusedFieldName())
	}
	// Tab traversal walks metadata anchors in declaration order and then
	// lands on the inline description editor (the synthetic "description"
	// field) before stopping.
	want := []string{"status", "severity", "priority", "description"}
	if len(visited) != len(want) {
		t.Fatalf("visited %v, want %v", visited, want)
	}
	for i, name := range want {
		if visited[i] != name {
			t.Errorf("visited[%d] = %q, want %q (workflow-only enum must be reachable)", i, visited[i], name)
		}
	}
}

// TestConfigurableDetailView_TabTraversesEditableFields ensures Tab
// advances focus across editable metadata in `metadata:` order, and
// Shift-Tab moves backward, both stopping at the boundaries.
func TestConfigurableDetailView_TabTraversesEditableFields(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI101")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode failed")
	}
	if got := cv.GetFocusedFieldName(); got != "status" {
		t.Errorf("initial focus = %q, want %q", got, "status")
	}
	if !cv.FocusNextField() {
		t.Fatal("FocusNextField returned false at status")
	}
	if got := cv.GetFocusedFieldName(); got != "type" {
		t.Errorf("after Tab = %q, want %q", got, "type")
	}
	if !cv.FocusNextField() {
		t.Fatal("FocusNextField returned false at type")
	}
	if got := cv.GetFocusedFieldName(); got != "priority" {
		t.Errorf("after Tab = %q, want %q", got, "priority")
	}
	// Tab past the last metadata field lands on the inline description
	// editor (synthetic "description" pseudo-field).
	if !cv.FocusNextField() {
		t.Error("FocusNextField at last metadata field should land on description")
	}
	if got := cv.GetFocusedFieldName(); got != "description" {
		t.Errorf("after Tab past last metadata = %q, want %q", got, "description")
	}
	if cv.FocusNextField() {
		t.Error("FocusNextField should return false past description")
	}
	if !cv.FocusPrevField() {
		t.Fatal("FocusPrevField at description returned false")
	}
	if got := cv.GetFocusedFieldName(); got != "priority" {
		t.Errorf("after Shift+Tab from description = %q, want %q", got, "priority")
	}
}

// TestConfigurableDetailView_ReadOnlyFieldsAreSkippedInTraversal asserts
// read-only descriptors (createdBy/createdAt/updatedAt) render but do not
// participate in Tab traversal.
func TestConfigurableDetailView_ReadOnlyFieldsAreSkippedInTraversal(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI102")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "createdBy", "type", "createdAt", "priority", "updatedAt"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode failed")
	}
	visited := []string{cv.GetFocusedFieldName()}
	for cv.FocusNextField() {
		visited = append(visited, cv.GetFocusedFieldName())
	}
	// Tab traversal includes the inline description editor at the end —
	// read-only descriptors (createdBy/createdAt/updatedAt) are still
	// skipped, which is the property this test pins.
	want := []string{"status", "type", "priority", "description"}
	if len(visited) != len(want) {
		t.Fatalf("visited %v, want %v", visited, want)
	}
	for i, name := range want {
		if visited[i] != name {
			t.Errorf("visited[%d] = %q, want %q", i, visited[i], name)
		}
	}
}

// TestConfigurableDetailView_TabReachesDescription pins that Tab lands on
// the synthetic "description" pseudo-field after the last metadata field,
// and Shift-Tab from description steps back to the last metadata field.
// Description body editing happens inline (TextArea), so this contract is
// what makes "n" → fill metadata → Tab → write description → Ctrl-S a
// single-flow new-tiki creation experience.
func TestConfigurableDetailView_TabReachesDescription(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI116")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}
	if got := cv.GetFocusedFieldName(); got != "status" {
		t.Fatalf("initial focus = %q, want %q", got, "status")
	}
	if !cv.FocusNextField() || cv.GetFocusedFieldName() != "priority" {
		t.Fatalf("Tab to priority failed: focus=%q", cv.GetFocusedFieldName())
	}
	if !cv.FocusNextField() {
		t.Fatal("Tab from last metadata field returned false; want description")
	}
	if got := cv.GetFocusedFieldName(); got != "description" {
		t.Errorf("Tab past metadata focused %q, want %q", got, "description")
	}
	if !cv.FocusPrevField() || cv.GetFocusedFieldName() != "priority" {
		t.Errorf("Shift-Tab from description focused %q, want %q", cv.GetFocusedFieldName(), "priority")
	}
}

// TestConfigurableDetailView_NoEditableFieldsLeavesViewMode asserts the
// edit-mode toggle is a no-op when no configured field has an
// implemented editor — the view stays in read-only mode rather than
// trapping the user with no usable fields.
func TestConfigurableDetailView_NoEditableFieldsLeavesViewMode(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI103")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"createdBy", "createdAt", "updatedAt"}), // all read-only
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if cv.EnterEditMode() {
		t.Error("EnterEditMode should return false when no field has an implemented editor")
	}
	if cv.IsEditMode() {
		t.Error("IsEditMode should remain false")
	}
}

// TestFieldRegistry_ImplementedAndStubCapabilities asserts which semantic
// types advertise editor implementations vs remain stubs. After the
// SemanticEnum unification, status/type/priority all route through the
// single SemanticEnum implementation, so the implemented list collapses to
// the unique editor categories.
func TestFieldRegistry_ImplementedAndStubCapabilities(t *testing.T) {
	implemented := []SemanticType{
		SemanticEnum,
		SemanticText, SemanticDate,
		SemanticRecurrence, SemanticStringList,
	}
	for _, sem := range implemented {
		t.Run(string(sem), func(t *testing.T) {
			ui, _ := LookupType(sem)
			if ui.Capability != EditorImplemented {
				t.Errorf("%q: Capability = %v, want EditorImplemented", sem, ui.Capability)
			}
			if ui.Edit == nil {
				t.Errorf("%q: Edit factory is nil", sem)
			}
		})
	}
	stubs := []SemanticType{
		SemanticBoolean, SemanticDateTime, SemanticInteger,
		SemanticTikiIDList,
	}
	for _, sem := range stubs {
		t.Run(string(sem)+"_stub", func(t *testing.T) {
			ui, _ := LookupType(sem)
			if ui.Capability != EditorStub {
				t.Errorf("%q: expected EditorStub, got %v", sem, ui.Capability)
			}
		})
	}
}

// TestConfigurableDetailView_FiresActionChangeHandlerOnToggle locks in
// the contract RootLayout depends on: edit-mode toggles must invoke the
// ActionChangeNotifier handler so the header bar and palette resync to
// the new registry. Without this fire, dispatch keeps working but the UI
// keeps showing read-only actions while edit mode is active.
func TestConfigurableDetailView_FiresActionChangeHandlerOnToggle(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI104")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	calls := 0
	cv.SetActionChangeHandler(func() { calls++ })

	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode failed")
	}
	if calls != 1 {
		t.Errorf("after EnterEditMode: handler fired %d times, want 1", calls)
	}
	cv.ExitEditMode()
	if calls != 2 {
		t.Errorf("after ExitEditMode: handler fired %d times, want 2", calls)
	}
}

// TestFieldHasEditor_OnlyImplementedFieldsReturnTrue verifies the
// FieldHasEditor predicate the view uses to gate Tab traversal.
func TestFieldHasEditor_OnlyImplementedFieldsReturnTrue(t *testing.T) {
	implemented := []string{"status", "type", "priority", "points", "assignee", "due", "recurrence", "tags"}
	for _, name := range implemented {
		if !FieldHasEditor(name) {
			t.Errorf("FieldHasEditor(%q) = false, want true", name)
		}
	}
	// dependsOn renderer exists but no in-place editor yet.
	if FieldHasEditor("dependsOn") {
		t.Error("FieldHasEditor(dependsOn) = true, want false (stub editor)")
	}
	// read-only descriptors must never report editable.
	for _, name := range []string{"createdBy", "createdAt", "updatedAt"} {
		if FieldHasEditor(name) {
			t.Errorf("FieldHasEditor(%q) = true, want false (read-only)", name)
		}
	}
	if FieldHasEditor("not_a_field") {
		t.Error("FieldHasEditor on unknown field should return false")
	}
}

// TestGetPreferredFocus_ReturnsActiveEditor pins that, when the view is in
// edit mode, GetPreferredFocus returns the focused editor primitive. The
// view factory enters edit mode during construction (mode: new from "n");
// RootLayout reads this hook after OnFocus to push keyboard focus to the
// title input instead of the view root. Without it, the user lands in
// edit mode but cannot type.
func TestGetPreferredFocus_ReturnsActiveEditor(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI115")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"title", "status", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if got := cv.GetPreferredFocus(); got != nil {
		t.Errorf("GetPreferredFocus() in view mode = %T, want nil", got)
	}

	if !cv.EnterEditModeWithFocus(model.EditFieldTitle) {
		t.Fatal("EnterEditModeWithFocus(title) returned false")
	}

	titleEditor, ok := cv.editors["title"]
	if !ok || titleEditor == nil {
		t.Fatal("title editor was not built during EnterEditMode")
	}
	if got := cv.GetPreferredFocus(); got != titleEditor {
		t.Errorf("GetPreferredFocus() = %T, want title editor %T", got, titleEditor)
	}
}

// TestEnterEditModeWithFocus_FocusesGivenField verifies that passing a
// specific field name lands focus on that field rather than the layout's
// first editable position.
func TestEnterEditModeWithFocus_FocusesGivenField(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI112")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"title", "status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditModeWithFocus(model.EditFieldPriority) {
		t.Fatal("EnterEditModeWithFocus returned false")
	}
	if !cv.IsEditMode() {
		t.Error("expected IsEditMode true after EnterEditModeWithFocus")
	}
	if got := cv.GetFocusedFieldName(); got != "priority" {
		t.Errorf("focused field = %q, want %q", got, "priority")
	}
}

// TestEnterEditModeWithFocus_EmptyEqualsEnterEditMode verifies that an
// empty focus argument preserves the default first-editable-field behavior.
func TestEnterEditModeWithFocus_EmptyEqualsEnterEditMode(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI113")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type", "priority"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditModeWithFocus("") {
		t.Fatal(`EnterEditModeWithFocus("") returned false`)
	}
	if got := cv.GetFocusedFieldName(); got == "" {
		t.Error("expected a non-empty default focus")
	} else if got != "status" {
		t.Errorf("default focus = %q, want %q", got, "status")
	}
}

// TestEnterEditModeWithFocus_UnknownFieldFallsBackToDefault verifies that
// asking for a field that is absent from the layout (or non-editable)
// quietly falls back to the EnterEditMode default rather than failing.
func TestEnterEditModeWithFocus_UnknownFieldFallsBackToDefault(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI114")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "type"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditModeWithFocus(model.EditFieldPriority) {
		t.Fatal("EnterEditModeWithFocus returned false on missing field")
	}
	if got := cv.GetFocusedFieldName(); got != "status" {
		t.Errorf("fallback focus = %q, want %q (first editable in layout)", got, "status")
	}
}

// TestConfigurableDetailView_RecurrencePartNavigation pins the
// RecurrencePartNavigable contract on ConfigurableDetailView: the three
// methods forward to the underlying RecurrenceEdit only when the focused
// field is recurrence and the cached editor is a recurrenceEditAdapter.
// Off-field calls return false without panicking.
func TestConfigurableDetailView_RecurrencePartNavigation(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI115")
	// seed a Weekly cron so the recurrence editor has both parts
	// (frequency + value); MovePartRight is a no-op without a value part.
	tk.Set(tikipkg.FieldRecurrence, "0 0 * * MON")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"status", "recurrence"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditModeWithFocus(model.EditFieldRecurrence) {
		t.Fatal("EnterEditModeWithFocus(recurrence) returned false")
	}
	if got := cv.GetFocusedFieldName(); got != tikipkg.FieldRecurrence {
		t.Fatalf("focused field = %q, want %q", got, tikipkg.FieldRecurrence)
	}

	// build the real recurrence editor through the registry and seed it
	// with a Weekly cron so the value part exists and cursor moves are
	// observable.
	ctx := FieldRenderContext{Mode: RenderModeEdit, Roles: theme.Roles(), FieldName: tikipkg.FieldRecurrence}
	editor := buildFieldEditor(tikipkg.FieldRecurrence, tk, ctx, cv.onEditFieldChange[tikipkg.FieldRecurrence])
	if editor == nil {
		t.Fatal("recurrence editor nil")
	}
	re, ok := editor.(*recurrenceEditAdapter)
	if !ok {
		t.Fatalf("expected *recurrenceEditAdapter, got %T", editor)
	}
	re.RecurrenceEdit.SetInitialValue("0 0 * * MON")
	cv.editors[tikipkg.FieldRecurrence] = editor

	// initial state: frequency part active, value part not focused.
	if cv.IsRecurrenceValueFocused() {
		t.Error("IsRecurrenceValueFocused = true at start, want false (frequency active)")
	}

	if !cv.MoveRecurrencePartRight() {
		t.Fatal("MoveRecurrencePartRight returned false on focused recurrence field")
	}
	if !cv.IsRecurrenceValueFocused() {
		t.Error("IsRecurrenceValueFocused = false after MoveRecurrencePartRight, want true")
	}

	if !cv.MoveRecurrencePartLeft() {
		t.Fatal("MoveRecurrencePartLeft returned false on focused recurrence field")
	}
	if cv.IsRecurrenceValueFocused() {
		t.Error("IsRecurrenceValueFocused = true after MoveRecurrencePartLeft, want false")
	}

	// move focus to a non-recurrence field; all three methods must
	// return false without crashing.
	if !cv.FocusPrevField() {
		t.Fatal("FocusPrevField to status returned false")
	}
	if got := cv.GetFocusedFieldName(); got != "status" {
		t.Fatalf("after FocusPrevField, focused = %q, want %q", got, "status")
	}
	if cv.MoveRecurrencePartLeft() {
		t.Error("MoveRecurrencePartLeft returned true on non-recurrence field")
	}
	if cv.MoveRecurrencePartRight() {
		t.Error("MoveRecurrencePartRight returned true on non-recurrence field")
	}
	if cv.IsRecurrenceValueFocused() {
		t.Error("IsRecurrenceValueFocused = true on non-recurrence field, want false")
	}
}

// TestConfigurableDetailView_RecurrencePartNavigation_NoEditorCached pins
// the guard against a missing editor entry: when recurrence is the focused
// field but no editor has been cached yet (e.g. before first refresh),
// the methods return false rather than nil-deref.
func TestConfigurableDetailView_RecurrencePartNavigation_NoEditorCached(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI116")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"recurrence", "status"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())

	if !cv.EnterEditModeWithFocus(model.EditFieldRecurrence) {
		t.Fatal("EnterEditModeWithFocus(recurrence) returned false")
	}
	// editors map exists but has no recurrence entry — refresh hasn't
	// run in tests because there's no tview Application.
	delete(cv.editors, tikipkg.FieldRecurrence)
	if cv.MoveRecurrencePartLeft() {
		t.Error("MoveRecurrencePartLeft returned true when no editor cached")
	}
	if cv.MoveRecurrencePartRight() {
		t.Error("MoveRecurrencePartRight returned true when no editor cached")
	}
	if cv.IsRecurrenceValueFocused() {
		t.Error("IsRecurrenceValueFocused returned true when no editor cached")
	}
}

// TestConfigurableDetailView_RecurrencePartNavigation_WrongAdapterType pins
// the type-assertion guard: a cached editor that isn't *recurrenceEditAdapter
// (defensive — shouldn't happen in production) returns false without
// panicking.
func TestConfigurableDetailView_RecurrencePartNavigation_WrongAdapterType(t *testing.T) {
	s := store.NewInMemoryStore()
	tk := newTestViewTiki("TIKI117")
	if err := s.CreateTiki(tk); err != nil {
		t.Fatalf("CreateTiki: %v", err)
	}
	cv := NewConfigurableDetailView(
		s, tk.ID, detailPluginFromFields([]string{"recurrence"}),
		controller.DetailViewActions(),
		nil, nil,
	)
	cv.SetEditModeRegistry(controller.DetailEditModeActions())
	if !cv.EnterEditMode() {
		t.Fatal("EnterEditMode")
	}
	cv.editors[tikipkg.FieldRecurrence] = &fakeFlushWidget{text: ""}
	if cv.MoveRecurrencePartLeft() {
		t.Error("MoveRecurrencePartLeft returned true on wrong adapter type")
	}
	if cv.MoveRecurrencePartRight() {
		t.Error("MoveRecurrencePartRight returned true on wrong adapter type")
	}
	if cv.IsRecurrenceValueFocused() {
		t.Error("IsRecurrenceValueFocused returned true on wrong adapter type")
	}
}

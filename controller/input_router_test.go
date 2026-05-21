package controller

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/model"
)

// routerFakeView is the minimal View used to exercise routeFieldAwareSave.
// Concrete capability variants embed it and add the optional methods the
// router probes via type assertion.
type routerFakeView struct {
	focused    model.EditField
	tagsCalled bool
	descCalled bool
}

func (f *routerFakeView) GetPrimitive() tview.Primitive      { return nil }
func (f *routerFakeView) GetActionRegistry() *ActionRegistry { return nil }
func (f *routerFakeView) GetViewID() model.ViewID            { return "fake" }
func (f *routerFakeView) OnFocus()                           {}
func (f *routerFakeView) OnBlur()                            {}

// routerFakeFocusable provides the shared FieldFocusableView stubs.
type routerFakeFocusable struct{ *routerFakeView }

func (f *routerFakeFocusable) GetFocusedField() model.EditField  { return f.focused }
func (f *routerFakeFocusable) SetFocusedField(_ model.EditField) {}
func (f *routerFakeFocusable) FocusNextField() bool              { return false }
func (f *routerFakeFocusable) FocusPrevField() bool              { return false }
func (f *routerFakeFocusable) IsEditFieldFocused() bool          { return true }

// routerFakeTags satisfies FieldFocusableView + TagsTextAreaSavable.
type routerFakeTags struct{ *routerFakeFocusable }

func (f *routerFakeTags) SaveTagsFromTextArea() { f.tagsCalled = true }

// routerFakeDesc satisfies FieldFocusableView + DescriptionTextAreaSavable.
type routerFakeDesc struct{ *routerFakeFocusable }

func (f *routerFakeDesc) SaveDescriptionFromTextArea() { f.descCalled = true }

// routerFakeNoSave satisfies FieldFocusableView but neither Savable.
type routerFakeNoSave struct{ *routerFakeFocusable }

// TestRouteFieldAwareSave_DispatchesTagsHook verifies that when a tags field
// has focus and the view implements TagsTextAreaSavable, Ctrl-S routes to
// SaveTagsFromTextArea instead of falling through.
func TestRouteFieldAwareSave_DispatchesTagsHook(t *testing.T) {
	base := &routerFakeView{focused: model.EditFieldTags}
	view := &routerFakeTags{routerFakeFocusable: &routerFakeFocusable{routerFakeView: base}}

	if !routeFieldAwareSave(view) {
		t.Fatal("routeFieldAwareSave returned false; want true (hook fired)")
	}
	if !base.tagsCalled {
		t.Error("SaveTagsFromTextArea was not invoked")
	}
}

// TestRouteFieldAwareSave_DispatchesDescriptionHook mirrors the tags case
// for the description field.
func TestRouteFieldAwareSave_DispatchesDescriptionHook(t *testing.T) {
	base := &routerFakeView{focused: model.EditFieldDescription}
	view := &routerFakeDesc{routerFakeFocusable: &routerFakeFocusable{routerFakeView: base}}

	if !routeFieldAwareSave(view) {
		t.Fatal("routeFieldAwareSave returned false; want true (hook fired)")
	}
	if !base.descCalled {
		t.Error("SaveDescriptionFromTextArea was not invoked")
	}
}

// TestRouteFieldAwareSave_FallsThroughOnNonBufferedField pins that fields
// other than tags/description return false so Ctrl-S falls through to the
// standard ActionDetailSave dispatch.
func TestRouteFieldAwareSave_FallsThroughOnNonBufferedField(t *testing.T) {
	base := &routerFakeView{focused: model.EditFieldTitle}
	view := &routerFakeTags{routerFakeFocusable: &routerFakeFocusable{routerFakeView: base}}

	if routeFieldAwareSave(view) {
		t.Error("routeFieldAwareSave returned true for Title; want false")
	}
	if base.tagsCalled {
		t.Error("SaveTagsFromTextArea unexpectedly invoked for Title field")
	}
}

// TestRouteFieldAwareSave_FallsThroughWhenNotFocusable pins that a view
// that doesn't implement FieldFocusableView returns false (no panic).
func TestRouteFieldAwareSave_FallsThroughWhenNotFocusable(t *testing.T) {
	view := &routerFakeView{}

	if routeFieldAwareSave(view) {
		t.Error("routeFieldAwareSave returned true for non-focusable view; want false")
	}
}

// TestRouteFieldAwareSave_FallsThroughWhenNoSavableHook covers the case
// where the field is tags/description but the view lacks the matching
// Savable interface — Ctrl-S should fall through to ActionDetailSave.
func TestRouteFieldAwareSave_FallsThroughWhenNoSavableHook(t *testing.T) {
	base := &routerFakeView{focused: model.EditFieldTags}
	view := &routerFakeNoSave{routerFakeFocusable: &routerFakeFocusable{routerFakeView: base}}

	if routeFieldAwareSave(view) {
		t.Error("routeFieldAwareSave returned true without Savable hook; want false")
	}
}

// routerRecurrenceView is a thin detailEditModeView + RecurrencePartNavigable
// implementation that wraps a real *component.RecurrenceEdit. The part-nav
// methods forward to the underlying component's MovePartLeft/MovePartRight,
// so assertions can read the genuine activePart state via IsValueFocused()
// instead of a hand-toggled fake bool. When recurrenceFocused is false the
// methods return false to mirror the production guard, so non-recurrence
// Left/Right keys fall through.
type routerRecurrenceView struct {
	*routerFakeView
	editing           bool
	recurrenceFocused bool
	rec               *component.RecurrenceEdit
}

func (r *routerRecurrenceView) IsEditMode() bool         { return r.editing }
func (r *routerRecurrenceView) IsEditFieldFocused() bool { return r.editing }

func (r *routerRecurrenceView) MoveRecurrencePartLeft() bool {
	if !r.recurrenceFocused {
		return false
	}
	r.rec.MovePartLeft()
	return true
}

func (r *routerRecurrenceView) MoveRecurrencePartRight() bool {
	if !r.recurrenceFocused {
		return false
	}
	r.rec.MovePartRight()
	return true
}

func (r *routerRecurrenceView) IsRecurrenceValueFocused() bool {
	return r.rec.IsValueFocused()
}

// newRouterWithDetailPlugin builds an InputRouter wired to a single plugin
// name backed by a zero-value DetailController. maybeHandleDetailEditMode
// only requires the controller to exist in the lookup map for arrow-key
// dispatch — the Left/Right path never invokes ctrl.HandleAction.
func newRouterWithDetailPlugin(pluginName string) *InputRouter {
	return &InputRouter{
		pluginControllers: map[string]PluginControllerInterface{
			pluginName: &DetailController{},
		},
	}
}

// newRouterRecurrenceView builds a routerRecurrenceView wrapping a real
// *component.RecurrenceEdit seeded to a Weekly cron. Weekly is required so
// the value part exists — MovePartRight is a no-op without one. The editor
// starts on the value part (activePart=1) when startOnValue is true, by
// driving MovePartRight after construction.
func newRouterRecurrenceView(t *testing.T, recurrenceFocused, startOnValue bool) *routerRecurrenceView {
	t.Helper()
	rec := component.NewRecurrenceEdit()
	rec.SetInitialValue("0 0 * * MON")
	if startOnValue {
		rec.MovePartRight()
		if !rec.IsValueFocused() {
			t.Fatal("seed: MovePartRight did not produce value-focused state")
		}
	}
	return &routerRecurrenceView{
		routerFakeView:    &routerFakeView{},
		editing:           true,
		recurrenceFocused: recurrenceFocused,
		rec:               rec,
	}
}

// TestMaybeHandleDetailEditMode_LeftMovesRecurrencePart pins that a KeyLeft
// routed through the input router while the recurrence field is focused
// flips the underlying *component.RecurrenceEdit out of value-focused state.
// The assertion reads RecurrenceEdit.IsValueFocused() — the genuine
// activePart-derived flag — not a hand-rolled fake bool.
func TestMaybeHandleDetailEditMode_LeftMovesRecurrencePart(t *testing.T) {
	const pluginName = "Detail"
	ir := newRouterWithDetailPlugin(pluginName)
	view := newRouterRecurrenceView(t, true, true)
	entry := &ViewEntry{ViewID: model.MakePluginViewID(pluginName)}
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)

	stop, handled := ir.maybeHandleDetailEditMode(view, entry, event)
	if !stop || !handled {
		t.Fatalf("KeyLeft on recurrence: stop=%v handled=%v, want true/true", stop, handled)
	}
	if view.rec.IsValueFocused() {
		t.Error("RecurrenceEdit.IsValueFocused = true after KeyLeft, want false")
	}
}

// TestMaybeHandleDetailEditMode_RightMovesRecurrencePart pins the symmetric
// KeyRight path: the underlying RecurrenceEdit advances from frequency to
// value part. Asserts via the real component's IsValueFocused().
func TestMaybeHandleDetailEditMode_RightMovesRecurrencePart(t *testing.T) {
	const pluginName = "Detail"
	ir := newRouterWithDetailPlugin(pluginName)
	view := newRouterRecurrenceView(t, true, false)
	entry := &ViewEntry{ViewID: model.MakePluginViewID(pluginName)}
	event := tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone)

	stop, handled := ir.maybeHandleDetailEditMode(view, entry, event)
	if !stop || !handled {
		t.Fatalf("KeyRight on recurrence: stop=%v handled=%v, want true/true", stop, handled)
	}
	if !view.rec.IsValueFocused() {
		t.Error("RecurrenceEdit.IsValueFocused = false after KeyRight, want true")
	}
}

// TestMaybeHandleDetailEditMode_LeftFallsThroughOnNonRecurrenceField pins
// that Left/Right do not consume the event when the focused field is not
// recurrence — the wrapper returns false and the router lets the key fall
// through (stop=false) to the focused widget's input handler. The
// underlying RecurrenceEdit must remain untouched.
func TestMaybeHandleDetailEditMode_LeftFallsThroughOnNonRecurrenceField(t *testing.T) {
	const pluginName = "Detail"
	ir := newRouterWithDetailPlugin(pluginName)
	view := newRouterRecurrenceView(t, false, false)
	entry := &ViewEntry{ViewID: model.MakePluginViewID(pluginName)}
	event := tcell.NewEventKey(tcell.KeyLeft, 0, tcell.ModNone)

	stop, handled := ir.maybeHandleDetailEditMode(view, entry, event)
	if stop || handled {
		t.Errorf("KeyLeft on non-recurrence: stop=%v handled=%v, want false/false", stop, handled)
	}
	if view.rec.IsValueFocused() {
		t.Error("RecurrenceEdit.IsValueFocused = true after fall-through, want false (untouched)")
	}
}

package controller

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/boolean-maybe/tiki/component"
	"github.com/boolean-maybe/tiki/model"
)

// routerFakeView is the minimal View used by the recurrence part-nav tests.
type routerFakeView struct{}

func (f *routerFakeView) GetPrimitive() tview.Primitive      { return nil }
func (f *routerFakeView) GetActionRegistry() *ActionRegistry { return nil }
func (f *routerFakeView) GetViewID() model.ViewID            { return "fake" }
func (f *routerFakeView) OnFocus()                           {}
func (f *routerFakeView) OnBlur()                            {}

// adapterEmbeddingInputField mirrors view/tikidetail.titleEditAdapter — a
// struct that embeds *tview.InputField. isTextInputFocused must recognise it.
type adapterEmbeddingInputField struct {
	*tview.InputField
}

// adapterEmbeddingTextArea mirrors the detail view's string-list adapter.
type adapterEmbeddingTextArea struct {
	*tview.TextArea
}

// adapterEmbeddingEditSelectList mirrors view/tikidetail.selectListAdapter — the
// assignee free-text editor after the "unify multi-part editors" refactor. It
// wraps a *component.EditSelectList, which itself embeds *tview.InputField, so
// the focused primitive nests the text input TWO levels deep. isTextInputFocused
// must still recognise it when the list allows free typing.
type adapterEmbeddingEditSelectList struct {
	*component.EditSelectList
}

// TestIsTextInputFocused_DirectInputField pins that a bare InputField is
// recognised as a text-input target.
func TestIsTextInputFocused_DirectInputField(t *testing.T) {
	app := tview.NewApplication()
	app.SetRoot(tview.NewInputField(), true)
	if !isTextInputFocused(app) {
		t.Error("expected isTextInputFocused=true for *tview.InputField root")
	}
}

// TestIsTextInputFocused_EmbeddedInputFieldAdapter pins the adapter case
// (titleEditAdapter wraps *tview.InputField via embedding). Without this,
// typing 'r' / 'q' / etc. into the title input would be intercepted by
// global hotkeys because the focus-type check would miss the adapter.
func TestIsTextInputFocused_EmbeddedInputFieldAdapter(t *testing.T) {
	adapter := &adapterEmbeddingInputField{InputField: tview.NewInputField()}
	app := tview.NewApplication()
	app.SetRoot(adapter, true)
	if !isTextInputFocused(app) {
		t.Error("expected isTextInputFocused=true for adapter embedding *tview.InputField")
	}
}

// TestIsTextInputFocused_EmbeddedTextAreaAdapter mirrors the InputField
// adapter case for *tview.TextArea (tags/description editors).
func TestIsTextInputFocused_EmbeddedTextAreaAdapter(t *testing.T) {
	adapter := &adapterEmbeddingTextArea{TextArea: tview.NewTextArea()}
	app := tview.NewApplication()
	app.SetRoot(adapter, true)
	if !isTextInputFocused(app) {
		t.Error("expected isTextInputFocused=true for adapter embedding *tview.TextArea")
	}
}

// TestIsTextInputFocused_EmbeddedEditSelectListAdapter pins the assignee
// free-text editor (selectListAdapter → EditSelectList → *tview.InputField).
// The reproduction for the smoke-test "q quits the app" defect: a free-typing
// EditSelectList holds focus, so typed runes must reach the widget rather than
// falling through to the global 'q' → Quit action. The primitive nests the
// text input two levels deep, which the one-level reflection walk missed.
func TestIsTextInputFocused_EmbeddedEditSelectListAdapter(t *testing.T) {
	editor := component.NewEditSelectList([]string{"alice", "bob"}, true)
	adapter := &adapterEmbeddingEditSelectList{EditSelectList: editor}
	app := tview.NewApplication()
	app.SetRoot(adapter, true)
	if !isTextInputFocused(app) {
		t.Error("expected isTextInputFocused=true for adapter embedding a free-typing *component.EditSelectList")
	}
}

// TestIsTextInputFocused_ReadOnlyEditSelectList pins that a NON-typing
// EditSelectList (enum/boolean pickers, allowTyping=false) is NOT treated as a
// text input — arrow keys cycle values and single-letter globals stay active.
func TestIsTextInputFocused_ReadOnlyEditSelectList(t *testing.T) {
	editor := component.NewEditSelectList([]string{"low", "high"}, false)
	app := tview.NewApplication()
	app.SetRoot(editor, true)
	if isTextInputFocused(app) {
		t.Error("expected isTextInputFocused=false for a non-typing *component.EditSelectList")
	}
}

// TestIsTextInputFocused_NonTextPrimitive pins that non-text primitives
// (TextView, Box, custom widgets) are NOT classified as text inputs so the
// global hotkey registry stays in charge for those views.
func TestIsTextInputFocused_NonTextPrimitive(t *testing.T) {
	app := tview.NewApplication()
	app.SetRoot(tview.NewTextView(), true)
	if isTextInputFocused(app) {
		t.Error("expected isTextInputFocused=false for *tview.TextView")
	}
}

// TestIsTextInputKey_RuneAndEditingKeys pins which keys the gate forwards
// to a text editor. Tab/Backtab/Esc/Ctrl-S must NOT be in this set —
// they belong to the edit-mode action registry.
func TestIsTextInputKey_RuneAndEditingKeys(t *testing.T) {
	allow := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyDelete, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone),
	}
	for _, ev := range allow {
		if !isTextInputKey(ev) {
			t.Errorf("isTextInputKey(%s) = false, want true", ev.Name())
		}
	}
	deny := []*tcell.EventKey{
		tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModNone),
	}
	for _, ev := range deny {
		if isTextInputKey(ev) {
			t.Errorf("isTextInputKey(%s) = true, want false", ev.Name())
		}
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

// detailEditFakeView is a minimal detailEditModeView in edit mode, used by the
// Enter save-and-close tests. The focus discrimination (TextArea vs. not) is
// driven by the app's focused root, not this fake.
type detailEditFakeView struct {
	*routerFakeView
}

func (v *detailEditFakeView) IsEditMode() bool         { return true }
func (v *detailEditFakeView) IsEditFieldFocused() bool { return true }

// newRouterWithApp builds an InputRouter wired to a Detail plugin controller
// and a NavigationController whose app has root focused for the focus checks.
func newRouterWithApp(pluginName string, root tview.Primitive) *InputRouter {
	app := tview.NewApplication()
	app.SetRoot(root, true)
	return &InputRouter{
		pluginControllers: map[string]PluginControllerInterface{
			pluginName: &DetailController{},
		},
		navController: NewNavigationController(app),
	}
}

// TestMaybeHandleDetailEditMode_EnterFallsThroughOnTextArea pins that Enter on
// a multi-line TextArea (description/tags) returns stop=true, handled=false:
// stop=true prevents the edit-mode registry (which binds Enter to
// ActionDetailSaveAndClose for the footer) from firing it, and handled=false
// lets tview deliver the key to the widget so it inserts a newline. Returning
// false/false here would be a bug — the registry would then match Enter and
// save-and-close the view instead of inserting a newline.
func TestMaybeHandleDetailEditMode_EnterFallsThroughOnTextArea(t *testing.T) {
	const pluginName = "Detail"
	ir := newRouterWithApp(pluginName, tview.NewTextArea())
	view := &detailEditFakeView{routerFakeView: &routerFakeView{}}
	entry := &ViewEntry{ViewID: model.MakePluginViewID(pluginName)}
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)

	stop, handled := ir.maybeHandleDetailEditMode(view, entry, event)
	if !stop || handled {
		t.Errorf("Enter on TextArea: stop=%v handled=%v, want true/false (consume routing, let widget insert newline)", stop, handled)
	}
}

// TestMaybeHandleDetailEditMode_EnterInterceptedOnSingleLineField pins that
// Enter IS intercepted (stop=true) when a single-line InputField field holds
// focus, dispatching the save-and-close action.
func TestMaybeHandleDetailEditMode_EnterInterceptedOnSingleLineField(t *testing.T) {
	const pluginName = "Detail"
	ir := newRouterWithApp(pluginName, tview.NewInputField())
	view := &detailEditFakeView{routerFakeView: &routerFakeView{}}
	entry := &ViewEntry{ViewID: model.MakePluginViewID(pluginName)}
	event := tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)

	stop, _ := ir.maybeHandleDetailEditMode(view, entry, event)
	if !stop {
		t.Error("Enter on InputField: stop=false, want true (intercepted for save & close)")
	}
}

// TestIsTextAreaFocused pins the discriminator: only *tview.TextArea (direct or
// embedded) reports true; a bare *tview.InputField does not.
func TestIsTextAreaFocused(t *testing.T) {
	textAreaApp := tview.NewApplication()
	textAreaApp.SetRoot(tview.NewTextArea(), true)
	if !isTextAreaFocused(textAreaApp) {
		t.Error("isTextAreaFocused=false for *tview.TextArea root, want true")
	}

	adapterApp := tview.NewApplication()
	adapterApp.SetRoot(&adapterEmbeddingTextArea{TextArea: tview.NewTextArea()}, true)
	if !isTextAreaFocused(adapterApp) {
		t.Error("isTextAreaFocused=false for adapter embedding *tview.TextArea, want true")
	}

	inputApp := tview.NewApplication()
	inputApp.SetRoot(tview.NewInputField(), true)
	if isTextAreaFocused(inputApp) {
		t.Error("isTextAreaFocused=true for *tview.InputField, want false")
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

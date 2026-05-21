package controller

import (
	"testing"

	"github.com/rivo/tview"

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

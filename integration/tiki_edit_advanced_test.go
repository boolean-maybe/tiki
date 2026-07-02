package integration

// This file previously held integration tests for a deleted built-in
// edit view that rendered a rigid 6+ field form (Title, Status, Type,
// Priority, Points, Assignee) with Tab/Shift-Tab traversal, Up/Down
// status cycling, type toggling, assignee input, and points/priority
// range checks.
//
// The canonical "edit existing tiki" path is now in-place edit mode on
// the configurable detail view (kind: detail). Its metadata is
// workflow-declared (default [status, type, priority]), so the old
// rigid 6-field assertions no longer match the running app.
//
// Equivalent unit-level coverage lives in:
//   - view/tikidetail/configurable_detail_edit_test.go
//     (enter/exit edit mode, Tab traversal across editable fields,
//      stub fields skipped, change handlers fired)
//   - view/tikidetail/field_registry.go and its tests
//     (per-field editor wiring for status/type/priority)
//   - controller/detail_controller_edit_test.go
//     (Save/Cancel session lifecycle, registry swap on edit toggle)
//
// Integration coverage for the surviving Tab/Save/Cancel surface lives
// in view/tikidetail/configurable_detail_*_test.go; running it via the
// integration harness would duplicate that without exercising any
// router-only code path.
//
// Whoever revives this file should write tests for the configurable
// detail view's in-place edit flow (e on a kind: detail view → Tab →
// Down/Up → Ctrl+S).

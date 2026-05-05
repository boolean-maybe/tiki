package integration

// Phase 3 cleanup: this file previously held integration tests for the
// legacy built-in task edit view (TaskEditViewID): Tab/Shift-Tab
// traversal across the 6+ fields the legacy view rendered, status
// cycling via Up/Down, type toggling, assignee input, points/priority
// range checks, save/cancel from arbitrary fields, etc.
//
// After Phase 2 the canonical "edit existing task" path is in-place
// edit mode on the configurable detail view (kind: detail). The
// configurable view's metadata is workflow-declared (default
// [status, type, priority]), so the legacy 6-field assertions no
// longer match the running app and the tests as written gave false
// failures rather than coverage.
//
// Equivalent unit-level coverage lives in:
//   - view/taskdetail/configurable_detail_edit_test.go
//     (enter/exit edit mode, Tab traversal across editable fields,
//      stub fields skipped, change handlers fired)
//   - view/taskdetail/field_registry.go and its tests
//     (per-field editor wiring for status/type/priority)
//   - controller/detail_controller_edit_test.go
//     (Save/Cancel session lifecycle, registry swap on edit toggle)
//
// Integration coverage for the surviving Tab/Save/Cancel surface lives
// in view/taskdetail/configurable_detail_*_test.go; running it via the
// integration harness would duplicate that without exercising any
// router-only code path.
//
// Whoever revives this file should write tests for the configurable
// detail view's in-place edit flow (e on a kind: detail view → Tab →
// Down/Up → Ctrl+S), not the deleted TaskEditView path.

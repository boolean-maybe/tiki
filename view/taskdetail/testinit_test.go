package taskdetail

import (
	"time"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func init() {
	teststatuses.Init()
}

// newTestViewTiki returns a fully-populated tiki used by view tests. The
// shape was previously defined alongside the now-deleted legacy detail
// view tests; centralizing it here keeps the configurable-view tests
// compiling without duplicating the construction.
func newTestViewTiki(id string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Title = "Test Task"
	tk.Set(tikipkg.FieldStatus, string(taskpkg.StatusReady))
	tk.Set(tikipkg.FieldType, string(taskpkg.TypeStory))
	tk.Set(tikipkg.FieldPriority, 3)
	tk.Set(tikipkg.FieldPoints, 5)
	tk.Set(tikipkg.FieldAssignee, "user@example.com")
	tk.Set("createdBy", "creator@example.com")
	tk.CreatedAt = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	tk.UpdatedAt = time.Date(2024, 1, 2, 14, 30, 0, 0, time.UTC)
	return tk
}

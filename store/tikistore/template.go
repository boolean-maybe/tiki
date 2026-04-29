package tikistore

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

// setAuthorFromIdentity best-effort populates CreatedBy using the current
// Tiki identity (configured identity → git → OS user).
func (s *TikiStore) setAuthorFromIdentity(task *taskpkg.Task) {
	if task == nil || task.CreatedBy != "" {
		return
	}

	name, email, err := s.GetCurrentUser()
	if err != nil {
		return
	}

	switch {
	case name != "" && email != "":
		task.CreatedBy = fmt.Sprintf("%s <%s>", name, email)
	case name != "":
		task.CreatedBy = name
	case email != "":
		task.CreatedBy = email
	}
}

// NewTaskTemplate returns a new task populated with creation defaults.
// Built-in defaults: priority=3, points=1, tags=["idea"].
// Type and status come from their registries (explicit default or first entry).
// Custom field defaults come from workflow.yaml field definitions.
func (s *TikiStore) NewTaskTemplate() (*taskpkg.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := config.RequireWorkflowRegistriesLoaded(); err != nil {
		return nil, err
	}

	// Identity uniqueness is checked against the in-memory index (s.tasks),
	// not the filesystem — a task loaded from a renamed file occupies an id
	// without occupying <taskdir>/<id>.md, so an os.Stat probe would falsely
	// report the id free.
	var taskID string
	for i := 0; ; i++ {
		candidate := normalizeTaskID(config.GenerateRandomID())
		if _, taken := s.tasks[candidate]; !taken {
			taskID = candidate
			break
		}
		if i > maxGenerateRetries {
			return nil, fmt.Errorf("failed to generate unique task id after %d attempts", maxGenerateRetries)
		}
		slog.Debug("ID collision detected in index, regenerating", "id", candidate)
	}

	task := &taskpkg.Task{
		ID:         taskID,
		Status:     taskpkg.DefaultStatus(),
		Type:       taskpkg.DefaultType(),
		Priority:   3,
		Points:     1,
		Tags:       []string{"idea"},
		CreatedAt:  time.Now(),
		IsWorkflow: true,
	}

	task.CustomFields = buildCustomFieldDefaults()

	s.setAuthorFromIdentity(task)

	return task, nil
}

// buildCustomFieldDefaults collects DefaultValue from all custom field
// definitions into a map suitable for task.CustomFields.
func buildCustomFieldDefaults() map[string]interface{} {
	var defaults map[string]interface{}
	for _, fd := range workflow.Fields() {
		if !fd.Custom || fd.DefaultValue == nil {
			continue
		}
		if defaults == nil {
			defaults = make(map[string]interface{})
		}
		if ss, ok := fd.DefaultValue.([]string); ok {
			switch fd.Type {
			case workflow.TypeListRef:
				defaults[fd.Name] = collectionutil.NormalizeRefSet(ss)
			case workflow.TypeListString:
				defaults[fd.Name] = collectionutil.NormalizeStringSet(ss)
			default:
				cp := make([]string, len(ss))
				copy(cp, ss)
				defaults[fd.Name] = cp
			}
		} else {
			defaults[fd.Name] = fd.DefaultValue
		}
	}
	return defaults
}

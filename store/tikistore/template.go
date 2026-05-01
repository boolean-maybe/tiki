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

// NewTaskTemplate returns a new document populated with creation defaults.
// When the active workflow has a default status, this returns a workflow task
// (built-in defaults: priority=3, points=1, tags=["idea"]; type and status from
// the registries; custom field defaults from workflow.yaml). When no default
// status is configured, this returns a plain document with IsWorkflow=false
// and only id/createdAt populated — callers fill in title/body. The status
// registry treats a missing `default: true` as an explicit opt-out of
// workflow capture for this workflow.
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

	defaultStatus := taskpkg.DefaultStatus()
	task := &taskpkg.Task{
		ID:        taskID,
		CreatedAt: time.Now(),
	}

	if defaultStatus != "" {
		task.Status = defaultStatus
		task.Type = taskpkg.DefaultType()
		task.Priority = 3
		task.Points = 1
		task.Tags = []string{"idea"}
		task.IsWorkflow = true
		task.CustomFields = buildCustomFieldDefaults()
	}

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

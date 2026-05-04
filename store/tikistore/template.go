package tikistore

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	collectionutil "github.com/boolean-maybe/tiki/util/collections"
	"github.com/boolean-maybe/tiki/workflow"
)

func (s *TikiStore) setAuthorFromIdentityTiki(tk *tikipkg.Tiki) {
	name, email, err := s.GetCurrentUser()
	if err != nil || (name == "" && email == "") {
		return
	}
	switch {
	case name != "" && email != "":
		tk.Set("createdBy", fmt.Sprintf("%s <%s>", name, email))
	case name != "":
		tk.Set("createdBy", name)
	default:
		tk.Set("createdBy", email)
	}
}

// NewTikiTemplate returns a new tiki populated with creation defaults.
// When the active workflow has a default status, this returns a workflow tiki
// (built-in defaults: priority=3, points=1, tags=["idea"]; type and status from
// the registries; custom field defaults from workflow.yaml). When no default
// status is configured, this returns a plain tiki with only id/createdAt
// populated — callers fill in title/body. The status registry treats a missing
// `default: true` as an explicit opt-out of workflow capture for this workflow.
func (s *TikiStore) NewTikiTemplate() (*tikipkg.Tiki, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.newTikiTemplateLocked()
}

// newTikiTemplateLocked builds a template tiki. Caller must hold s.mu.
func (s *TikiStore) newTikiTemplateLocked() (*tikipkg.Tiki, error) {
	if err := config.RequireWorkflowRegistriesLoaded(); err != nil {
		return nil, err
	}

	// Identity uniqueness is checked against the in-memory index (s.tikis),
	// not the filesystem — a tiki loaded from a renamed file occupies an id
	// without occupying <taskdir>/<id>.md, so an os.Stat probe would falsely
	// report the id free.
	var taskID string
	for i := 0; ; i++ {
		candidate := normalizeTaskID(config.GenerateRandomID())
		if _, taken := s.tikis[candidate]; !taken {
			taskID = candidate
			break
		}
		if i > maxGenerateRetries {
			return nil, fmt.Errorf("failed to generate unique task id after %d attempts", maxGenerateRetries)
		}
		slog.Debug("ID collision detected in index, regenerating", "id", candidate)
	}

	tk := tikipkg.New()
	tk.ID = taskID
	tk.CreatedAt = time.Now()

	if defaultStatus := taskpkg.DefaultStatus(); defaultStatus != "" {
		tk.Set(tikipkg.FieldStatus, string(defaultStatus))
		tk.Set(tikipkg.FieldType, string(taskpkg.DefaultType()))
		tk.Set(tikipkg.FieldPriority, 3)
		tk.Set(tikipkg.FieldPoints, 1)
		tk.Set(tikipkg.FieldTags, []string{"idea"})
		for k, v := range buildCustomFieldDefaults() {
			tk.Set(k, v)
		}
	}

	s.setAuthorFromIdentityTiki(tk)

	return tk, nil
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

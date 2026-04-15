package task

import "time"

// Task represents a work item (user story, bug, etc.)
type Task struct {
	ID            string
	Title         string
	Description   string
	Type          Type
	Status        Status
	Tags          []string
	DependsOn     []string
	Due           time.Time
	Recurrence    Recurrence
	Assignee      string
	Priority      int // lower = higher priority
	Points        int
	Comments      []Comment
	CreatedBy     string // User who initially created the task
	CreatedAt     time.Time
	UpdatedAt     time.Time
	CustomFields  map[string]interface{} // user-defined fields from workflow.yaml
	UnknownFields map[string]interface{} // non-builtin keys not matching any registered custom field
	LoadedMtime   time.Time              // File mtime when loaded (for optimistic locking)
}

// Clone creates a deep copy of the task
func (t *Task) Clone() *Task {
	if t == nil {
		return nil
	}

	clone := &Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Type:        t.Type,
		Status:      t.Status,
		Priority:    t.Priority,
		Due:         t.Due,
		Recurrence:  t.Recurrence,
		Assignee:    t.Assignee,
		Points:      t.Points,
		CreatedBy:   t.CreatedBy,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		LoadedMtime: t.LoadedMtime,
	}

	// Deep copy slices
	if t.Tags != nil {
		clone.Tags = make([]string, len(t.Tags))
		copy(clone.Tags, t.Tags)
	}

	if t.DependsOn != nil {
		clone.DependsOn = make([]string, len(t.DependsOn))
		copy(clone.DependsOn, t.DependsOn)
	}

	if t.Comments != nil {
		clone.Comments = make([]Comment, len(t.Comments))
		copy(clone.Comments, t.Comments)
	}

	if t.CustomFields != nil {
		clone.CustomFields = make(map[string]interface{}, len(t.CustomFields))
		for k, v := range t.CustomFields {
			if ss, ok := v.([]string); ok {
				cp := make([]string, len(ss))
				copy(cp, ss)
				clone.CustomFields[k] = cp
			} else {
				clone.CustomFields[k] = v
			}
		}
	}

	if t.UnknownFields != nil {
		clone.UnknownFields = make(map[string]interface{}, len(t.UnknownFields))
		for k, v := range t.UnknownFields {
			if ss, ok := v.([]string); ok {
				cp := make([]string, len(ss))
				copy(cp, ss)
				clone.UnknownFields[k] = cp
			} else {
				clone.UnknownFields[k] = v
			}
		}
	}

	return clone
}

// Comment represents a comment on a task
type Comment struct {
	ID        string
	Author    string
	Text      string
	CreatedAt time.Time
}

// SearchResult represents a task match with relevance score
type SearchResult struct {
	Task  *Task
	Score float64 // relevance score (higher = better match)
}

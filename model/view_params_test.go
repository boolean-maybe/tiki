package model

import (
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

func TestTaskDetailParams_EncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		params TaskDetailParams
	}{
		{
			name:   "simple task ID",
			params: TaskDetailParams{TaskID: "TIKI-1"},
		},
		{
			name:   "task ID with hyphen",
			params: TaskDetailParams{TaskID: "TIKI-123"},
		},
		{
			name:   "task ID with special format",
			params: TaskDetailParams{TaskID: "PROJECT-999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := EncodeTaskDetailParams(tt.params)

			// Decode
			decoded := DecodeTaskDetailParams(encoded)

			// Verify round-trip
			if decoded.TaskID != tt.params.TaskID {
				t.Errorf("round-trip failed: TaskID = %q, want %q", decoded.TaskID, tt.params.TaskID)
			}
		})
	}
}

func TestTaskDetailParams_EmptyTaskID(t *testing.T) {
	// Empty task ID should encode to nil
	params := TaskDetailParams{TaskID: ""}
	encoded := EncodeTaskDetailParams(params)

	if encoded != nil {
		t.Errorf("EncodeTaskDetailParams with empty TaskID = %v, want nil", encoded)
	}

	// Decoding nil should return zero value
	decoded := DecodeTaskDetailParams(nil)
	if decoded.TaskID != "" {
		t.Errorf("DecodeTaskDetailParams(nil) TaskID = %q, want empty", decoded.TaskID)
	}
}

func TestTaskDetailParams_DecodeInvalidParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   TaskDetailParams
	}{
		{
			name:   "nil params",
			params: nil,
			want:   TaskDetailParams{},
		},
		{
			name:   "empty params",
			params: map[string]interface{}{},
			want:   TaskDetailParams{},
		},
		{
			name:   "wrong type for taskID",
			params: map[string]interface{}{"taskID": 123},
			want:   TaskDetailParams{},
		},
		{
			name:   "missing taskID",
			params: map[string]interface{}{"other": "value"},
			want:   TaskDetailParams{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeTaskDetailParams(tt.params)
			if decoded.TaskID != tt.want.TaskID {
				t.Errorf("DecodeTaskDetailParams() TaskID = %q, want %q", decoded.TaskID, tt.want.TaskID)
			}
		})
	}
}

func TestTaskEditParams_EncodeDecodeRoundTrip(t *testing.T) {
	draftTask := &taskpkg.Task{
		ID:       "TIKI-42",
		Title:    "Test Task",
		Status:   taskpkg.StatusReady,
		Type:     taskpkg.TypeStory,
		Priority: 3,
	}

	tests := []struct {
		name   string
		params TaskEditParams
	}{
		{
			name: "task ID only",
			params: TaskEditParams{
				TaskID: "TIKI-1",
			},
		},
		{
			name: "task ID with draft",
			params: TaskEditParams{
				TaskID: "TIKI-42",
				Draft:  draftTask,
			},
		},
		{
			name: "task ID with focus",
			params: TaskEditParams{
				TaskID: "TIKI-1",
				Focus:  EditFieldTitle,
			},
		},
		{
			name: "all fields",
			params: TaskEditParams{
				TaskID: "TIKI-42",
				Draft:  draftTask,
				Focus:  EditFieldDescription,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			encoded := EncodeTaskEditParams(tt.params)

			// Decode
			decoded := DecodeTaskEditParams(encoded)

			// Verify round-trip
			if decoded.TaskID != tt.params.TaskID {
				t.Errorf("round-trip failed: TaskID = %q, want %q", decoded.TaskID, tt.params.TaskID)
			}

			if tt.params.Draft != nil {
				if decoded.Draft == nil {
					t.Error("round-trip failed: Draft = nil, want non-nil")
				} else if decoded.Draft.ID != tt.params.Draft.ID {
					t.Errorf("round-trip failed: Draft.ID = %q, want %q",
						decoded.Draft.ID, tt.params.Draft.ID)
				}
			} else if decoded.Draft != nil {
				t.Error("round-trip failed: Draft != nil, want nil")
			}

			if decoded.Focus != tt.params.Focus {
				t.Errorf("round-trip failed: Focus = %v, want %v", decoded.Focus, tt.params.Focus)
			}
		})
	}
}

func TestTaskEditParams_DraftWithoutTaskID(t *testing.T) {
	// When Draft is present but TaskID is empty, TaskID should be inferred from Draft
	draftTask := &taskpkg.Task{
		ID:    "TIKI-100",
		Title: "Draft Task",
	}

	params := TaskEditParams{
		TaskID: "",
		Draft:  draftTask,
	}

	encoded := EncodeTaskEditParams(params)

	// TaskID should be inferred
	if encoded == nil {
		t.Fatal("EncodeTaskEditParams returned nil")
	}

	if encoded["taskID"] != "TIKI-100" {
		t.Errorf("taskID = %v, want TIKI-100", encoded["taskID"])
	}

	// Decoding should preserve the inference
	decoded := DecodeTaskEditParams(encoded)
	if decoded.TaskID != "TIKI-100" {
		t.Errorf("decoded TaskID = %q, want TIKI-100", decoded.TaskID)
	}
}

func TestTaskEditParams_EmptyTaskID(t *testing.T) {
	// Empty task ID and nil draft should encode to nil
	params := TaskEditParams{
		TaskID: "",
		Draft:  nil,
	}

	encoded := EncodeTaskEditParams(params)

	if encoded != nil {
		t.Errorf("EncodeTaskEditParams with empty TaskID and nil Draft = %v, want nil", encoded)
	}
}

func TestTaskEditParams_FocusStringEncoding(t *testing.T) {
	// Focus should be encoded as string for interop
	params := TaskEditParams{
		TaskID: "TIKI-1",
		Focus:  EditFieldTitle,
	}

	encoded := EncodeTaskEditParams(params)

	// Verify focus is stored as string
	focusVal, ok := encoded["focus"]
	if !ok {
		t.Fatal("focus not in encoded params")
	}

	focusStr, ok := focusVal.(string)
	if !ok {
		t.Errorf("focus type = %T, want string", focusVal)
	}

	if focusStr != string(EditFieldTitle) {
		t.Errorf("focus string = %q, want %q", focusStr, string(EditFieldTitle))
	}

	// Decoding string focus should work
	decoded := DecodeTaskEditParams(encoded)
	if decoded.Focus != EditFieldTitle {
		t.Errorf("decoded Focus = %v, want %v", decoded.Focus, EditFieldTitle)
	}
}

func TestTaskEditParams_FocusEditFieldType(t *testing.T) {
	// Decode should handle focus as EditField type too
	params := map[string]interface{}{
		"taskID": "TIKI-1",
		"focus":  EditFieldDescription, // EditField type, not string
	}

	decoded := DecodeTaskEditParams(params)

	if decoded.Focus != EditFieldDescription {
		t.Errorf("Focus = %v, want %v", decoded.Focus, EditFieldDescription)
	}
}

func TestTaskEditParams_DecodeInvalidParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   TaskEditParams
	}{
		{
			name:   "nil params",
			params: nil,
			want:   TaskEditParams{},
		},
		{
			name:   "empty params",
			params: map[string]interface{}{},
			want:   TaskEditParams{},
		},
		{
			name:   "wrong type for taskID",
			params: map[string]interface{}{"taskID": 123},
			want:   TaskEditParams{},
		},
		{
			name: "wrong type for draft",
			params: map[string]interface{}{
				"taskID":    "TIKI-1",
				"draftTask": "not a task",
			},
			want: TaskEditParams{TaskID: "TIKI-1"},
		},
		{
			name: "wrong type for focus",
			params: map[string]interface{}{
				"taskID": "TIKI-1",
				"focus":  123,
			},
			want: TaskEditParams{TaskID: "TIKI-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeTaskEditParams(tt.params)

			if decoded.TaskID != tt.want.TaskID {
				t.Errorf("TaskID = %q, want %q", decoded.TaskID, tt.want.TaskID)
			}

			if tt.want.Draft == nil && decoded.Draft != nil {
				t.Error("Draft != nil, want nil")
			}

			if tt.want.Focus == "" && decoded.Focus != "" {
				t.Errorf("Focus = %v, want empty", decoded.Focus)
			}
		})
	}
}

func TestTaskEditParams_DraftTaskIDInference(t *testing.T) {
	// When Draft has an ID but TaskID param is empty, it should be inferred
	draftTask := &taskpkg.Task{
		ID:    "TIKI-999",
		Title: "Draft",
	}

	params := map[string]interface{}{
		"taskID":    "",
		"draftTask": draftTask,
	}

	decoded := DecodeTaskEditParams(params)

	// TaskID should be inferred from Draft
	if decoded.TaskID != "TIKI-999" {
		t.Errorf("TaskID = %q, want TIKI-999 (inferred from Draft)", decoded.TaskID)
	}

	if decoded.Draft == nil {
		t.Error("Draft = nil, want non-nil")
	}
}

func TestTaskEditParams_NilDraftNoInference(t *testing.T) {
	// When Draft is nil, TaskID should not be inferred
	params := map[string]interface{}{
		"taskID":    "",
		"draftTask": (*taskpkg.Task)(nil),
	}

	decoded := DecodeTaskEditParams(params)

	if decoded.TaskID != "" {
		t.Errorf("TaskID = %q, want empty (no draft to infer from)", decoded.TaskID)
	}

	if decoded.Draft != nil {
		t.Error("Draft != nil, want nil")
	}
}

func TestViewParams_ParamKeyConstants(t *testing.T) {
	// Verify that the param keys used internally match expectations
	// This is more of a documentation test

	detailParams := EncodeTaskDetailParams(TaskDetailParams{TaskID: "TIKI-1"})
	if _, ok := detailParams["taskID"]; !ok {
		t.Error("TaskDetailParams should use 'taskID' key")
	}

	editParams := EncodeTaskEditParams(TaskEditParams{
		TaskID: "TIKI-1",
		Draft: &taskpkg.Task{
			ID:    "TIKI-1",
			Title: "Test",
		},
		Focus: EditFieldTitle,
	})

	if _, ok := editParams["taskID"]; !ok {
		t.Error("TaskEditParams should use 'taskID' key")
	}

	if _, ok := editParams["draftTask"]; !ok {
		t.Error("TaskEditParams should use 'draftTask' key for Draft")
	}

	if _, ok := editParams["focus"]; !ok {
		t.Error("TaskEditParams should use 'focus' key for Focus")
	}
}

package model

import (
	"reflect"
	"testing"

	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func newTestDraftTiki(id, title string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Title = title
	return tk
}

func TestPluginViewParams_EncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		params PluginViewParams
	}{
		{name: "simple task ID", params: PluginViewParams{TaskID: "TIKI-1"}},
		{name: "task ID with hyphen", params: PluginViewParams{TaskID: "TIKI-123"}},
		{name: "task ID with special format", params: PluginViewParams{TaskID: "PROJECT-999"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodePluginViewParams(tt.params)
			decoded := DecodePluginViewParams(encoded)
			if decoded.TaskID != tt.params.TaskID {
				t.Errorf("round-trip failed: TaskID = %q, want %q", decoded.TaskID, tt.params.TaskID)
			}
		})
	}
}

func TestPluginViewParams_EmptyTaskID(t *testing.T) {
	params := PluginViewParams{TaskID: ""}
	encoded := EncodePluginViewParams(params)
	if encoded != nil {
		t.Errorf("EncodePluginViewParams with empty TaskID = %v, want nil", encoded)
	}
	decoded := DecodePluginViewParams(nil)
	if decoded.TaskID != "" {
		t.Errorf("DecodePluginViewParams(nil) TaskID = %q, want empty", decoded.TaskID)
	}
}

func TestPluginViewParams_DecodeInvalidParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   PluginViewParams
	}{
		{"nil params", nil, PluginViewParams{}},
		{"empty params", map[string]interface{}{}, PluginViewParams{}},
		{"wrong type for taskID", map[string]interface{}{"taskID": 123}, PluginViewParams{}},
		{"missing taskID", map[string]interface{}{"other": "value"}, PluginViewParams{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodePluginViewParams(tt.params)
			if decoded.TaskID != tt.want.TaskID {
				t.Errorf("TaskID = %q, want %q", decoded.TaskID, tt.want.TaskID)
			}
		})
	}
}

func TestTaskEditParams_EncodeDecodeRoundTrip(t *testing.T) {
	draftTask := newTestDraftTiki("T1KI42", "Test Task")

	tests := []struct {
		name   string
		params TaskEditParams
	}{
		{
			name:   "task ID only",
			params: TaskEditParams{TaskID: "TIKI-1"},
		},
		{
			name:   "task ID with draft",
			params: TaskEditParams{TaskID: "T1KI42", Draft: draftTask},
		},
		{
			name:   "task ID with focus",
			params: TaskEditParams{TaskID: "TIKI-1", Focus: EditFieldTitle},
		},
		{
			name:   "all fields",
			params: TaskEditParams{TaskID: "T1KI42", Draft: draftTask, Focus: EditFieldDescription},
		},
		{
			name:   "with metadata",
			params: TaskEditParams{TaskID: "TIKI-1", Metadata: []string{"status", "type", "tags"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeTaskEditParams(tt.params)
			decoded := DecodeTaskEditParams(encoded)

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

			if !reflect.DeepEqual(decoded.Metadata, tt.params.Metadata) {
				t.Errorf("round-trip failed: Metadata = %v, want %v", decoded.Metadata, tt.params.Metadata)
			}
		})
	}
}

func TestTaskEditParams_MetadataNilEmptyAndCopySemantics(t *testing.T) {
	// nil metadata round-trips as nil.
	encoded := EncodeTaskEditParams(TaskEditParams{TaskID: "TIKI-1"})
	decoded := DecodeTaskEditParams(encoded)
	if decoded.Metadata != nil {
		t.Errorf("nil-input metadata round-trip: got %v, want nil", decoded.Metadata)
	}

	// empty metadata also round-trips as nil (encoding skips empty slice).
	encoded = EncodeTaskEditParams(TaskEditParams{TaskID: "TIKI-1", Metadata: []string{}})
	decoded = DecodeTaskEditParams(encoded)
	if len(decoded.Metadata) != 0 {
		t.Errorf("empty-input metadata round-trip: got %v, want empty", decoded.Metadata)
	}

	// encoded slice is a defensive copy — mutating the source after encode
	// must not affect what decode returns.
	src := []string{"status", "type"}
	encoded = EncodeTaskEditParams(TaskEditParams{TaskID: "TIKI-1", Metadata: src})
	src[0] = "MUTATED"
	decoded = DecodeTaskEditParams(encoded)
	if decoded.Metadata[0] != "status" {
		t.Errorf("encoded metadata not defensively copied: got %v", decoded.Metadata)
	}
}

func TestTaskEditParams_DescOnlyRoundTrip(t *testing.T) {
	params := TaskEditParams{
		TaskID:   "TIKI-1",
		Focus:    EditFieldDescription,
		DescOnly: true,
	}
	encoded := EncodeTaskEditParams(params)
	decoded := DecodeTaskEditParams(encoded)
	if !decoded.DescOnly {
		t.Error("round-trip failed: DescOnly = false, want true")
	}
	if decoded.Focus != EditFieldDescription {
		t.Errorf("round-trip failed: Focus = %v, want %v", decoded.Focus, EditFieldDescription)
	}

	paramsNoDesc := TaskEditParams{TaskID: "TIKI-2"}
	encodedNoDesc := EncodeTaskEditParams(paramsNoDesc)
	if _, exists := encodedNoDesc[paramDescOnly]; exists {
		t.Error("DescOnly=false should not be stored in encoded params")
	}
	decodedNoDesc := DecodeTaskEditParams(encodedNoDesc)
	if decodedNoDesc.DescOnly {
		t.Error("DescOnly should default to false")
	}
}

func TestTaskEditParams_DraftWithoutTaskID(t *testing.T) {
	draftTask := newTestDraftTiki("T1KI00", "Draft Task")
	params := TaskEditParams{TaskID: "", Draft: draftTask}
	encoded := EncodeTaskEditParams(params)
	if encoded == nil {
		t.Fatal("EncodeTaskEditParams returned nil")
	}
	if encoded["taskID"] != "T1KI00" {
		t.Errorf("taskID = %v, want T1KI00", encoded["taskID"])
	}
	decoded := DecodeTaskEditParams(encoded)
	if decoded.TaskID != "T1KI00" {
		t.Errorf("decoded TaskID = %q, want T1KI00", decoded.TaskID)
	}
}

func TestTaskEditParams_EmptyTaskID(t *testing.T) {
	params := TaskEditParams{TaskID: "", Draft: nil}
	encoded := EncodeTaskEditParams(params)
	if encoded != nil {
		t.Errorf("EncodeTaskEditParams with empty TaskID and nil Draft = %v, want nil", encoded)
	}
}

func TestTaskEditParams_FocusStringEncoding(t *testing.T) {
	params := TaskEditParams{TaskID: "TIKI-1", Focus: EditFieldTitle}
	encoded := EncodeTaskEditParams(params)
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
	decoded := DecodeTaskEditParams(encoded)
	if decoded.Focus != EditFieldTitle {
		t.Errorf("decoded Focus = %v, want %v", decoded.Focus, EditFieldTitle)
	}
}

func TestTaskEditParams_FocusEditFieldType(t *testing.T) {
	params := map[string]interface{}{
		"taskID": "TIKI-1",
		"focus":  EditFieldDescription,
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
		{"nil params", nil, TaskEditParams{}},
		{"empty params", map[string]interface{}{}, TaskEditParams{}},
		{"wrong type for taskID", map[string]interface{}{"taskID": 123}, TaskEditParams{}},
		{
			"wrong type for draft",
			map[string]interface{}{"taskID": "TIKI-1", "draftTask": "not a tiki"},
			TaskEditParams{TaskID: "TIKI-1"},
		},
		{
			"wrong type for focus",
			map[string]interface{}{"taskID": "TIKI-1", "focus": 123},
			TaskEditParams{TaskID: "TIKI-1"},
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
	draftTask := newTestDraftTiki("T1KI99", "Draft")
	params := map[string]interface{}{"taskID": "", "draftTask": draftTask}
	decoded := DecodeTaskEditParams(params)
	if decoded.TaskID != "T1KI99" {
		t.Errorf("TaskID = %q, want T1KI99 (inferred from Draft)", decoded.TaskID)
	}
	if decoded.Draft == nil {
		t.Error("Draft = nil, want non-nil")
	}
}

func TestTaskEditParams_NilDraftNoInference(t *testing.T) {
	params := map[string]interface{}{"taskID": "", "draftTask": (*tikipkg.Tiki)(nil)}
	decoded := DecodeTaskEditParams(params)
	if decoded.TaskID != "" {
		t.Errorf("TaskID = %q, want empty (no draft to infer from)", decoded.TaskID)
	}
	if decoded.Draft != nil {
		t.Error("Draft != nil, want nil")
	}
}

func TestViewParams_ParamKeyConstants(t *testing.T) {
	detailParams := EncodePluginViewParams(PluginViewParams{TaskID: "TIKI-1"})
	if _, ok := detailParams["taskID"]; !ok {
		t.Error("PluginViewParams should use 'taskID' key")
	}
	draft := newTestDraftTiki("TIKI01", "Test")
	editParams := EncodeTaskEditParams(TaskEditParams{TaskID: "TIKI01", Draft: draft, Focus: EditFieldTitle})
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

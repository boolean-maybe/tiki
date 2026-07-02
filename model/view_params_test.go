package model

import (
	"testing"

	"github.com/boolean-maybe/tiki/plugin"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

func newTestDraftTiki(id, title string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.SetID(id)
	tk.SetTitle(title)
	return tk
}

func TestPluginViewParams_EncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		params PluginViewParams
	}{
		{name: "simple tiki ID", params: PluginViewParams{TikiID: "TIKI-1"}},
		{name: "tiki ID with hyphen", params: PluginViewParams{TikiID: "TIKI-123"}},
		{name: "tiki ID with special format", params: PluginViewParams{TikiID: "PROJECT-999"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodePluginViewParams(tt.params)
			decoded := DecodePluginViewParams(encoded)
			if decoded.TikiID != tt.params.TikiID {
				t.Errorf("round-trip failed: TikiID = %q, want %q", decoded.TikiID, tt.params.TikiID)
			}
		})
	}
}

func TestPluginViewParams_EmptyTikiID(t *testing.T) {
	params := PluginViewParams{TikiID: ""}
	encoded := EncodePluginViewParams(params)
	if encoded != nil {
		t.Errorf("EncodePluginViewParams with empty TikiID = %v, want nil", encoded)
	}
	decoded := DecodePluginViewParams(nil)
	if decoded.TikiID != "" {
		t.Errorf("DecodePluginViewParams(nil) TikiID = %q, want empty", decoded.TikiID)
	}
}

func TestPluginViewParams_ModeFocusDraftRoundTrip(t *testing.T) {
	draft := newTestDraftTiki("DRAFT1", "Draft")
	tests := []struct {
		name string
		in   PluginViewParams
		want PluginViewParams
	}{
		{
			name: "plain view",
			in:   PluginViewParams{TikiID: "T1KI42"},
			want: PluginViewParams{TikiID: "T1KI42"},
		},
		{
			name: "edit mode",
			in:   PluginViewParams{TikiID: "T1KI42", Mode: plugin.DetailModeEdit},
			want: PluginViewParams{TikiID: "T1KI42", Mode: plugin.DetailModeEdit},
		},
		{
			name: "new mode with draft",
			in:   PluginViewParams{Draft: draft, Mode: plugin.DetailModeNew, Focus: EditFieldTitle},
			want: PluginViewParams{TikiID: "DRAFT1", Draft: draft, Mode: plugin.DetailModeNew, Focus: EditFieldTitle},
		},
		{
			name: "edit-desc mode",
			in:   PluginViewParams{TikiID: "T1KI42", Mode: plugin.DetailModeEditDesc, Focus: EditFieldDescription},
			want: PluginViewParams{TikiID: "T1KI42", Mode: plugin.DetailModeEditDesc, Focus: EditFieldDescription},
		},
		{
			name: "edit-tags mode",
			in:   PluginViewParams{TikiID: "T1KI42", Mode: plugin.DetailModeEditTags, Focus: EditFieldTags},
			want: PluginViewParams{TikiID: "T1KI42", Mode: plugin.DetailModeEditTags, Focus: EditFieldTags},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DecodePluginViewParams(EncodePluginViewParams(tt.in))
			if got.TikiID != tt.want.TikiID || got.Mode != tt.want.Mode || got.Focus != tt.want.Focus {
				t.Errorf("round-trip = %+v, want %+v", got, tt.want)
			}
			if (got.Draft == nil) != (tt.want.Draft == nil) {
				t.Errorf("Draft presence mismatch: got %v, want %v", got.Draft, tt.want.Draft)
			}
		})
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
		{"wrong type for tikiID", map[string]interface{}{"tikiID": 123}, PluginViewParams{}},
		{"missing tikiID", map[string]interface{}{"other": "value"}, PluginViewParams{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodePluginViewParams(tt.params)
			if decoded.TikiID != tt.want.TikiID {
				t.Errorf("TikiID = %q, want %q", decoded.TikiID, tt.want.TikiID)
			}
		})
	}
}

func TestViewParams_ParamKeyConstants(t *testing.T) {
	pluginParams := EncodePluginViewParams(PluginViewParams{TikiID: "TIKI-1"})
	if _, ok := pluginParams["tikiID"]; !ok {
		t.Error("PluginViewParams should use 'tikiID' key")
	}
	draft := newTestDraftTiki("TIKI01", "Test")
	encoded := EncodePluginViewParams(PluginViewParams{Draft: draft, Focus: EditFieldTitle, Mode: plugin.DetailModeNew})
	if _, ok := encoded["draftTiki"]; !ok {
		t.Error("PluginViewParams should use 'draftTiki' key for Draft")
	}
	if _, ok := encoded["focus"]; !ok {
		t.Error("PluginViewParams should use 'focus' key for Focus")
	}
	if _, ok := encoded["mode"]; !ok {
		t.Error("PluginViewParams should use 'mode' key for Mode")
	}
}

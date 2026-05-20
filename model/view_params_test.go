package model

import (
	"reflect"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
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

func TestTikiEditParams_EncodeDecodeRoundTrip(t *testing.T) {
	draftTiki := newTestDraftTiki("T1KI42", "Test Tiki")

	tests := []struct {
		name   string
		params TikiEditParams
	}{
		{
			name:   "tiki ID only",
			params: TikiEditParams{TikiID: "TIKI-1"},
		},
		{
			name:   "tiki ID with draft",
			params: TikiEditParams{TikiID: "T1KI42", Draft: draftTiki},
		},
		{
			name:   "tiki ID with focus",
			params: TikiEditParams{TikiID: "TIKI-1", Focus: EditFieldTitle},
		},
		{
			name:   "all fields",
			params: TikiEditParams{TikiID: "T1KI42", Draft: draftTiki, Focus: EditFieldDescription},
		},
		{
			name:   "with layout",
			params: TikiEditParams{TikiID: "TIKI-1", Layout: []string{"status", "type", "tags"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeTikiEditParams(tt.params)
			decoded := DecodeTikiEditParams(encoded)

			if decoded.TikiID != tt.params.TikiID {
				t.Errorf("round-trip failed: TikiID = %q, want %q", decoded.TikiID, tt.params.TikiID)
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

			if !reflect.DeepEqual(decoded.Layout, tt.params.Layout) {
				t.Errorf("round-trip failed: Layout = %v, want %v", decoded.Layout, tt.params.Layout)
			}
		})
	}
}

func TestTikiEditParams_LayoutNilEmptyAndCopySemantics(t *testing.T) {
	// nil layout round-trips as nil.
	encoded := EncodeTikiEditParams(TikiEditParams{TikiID: "TIKI-1"})
	decoded := DecodeTikiEditParams(encoded)
	if decoded.Layout != nil {
		t.Errorf("nil-input layout round-trip: got %v, want nil", decoded.Layout)
	}

	// empty layout also round-trips as nil (encoding skips empty slice).
	encoded = EncodeTikiEditParams(TikiEditParams{TikiID: "TIKI-1", Layout: []string{}})
	decoded = DecodeTikiEditParams(encoded)
	if len(decoded.Layout) != 0 {
		t.Errorf("empty-input layout round-trip: got %v, want empty", decoded.Layout)
	}

	// encoded slice is a defensive copy — mutating the source after encode
	// must not affect what decode returns.
	src := []string{"status", "type"}
	encoded = EncodeTikiEditParams(TikiEditParams{TikiID: "TIKI-1", Layout: src})
	src[0] = "MUTATED"
	decoded = DecodeTikiEditParams(encoded)
	if decoded.Layout[0] != "status" {
		t.Errorf("encoded layout not defensively copied: got %v", decoded.Layout)
	}
}

func TestTikiEditParams_DescOnlyRoundTrip(t *testing.T) {
	params := TikiEditParams{
		TikiID:   "TIKI-1",
		Focus:    EditFieldDescription,
		DescOnly: true,
	}
	encoded := EncodeTikiEditParams(params)
	decoded := DecodeTikiEditParams(encoded)
	if !decoded.DescOnly {
		t.Error("round-trip failed: DescOnly = false, want true")
	}
	if decoded.Focus != EditFieldDescription {
		t.Errorf("round-trip failed: Focus = %v, want %v", decoded.Focus, EditFieldDescription)
	}

	paramsNoDesc := TikiEditParams{TikiID: "TIKI-2"}
	encodedNoDesc := EncodeTikiEditParams(paramsNoDesc)
	if _, exists := encodedNoDesc[paramDescOnly]; exists {
		t.Error("DescOnly=false should not be stored in encoded params")
	}
	decodedNoDesc := DecodeTikiEditParams(encodedNoDesc)
	if decodedNoDesc.DescOnly {
		t.Error("DescOnly should default to false")
	}
}

func TestTikiEditParams_DraftWithoutTikiID(t *testing.T) {
	draftTiki := newTestDraftTiki("T1KI00", "Draft Tiki")
	params := TikiEditParams{TikiID: "", Draft: draftTiki}
	encoded := EncodeTikiEditParams(params)
	if encoded == nil {
		t.Fatal("EncodeTikiEditParams returned nil")
	}
	if encoded["tikiID"] != "T1KI00" {
		t.Errorf("tikiID = %v, want T1KI00", encoded["tikiID"])
	}
	decoded := DecodeTikiEditParams(encoded)
	if decoded.TikiID != "T1KI00" {
		t.Errorf("decoded TikiID = %q, want T1KI00", decoded.TikiID)
	}
}

func TestTikiEditParams_EmptyTikiID(t *testing.T) {
	params := TikiEditParams{TikiID: "", Draft: nil}
	encoded := EncodeTikiEditParams(params)
	if encoded != nil {
		t.Errorf("EncodeTikiEditParams with empty TikiID and nil Draft = %v, want nil", encoded)
	}
}

func TestTikiEditParams_FocusStringEncoding(t *testing.T) {
	params := TikiEditParams{TikiID: "TIKI-1", Focus: EditFieldTitle}
	encoded := EncodeTikiEditParams(params)
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
	decoded := DecodeTikiEditParams(encoded)
	if decoded.Focus != EditFieldTitle {
		t.Errorf("decoded Focus = %v, want %v", decoded.Focus, EditFieldTitle)
	}
}

func TestTikiEditParams_FocusEditFieldType(t *testing.T) {
	params := map[string]interface{}{
		"tikiID": "TIKI-1",
		"focus":  EditFieldDescription,
	}
	decoded := DecodeTikiEditParams(params)
	if decoded.Focus != EditFieldDescription {
		t.Errorf("Focus = %v, want %v", decoded.Focus, EditFieldDescription)
	}
}

func TestTikiEditParams_DecodeInvalidParams(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]interface{}
		want   TikiEditParams
	}{
		{"nil params", nil, TikiEditParams{}},
		{"empty params", map[string]interface{}{}, TikiEditParams{}},
		{"wrong type for tikiID", map[string]interface{}{"tikiID": 123}, TikiEditParams{}},
		{
			"wrong type for draft",
			map[string]interface{}{"tikiID": "TIKI-1", "draftTiki": "not a tiki"},
			TikiEditParams{TikiID: "TIKI-1"},
		},
		{
			"wrong type for focus",
			map[string]interface{}{"tikiID": "TIKI-1", "focus": 123},
			TikiEditParams{TikiID: "TIKI-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded := DecodeTikiEditParams(tt.params)
			if decoded.TikiID != tt.want.TikiID {
				t.Errorf("TikiID = %q, want %q", decoded.TikiID, tt.want.TikiID)
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

func TestTikiEditParams_DraftTikiIDInference(t *testing.T) {
	draftTiki := newTestDraftTiki("T1KI99", "Draft")
	params := map[string]interface{}{"tikiID": "", "draftTiki": draftTiki}
	decoded := DecodeTikiEditParams(params)
	if decoded.TikiID != "T1KI99" {
		t.Errorf("TikiID = %q, want T1KI99 (inferred from Draft)", decoded.TikiID)
	}
	if decoded.Draft == nil {
		t.Error("Draft = nil, want non-nil")
	}
}

func TestTikiEditParams_NilDraftNoInference(t *testing.T) {
	params := map[string]interface{}{"tikiID": "", "draftTiki": (*tikipkg.Tiki)(nil)}
	decoded := DecodeTikiEditParams(params)
	if decoded.TikiID != "" {
		t.Errorf("TikiID = %q, want empty (no draft to infer from)", decoded.TikiID)
	}
	if decoded.Draft != nil {
		t.Error("Draft != nil, want nil")
	}
}

func TestTikiEditParams_RoundTripCarriesSpec(t *testing.T) {
	spec := gridlayout.GridSpec{
		Rows: 1, Cols: 1,
		Anchors:   []gridlayout.Anchor{{Name: "title", Row: 0, Col: 0, RowSpan: 1, ColSpan: 1}},
		Stretcher: []bool{false},
		Cells:     [][]gridlayout.Cell{{gridlayout.FieldCell{Name: "title"}}},
	}
	in := TikiEditParams{TikiID: "ABC123", Spec: spec}
	encoded := EncodeTikiEditParams(in)
	out := DecodeTikiEditParams(encoded)
	if !reflect.DeepEqual(out.Spec, spec) {
		t.Fatalf("spec did not round-trip: in=%v out=%v", spec, out.Spec)
	}
}

func TestViewParams_ParamKeyConstants(t *testing.T) {
	detailParams := EncodePluginViewParams(PluginViewParams{TikiID: "TIKI-1"})
	if _, ok := detailParams["tikiID"]; !ok {
		t.Error("PluginViewParams should use 'tikiID' key")
	}
	draft := newTestDraftTiki("TIKI01", "Test")
	editParams := EncodeTikiEditParams(TikiEditParams{TikiID: "TIKI01", Draft: draft, Focus: EditFieldTitle})
	if _, ok := editParams["tikiID"]; !ok {
		t.Error("TikiEditParams should use 'tikiID' key")
	}
	if _, ok := editParams["draftTiki"]; !ok {
		t.Error("TikiEditParams should use 'draftTiki' key for Draft")
	}
	if _, ok := editParams["focus"]; !ok {
		t.Error("TikiEditParams should use 'focus' key for Focus")
	}
}

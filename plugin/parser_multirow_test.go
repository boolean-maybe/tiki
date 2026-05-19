package plugin

import (
	"testing"

	rukiRuntime "github.com/boolean-maybe/tiki/internal/ruki/runtime"
)

func TestWarnMultiRowFieldsInTikiBox(t *testing.T) {
	cfg := pluginFileConfig{
		Name:   "WithTags",
		Kind:   "board",
		Key:    "T",
		Layout: "id\ntags",
		Lanes:  []PluginLaneConfig{{Name: "L", Filter: "select"}},
	}
	_, err := parsePluginConfig(cfg, "test", rukiRuntime.NewSchema(), nil)
	if err != nil {
		t.Fatalf("expected success with warning, got error: %v", err)
	}
}

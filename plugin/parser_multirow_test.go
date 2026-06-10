package plugin

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
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

// captureWarn runs fn with a temporary slog handler and returns everything
// logged, so tests can assert presence/absence of a specific warning.
func captureWarn(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)
	fn()
	return buf.String()
}

const multiRowWarn = "only the first row will render"

// TestWarnMultiRowFieldsInTikiBox_StandaloneListWarns pins that a standalone
// list-value anchor (the genuinely multi-row case) still warns.
func TestWarnMultiRowFieldsInTikiBox_StandaloneListWarns(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{{"id"}, {"tags"}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := captureWarn(t, func() { warnMultiRowFieldsInTikiBox("Board", spec) })
	if !strings.Contains(out, multiRowWarn) {
		t.Errorf("standalone list value should warn, got: %q", out)
	}
}

// TestWarnMultiRowFieldsInTikiBox_CountDoesNotWarn pins that a `.count` anchor
// renders a single integer line, so it must NOT trigger the multi-row warning.
func TestWarnMultiRowFieldsInTikiBox_CountDoesNotWarn(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{{"id"}, {"dependsOn.count"}})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := captureWarn(t, func() { warnMultiRowFieldsInTikiBox("Board", spec) })
	if strings.Contains(out, multiRowWarn) {
		t.Errorf(".count anchor must not warn (renders one line), got: %q", out)
	}
}

// TestWarnMultiRowFieldsInTikiBox_CompositeDoesNotWarn pins that a list field
// used inside a composite (one-line comma-join / count) must NOT warn — a
// composite always renders a single line on a card.
func TestWarnMultiRowFieldsInTikiBox_CompositeDoesNotWarn(t *testing.T) {
	spec, err := gridlayout.ParseGrid([][]string{
		{"id"},
		{`<text.muted>"tags: " + tags`},
		{`dependsOn.count + " tasks"`},
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := captureWarn(t, func() { warnMultiRowFieldsInTikiBox("Board", spec) })
	if strings.Contains(out, multiRowWarn) {
		t.Errorf("composite list usage must not warn (one line), got: %q", out)
	}
}

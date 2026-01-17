package header

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
)

func TestHeaderWidget_chartVisibilityThreshold_default(t *testing.T) {
	headerConfig := model.NewHeaderConfig()
	h := NewHeaderWidget(headerConfig)
	defer h.Cleanup()

	// ensure the classic threshold is preserved when context help is small.
	h.contextHelp.width = 10
	h.rebuildLayout(119)
	if !h.chartVisible {
		t.Fatalf("expected chart visible at width=119")
	}
	h.rebuildLayout(118)
	if h.chartVisible {
		t.Fatalf("expected chart hidden at width=118")
	}
}

func TestHeaderWidget_chartVisibilityThreshold_growsWithContextHelp(t *testing.T) {
	headerConfig := model.NewHeaderConfig()
	h := NewHeaderWidget(headerConfig)
	defer h.Cleanup()

	h.contextHelp.width = 60

	h.rebuildLayout(138)
	if h.chartVisible {
		t.Fatalf("expected chart hidden at width=138 for context=60")
	}

	h.rebuildLayout(139)
	if !h.chartVisible {
		t.Fatalf("expected chart visible at width=139 for context=60")
	}
}

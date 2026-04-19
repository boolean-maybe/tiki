package header

import (
	"testing"

	"github.com/boolean-maybe/tiki/model"
)

// --- pure layout function tests ---

func TestCalculateHeaderLayout_chartVisibleAtThreshold(t *testing.T) {
	// availableBetween = 129 - 40 - 25 = 64
	// requiredContext = max(10, 40) = 40
	// required for chart = 40 + 10 + 14 = 64 → exactly fits
	layout := CalculateHeaderLayout(129, 10)
	if !layout.ChartVisible {
		t.Fatal("expected chart visible at width=129, contextHelp=10")
	}
}

func TestCalculateHeaderLayout_chartHiddenJustBelow(t *testing.T) {
	// availableBetween = 128 - 40 - 25 = 63 < 64
	layout := CalculateHeaderLayout(128, 10)
	if layout.ChartVisible {
		t.Fatal("expected chart hidden at width=128, contextHelp=10")
	}
}

func TestCalculateHeaderLayout_chartThresholdGrowsWithContextHelp(t *testing.T) {
	// contextHelpWidth=60 already >= MinContextWidth so requiredContext=60
	// required = 60 + 10 + 14 = 84; availableBetween must be >= 84
	// totalWidth = 84 + 40 + 25 = 149
	layout := CalculateHeaderLayout(149, 60)
	if !layout.ChartVisible {
		t.Fatal("expected chart visible at width=149, contextHelp=60")
	}
	layout = CalculateHeaderLayout(148, 60)
	if layout.ChartVisible {
		t.Fatal("expected chart hidden at width=148, contextHelp=60")
	}
}

func TestCalculateHeaderLayout_contextWidthWithChart(t *testing.T) {
	// width=200, contextHelp=50
	// availableBetween = 200 - 40 - 25 = 135
	// requiredContext = 50, chart required = 50+10+14 = 74 <= 135 → chart visible
	// maxContextWidth = 135 - (10+14) = 111; contextWidth = min(50, 111) = 50
	layout := CalculateHeaderLayout(200, 50)
	if !layout.ChartVisible {
		t.Fatal("expected chart visible")
	}
	if layout.ContextWidth != 50 {
		t.Errorf("contextWidth = %d, want 50", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_contextWidthClampedByAvailable(t *testing.T) {
	// width=100, contextHelp=200 (too wide)
	// availableBetween = 100 - 40 - 25 = 35
	// requiredContext = 200; chart required = 214 > 35 → chart hidden
	// maxContextWidth = 35; contextWidth clamped to 35
	layout := CalculateHeaderLayout(100, 200)
	if layout.ChartVisible {
		t.Fatal("expected chart hidden when context too wide")
	}
	if layout.ContextWidth != 35 {
		t.Errorf("contextWidth = %d, want 35", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_contextWidthFlooredAtMinContextWidth(t *testing.T) {
	// contextHelpWidth=10 < MinContextWidth=40, so requiredContext=40
	// but contextWidth itself stays at 10 (the floor only affects chart threshold)
	layout := CalculateHeaderLayout(200, 10)
	if layout.ContextWidth != 10 {
		t.Errorf("contextWidth = %d, want 10 (min floor only affects chart threshold)", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_zeroContextHelp(t *testing.T) {
	// contextHelpWidth=0: requiredContext stays 0 (guard: > 0 check)
	// required for chart = 0 + 10 + 14 = 24
	// availableBetween at width=129 = 64 >= 24 → chart visible
	layout := CalculateHeaderLayout(129, 0)
	if !layout.ChartVisible {
		t.Fatal("expected chart visible with zero-width context help")
	}
	if layout.ContextWidth != 0 {
		t.Errorf("contextWidth = %d, want 0", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_negativeContextHelp(t *testing.T) {
	layout := CalculateHeaderLayout(200, -5)
	if layout.ContextWidth != 0 {
		t.Errorf("contextWidth = %d, want 0 for negative input", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_veryNarrowTerminal(t *testing.T) {
	// width < InfoWidth + LogoWidth → availableBetween clamped to 0
	// chart cannot be visible; contextWidth = 0
	layout := CalculateHeaderLayout(40, 30)
	if layout.ChartVisible {
		t.Fatal("expected chart hidden on very narrow terminal")
	}
	if layout.ContextWidth != 0 {
		t.Errorf("contextWidth = %d, want 0 on very narrow terminal", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_exactlyInfoAndLogo(t *testing.T) {
	// width = InfoWidth + LogoWidth = 65 → availableBetween = 0
	layout := CalculateHeaderLayout(InfoWidth+LogoWidth, 10)
	if layout.ChartVisible {
		t.Fatal("expected chart hidden when no space between info and logo")
	}
	if layout.ContextWidth != 0 {
		t.Errorf("contextWidth = %d, want 0", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_chartHiddenContextFillsAvailable(t *testing.T) {
	// chart hidden; contextWidth should use full availableBetween
	// width=128, contextHelp=63
	// availableBetween = 128 - 40 - 25 = 63; chart requires 63+24=87 > 63 → hidden
	// maxContextWidth = 63; contextWidth = min(63, 63) = 63
	layout := CalculateHeaderLayout(128, 63)
	if layout.ChartVisible {
		t.Fatal("expected chart hidden")
	}
	if layout.ContextWidth != 63 {
		t.Errorf("contextWidth = %d, want 63", layout.ContextWidth)
	}
}

// --- integration tests (require widget construction) ---

func TestHeaderWidget_chartVisibilityThreshold_default(t *testing.T) {
	headerConfig := model.NewHeaderConfig()
	h := NewHeaderWidget(headerConfig, model.NewViewContext())
	defer h.Cleanup()

	h.contextHelp.width = 10
	h.rebuildLayout(129)
	if !h.chartVisible {
		t.Fatalf("expected chart visible at width=129")
	}
	h.rebuildLayout(128)
	if h.chartVisible {
		t.Fatalf("expected chart hidden at width=128")
	}
}

func TestHeaderWidget_chartVisibilityThreshold_growsWithContextHelp(t *testing.T) {
	headerConfig := model.NewHeaderConfig()
	h := NewHeaderWidget(headerConfig, model.NewViewContext())
	defer h.Cleanup()

	h.contextHelp.width = 60

	h.rebuildLayout(148)
	if h.chartVisible {
		t.Fatalf("expected chart hidden at width=148 for context=60")
	}

	h.rebuildLayout(149)
	if !h.chartVisible {
		t.Fatalf("expected chart visible at width=149 for context=60")
	}
}

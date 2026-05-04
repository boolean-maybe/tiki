package header

import "testing"

// --- pure layout function tests ---

func TestCalculateHeaderLayout_contextWidthPreserved(t *testing.T) {
	// availableBetween = 200 - 40 - 25 = 135; contextWidth=50 fits
	layout := CalculateHeaderLayout(200, 50)
	if layout.ContextWidth != 50 {
		t.Errorf("contextWidth = %d, want 50", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_contextWidthClampedByAvailable(t *testing.T) {
	// width=100, contextHelp=200 (wider than space between info and logo)
	// availableBetween = 100 - 40 - 25 = 35; contextWidth clamped to 35
	layout := CalculateHeaderLayout(100, 200)
	if layout.ContextWidth != 35 {
		t.Errorf("contextWidth = %d, want 35", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_zeroContextHelp(t *testing.T) {
	layout := CalculateHeaderLayout(129, 0)
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
	layout := CalculateHeaderLayout(40, 30)
	if layout.ContextWidth != 0 {
		t.Errorf("contextWidth = %d, want 0 on very narrow terminal", layout.ContextWidth)
	}
}

func TestCalculateHeaderLayout_exactlyInfoAndLogo(t *testing.T) {
	// width = InfoWidth + LogoWidth → availableBetween = 0
	layout := CalculateHeaderLayout(InfoWidth+LogoWidth, 10)
	if layout.ContextWidth != 0 {
		t.Errorf("contextWidth = %d, want 0", layout.ContextWidth)
	}
}

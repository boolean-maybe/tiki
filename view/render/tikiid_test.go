package render

import (
	"strings"
	"testing"

	gradcore "github.com/boolean-maybe/tiki/internal/gradient"
	"github.com/boolean-maybe/tiki/theme"
)

func TestRenderTikiIDPaint_SolidWhenGradientsDisabled(t *testing.T) {
	theme.SetTheme(theme.LoadByName("dark"))
	gradcore.UseGradients.Store(false)
	defer gradcore.UseGradients.Store(false)
	got := RenderTikiIDPaint("K3X9M2", theme.Roles())
	// solid mode shape: a single opening color tag, the literal id, and the reset suffix.
	// the opening tag may be a hex (`[#rrggbb]`) or a named color (`[deepskyblue]`),
	// so don't assume a specific tag prefix — just count tags.
	if !strings.HasSuffix(got, "[-]") || !strings.HasPrefix(got, "[") {
		t.Errorf("expected `[...]...[-]` shape, got %q", got)
	}
	if !strings.Contains(got, "K3X9M2") {
		t.Errorf("expected output to contain id %q, got %q", "K3X9M2", got)
	}
	// solid mode emits exactly one opening color tag plus the closing reset tag —
	// total 2 `[` characters.
	if strings.Count(got, "[") != 2 {
		t.Errorf("expected exactly 2 tags ([color] and [-]) in solid mode, got %d in %q", strings.Count(got, "["), got)
	}
}

func TestRenderTikiIDPaint_GradientWhenEnabled(t *testing.T) {
	theme.SetTheme(theme.LoadByName("dark"))
	gradcore.UseGradients.Store(true)
	defer gradcore.UseGradients.Store(false)
	got := RenderTikiIDPaint("AB", theme.Roles())
	// gradient mode emits one tag per rune (via gradcore.RenderTaggedGradient)
	if strings.Count(got, "[#") < 2 {
		t.Errorf("expected per-rune color tags in gradient mode, got %q", got)
	}
}

func TestRenderTikiIDPaint_Empty(t *testing.T) {
	theme.SetTheme(theme.LoadByName("dark"))
	got := RenderTikiIDPaint("", theme.Roles())
	if got != "" {
		t.Errorf("RenderTikiIDPaint(\"\") = %q, want empty", got)
	}
}

package gradient

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/gdamore/tcell/v2"
)

func TestInterpolateRGB(t *testing.T) {
	tests := []struct {
		name string
		from [3]int
		to   [3]int
		t    float64
		want [3]int
	}{
		{
			name: "t=0 returns from color",
			from: [3]int{0, 0, 0},
			to:   [3]int{100, 200, 250},
			t:    0,
			want: [3]int{0, 0, 0},
		},
		{
			name: "t=1 returns to color",
			from: [3]int{0, 0, 0},
			to:   [3]int{100, 200, 250},
			t:    1,
			want: [3]int{100, 200, 250},
		},
		{
			name: "t=0.5 midpoint with rounding",
			from: [3]int{0, 0, 0},
			to:   [3]int{100, 200, 250},
			t:    0.5,
			want: [3]int{50, 100, 125},
		},
		{
			name: "t clamped below 0",
			from: [3]int{50, 100, 150},
			to:   [3]int{100, 200, 250},
			t:    -0.5,
			want: [3]int{50, 100, 150},
		},
		{
			name: "t clamped above 1",
			from: [3]int{50, 100, 150},
			to:   [3]int{100, 200, 250},
			t:    1.5,
			want: [3]int{100, 200, 250},
		},
		{
			name: "odd value rounding",
			from: [3]int{0, 0, 0},
			to:   [3]int{99, 99, 99},
			t:    0.5,
			want: [3]int{50, 50, 50},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InterpolateRGB(tt.from, tt.to, tt.t)
			if got != tt.want {
				t.Errorf("InterpolateRGB(%v, %v, %v) = %v, want %v",
					tt.from, tt.to, tt.t, got, tt.want)
			}
		})
	}
}

func TestClampRGB(t *testing.T) {
	tests := []struct {
		name  string
		value int
		want  int
	}{
		{"below zero", -10, 0},
		{"zero", 0, 0},
		{"mid range", 128, 128},
		{"max value", 255, 255},
		{"above max", 300, 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampRGB(tt.value)
			if got != tt.want {
				t.Errorf("ClampRGB(%d) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestLightenRGB(t *testing.T) {
	tests := []struct {
		name  string
		rgb   [3]int
		ratio float64
		want  [3]int
	}{
		{
			name:  "ratio 0 no change",
			rgb:   [3]int{100, 100, 100},
			ratio: 0,
			want:  [3]int{100, 100, 100},
		},
		{
			name:  "ratio 1 full white",
			rgb:   [3]int{100, 100, 100},
			ratio: 1,
			want:  [3]int{255, 255, 255},
		},
		{
			name:  "ratio 0.5 halfway to white",
			rgb:   [3]int{100, 100, 100},
			ratio: 0.5,
			want:  [3]int{178, 178, 178},
		},
		{
			name:  "already white stays white",
			rgb:   [3]int{255, 255, 255},
			ratio: 0.5,
			want:  [3]int{255, 255, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LightenRGB(tt.rgb, tt.ratio)
			if got != tt.want {
				t.Errorf("LightenRGB(%v, %v) = %v, want %v",
					tt.rgb, tt.ratio, got, tt.want)
			}
		})
	}
}

func TestDarkenRGB(t *testing.T) {
	tests := []struct {
		name  string
		rgb   [3]int
		ratio float64
		want  [3]int
	}{
		{
			name:  "ratio 0 no change",
			rgb:   [3]int{100, 100, 100},
			ratio: 0,
			want:  [3]int{100, 100, 100},
		},
		{
			name:  "ratio 1 full black",
			rgb:   [3]int{100, 100, 100},
			ratio: 1,
			want:  [3]int{0, 0, 0},
		},
		{
			name:  "ratio 0.5 halfway to black",
			rgb:   [3]int{100, 100, 100},
			ratio: 0.5,
			want:  [3]int{50, 50, 50},
		},
		{
			name:  "already black stays black",
			rgb:   [3]int{0, 0, 0},
			ratio: 0.5,
			want:  [3]int{0, 0, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DarkenRGB(tt.rgb, tt.ratio)
			if got != tt.want {
				t.Errorf("DarkenRGB(%v, %v) = %v, want %v",
					tt.rgb, tt.ratio, got, tt.want)
			}
		})
	}
}

func TestRenderGradientText(t *testing.T) {
	gradient := config.Gradient{
		Start: [3]int{0, 0, 0},
		End:   [3]int{255, 255, 255},
	}

	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "empty string",
			text: "",
			want: "",
		},
		{
			name: "single character",
			text: "A",
			want: "[#000000]A",
		},
		{
			name: "two characters",
			text: "AB",
			want: "[#000000]A[#ffffff]B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderGradientText(tt.text, gradient)
			if got != tt.want {
				t.Errorf("RenderGradientText(%q, gradient) = %q, want %q",
					tt.text, got, tt.want)
			}
		})
	}
}

func TestGradientFromColor(t *testing.T) {
	fallback := config.Gradient{
		Start: [3]int{255, 0, 0},
		End:   [3]int{0, 255, 0},
	}

	t.Run("black color uses fallback", func(t *testing.T) {
		black := tcell.NewRGBColor(0, 0, 0)
		got := GradientFromColor(black, 0.5, fallback)
		if got != fallback {
			t.Errorf("GradientFromColor(black) should return fallback, got %v", got)
		}
	})

	t.Run("non-black color creates gradient", func(t *testing.T) {
		blue := tcell.NewRGBColor(0, 0, 200)
		got := GradientFromColor(blue, 0.5, fallback)

		// Should have base color and lighter version
		if got.Start != [3]int{0, 0, 200} {
			t.Errorf("Expected Start to be [0, 0, 200], got %v", got.Start)
		}

		// End should be lighter than Start
		if got.End[2] <= got.Start[2] {
			t.Errorf("Expected End[2] > Start[2], got End=%v Start=%v", got.End, got.Start)
		}
	})
}

func TestGradientFromColorVibrant(t *testing.T) {
	fallback := config.Gradient{
		Start: [3]int{255, 0, 0},
		End:   [3]int{0, 255, 0},
	}

	t.Run("black color uses fallback", func(t *testing.T) {
		black := tcell.NewRGBColor(0, 0, 0)
		got := GradientFromColorVibrant(black, 1.5, fallback)
		if got != fallback {
			t.Errorf("GradientFromColorVibrant(black) should return fallback, got %v", got)
		}
	})

	t.Run("non-black color creates boosted gradient", func(t *testing.T) {
		blue := tcell.NewRGBColor(0, 0, 100)
		got := GradientFromColorVibrant(blue, 1.5, fallback)

		// Should have base color and boosted version
		if got.Start != [3]int{0, 0, 100} {
			t.Errorf("Expected Start to be [0, 0, 100], got %v", got.Start)
		}

		// End should be boosted (150) and clamped to 150
		if got.End[2] != 150 {
			t.Errorf("Expected End[2] to be 150, got %v", got.End[2])
		}
	})
}

func TestInterpolateColor(t *testing.T) {
	gradient := config.Gradient{
		Start: [3]int{0, 0, 0},
		End:   [3]int{100, 200, 250},
	}

	color := InterpolateColor(gradient, 0.5)
	r, g, b := color.RGB()

	// Should match InterpolateRGB result
	if r != 50 || g != 100 || b != 125 {
		t.Errorf("InterpolateColor returned RGB(%d, %d, %d), want RGB(50, 100, 125)",
			r, g, b)
	}
}

func TestRenderAdaptiveGradientText(t *testing.T) {
	gradient := config.Gradient{
		Start: [3]int{30, 144, 255}, // Dodger Blue
		End:   [3]int{0, 191, 255},  // Deep Sky Blue
	}
	fallback := tcell.NewRGBColor(0, 191, 255) // Deep Sky Blue

	tests := []struct {
		name         string
		text         string
		useGradients bool
		checkSolid   bool // If true, verify result is a solid color
	}{
		{
			name:         "empty string with gradients enabled",
			text:         "",
			useGradients: true,
			checkSolid:   false,
		},
		{
			name:         "empty string with gradients disabled",
			text:         "",
			useGradients: false,
			checkSolid:   false,
		},
		{
			name:         "gradients enabled renders full gradient",
			text:         "TIKI-123",
			useGradients: true,
			checkSolid:   false,
		},
		{
			name:         "gradients disabled renders solid color",
			text:         "TIKI-123",
			useGradients: false,
			checkSolid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set gradient flag
			config.UseGradients = tt.useGradients

			got := RenderAdaptiveGradientText(tt.text, gradient, fallback)

			// Empty string should return empty
			if tt.text == "" {
				if got != "" {
					t.Errorf("Expected empty result for empty text, got %q", got)
				}
				return
			}

			// Check if we got a solid color when gradients disabled
			if tt.checkSolid {
				// Solid color format: [#rrggbb]text
				// Should not have multiple color codes
				expected := "[#00bfff]" + tt.text // Deep Sky Blue fallback
				if got != expected {
					t.Errorf("Expected solid color %q, got %q", expected, got)
				}
			} else if tt.useGradients {
				// When gradients enabled, should have multiple color codes
				// Check that result contains the text and color codes
				if len(got) <= len(tt.text) {
					t.Errorf("Expected gradient text longer than input, got %q", got)
				}
			}
		})
	}
}

func TestGradientCacheHit(t *testing.T) {
	ResetGradientCache()

	grad := config.Gradient{
		Start: [3]int{0, 100, 200},
		End:   [3]int{255, 200, 100},
	}

	first := RenderGradientText("TIKI-ABC", grad)
	second := RenderGradientText("TIKI-ABC", grad)

	if first != second {
		t.Errorf("cache miss: first %q != second %q", first, second)
	}

	// different text should produce different result
	other := RenderGradientText("TIKI-XYZ", grad)
	if first == other {
		t.Errorf("different text should produce different gradient output")
	}

	// reset should clear cache (no panic, and subsequent call still works)
	ResetGradientCache()
	afterReset := RenderGradientText("TIKI-ABC", grad)
	if afterReset != first {
		t.Errorf("result after cache reset differs: %q vs %q", afterReset, first)
	}
}

func BenchmarkRenderGradientText(b *testing.B) {
	grad := config.Gradient{
		Start: [3]int{30, 144, 255},
		End:   [3]int{0, 191, 255},
	}
	text := "TIKI-A1B2C3"

	b.Run("cold", func(b *testing.B) {
		for b.Loop() {
			ResetGradientCache()
			RenderGradientText(text, grad)
		}
	})

	b.Run("warm", func(b *testing.B) {
		ResetGradientCache()
		RenderGradientText(text, grad) // prime cache
		b.ResetTimer()
		for b.Loop() {
			RenderGradientText(text, grad)
		}
	})
}

func TestAdaptiveGradientRespectConfig(t *testing.T) {
	gradient := config.Gradient{
		Start: [3]int{100, 100, 100},
		End:   [3]int{200, 200, 200},
	}
	fallback := tcell.NewRGBColor(200, 200, 200)
	text := "Test"

	// Test toggle behavior
	config.UseGradients = true
	resultWithGradients := RenderAdaptiveGradientText(text, gradient, fallback)

	config.UseGradients = false
	resultWithoutGradients := RenderAdaptiveGradientText(text, gradient, fallback)

	// Results should be different
	if resultWithGradients == resultWithoutGradients {
		t.Errorf("Expected different results when UseGradients changes, both returned: %q", resultWithGradients)
	}

	// Without gradients should be shorter (single color code)
	if len(resultWithoutGradients) >= len(resultWithGradients) {
		t.Errorf("Expected solid color result to be shorter than gradient result")
	}
}

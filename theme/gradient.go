package theme

// Gradient defines a start and end RGB color for a gradient transition.
type Gradient struct {
	Start [3]int // R, G, B (0-255)
	End   [3]int // R, G, B (0-255)
}

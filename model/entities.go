package model

// Column represents a board column with its status mapping
type Column struct {
	ID       string
	Name     string
	Status   string // which status this column displays
	Position int    // display order (left to right)
}

// ViewMode represents the display mode for task boxes
type ViewMode string

const (
	ViewModeCompact  ViewMode = "compact"  // 3-line display (5 total height with border)
	ViewModeExpanded ViewMode = "expanded" // 7-line display (9 total height with border)
)

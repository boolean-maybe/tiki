package config

// UI Dimension Constants
// These constants define the sizing and spacing for terminal UI components.

const (
	// Task box heights
	TaskBoxHeight         = 5 // Compact view mode
	TaskBoxHeightExpanded = 9 // Expanded view mode

	// Task box width padding
	TaskBoxPaddingCompact  = 12 // Width padding in compact mode
	TaskBoxPaddingExpanded = 4  // Width padding in expanded mode
	TaskBoxMinWidth        = 10 // Minimum width fallback

	// Search box dimensions
	SearchBoxHeight = 3

	// Note: Header dimensions are already centralized in view/header/header.go:
	// HeaderHeight, HeaderColumnSpacing, StatsWidth, ChartWidth, LogoWidth
)

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

	// Input box dimensions
	InputBoxHeight = 3

	// TaskList default visible rows
	TaskListDefaultMaxRows = 10

	// TaskList max rows when displayed inside the metadata box
	TaskListMetadataMaxRows = 4

	// Metadata box responsive layout
	MetadataSectionMinWidth = 30 // left-side section (status/people/due) min width
	MetadataDepMinWidth     = 30 // shedding threshold for Depends On (TaskList truncates gracefully)
	MetadataBlkMinWidth     = 30 // shedding threshold for Blocks (TaskList truncates gracefully)
	// Note: Header dimensions are already centralized in view/header/header.go:
	// HeaderHeight, HeaderColumnSpacing, InfoWidth, LogoWidth
)

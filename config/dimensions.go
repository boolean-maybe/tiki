package config

// UI Dimension Constants
// These constants define the sizing and spacing for terminal UI components.

const (
	// Tiki box heights
	TikiBoxHeight         = 5 // Compact view mode
	TikiBoxHeightExpanded = 9 // Expanded view mode

	// Tiki box width padding
	TikiBoxPaddingCompact  = 12 // Width padding in compact mode
	TikiBoxPaddingExpanded = 4  // Width padding in expanded mode
	TikiBoxMinWidth        = 10 // Minimum width fallback

	// Input box dimensions
	InputBoxHeight = 3

	// TikiList default visible rows
	TikiListDefaultMaxRows = 10

	// TikiList max rows when displayed inside the metadata box
	TikiListMetadataMaxRows = 4
	// Note: Header dimensions are already centralized in view/header/header.go:
	// HeaderHeight, HeaderColumnSpacing, InfoWidth, LogoWidth
)

package renderer

import (
	"github.com/boolean-maybe/tiki/util"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/styles"
)

// MarkdownRenderer defines the interface for rendering markdown text
type MarkdownRenderer interface {
	Render(text string) (string, error)
}

// GlamourRenderer implements MarkdownRenderer using the charmbracelet/glamour library
type GlamourRenderer struct {
	renderer      *glamour.TermRenderer
	ansiConverter *util.AnsiConverter
	useTviewANSI  bool // if true, use tview.TranslateANSI; if false, use custom converter
}

// NewGlamourRenderer creates a new GlamourRenderer with custom styles
func NewGlamourRenderer() (*GlamourRenderer, error) {
	return NewGlamourRendererWithOptions(true) // default: use custom ANSI converter
}

// NewGlamourRendererWithOptions creates a new GlamourRenderer with options
// useCustomANSI: if true, use custom ANSI converter (preserves backgrounds); if false, use tview.TranslateANSI
func NewGlamourRendererWithOptions(useCustomANSI bool) (*GlamourRenderer, error) {
	// customize glamour style to remove margins
	style := styles.DarkStyleConfig
	zero := uint(0)
	style.Document.Margin = &zero
	style.CodeBlock.Margin = &zero

	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(style),
		glamour.WithWordWrap(0), // let tview handle wrapping
	)
	if err != nil {
		return nil, err
	}

	return &GlamourRenderer{
		renderer:      r,
		ansiConverter: util.NewAnsiConverter(useCustomANSI),
		useTviewANSI:  !useCustomANSI,
	}, nil
}

// Render renders markdown text to ANSI string, then converts to tview format
func (g *GlamourRenderer) Render(text string) (string, error) {
	// First render markdown to ANSI
	ansiOutput, err := g.renderer.Render(text)
	if err != nil {
		return "", err
	}

	// Convert ANSI to tview format
	if g.useTviewANSI {
		// Use tview's built-in converter (doesn't handle background colors)
		return ansiOutput, nil // taskdetail.go will call tview.TranslateANSI
	}

	// Use custom converter (handles background colors properly)
	return g.ansiConverter.Convert(ansiOutput), nil
}

// FallbackRenderer implements MarkdownRenderer returning plain text
type FallbackRenderer struct{}

// Render returns the text as is
func (f *FallbackRenderer) Render(text string) (string, error) {
	return text, nil
}

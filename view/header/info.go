package header

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/tiki/config"

	"github.com/rivo/tview"
)

// InfoWidget displays the current view name and description in the header
type InfoWidget struct {
	*tview.TextView
}

// NewInfoWidget creates a new info display widget
func NewInfoWidget() *InfoWidget {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignLeft)
	tv.SetWrap(true)
	tv.SetWordWrap(true)
	tv.SetBorderPadding(0, 0, 1, 0)

	return &InfoWidget{TextView: tv}
}

// SetViewInfo updates the displayed view name and description
func (iw *InfoWidget) SetViewInfo(name, description string) {
	colors := config.GetColors()

	// convert "[orange]" to "[orange::b]" for bold name
	boldColor := makeBold(colors.HeaderInfoLabel)

	separator := strings.Repeat("─", InfoWidth)

	var text string
	if description != "" {
		text = fmt.Sprintf("%s%s[-::-]\n%s%s[-]\n%s%s", boldColor, name, colors.HeaderInfoSeparator, separator, colors.HeaderInfoDesc, description)
	} else {
		text = fmt.Sprintf("%s%s[-::-]", boldColor, name)
	}

	iw.SetText(text)
}

// makeBold converts a tview color tag like "[orange]" to "[orange::b]"
func makeBold(colorTag string) string {
	return strings.TrimSuffix(colorTag, "]") + "::b]"
}

// Primitive returns the underlying tview primitive
func (iw *InfoWidget) Primitive() tview.Primitive {
	return iw.TextView
}

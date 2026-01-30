package view

import (
	"strings"

	"github.com/boolean-maybe/tiki/config"

	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	navutil "github.com/boolean-maybe/navidown/util"
)

// NavigableMarkdown wraps navidown TextViewViewer with link/anchor handling.
type NavigableMarkdown struct {
	viewer        *navtview.TextViewViewer
	provider      nav.ContentProvider
	searchRoots   []string
	onStateChange func()
}

// NavigableMarkdownConfig configures a NavigableMarkdown component.
type NavigableMarkdownConfig struct {
	Provider      nav.ContentProvider
	SearchRoots   []string
	OnStateChange func() // called on navigation state changes
}

// NewNavigableMarkdown creates a new navigable markdown viewer.
func NewNavigableMarkdown(cfg NavigableMarkdownConfig) *NavigableMarkdown {
	nm := &NavigableMarkdown{
		viewer:        navtview.NewTextView(),
		provider:      cfg.Provider,
		searchRoots:   cfg.SearchRoots,
		onStateChange: cfg.OnStateChange,
	}
	nm.viewer.SetAnsiConverter(navutil.NewAnsiConverter(true))
	nm.viewer.SetRenderer(nav.NewANSIRendererWithStyle(config.GetEffectiveTheme()))
	nm.viewer.SetBackgroundColor(config.GetContentBackgroundColor())
	nm.viewer.SetStateChangedHandler(func(_ *navtview.TextViewViewer) {
		if nm.onStateChange != nil {
			nm.onStateChange()
		}
	})
	nm.setupSelectHandler()
	return nm
}

func (nm *NavigableMarkdown) setupSelectHandler() {
	nm.viewer.SetSelectHandler(func(v *navtview.TextViewViewer, elem nav.NavElement) {
		if elem.Type != nav.NavElementURL {
			return
		}
		// Internal anchor (same file)
		if elem.IsInternalLink() {
			v.ScrollToAnchor(elem.AnchorTarget(), true)
			return
		}
		// Cross-file (possibly with anchor)
		path, fragment := splitURLFragment(elem.URL)
		content, err := nm.provider.FetchContent(nav.NavElement{
			URL:            path,
			SourceFilePath: elem.SourceFilePath,
			Type:           elem.Type,
		})
		if err != nil {
			v.SetMarkdown(FormatErrorContent(err))
			return
		}
		if content == "" {
			return
		}
		v.SetMarkdownWithSource(content, nm.resolveSourcePath(path, elem.SourceFilePath), true)
		if fragment != "" {
			v.ScrollToAnchor(fragment, false)
		}
	})
}

func (nm *NavigableMarkdown) resolveSourcePath(url, sourceFile string) string {
	if sourceFile == "" {
		return url
	}
	resolved, err := nav.ResolveMarkdownPath(url, sourceFile, nm.searchRoots)
	if err != nil || resolved == "" {
		return url
	}
	return resolved
}

// Viewer returns the underlying TextViewViewer for layout embedding.
func (nm *NavigableMarkdown) Viewer() *navtview.TextViewViewer {
	return nm.viewer
}

// CanGoBack returns true if back navigation is available.
func (nm *NavigableMarkdown) CanGoBack() bool {
	return nm.viewer.Core().CanGoBack()
}

// CanGoForward returns true if forward navigation is available.
func (nm *NavigableMarkdown) CanGoForward() bool {
	return nm.viewer.Core().CanGoForward()
}

// SourceFilePath returns the current source file path.
func (nm *NavigableMarkdown) SourceFilePath() string {
	return nm.viewer.Core().SourceFilePath()
}

// SetMarkdown sets markdown content without source context.
func (nm *NavigableMarkdown) SetMarkdown(content string) {
	nm.viewer.SetMarkdown(content)
}

// SetMarkdownWithSource sets markdown content with source context.
func (nm *NavigableMarkdown) SetMarkdownWithSource(content, source string, pushHistory bool) {
	nm.viewer.SetMarkdownWithSource(content, source, pushHistory)
}

// SetStateChangedHandler sets the callback for navigation state changes.
// This is useful when the handler needs to reference the NavigableMarkdown instance.
func (nm *NavigableMarkdown) SetStateChangedHandler(handler func()) {
	nm.onStateChange = handler
}

func splitURLFragment(url string) (path, fragment string) {
	path, fragment, _ = strings.Cut(url, "#")
	return path, fragment
}

// FormatErrorContent formats an error as markdown content.
func FormatErrorContent(err error) string {
	return "# Error\n\n```\n" + err.Error() + "\n```"
}

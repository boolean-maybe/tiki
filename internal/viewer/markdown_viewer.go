package viewer

import (
	"fmt"
	"io"
	"os"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	navutil "github.com/boolean-maybe/navidown/util"
	"github.com/boolean-maybe/tiki/config"
	"github.com/rivo/tview"
)

// Markdown viewer runner: loads content from input spec and renders it with
// navidown, allowing in-document link navigation for file and url sources.

func Run(input InputSpec) error {
	if _, err := config.LoadConfig(); err != nil {
		return err
	}

	app := tview.NewApplication()
	viewer := navtview.NewTextView()
	viewer.SetAnsiConverter(navutil.NewAnsiConverter(true))
	viewer.SetRenderer(nav.NewANSIRendererWithStyle(config.GetEffectiveTheme()))
	viewer.SetBackgroundColor(config.GetContentBackgroundColor())

	provider := &loaders.FileHTTP{SearchRoots: input.SearchRoots}

	content, sourcePath, err := loadInitialContent(input, provider)
	if err != nil {
		content = formatErrorContent(err)
	}

	if sourcePath != "" {
		viewer.SetMarkdownWithSource(content, sourcePath, false)
	} else {
		viewer.SetMarkdown(content)
	}

	viewer.SetSelectHandler(func(v *navtview.TextViewViewer, elem nav.NavElement) {
		if elem.Type != nav.NavElementURL {
			return
		}
		content, err := provider.FetchContent(elem)
		if err != nil {
			v.SetMarkdown(formatErrorContent(err))
			return
		}
		if content == "" {
			return
		}
		v.SetMarkdownWithSource(content, resolveSourcePath(elem, input.SearchRoots), true)
	})

	app.SetRoot(viewer, true).EnableMouse(false)
	if err := app.Run(); err != nil {
		return fmt.Errorf("viewer error: %w", err)
	}
	return nil
}

func loadInitialContent(input InputSpec, provider *loaders.FileHTTP) (string, string, error) {
	if input.Kind == InputStdin {
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", "", fmt.Errorf("read stdin: %w", err)
		}
		if len(content) == 0 {
			return "", "", fmt.Errorf("stdin is empty")
		}
		return string(content), "", nil
	}

	if len(input.Candidates) == 0 {
		return "", "", fmt.Errorf("no input candidates provided")
	}

	var lastErr error
	for _, candidate := range input.Candidates {
		content, err := provider.FetchContent(nav.NavElement{URL: candidate})
		if err != nil {
			lastErr = err
			continue
		}
		if content == "" {
			lastErr = fmt.Errorf("no content found for %s", candidate)
			continue
		}
		return content, resolveInitialSource(candidate, input.SearchRoots), nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to load content")
	}
	return "", "", lastErr
}

func resolveInitialSource(candidate string, searchRoots []string) string {
	if len(searchRoots) == 0 {
		return candidate
	}
	resolved, err := nav.ResolveMarkdownPath(candidate, "", searchRoots)
	if err != nil || resolved == "" {
		return candidate
	}
	return resolved
}

func resolveSourcePath(elem nav.NavElement, searchRoots []string) string {
	if elem.SourceFilePath == "" {
		return elem.URL
	}
	resolved, err := nav.ResolveMarkdownPath(elem.URL, elem.SourceFilePath, searchRoots)
	if err != nil || resolved == "" {
		return elem.URL
	}
	return resolved
}

func formatErrorContent(err error) string {
	return "# Error\n\n```\n" + err.Error() + "\n```"
}

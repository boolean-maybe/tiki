package viewer

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	navtview "github.com/boolean-maybe/navidown/navidown/tview"
	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/util"
	"github.com/boolean-maybe/tiki/view/dokiindex"
	"github.com/boolean-maybe/tiki/view/markdown"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Markdown viewer runner: loads content from input spec and renders it with
// navidown, allowing in-document link navigation for file and url sources.

func Run(input InputSpec) error {
	if _, err := config.LoadConfig(); err != nil {
		return err
	}

	app := tview.NewApplication()
	provider := &loaders.FileHTTP{SearchRoots: input.SearchRoots}

	// create status bar
	statusBar := tview.NewTextView()
	statusBar.SetDynamicColors(true)
	statusBar.SetTextAlign(tview.AlignLeft)

	// Set up image rendering for Kitty-compatible terminals
	resolver := nav.NewImageResolver(input.SearchRoots)
	resolver.SetDarkMode(!config.IsLightTheme())
	imgMgr := navtview.NewImageManager(resolver, 8, 16)
	imgMgr.SetMaxRows(config.GetMaxImageRows())
	imgMgr.SetSupported(util.SupportsKittyGraphics())

	// Create NavigableMarkdown - OnStateChange is set after creation to avoid forward reference
	md := markdown.NewNavigableMarkdown(markdown.NavigableMarkdownConfig{
		Provider:       provider,
		SearchRoots:    input.SearchRoots,
		ImageManager:   imgMgr,
		MermaidOptions: &nav.MermaidOptions{},
	})
	defer md.Close()
	md.SetStateChangedHandler(func() {
		updateStatusBar(statusBar, md.Viewer())
	})

	content, sourcePath, err := loadInitialContent(input, provider)
	if err != nil {
		content = markdown.FormatErrorContent(err)
	} else {
		content = dokiindex.InjectTags(content, sourcePath)
	}

	if sourcePath != "" {
		md.SetMarkdownWithSource(content, sourcePath, false)
	} else {
		md.SetMarkdown(content)
	}

	// initial status bar update
	updateStatusBar(statusBar, md.Viewer())

	// create flex layout with status bar
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(md.Viewer(), 0, 1, true).
		AddItem(statusBar, 1, 0, false)

	// key handlers
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q':
			app.Stop()
			return nil
		case 'r':
			refreshContent(app, md, provider)
			return nil
		case 'e':
			srcPath := md.SourceFilePath()
			if srcPath == "" || strings.HasPrefix(srcPath, "http://") || strings.HasPrefix(srcPath, "https://") {
				return nil
			}
			var editorErr error
			app.Suspend(func() {
				editorErr = util.OpenInEditor(srcPath)
			})
			if editorErr != nil {
				slog.Error("failed to open editor", "file", srcPath, "error", editorErr)
				return nil
			}
			// reload content after editor exits successfully
			data, err := os.ReadFile(srcPath)
			if err != nil {
				slog.Error("failed to reload file after edit", "file", srcPath, "error", err)
				return nil
			}
			md.SetMarkdownWithSource(string(data), srcPath, false)
			updateStatusBar(statusBar, md.Viewer())
			return nil
		}
		return event
	})

	app.SetRoot(flex, true).EnableMouse(false)
	if err := app.Run(); err != nil {
		return fmt.Errorf("viewer error: %w", err)
	}
	return nil
}

// refreshContent clears image/diagram caches, re-reads the current file from disk, and re-renders.
func refreshContent(app *tview.Application, md *markdown.NavigableMarkdown, provider *loaders.FileHTTP) {
	srcPath := md.SourceFilePath()
	if srcPath == "" {
		return // stdin content — nothing to reload
	}

	content, err := provider.FetchContent(nav.NavElement{URL: srcPath})
	if err != nil {
		content = markdown.FormatErrorContent(err)
	} else {
		content = dokiindex.InjectTags(content, srcPath)
	}

	// one-shot before-draw to get the screen for Kitty image purge
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		md.Viewer().InvalidateForDocument(screen)
		md.SetMarkdownWithSource(content, srcPath, false)
		app.SetBeforeDrawFunc(nil)
		return false
	})
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

	// image files: wrap in synthetic markdown so the image pipeline renders them
	if input.Kind == InputImage || (input.Kind == InputURL && isImageFile(input.Raw)) {
		src := input.Candidates[0]
		return fmt.Sprintf("![%s](%s)\n", filepath.Base(src), src), src, nil
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

// updateStatusBar refreshes the status bar with current viewer state.
func updateStatusBar(statusBar *tview.TextView, v *navtview.TextViewViewer) {
	core := v.Core()
	srcPath := core.SourceFilePath()
	fileName := filepath.Base(srcPath)
	if fileName == "" || fileName == "." {
		fileName = "tiki"
	}

	canBack := core.CanGoBack()
	canForward := core.CanGoForward()

	colors := config.GetColors()
	labelColor := colors.TaskBoxTitleColor.Hex()
	keyColor := colors.CompletionHintColor.Hex()
	activeColor := colors.ContentTextColor.Hex()
	mutedColor := colors.CompletionHintColor.Hex()
	accentColor := colors.HeaderInfoLabel.Tag().Bold().String()
	status := fmt.Sprintf(" %s%s[-] | [%s]Link:[-][%s]Tab/Shift-Tab[-] | [%s]Back:[-]", accentColor, fileName, labelColor, keyColor, labelColor)
	if canBack {
		status += fmt.Sprintf("[%s]◀[-]", activeColor)
	} else {
		status += fmt.Sprintf("[%s]◀[-]", mutedColor)
	}
	status += fmt.Sprintf(" [%s]Fwd:[-]", labelColor)
	if canForward {
		status += fmt.Sprintf("[%s]▶[-]", activeColor)
	} else {
		status += fmt.Sprintf("[%s]▶[-]", mutedColor)
	}
	status += fmt.Sprintf(" | [%s]Scroll:[-][%s]j/k[-] [%s]Top/End:[-][%s]g/G[-] [%s]Refresh:[-][%s]r[-] [%s]Edit:[-][%s]e[-] [%s]Quit:[-][%s]q[-]", labelColor, keyColor, labelColor, keyColor, labelColor, keyColor, labelColor, keyColor, labelColor, keyColor)

	statusBar.SetText(status)
}

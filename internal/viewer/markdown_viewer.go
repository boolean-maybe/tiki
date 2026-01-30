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
	"github.com/boolean-maybe/tiki/view"
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

	// Create NavigableMarkdown - OnStateChange is set after creation to avoid forward reference
	md := view.NewNavigableMarkdown(view.NavigableMarkdownConfig{
		Provider:    provider,
		SearchRoots: input.SearchRoots,
	})
	md.SetStateChangedHandler(func() {
		updateStatusBar(statusBar, md.Viewer())
	})

	content, sourcePath, err := loadInitialContent(input, provider)
	if err != nil {
		content = view.FormatErrorContent(err)
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

	keyColor := "gray"
	activeColor := "white"
	status := fmt.Sprintf(" [yellow]%s[-] | Link:[%s]Tab/Shift-Tab[-] | Back:", fileName, keyColor)
	if canBack {
		status += fmt.Sprintf("[%s]◀[-]", activeColor)
	} else {
		status += "[gray]◀[-]"
	}
	status += " Fwd:"
	if canForward {
		status += fmt.Sprintf("[%s]▶[-]", activeColor)
	} else {
		status += "[gray]▶[-]"
	}
	status += fmt.Sprintf(" | Scroll:[%s]j/k[-] Top/End:[%s]g/G[-] Edit:[%s]e[-] Quit:[%s]q[-]", keyColor, keyColor, keyColor, keyColor)

	statusBar.SetText(status)
}

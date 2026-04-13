package taskdetail

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// taskDescriptionProvider is a ContentProvider for task detail descriptions.
// It resolves tiki IDs (e.g., "TIKI-ABC123") from the store and delegates
// file-based links to FileHTTP.
type taskDescriptionProvider struct {
	store    store.Store
	fileHTTP *loaders.FileHTTP
}

func newTaskDescriptionProvider(taskStore store.Store, searchRoots []string) *taskDescriptionProvider {
	return &taskDescriptionProvider{
		store:    taskStore,
		fileHTTP: &loaders.FileHTTP{SearchRoots: searchRoots},
	}
}

func (p *taskDescriptionProvider) FetchContent(elem nav.NavElement) (string, error) {
	if looksLikeTikiID(elem.URL) {
		task := p.store.GetTask(elem.URL)
		if task == nil {
			return "", fmt.Errorf("task %s not found", strings.ToUpper(elem.URL))
		}
		return formatTaskAsMarkdown(task), nil
	}
	return p.fileHTTP.FetchContent(elem)
}

// looksLikeTikiID checks if a URL looks like a tiki ID (TIKI-XXXXXX, case-insensitive).
func looksLikeTikiID(url string) bool {
	return taskpkg.IsValidTikiIDFormat(strings.ToUpper(strings.TrimSpace(url)))
}

// formatTaskAsMarkdown renders a task as a readable markdown document.
func formatTaskAsMarkdown(task *taskpkg.Task) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", task.Title)
	fmt.Fprintf(&b, "**%s** · %s · %s", task.ID, task.Status, task.Type)
	if task.Priority > 0 {
		fmt.Fprintf(&b, " · P%d", task.Priority)
	}
	b.WriteString("\n\n")
	if task.Description != "" {
		b.WriteString(task.Description)
		b.WriteString("\n")
	}
	return b.String()
}

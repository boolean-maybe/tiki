package taskdetail

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
	taskpkg "github.com/boolean-maybe/tiki/task"
)

// taskDescriptionProvider is a ContentProvider for task detail descriptions.
// It resolves bare document ids (`ABC123`) from the store and delegates all
// other links — including pre-unification `TIKI-*` URLs, which are no longer
// parsed as document references — to FileHTTP.
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
	// Bare 6-char URLs are ambiguous: they can be document ids OR filenames
	// (a link to `ABC123.md` or a file literally called `ABC123` on disk).
	// Try the store first; if nothing matches, fall through to FileHTTP so
	// valid file links keep working. The file resolver produces its own
	// not-found error if nothing is on disk either.
	if id, ok := extractTaskID(elem.URL); ok {
		if task := p.store.GetTask(id); task != nil {
			return formatTaskAsMarkdown(task), nil
		}
	}
	return p.fileHTTP.FetchContent(elem)
}

// extractTaskID returns the canonical bare document id for url and whether
// the URL was shaped like one. Only bare ids are recognized; the unified
// format has no legacy identity to parse.
func extractTaskID(url string) (string, bool) {
	upper := strings.ToUpper(url)
	if document.IsValidID(upper) {
		return upper, true
	}
	return "", false
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

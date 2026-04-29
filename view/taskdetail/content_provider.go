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
// It resolves document IDs (bare `ABC123` or legacy `TIKI-ABC123`) from the
// store and delegates file-based links to FileHTTP.
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
	id, shape, taskLike := extractTaskID(elem.URL)
	if taskLike {
		if task := p.store.GetTask(id); task != nil {
			return formatTaskAsMarkdown(task), nil
		}
		// Bare 6-char URLs are ambiguous: they can be document ids OR
		// filenames (a link to `ABC123.md` or a file literally called
		// `ABC123` on disk). When the store doesn't have this id, fall
		// through to FileHTTP so valid file links keep working; the
		// file-resolver will report its own not-found if there's really
		// nothing on disk either.
		//
		// Legacy `TIKI-*` URLs are NOT ambiguous — nothing else on disk
		// uses that prefix — so we preserve the stricter "report not
		// found" behavior there to give a clearer error.
		if shape == urlShapeLegacyTiki {
			return "", fmt.Errorf("task %s not found", id)
		}
	}
	return p.fileHTTP.FetchContent(elem)
}

// urlShape classifies what kind of task-reference a URL looked like when
// extractTaskID recognized it. Only the legacy TIKI- form is an unambiguous
// task-reference; a bare 6-char URL could also be a filename.
type urlShape int

const (
	urlShapeNone urlShape = iota
	urlShapeBareID
	urlShapeLegacyTiki
)

// extractTaskID returns the canonical bare document id for url, the shape
// that matched, and whether any shape matched. Phase 2 introduces bare ids
// as the authoritative form; legacy refs are still accepted so existing
// markdown docs keep resolving during the migration window.
func extractTaskID(url string) (string, urlShape, bool) {
	upper := strings.ToUpper(url)
	if document.IsValidID(upper) {
		return upper, urlShapeBareID, true
	}
	if len(upper) == document.IDLength+len("TIKI-") && strings.HasPrefix(upper, "TIKI-") {
		candidate := upper[len("TIKI-"):]
		if document.IsValidID(candidate) {
			return candidate, urlShapeLegacyTiki, true
		}
	}
	return "", urlShapeNone, false
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

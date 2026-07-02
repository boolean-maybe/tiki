package view

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	nav "github.com/boolean-maybe/navidown/navidown"
)

// stubProvider is a minimal nav.ContentProvider for exercising loadWikiContent
// without touching disk or the document store.
type stubProvider struct {
	content string
	err     error
}

func (s stubProvider) FetchContent(nav.NavElement) (string, error) {
	return s.content, s.err
}

func TestLoadWikiContent_MissingFileShowsEmptyState(t *testing.T) {
	// a wiki view pointed at index.md in a directory with no such file: the
	// inner provider reports nav.ErrFileNotFound. The user should see a
	// friendly empty-state, not the raw "failed to resolve path" error.
	prov := stubProvider{err: nav.ErrFileNotFound}

	content, _ := loadWikiContent(prov, "index.md", nil)

	if strings.Contains(content, "failed to resolve path") || strings.Contains(content, "file not found") {
		t.Fatalf("missing file should not surface the raw error, got: %q", content)
	}
	if !strings.Contains(content, "No documentation yet") {
		t.Errorf("missing file should show the empty-state placeholder, got: %q", content)
	}
	if !strings.Contains(content, "index.md") {
		t.Errorf("empty-state should name the target document, got: %q", content)
	}
}

func TestLoadWikiContent_WrappedMissingFileShowsEmptyState(t *testing.T) {
	// the FileHTTP loader wraps ErrFileNotFound with %w; errors.Is must still
	// classify it as the empty-state case through the wrapper.
	prov := stubProvider{err: fmt.Errorf("failed to resolve path %q: %w", "index.md", nav.ErrFileNotFound)}

	content, _ := loadWikiContent(prov, "index.md", nil)

	if !strings.Contains(content, "No documentation yet") {
		t.Errorf("wrapped missing file should show the empty-state placeholder, got: %q", content)
	}
}

func TestLoadWikiContent_RealErrorStillSurfaces(t *testing.T) {
	// a genuine error (not file-not-found) must still be surfaced loudly so
	// real problems are not masked by the friendly empty-state.
	prov := stubProvider{err: errors.New("permission denied")}

	content, _ := loadWikiContent(prov, "index.md", nil)

	if strings.Contains(content, "No documentation yet") {
		t.Errorf("a real error must not be masked by the empty-state, got: %q", content)
	}
	if !strings.Contains(content, "permission denied") {
		t.Errorf("a real error should be surfaced, got: %q", content)
	}
}

func TestLoadWikiContent_SuccessReturnsContent(t *testing.T) {
	prov := stubProvider{content: "# Hello\n\nbody"}

	content, _ := loadWikiContent(prov, "index.md", nil)

	if content != "# Hello\n\nbody" {
		t.Errorf("successful load should return the document verbatim, got: %q", content)
	}
}

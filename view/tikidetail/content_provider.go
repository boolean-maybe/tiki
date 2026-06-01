package tikidetail

import (
	"fmt"
	"strings"

	"github.com/boolean-maybe/navidown/loaders"
	nav "github.com/boolean-maybe/navidown/navidown"
	"github.com/boolean-maybe/ruki/idfmt"
	"github.com/boolean-maybe/tiki/store"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
)

// tikiDescriptionProvider is a ContentProvider for tiki detail descriptions.
// It resolves bare document ids (`ABC123`) from the store and delegates all
// other links — including pre-unification `TIKI-*` URLs, which are no longer
// parsed as document references — to FileHTTP.
type tikiDescriptionProvider struct {
	store    store.Store
	fileHTTP *loaders.FileHTTP
}

func newTikiDescriptionProvider(tikiStore store.Store, searchRoots []string) *tikiDescriptionProvider {
	return &tikiDescriptionProvider{
		store:    tikiStore,
		fileHTTP: &loaders.FileHTTP{SearchRoots: searchRoots},
	}
}

func (p *tikiDescriptionProvider) FetchContent(elem nav.NavElement) (string, error) {
	// Bare 6-char URLs are ambiguous: they can be document ids OR filenames
	// (a link to `ABC123.md` or a file literally called `ABC123` on disk).
	// Try the store first; if nothing matches, fall through to FileHTTP so
	// valid file links keep working. The file resolver produces its own
	// not-found error if nothing is on disk either.
	if id, ok := extractTikiID(elem.URL); ok {
		if tk := p.store.GetTiki(id); tk != nil {
			return formatTikiAsMarkdown(tk), nil
		}
	}
	return p.fileHTTP.FetchContent(elem)
}

// extractTikiID returns the canonical bare document id for url and whether
// the URL was shaped like one. Only bare ids are recognized; the unified
// format has no legacy identity to parse.
func extractTikiID(url string) (string, bool) {
	upper := strings.ToUpper(url)
	if idfmt.IsValidID(upper) {
		return upper, true
	}
	return "", false
}

// formatTikiAsMarkdown renders a tiki as a readable markdown document.
func formatTikiAsMarkdown(tk *tikipkg.Tiki) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", tk.Title())

	statusStr, _, _ := tk.StringField(tikipkg.FieldStatus)
	typeStr, _, _ := tk.StringField(tikipkg.FieldType)
	fmt.Fprintf(&b, "**%s** · %s · %s", tk.ID(), statusStr, typeStr)

	if priority, _, _ := tk.StringField(tikipkg.FieldPriority); priority != "" {
		fmt.Fprintf(&b, " · %s", priority)
	}
	b.WriteString("\n\n")
	if tk.Body() != "" {
		b.WriteString(tk.Body())
		b.WriteString("\n")
	}
	return b.String()
}

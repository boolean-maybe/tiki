package markdown

import (
	"regexp"
	"strings"

	nav "github.com/boolean-maybe/navidown/navidown"

	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store"
)

// wikilinkPattern matches `[[ID]]` where ID is a bare document id in the
// canonical shape — exactly IDLength uppercase alphanumerics, matching
// `document.IsValidID`. Anything else (obsidian-style `[[Some Page Title]]`,
// `[[README]]`, `[[image.png]]`, `[[abc123]]`) is left untouched by the
// matcher and never rewritten.
//
// The leading bracket is guarded against a preceding `!` by the replacement
// function (Go regexp has no lookbehind), so `![[ABC123]]` — obsidian-style
// image embed — is recognized and skipped rather than rewritten into a
// broken markdown image link.
//
// NOTE: the `{6}` width mirrors document.IDLength. If that constant changes,
// this pattern must move in lockstep; there is a test that asserts an
// IDLength-shaped bare id matches and an off-by-one shape does not.
var wikilinkPattern = regexp.MustCompile(`\[\[([A-Z0-9]{6})\]\]`)

// Resolver looks up a document id and returns its title and current on-disk
// path. Callers pass a concrete implementation backed by store.Store so the
// markdown package does not import store directly for non-test usage.
type Resolver interface {
	// Resolve returns the document's title, its path, and whether the id is
	// known. An unknown id must return ok=false; title/path are ignored.
	Resolve(id string) (title string, path string, ok bool)
}

// BodyResolver extends Resolver with the ability to render a full document
// body by id. Used by WikilinkProvider's bare-id short-circuit so a click on
// `[[ABC123]]` (rewritten to `[Title](ABC123)`) renders the target task
// without falling through to filesystem lookup.
//
// Implementations may return an empty body with ok=true to mean "known id,
// render nothing"; callers treat that as a blank pane rather than an error.
type BodyResolver interface {
	Resolver
	ResolveBody(id string) (body string, ok bool)
}

// StoreResolver adapts a store.ReadStore into a wikilink Resolver.
type StoreResolver struct {
	Store store.ReadStore
}

// Resolve implements Resolver against the task store. Only tasks whose ID
// matches the canonical bare form resolve — legacy prefixed IDs do not.
func (r *StoreResolver) Resolve(id string) (string, string, bool) {
	if r == nil || r.Store == nil {
		return "", "", false
	}
	if !document.IsValidID(id) {
		return "", "", false
	}
	t := r.Store.GetTask(id)
	if t == nil {
		return "", "", false
	}
	title := t.Title
	if title == "" {
		title = id
	}
	return title, r.Store.PathForID(id), true
}

// ResolveBody implements BodyResolver by rendering the task as markdown.
// Returns ok=false when the id is unknown to the store.
func (r *StoreResolver) ResolveBody(id string) (string, bool) {
	if r == nil || r.Store == nil {
		return "", false
	}
	if !document.IsValidID(id) {
		return "", false
	}
	t := r.Store.GetTask(id)
	if t == nil {
		return "", false
	}
	var b strings.Builder
	b.WriteString("# " + t.Title + "\n\n")
	b.WriteString("**" + t.ID + "**\n\n")
	if t.Description != "" {
		b.WriteString(t.Description)
		b.WriteString("\n")
	}
	return b.String(), true
}

// RewriteWikilinks rewrites every `[[ID]]` span in body to a standard
// markdown link. Resolved ids become `[title](ID)` so navidown's existing
// link pipeline can handle them — taskDescriptionProvider already recognizes
// bare-id URLs via extractTaskID. Unresolved ids keep their literal
// `[[ID]]` form with a trailing `(not found)` hint so the reader sees the
// broken reference instead of silent omission.
//
// Rewriting is preserving: the full body is scanned exactly once and all
// non-matching text is copied verbatim, so code fences and other prose are
// untouched. A preceding `!` (obsidian image-embed syntax, `![[ID]]`) is
// detected manually and the match is left alone so we don't produce a
// broken `![title](ID)` image reference.
func RewriteWikilinks(body string, resolver Resolver) string {
	if resolver == nil || body == "" {
		return body
	}
	locs := wikilinkPattern.FindAllStringSubmatchIndex(body, -1)
	if len(locs) == 0 {
		return body
	}
	var b strings.Builder
	b.Grow(len(body))
	cursor := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		idStart, idEnd := loc[2], loc[3]

		// obsidian-style image embed — preserve the whole `![[...]]` span
		// so the image pipeline still sees it as an embed.
		if start > 0 && body[start-1] == '!' {
			b.WriteString(body[cursor:end])
			cursor = end
			continue
		}

		b.WriteString(body[cursor:start])
		id := body[idStart:idEnd]
		title, _, ok := resolver.Resolve(id)
		if !ok {
			b.WriteString(body[start:end])
			b.WriteString(" *(not found)*")
		} else {
			b.WriteString("[")
			b.WriteString(title)
			b.WriteString("](")
			b.WriteString(id)
			b.WriteString(")")
		}
		cursor = end
	}
	b.WriteString(body[cursor:])
	return b.String()
}

// WikilinkProvider wraps a navidown ContentProvider so every piece of fetched
// markdown has its `[[ID]]` spans rewritten before reaching the renderer.
// Navigation back through FetchContent on every link click means wrapping
// here covers both initial load and subsequent navigation in one place.
type WikilinkProvider struct {
	inner    nav.ContentProvider
	resolver Resolver
}

// NewWikilinkProvider wraps inner. If resolver is nil the wrapper becomes a
// pass-through — callers don't need to guard the construction.
func NewWikilinkProvider(inner nav.ContentProvider, resolver Resolver) *WikilinkProvider {
	return &WikilinkProvider{inner: inner, resolver: resolver}
}

// FetchContent implements nav.ContentProvider. A URL that parses as a bare
// document id is short-circuited: the body comes from the resolver (when it
// implements BodyResolver), bypassing the file-loading inner provider so
// `[title](ID)` links rewritten from `[[ID]]` resolve through the document
// store instead of hitting disk. Everything else falls through to the
// inner provider and then has `[[ID]]` spans rewritten.
func (p *WikilinkProvider) FetchContent(elem nav.NavElement) (string, error) {
	if br, ok := p.resolver.(BodyResolver); ok && document.IsValidID(strings.ToUpper(elem.URL)) {
		if body, ok := br.ResolveBody(strings.ToUpper(elem.URL)); ok {
			return RewriteWikilinks(body, p.resolver), nil
		}
	}
	content, err := p.inner.FetchContent(elem)
	if err != nil {
		return content, err
	}
	if p.resolver == nil {
		return content, nil
	}
	return RewriteWikilinks(content, p.resolver), nil
}

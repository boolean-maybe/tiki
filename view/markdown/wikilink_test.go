package markdown

import (
	"errors"
	"strings"
	"testing"

	nav "github.com/boolean-maybe/navidown/navidown"
)

// fakeResolver is a Resolver backed by an in-memory id→entry table. Lets
// wikilink rewriting be unit-tested without dragging the full task store in.
type fakeResolver struct {
	entries map[string]fakeEntry
}

type fakeEntry struct {
	title string
	path  string
}

func (r *fakeResolver) Resolve(id string) (string, string, bool) {
	if r == nil {
		return "", "", false
	}
	e, ok := r.entries[id]
	if !ok {
		return "", "", false
	}
	return e.title, e.path, true
}

func TestRewriteWikilinks_ResolvedIDUsesTitle(t *testing.T) {
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "My Doc", path: ".doc/ABC123.md"},
	}}
	got := RewriteWikilinks("see [[ABC123]] for details", resolver)
	want := "see [My Doc](ABC123) for details"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRewriteWikilinks_UnknownIDKeepsLiteralAndMarksNotFound(t *testing.T) {
	resolver := &fakeResolver{entries: map[string]fakeEntry{}}
	got := RewriteWikilinks("missing [[ZZZ999]] here", resolver)
	if !strings.Contains(got, "[[ZZZ999]]") {
		t.Errorf("expected literal [[ZZZ999]] to survive, got %q", got)
	}
	if !strings.Contains(got, "not found") {
		t.Errorf("expected not-found marker, got %q", got)
	}
}

func TestRewriteWikilinks_MultipleLinks(t *testing.T) {
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"AAA111": {title: "A", path: "a.md"},
		"BBB222": {title: "B", path: "b.md"},
	}}
	got := RewriteWikilinks("[[AAA111]] and [[BBB222]]", resolver)
	want := "[A](AAA111) and [B](BBB222)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Post-6B.12 the obsidian image-embed case lives in
// TestRewriteWikilinks_ImageEmbedPreserved, which exercises a canonical
// ID shape (the only shape the matcher sees). The pre-6B.12 test used a
// non-canonical "image" id that no longer matches the regex and is
// therefore no longer load-bearing.

func TestRewriteWikilinks_NilResolverIsIdentity(t *testing.T) {
	body := "has [[ABC123]] inside"
	got := RewriteWikilinks(body, nil)
	if got != body {
		t.Errorf("nil resolver should be identity; got %q want %q", got, body)
	}
}

func TestRewriteWikilinks_EmptyBody(t *testing.T) {
	resolver := &fakeResolver{}
	if got := RewriteWikilinks("", resolver); got != "" {
		t.Errorf("empty body must stay empty, got %q", got)
	}
}

func TestRewriteWikilinks_CodeFenceNotCurrentlyExcluded(t *testing.T) {
	// Documents current behavior: the regex rewriter does not understand
	// markdown structure, so a `[[ID]]` inside a fenced code block is still
	// rewritten. That's acceptable at 6B because it matches how navidown
	// itself handles inline markdown inside fences (i.e. it renders the
	// source verbatim so the user sees the link text either way). If this
	// surfaces as a real complaint, tighten later by walking the AST.
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "Doc", path: "abc.md"},
	}}
	got := RewriteWikilinks("```\nsee [[ABC123]]\n```", resolver)
	if !strings.Contains(got, "[Doc](ABC123)") {
		t.Errorf("current behavior rewrites inside fences; got %q", got)
	}
}

func TestRewriteWikilinks_LowercaseLeftUntouched(t *testing.T) {
	// 6B.12: the pattern only matches the canonical uppercase ID shape.
	// `[[abc123]]` does not match, so the rewriter is a pure identity —
	// not even a `*(not found)*` marker, since the matcher never saw it.
	// This prevents false positives on obsidian-style `[[Some Title]]`
	// links where the inner text happens to be alphanumeric.
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "Doc", path: "abc.md"},
	}}
	body := "see [[abc123]] here"
	got := RewriteWikilinks(body, resolver)
	if got != body {
		t.Errorf("lowercase id must not match the canonical pattern; got %q", got)
	}
}

// TestRewriteWikilinks_NonCanonicalShapesIgnored asserts the matcher
// refuses id-shaped-but-non-canonical strings that a more permissive regex
// would rewrite. README happens to be 6 uppercase alphanumerics and would
// pass the shape check — but since the resolver doesn't know it, the
// not-found path kicks in instead. Pre-6B.12 the pattern accepted 1-16
// chars and would have flagged `[[img.png]]` etc.; these cases assert
// those ambiguous shapes are now ignored at the match step.
func TestRewriteWikilinks_NonCanonicalShapesIgnored(t *testing.T) {
	resolver := &fakeResolver{entries: map[string]fakeEntry{}}
	cases := []string{
		"[[ABCDEFG]]",   // 7 chars — too long
		"[[ABC]]",       // 3 chars — too short
		"[[abc123]]",    // lowercase — violates canonical shape
		"[[ABC-12]]",    // contains '-'
		"[[image.png]]", // contains '.'
		"[[My Page]]",   // contains space — obsidian-style title link
	}
	for _, c := range cases {
		if got := RewriteWikilinks(c, resolver); got != c {
			t.Errorf("non-canonical %q must pass through; got %q", c, got)
		}
	}
}

// TestRewriteWikilinks_ImageEmbedPreserved asserts `![[ABC123]]` is left
// as-is so the obsidian-style image-embed pipeline still sees it as an
// embed. Rewriting into `![title](ID)` would produce a broken image
// reference — ID is not a URL.
func TestRewriteWikilinks_ImageEmbedPreserved(t *testing.T) {
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "Doc", path: "doc.md"},
	}}
	body := "embed ![[ABC123]] and link [[ABC123]]"
	got := RewriteWikilinks(body, resolver)
	// embed half is untouched; link half is rewritten
	if !strings.Contains(got, "![[ABC123]]") {
		t.Errorf("image embed must survive, got %q", got)
	}
	if !strings.Contains(got, "[Doc](ABC123)") {
		t.Errorf("non-embed link should still rewrite, got %q", got)
	}
}

// stubContentProvider returns fixed content (or error) from FetchContent;
// lets WikilinkProvider be exercised without a real filesystem or store.
type stubContentProvider struct {
	content string
	err     error
}

func (s *stubContentProvider) FetchContent(nav.NavElement) (string, error) {
	return s.content, s.err
}

func TestWikilinkProvider_RewritesFetchedContent(t *testing.T) {
	inner := &stubContentProvider{content: "see [[ABC123]] for details"}
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "My Doc", path: ".doc/ABC123.md"},
	}}
	p := NewWikilinkProvider(inner, resolver)

	got, err := p.FetchContent(nav.NavElement{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "[My Doc](ABC123)") {
		t.Errorf("expected wikilink rewritten to `[My Doc](ABC123)`, got %q", got)
	}
}

func TestWikilinkProvider_NilResolverIsPassThrough(t *testing.T) {
	body := "see [[ABC123]] here"
	inner := &stubContentProvider{content: body}
	p := NewWikilinkProvider(inner, nil)

	got, err := p.FetchContent(nav.NavElement{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != body {
		t.Errorf("nil resolver must pass through verbatim; got %q", got)
	}
}

// TestRewriteWikilinks_SurvivesResolverPathChange asserts the invariant the
// plan calls out explicitly: a wikilink's text/target must reflect the
// *current* path of the target document, not a cached value. Since the
// rewriter calls Resolve each time it runs, a store that tracks file moves
// will automatically produce post-move links the next time the body is
// rewritten.
func TestRewriteWikilinks_SurvivesResolverPathChange(t *testing.T) {
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "Doc", path: ".doc/old/abc.md"},
	}}

	// Body is the same across both rewrites; only the resolver's state
	// changes. The rewriter does not carry any cache of its own.
	body := "see [[ABC123]]"

	_ = RewriteWikilinks(body, resolver)

	// Simulate a file rename: update the resolver's path.
	resolver.entries["ABC123"] = fakeEntry{title: "Doc", path: ".doc/new/nested/abc.md"}

	got := RewriteWikilinks(body, resolver)
	// The rewriter only consults title for the visible text — the path
	// information is not embedded in the rewritten link today (the URL is
	// the id itself). The survives-move invariant therefore holds at the
	// *resolver* layer: as long as the store's PathForID reflects current
	// state, a later file-open attempt via the id link hits the right file.
	// We assert the id still resolves, not a specific path string.
	if !strings.Contains(got, "[Doc](ABC123)") {
		t.Errorf("expected id to still resolve after path change, got %q", got)
	}
}

func TestWikilinkProvider_PropagatesInnerError(t *testing.T) {
	// When the inner provider fails — e.g. the file isn't on disk — the
	// wrapper must surface the error unchanged and not attempt rewriting.
	// Rewriting a partial/empty body would mask the real failure.
	wantErr := errors.New("file not found")
	inner := &stubContentProvider{err: wantErr}
	resolver := &fakeResolver{entries: map[string]fakeEntry{
		"ABC123": {title: "Doc", path: "abc.md"},
	}}
	p := NewWikilinkProvider(inner, resolver)

	_, err := p.FetchContent(nav.NavElement{})
	if !errors.Is(err, wantErr) {
		t.Errorf("expected inner error to propagate, got %v", err)
	}
}

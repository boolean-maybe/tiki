package tiki

import (
	"fmt"
	"time"

	"github.com/boolean-maybe/ruki"
)

// Doc adapts *Tiki to ruki.Document. The glue wraps tikis on the way into ruki
// and unwraps on the way out.
type Doc struct{ T *Tiki }

func (d Doc) ID() string                       { return d.T.ID() }
func (d Doc) Title() string                    { return d.T.Title() }
func (d Doc) Body() string                     { return d.T.Body() }
func (d Doc) Path() string                     { return d.T.Path() }
func (d Doc) CreatedAt() time.Time             { return d.T.CreatedAt() }
func (d Doc) UpdatedAt() time.Time             { return d.T.UpdatedAt() }
func (d Doc) SetTitle(v string)                { d.T.SetTitle(v) }
func (d Doc) SetBody(v string)                 { d.T.SetBody(v) }
func (d Doc) Get(n string) (interface{}, bool) { return d.T.Get(n) }
func (d Doc) Set(n string, v interface{})      { d.T.Set(n, v) }
func (d Doc) Has(n string) bool                { return d.T.Has(n) }
func (d Doc) Delete(n string)                  { d.T.Delete(n) }
func (d Doc) Clone() ruki.Document             { return Doc{T: d.T.Clone()} }

// WrapDoc / WrapDocs / UnwrapDoc / UnwrapDocs bridge *Tiki and ruki.Document.
//
// WrapDoc preserves nil: a nil *Tiki wraps to a nil ruki.Document interface
// value (not a non-nil Doc with a nil T), so callers comparing the wrapped
// value against nil — e.g. trigger old/new snapshots — keep working.
func WrapDoc(t *Tiki) ruki.Document {
	if t == nil {
		return nil
	}
	return Doc{T: t}
}

// NewDoc returns a blank *Tiki wrapped as a ruki.Document — the canonical
// DocumentFactory body for constructing ruki executors.
func NewDoc() ruki.Document { return WrapDoc(New()) }

func WrapDocs(ts []*Tiki) []ruki.Document {
	out := make([]ruki.Document, len(ts))
	for i, t := range ts {
		out[i] = Doc{T: t}
	}
	return out
}

func UnwrapDoc(d ruki.Document) *Tiki {
	if d == nil {
		return nil
	}
	doc, ok := d.(Doc)
	if !ok {
		// host-side boundary: only tiki.Doc instances are ever passed into
		// ruki from this codebase, so a non-Doc here is a logic error
		// (e.g. a test fake reaching production code).
		panic(fmt.Sprintf("tiki.UnwrapDoc: expected tiki.Doc, got %T", d))
	}
	return doc.T
}

func UnwrapDocs(ds []ruki.Document) []*Tiki {
	out := make([]*Tiki, len(ds))
	for i, d := range ds {
		out[i] = UnwrapDoc(d)
	}
	return out
}

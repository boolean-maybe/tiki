package ruki

import (
	"testing"
	"time"
)

// fakeDoc is a minimal in-test Document implementation. It backs the generic
// field map with a map[string]interface{} and keeps the identity/audit
// attributes as plain struct fields.
type fakeDoc struct {
	id        string
	title     string
	body      string
	path      string
	createdAt time.Time
	updatedAt time.Time
	fields    map[string]interface{}
}

func newFakeDoc() *fakeDoc {
	return &fakeDoc{fields: map[string]interface{}{}}
}

func (d *fakeDoc) ID() string           { return d.id }
func (d *fakeDoc) Title() string        { return d.title }
func (d *fakeDoc) Body() string         { return d.body }
func (d *fakeDoc) Path() string         { return d.path }
func (d *fakeDoc) CreatedAt() time.Time { return d.createdAt }
func (d *fakeDoc) UpdatedAt() time.Time { return d.updatedAt }

func (d *fakeDoc) SetTitle(v string) { d.title = v }
func (d *fakeDoc) SetBody(v string)  { d.body = v }

func (d *fakeDoc) Get(name string) (interface{}, bool) {
	if d.fields == nil {
		return nil, false
	}
	v, ok := d.fields[name]
	return v, ok
}

func (d *fakeDoc) Set(name string, value interface{}) {
	if d.fields == nil {
		d.fields = map[string]interface{}{}
	}
	d.fields[name] = value
}

func (d *fakeDoc) Has(name string) bool {
	if d.fields == nil {
		return false
	}
	_, ok := d.fields[name]
	return ok
}

func (d *fakeDoc) Delete(name string) {
	if d.fields != nil {
		delete(d.fields, name)
	}
}

func (d *fakeDoc) Clone() Document {
	clone := &fakeDoc{
		id:        d.id,
		title:     d.title,
		body:      d.body,
		path:      d.path,
		createdAt: d.createdAt,
		updatedAt: d.updatedAt,
	}
	if d.fields != nil {
		clone.fields = make(map[string]interface{}, len(d.fields))
		for k, v := range d.fields {
			clone.fields[k] = v
		}
	}
	return clone
}

func TestDocument_InterfaceSatisfiedByFake(t *testing.T) {
	var _ Document = (*fakeDoc)(nil)

	factory := DocumentFactory(func() Document { return newFakeDoc() })
	if factory() == nil {
		t.Fatal("DocumentFactory returned nil")
	}
}

func TestIsIdentityField(t *testing.T) {
	identity := []string{
		"id", "title", "description", "body",
		"createdBy", "createdAt", "updatedAt",
		"filepath", "path",
	}
	for _, name := range identity {
		if !IsIdentityField(name) {
			t.Errorf("IsIdentityField(%q) = false, want true", name)
		}
	}
	if IsIdentityField("status") {
		t.Error(`IsIdentityField("status") = true, want false`)
	}
}

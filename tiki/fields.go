package tiki

import (
	"fmt"
	"math"
	"time"
)

// Schema-known field names. These mirror the YAML frontmatter keys used
// across the codebase (document.workflowFrontmatterKeys plus tags). Callers
// should reference these constants instead of hard-coding strings.
const (
	FieldStatus     = "status"
	FieldType       = "type"
	FieldPriority   = "priority"
	FieldPoints     = "points"
	FieldTags       = "tags"
	FieldDependsOn  = "dependsOn"
	FieldDue        = "due"
	FieldRecurrence = "recurrence"
	FieldAssignee   = "assignee"
)

// SchemaKnownFields lists the schema-known field names in a stable order.
// Ordering matters for callers that render fields in a deterministic sequence
// (for example, YAML frontmatter output). Use IsSchemaKnown for membership.
var SchemaKnownFields = []string{
	FieldStatus,
	FieldType,
	FieldPriority,
	FieldPoints,
	FieldTags,
	FieldDependsOn,
	FieldDue,
	FieldRecurrence,
	FieldAssignee,
}

var schemaKnownSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SchemaKnownFields))
	for _, k := range SchemaKnownFields {
		m[k] = struct{}{}
	}
	return m
}()

// IsSchemaKnown reports whether name is one of the schema-known field names.
func IsSchemaKnown(name string) bool {
	_, ok := schemaKnownSet[name]
	return ok
}

// Has reports whether the field is present in the map. A nil Fields map is
// treated as empty. This is the presence check that backs ruki's has(field).
func (t *Tiki) Has(name string) bool {
	if t == nil || t.Fields == nil {
		return false
	}
	_, ok := t.Fields[name]
	return ok
}

// Get returns the raw field value and a presence flag. The value is whatever
// was stored — typically a string, int, time.Time, or []string depending on
// the key. Prefer the typed accessors (StatusField, PriorityField, etc.) for
// schema-known fields.
func (t *Tiki) Get(name string) (interface{}, bool) {
	if t == nil || t.Fields == nil {
		return nil, false
	}
	v, ok := t.Fields[name]
	return v, ok
}

// Require returns the field value or a descriptive error if the field is
// absent. This matches the Phase 4 ruki contract: absent field reads hard-
// error unless a construct is explicitly presence-safe.
func (t *Tiki) Require(name string) (interface{}, error) {
	v, ok := t.Get(name)
	if !ok {
		return nil, fmt.Errorf("tiki %s: field %q is not set", t.identity(), name)
	}
	return v, nil
}

// Set writes a field value, allocating the Fields map lazily. Setting a nil
// value is allowed and represents an explicitly-present nil, distinct from an
// absent field — use Delete to remove.
func (t *Tiki) Set(name string, value interface{}) {
	if t.Fields == nil {
		t.Fields = map[string]interface{}{}
	}
	t.Fields[name] = value
}

// Delete removes a field. Safe to call for absent fields and on a Tiki with a
// nil Fields map.
func (t *Tiki) Delete(name string) {
	if t == nil || t.Fields == nil {
		return
	}
	delete(t.Fields, name)
}

// identity is a best-effort label for error messages.
func (t *Tiki) identity() string {
	if t == nil {
		return "<nil>"
	}
	if t.ID != "" {
		return t.ID
	}
	if t.Path != "" {
		return t.Path
	}
	return "<unidentified>"
}

// StringField returns a string-typed field value. The second return reports
// presence; the third reports whether the stored value coerces to a string
// (which should always be true for schema-known string fields). Callers that
// treat a non-coercible value as a programming error can use RequireString.
func (t *Tiki) StringField(name string) (string, bool, bool) {
	v, ok := t.Get(name)
	if !ok {
		return "", false, false
	}
	s, coerced := coerceString(v)
	return s, true, coerced
}

// RequireString returns the field as a string, erroring if absent or if the
// stored value is not a string.
func (t *Tiki) RequireString(name string) (string, error) {
	v, err := t.Require(name)
	if err != nil {
		return "", err
	}
	s, ok := coerceString(v)
	if !ok {
		return "", fmt.Errorf("tiki %s: field %q is not a string (got %T)", t.identity(), name, v)
	}
	return s, nil
}

// IntField returns an int-typed field value. Reports presence and whether the
// stored value coerces to an int (handles int/int64/float64/uint which all
// show up in YAML-decoded maps).
func (t *Tiki) IntField(name string) (int, bool, bool) {
	v, ok := t.Get(name)
	if !ok {
		return 0, false, false
	}
	n, coerced := coerceInt(v)
	return n, true, coerced
}

// RequireInt returns the field as an int, erroring if absent or non-numeric.
func (t *Tiki) RequireInt(name string) (int, error) {
	v, err := t.Require(name)
	if err != nil {
		return 0, err
	}
	n, ok := coerceInt(v)
	if !ok {
		return 0, fmt.Errorf("tiki %s: field %q is not numeric (got %T)", t.identity(), name, v)
	}
	return n, nil
}

// StringSliceField returns a []string field value. A single string is
// promoted to a one-element slice for YAML scalar-vs-sequence leniency.
func (t *Tiki) StringSliceField(name string) ([]string, bool, bool) {
	v, ok := t.Get(name)
	if !ok {
		return nil, false, false
	}
	ss, coerced := coerceStringSlice(v)
	return ss, true, coerced
}

// RequireStringSlice returns the field as a []string, erroring if absent or
// non-coercible.
func (t *Tiki) RequireStringSlice(name string) ([]string, error) {
	v, err := t.Require(name)
	if err != nil {
		return nil, err
	}
	ss, ok := coerceStringSlice(v)
	if !ok {
		return nil, fmt.Errorf("tiki %s: field %q is not a string list (got %T)", t.identity(), name, v)
	}
	return ss, nil
}

// TimeField returns a time.Time field value. Accepts time.Time directly or
// a string in YYYY-MM-DD form.
func (t *Tiki) TimeField(name string) (time.Time, bool, bool) {
	v, ok := t.Get(name)
	if !ok {
		return time.Time{}, false, false
	}
	tv, coerced := coerceTime(v)
	return tv, true, coerced
}

// RequireTime returns the field as a time.Time, erroring if absent or
// non-coercible.
func (t *Tiki) RequireTime(name string) (time.Time, error) {
	v, err := t.Require(name)
	if err != nil {
		return time.Time{}, err
	}
	tv, ok := coerceTime(v)
	if !ok {
		return time.Time{}, fmt.Errorf("tiki %s: field %q is not a date (got %T)", t.identity(), name, v)
	}
	return tv, nil
}

// coerceString accepts strings and a handful of near-string types.
func coerceString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case fmt.Stringer:
		return val.String(), true
	default:
		return "", false
	}
}

// coerceInt accepts the numeric shapes yaml.v3 emits into map[string]any as
// well as plain ints set programmatically.
func coerceInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int8:
		return int(val), true
	case int16:
		return int(val), true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case uint:
		if val > math.MaxInt {
			return 0, false
		}
		return int(val), true
	case uint8:
		return int(val), true
	case uint16:
		return int(val), true
	case uint32:
		return int(val), true
	case uint64:
		if val > math.MaxInt {
			return 0, false
		}
		return int(val), true
	case float32:
		return int(val), true
	case float64:
		return int(val), true
	default:
		return 0, false
	}
}

// coerceStringSlice accepts []string, []interface{} of strings, or a single
// string (promoted to a one-element slice).
func coerceStringSlice(v interface{}) ([]string, bool) {
	switch val := v.(type) {
	case []string:
		cp := make([]string, len(val))
		copy(cp, val)
		return cp, true
	case []interface{}:
		out := make([]string, 0, len(val))
		for _, e := range val {
			s, ok := coerceString(e)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	case string:
		return []string{val}, true
	default:
		return nil, false
	}
}

// coerceTime accepts time.Time directly or a YYYY-MM-DD string.
func coerceTime(v interface{}) (time.Time, bool) {
	switch val := v.(type) {
	case time.Time:
		return val, true
	case string:
		parsed, err := time.Parse("2006-01-02", val)
		if err != nil {
			return time.Time{}, false
		}
		return parsed, true
	default:
		return time.Time{}, false
	}
}

package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/boolean-maybe/tiki/ruki/keyword"
)

// ValueType identifies the semantic type of a task field.
type ValueType int

const (
	TypeString     ValueType = iota
	TypeInt                  // numeric (priority, points)
	TypeDate                 // midnight-UTC date (e.g. due)
	TypeTimestamp            // full timestamp (e.g. createdAt, updatedAt)
	TypeDuration             // reserved for future use
	TypeBool                 // reserved for future use
	TypeID                   // task identifier
	TypeRef                  // reference to another task ID
	TypeRecurrence           // structured cron-based recurrence pattern
	TypeListString           // []string (e.g. tags)
	TypeListRef              // []string of task ID references (e.g. dependsOn)
	TypeStatus               // workflow status enum backed by StatusRegistry
	TypeTaskType             // task type enum backed by TypeRegistry
	TypeEnum                 // custom enum backed by FieldDef.AllowedValues
)

// FieldDef describes a single task field's name and semantic type.
type FieldDef struct {
	Name          string
	Type          ValueType
	Custom        bool     // true for user-defined fields from workflow.yaml
	AllowedValues []string // non-nil only for TypeEnum fields
}

// fieldCatalog is the authoritative list of DSL-visible task fields.
var fieldCatalog = []FieldDef{
	{Name: "id", Type: TypeID},
	{Name: "title", Type: TypeString},
	{Name: "description", Type: TypeString},
	{Name: "status", Type: TypeStatus},
	{Name: "type", Type: TypeTaskType},
	{Name: "tags", Type: TypeListString},
	{Name: "dependsOn", Type: TypeListRef},
	{Name: "due", Type: TypeDate},
	{Name: "recurrence", Type: TypeRecurrence},
	{Name: "assignee", Type: TypeString},
	{Name: "priority", Type: TypeInt},
	{Name: "points", Type: TypeInt},
	{Name: "createdBy", Type: TypeString},
	{Name: "createdAt", Type: TypeTimestamp},
	{Name: "updatedAt", Type: TypeTimestamp},
}

// pre-built lookup for Field()
var fieldByName map[string]FieldDef

func init() {
	fieldByName = make(map[string]FieldDef, len(fieldCatalog))
	for _, f := range fieldCatalog {
		if keyword.IsReserved(f.Name) {
			panic(fmt.Sprintf("field catalog contains reserved keyword: %q", f.Name))
		}
		fieldByName[f.Name] = f
	}
}

// validIdentRE validates that field names are usable as ruki identifiers.
var validIdentRE = regexp.MustCompile(keyword.IdentPattern)

// custom field registry state
var (
	customMu              sync.RWMutex
	customFields          []FieldDef
	customFieldByName     map[string]FieldDef
	onCustomFieldsChanged []func() // callbacks invoked after registry changes
)

// Field returns the FieldDef for a given field name and whether it exists.
// Checks built-in fields first, then custom fields.
func Field(name string) (FieldDef, bool) {
	if f, ok := fieldByName[name]; ok {
		return f, ok
	}
	customMu.RLock()
	defer customMu.RUnlock()
	if f, ok := customFieldByName[name]; ok {
		return deepCopyFieldDef(f), true
	}
	return FieldDef{}, false
}

// ValidateFieldName rejects names that collide with ruki reserved keywords,
// boolean literal identifiers, or characters not valid in ruki identifiers.
func ValidateFieldName(name string) error {
	if !validIdentRE.MatchString(name) {
		return fmt.Errorf("field name %q is not a valid identifier (must match %s)", name, keyword.IdentPattern)
	}
	if keyword.IsReserved(name) {
		return fmt.Errorf("field name %q is reserved", name)
	}
	lower := strings.ToLower(name)
	if lower == "true" || lower == "false" {
		return fmt.Errorf("field name %q is reserved (boolean literal)", name)
	}
	return nil
}

// Fields returns the ordered list of all DSL-visible task fields
// (built-in + custom). Returns deep copies so callers cannot mutate registry state.
func Fields() []FieldDef {
	customMu.RLock()
	defer customMu.RUnlock()
	result := make([]FieldDef, 0, len(fieldCatalog)+len(customFields))
	for _, f := range fieldCatalog {
		result = append(result, f) // built-ins have no mutable slices
	}
	for _, f := range customFields {
		result = append(result, deepCopyFieldDef(f))
	}
	return result
}

// RegisterCustomFields validates and registers custom field definitions.
// Deep-copies incoming defs so the caller retains no mutable alias into registry state.
// Returns an error if any definition collides with built-in fields or reserved keywords.
func RegisterCustomFields(defs []FieldDef) error {
	// build case-insensitive lookup of built-in names
	builtInLower := make(map[string]string, len(fieldByName))
	for name := range fieldByName {
		builtInLower[strings.ToLower(name)] = name
	}

	byName := make(map[string]FieldDef, len(defs))
	seenLower := make(map[string]string, len(defs))
	copied := make([]FieldDef, 0, len(defs))
	for _, d := range defs {
		if err := ValidateFieldName(d.Name); err != nil {
			return fmt.Errorf("custom field %q: %w", d.Name, err)
		}
		lower := strings.ToLower(d.Name)
		if builtIn, ok := builtInLower[lower]; ok {
			return fmt.Errorf("custom field %q collides with built-in field %q (case-insensitive)", d.Name, builtIn)
		}
		if prev, ok := seenLower[lower]; ok {
			return fmt.Errorf("custom field %q collides with %q (case-insensitive)", d.Name, prev)
		}
		seenLower[lower] = d.Name
		if d.Type == TypeEnum && len(d.AllowedValues) == 0 {
			return fmt.Errorf("custom enum field %q requires non-empty values", d.Name)
		}
		c := FieldDef{
			Name:   d.Name,
			Type:   d.Type,
			Custom: true,
		}
		if d.AllowedValues != nil {
			c.AllowedValues = make([]string, len(d.AllowedValues))
			copy(c.AllowedValues, d.AllowedValues)
		}
		byName[d.Name] = c
		copied = append(copied, c)
	}
	customMu.Lock()
	customFields = copied
	customFieldByName = byName
	customMu.Unlock()
	notifyCustomFieldsChanged()
	return nil
}

// ClearCustomFields removes all custom field registrations. Intended for tests.
func ClearCustomFields() {
	customMu.Lock()
	customFields = nil
	customFieldByName = nil
	customMu.Unlock()
	notifyCustomFieldsChanged()
}

// OnCustomFieldsChanged registers a callback invoked whenever the custom field
// registry is modified (register or clear). Used by plugin/legacy_convert to
// invalidate its field-name cache without an import cycle.
func OnCustomFieldsChanged(fn func()) {
	customMu.Lock()
	onCustomFieldsChanged = append(onCustomFieldsChanged, fn)
	customMu.Unlock()
}

func notifyCustomFieldsChanged() {
	customMu.RLock()
	cbs := onCustomFieldsChanged
	customMu.RUnlock()
	for _, fn := range cbs {
		fn()
	}
}

// deepCopyFieldDef returns a copy of fd with a cloned AllowedValues slice.
func deepCopyFieldDef(fd FieldDef) FieldDef {
	if fd.AllowedValues != nil {
		vals := make([]string, len(fd.AllowedValues))
		copy(vals, fd.AllowedValues)
		fd.AllowedValues = vals
	}
	return fd
}

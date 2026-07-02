package workflow

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/boolean-maybe/ruki/keyword"
)

// ValueType identifies the semantic type of a document field.
type ValueType int

const (
	TypeString     ValueType = iota
	TypeInt                  // numeric (priority, points)
	TypeDate                 // midnight-UTC date (e.g. due)
	TypeTimestamp            // full timestamp (e.g. createdAt, updatedAt)
	TypeDuration             // reserved for future use
	TypeBool                 // reserved for future use
	TypeID                   // bare document identifier (^[A-Z0-9]{6}$)
	TypeRef                  // reference to another document ID
	TypeRecurrence           // structured cron-based recurrence pattern
	TypeListString           // []string (e.g. tags)
	TypeListRef              // []string of document ID references (e.g. dependsOn)
	TypeEnum                 // enum field with EnumValues metadata
)

// IsList reports whether the type is one of the list-valued types
// (TypeListString or TypeListRef). Both store a []string; the distinction is
// whether the elements are free strings (tags) or document-ID references
// (dependsOn). Several renderers/measurers/validators branch on list-ness, so
// the predicate lives here rather than being re-spelled at each site.
func (t ValueType) IsList() bool {
	return t == TypeListString || t == TypeListRef
}

// EnumValue describes one allowed value of a TypeEnum field with display
// metadata. Default marks the value as the creation default for the field;
// at most one value per enum may set Default: true. Visual is an optional
// short rendering of the value (a glyph, emoji, or <role>-tagged markup
// string consumed by workflow.ExpandVisual at render time).
type EnumValue struct {
	Value   string
	Label   string
	Visual  string
	Default bool
}

// FieldDef describes a single document field's name and semantic type.
// EnumValues is non-empty only for TypeEnum fields; AllowedValues() returns
// just the keys for ruki schema construction.
type FieldDef struct {
	Name         string
	Type         ValueType
	Custom       bool        // true for fields loaded from workflow.yaml
	Caption      string      // optional display caption; falls back to Name via DisplayCaption()
	EnumValues   []EnumValue // populated only for TypeEnum
	DefaultValue interface{} // creation default for non-enum fields; for enum, derived from EnumValues[i].Default
}

// DisplayCaption returns the field's display caption, falling back to the
// bare field Name when no caption was declared.
func (f FieldDef) DisplayCaption() string {
	if f.Caption != "" {
		return f.Caption
	}
	return f.Name
}

// AllowedValues returns the value keys for an enum FieldDef in declaration
// order. Returns nil for non-enum fields.
func (f FieldDef) AllowedValues() []string {
	if f.Type != TypeEnum {
		return nil
	}
	out := make([]string, len(f.EnumValues))
	for i, v := range f.EnumValues {
		out[i] = v.Value
	}
	return out
}

// EnumDefault returns the default enum value key (the EnumValue marked
// Default: true), or "" when no default is configured.
func (f FieldDef) EnumDefault() string {
	for _, v := range f.EnumValues {
		if v.Default {
			return v.Value
		}
	}
	return ""
}

// LookupEnum returns the EnumValue for the given key and whether it exists.
func (f FieldDef) LookupEnum(key string) (EnumValue, bool) {
	for _, v := range f.EnumValues {
		if v.Value == key {
			return v, true
		}
	}
	return EnumValue{}, false
}

// EnumDisplay returns the raw visual markup when Visual is set, otherwise
// the Label (or Value if Label is empty). Callers that render to a tview
// surface should pass non-empty visuals through ExpandVisual to resolve
// <role> markup into color tags. Unknown keys round-trip as themselves.
func (f FieldDef) EnumDisplay(key string) string {
	v, ok := f.LookupEnum(key)
	if !ok {
		return key
	}
	if visual := strings.TrimSpace(v.Visual); visual != "" {
		return visual
	}
	if v.Label != "" {
		return v.Label
	}
	return v.Value
}

// EnumLabel returns the human-readable label for an enum key, preferring
// Label over Visual. Falls back to Value if Label is empty. Use this for
// contexts where a textual name is wanted (detail view fields) rather than
// a compact visual indicator (board tiki boxes).
func (f FieldDef) EnumLabel(key string) string {
	v, ok := f.LookupEnum(key)
	if !ok {
		return key
	}
	if v.Label != "" {
		return v.Label
	}
	return v.Value
}

// EnumParseDisplay reverses EnumDisplay() back to a canonical key.
// Returns ("", false) for an unrecognized display string.
func (f FieldDef) EnumParseDisplay(display string) (string, bool) {
	for _, v := range f.EnumValues {
		if f.EnumDisplay(v.Value) == display {
			return v.Value, true
		}
	}
	return "", false
}

// EnumParseLabel reverses EnumLabel() back to a canonical key.
// Returns ("", false) for an unrecognized label string.
func (f FieldDef) EnumParseLabel(label string) (string, bool) {
	for _, v := range f.EnumValues {
		if f.EnumLabel(v.Value) == label {
			return v.Value, true
		}
	}
	return "", false
}

// IsValidEnum reports whether key is a recognized value for this enum.
func (f FieldDef) IsValidEnum(key string) bool {
	if f.Type != TypeEnum {
		return false
	}
	for _, v := range f.EnumValues {
		if v.Value == key {
			return true
		}
	}
	return false
}

// systemFieldCatalog lists the fields hardcoded in the runtime. Workflow
// fields are NOT in this list — they come exclusively from workflow.yaml.
// These names are reserved: workflow.yaml may not redefine them.
var systemFieldCatalog = []FieldDef{
	{Name: "id", Type: TypeID},
	{Name: "title", Type: TypeString},
	{Name: "description", Type: TypeString},
	{Name: "createdBy", Type: TypeString},
	{Name: "createdAt", Type: TypeTimestamp},
	{Name: "updatedAt", Type: TypeTimestamp},
	{Name: "filepath", Type: TypeString},
}

// systemFieldByName is a pre-built lookup over systemFieldCatalog.
var systemFieldByName map[string]FieldDef

func init() {
	systemFieldByName = make(map[string]FieldDef, len(systemFieldCatalog))
	for _, f := range systemFieldCatalog {
		if keyword.IsReserved(f.Name) {
			panic(fmt.Sprintf("system field catalog contains reserved keyword: %q", f.Name))
		}
		systemFieldByName[f.Name] = f
	}
}

// IsSystemField reports whether name is a reserved system field that may not
// be declared in workflow.yaml.
func IsSystemField(name string) bool {
	if _, ok := systemFieldByName[name]; ok {
		return true
	}
	// "body" is an alias for description in some contexts — treat as reserved
	return name == "body"
}

// SystemFields returns a copy of the system field catalog.
func SystemFields() []FieldDef {
	result := make([]FieldDef, len(systemFieldCatalog))
	copy(result, systemFieldCatalog)
	return result
}

// validIdentRE validates that field names are usable as ruki identifiers.
var validIdentRE = regexp.MustCompile(keyword.IdentPattern)

// workflow field state — populated by config.LoadWorkflowFields() at startup.
var (
	workflowMu          sync.RWMutex
	workflowFields      []FieldDef
	workflowFieldByName map[string]FieldDef
)

// Field returns the FieldDef for a given field name and whether it exists.
// Checks system fields first, then loaded workflow fields.
func Field(name string) (FieldDef, bool) {
	if f, ok := systemFieldByName[name]; ok {
		return f, ok
	}
	workflowMu.RLock()
	defer workflowMu.RUnlock()
	if f, ok := workflowFieldByName[name]; ok {
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

// Fields returns the ordered list of all DSL-visible document fields
// (system + loaded workflow fields). Returns deep copies so callers cannot
// mutate registry state.
func Fields() []FieldDef {
	workflowMu.RLock()
	defer workflowMu.RUnlock()
	result := make([]FieldDef, 0, len(systemFieldCatalog)+len(workflowFields))
	for _, f := range systemFieldCatalog {
		result = append(result, f) // system fields have no mutable slices
	}
	for _, f := range workflowFields {
		result = append(result, deepCopyFieldDef(f))
	}
	return result
}

// WorkflowFields returns just the workflow-declared fields (excludes system).
// Used by callers iterating only over user-defined fields (e.g. creation
// defaults, frontmatter coercion).
func WorkflowFields() []FieldDef {
	workflowMu.RLock()
	defer workflowMu.RUnlock()
	result := make([]FieldDef, 0, len(workflowFields))
	for _, f := range workflowFields {
		result = append(result, deepCopyFieldDef(f))
	}
	return result
}

// ValidateWorkflowFields checks workflow field definitions for collisions
// with system fields, case-insensitive duplicates, valid identifiers, and
// well-formed enum values, without modifying global state.
func ValidateWorkflowFields(defs []FieldDef) error {
	systemLower := make(map[string]string, len(systemFieldByName))
	for name := range systemFieldByName {
		systemLower[strings.ToLower(name)] = name
	}
	systemLower["body"] = "body"

	seenLower := make(map[string]string, len(defs))
	for _, d := range defs {
		if err := ValidateFieldName(d.Name); err != nil {
			return fmt.Errorf("workflow field %q: %w", d.Name, err)
		}
		lower := strings.ToLower(d.Name)
		if sysName, ok := systemLower[lower]; ok {
			return fmt.Errorf("workflow field %q collides with reserved system field %q", d.Name, sysName)
		}
		if prev, ok := seenLower[lower]; ok {
			return fmt.Errorf("workflow field %q collides with %q (case-insensitive)", d.Name, prev)
		}
		seenLower[lower] = d.Name
		if d.Type == TypeEnum {
			if err := validateEnumValues(d.Name, d.EnumValues); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateEnumValues checks that an enum field has at least one value, all
// values are non-empty, no duplicate keys exist (case-insensitively, since
// runtime matching uses strings.EqualFold), at most one value is marked
// default, and no display string collides with another.
//
// The case-insensitive value check matches runtime behavior in two
// places that compare enum keys: enumRank in ruki/executor.go uses
// strings.EqualFold for sort/filter, and coerceCustomValue in
// store/tikistore/persistence.go uses strings.EqualFold for load-time
// canonicalization. Without case-insensitive validation, a workflow could
// declare both "high" and "HIGH" as separate values; runtime would
// silently route both to whichever appeared first in AllowedValues,
// making the second value unreachable and sort ordering nondeterministic
// for tikis that wrote either casing.
func validateEnumValues(fieldName string, values []EnumValue) error {
	if len(values) == 0 {
		return fmt.Errorf("enum field %q requires non-empty values", fieldName)
	}
	seenKeys := make(map[string]string, len(values)) // lowercase → original
	seenDisplay := make(map[string]string, len(values))
	defaultCount := 0
	for i, v := range values {
		if v.Value == "" {
			return fmt.Errorf("enum field %q value at index %d has empty value", fieldName, i)
		}
		lower := strings.ToLower(v.Value)
		if prev, ok := seenKeys[lower]; ok {
			if prev == v.Value {
				return fmt.Errorf("enum field %q has duplicate value %q", fieldName, v.Value)
			}
			return fmt.Errorf("enum field %q values %q and %q collide case-insensitively (runtime treats them as the same value)", fieldName, prev, v.Value)
		}
		seenKeys[lower] = v.Value
		display := enumDisplayParts(v)
		if prev, ok := seenDisplay[display]; ok {
			return fmt.Errorf("enum field %q has duplicate display %q (values %q and %q)", fieldName, display, prev, v.Value)
		}
		seenDisplay[display] = v.Value
		if v.Default {
			defaultCount++
			if defaultCount > 1 {
				return fmt.Errorf("enum field %q has multiple values marked default: true", fieldName)
			}
		}
	}
	return nil
}

func enumDisplayParts(v EnumValue) string {
	if visual := strings.TrimSpace(v.Visual); visual != "" {
		return visual
	}
	if v.Label != "" {
		return v.Label
	}
	return v.Value
}

// RegisterWorkflowFields validates and registers workflow field definitions
// loaded from workflow.yaml. Deep-copies incoming defs so the caller retains
// no mutable alias into registry state. Returns an error if any definition
// collides with system fields or is malformed.
func RegisterWorkflowFields(defs []FieldDef) error {
	if err := ValidateWorkflowFields(defs); err != nil {
		return err
	}

	byName := make(map[string]FieldDef, len(defs))
	copied := make([]FieldDef, 0, len(defs))
	for _, d := range defs {
		c := deepCopyFieldDef(d)
		c.Custom = true
		byName[c.Name] = c
		copied = append(copied, c)
	}
	workflowMu.Lock()
	workflowFields = copied
	workflowFieldByName = byName
	workflowMu.Unlock()
	return nil
}

// ClearWorkflowFields removes all workflow field registrations. Intended for
// tests.
func ClearWorkflowFields() {
	workflowMu.Lock()
	workflowFields = nil
	workflowFieldByName = nil
	workflowMu.Unlock()
}

// deepCopyFieldDef returns a copy of fd with cloned EnumValues and DefaultValue slices.
func deepCopyFieldDef(fd FieldDef) FieldDef {
	if fd.EnumValues != nil {
		vals := make([]EnumValue, len(fd.EnumValues))
		copy(vals, fd.EnumValues)
		fd.EnumValues = vals
	}
	if ss, ok := fd.DefaultValue.([]string); ok {
		cp := make([]string, len(ss))
		copy(cp, ss)
		fd.DefaultValue = cp
	}
	return fd
}

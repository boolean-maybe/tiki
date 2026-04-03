package workflow

import (
	"fmt"

	"github.com/boolean-maybe/tiki/ruki"
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
)

// FieldDef describes a single task field's name and semantic type.
type FieldDef struct {
	Name string
	Type ValueType
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
		if ruki.IsReservedKeyword(f.Name) {
			panic(fmt.Sprintf("field catalog contains reserved keyword: %q", f.Name))
		}
		fieldByName[f.Name] = f
	}
}

// Field returns the FieldDef for a given field name and whether it exists.
func Field(name string) (FieldDef, bool) {
	f, ok := fieldByName[name]
	return f, ok
}

// ValidateFieldName rejects names that collide with ruki reserved keywords.
func ValidateFieldName(name string) error {
	if ruki.IsReservedKeyword(name) {
		return fmt.Errorf("field name %q is reserved", name)
	}
	return nil
}

// Fields returns the ordered list of all DSL-visible task fields.
func Fields() []FieldDef {
	result := make([]FieldDef, len(fieldCatalog))
	copy(result, fieldCatalog)
	return result
}

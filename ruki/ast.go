package ruki

import "time"

// --- top-level union types ---

// Statement is the result of parsing a CRUD command.
// Exactly one variant is non-nil.
type Statement struct {
	Select *SelectStmt
	Create *CreateStmt
	Update *UpdateStmt
	Delete *DeleteStmt
}

// SelectStmt represents "select [fields] [where <condition>] [order by ...] [limit N] [| run(...) | clipboard()]".
type SelectStmt struct {
	Fields  []string        // nil = all ("select" or "select *"); non-nil = specific fields
	Where   Condition       // nil = select all
	OrderBy []OrderByClause // nil = unordered
	Limit   *int            // nil = no limit; positive = max result count
	Pipe    *PipeAction     // optional pipe suffix: "| run(...)" or "| clipboard()"
}

// PipeAction is a discriminated union for pipe targets.
// Exactly one variant is non-nil.
type PipeAction struct {
	Run       *RunAction
	Clipboard *ClipboardAction
}

// ClipboardAction represents "clipboard()" as a pipe target.
type ClipboardAction struct{}

// CreateStmt represents "create <field>=<value>...".
type CreateStmt struct {
	Assignments []Assignment
}

// UpdateStmt represents "update where <condition> set <field>=<value>...".
type UpdateStmt struct {
	Where Condition
	Set   []Assignment
}

// DeleteStmt represents "delete where <condition>".
type DeleteStmt struct {
	Where Condition
}

// --- triggers ---

// Trigger is the result of parsing a reactive rule.
type Trigger struct {
	Timing string     // "before" or "after"
	Event  string     // "create", "update", or "delete"
	Where  Condition  // optional guard (nil if omitted)
	Action *Statement // after-triggers only (create/update/delete, not select)
	Run    *RunAction // after-triggers only (alternative to Action)
	Deny   *string    // before-triggers only
}

// RunAction represents "run(<string-expr>)" as a top-level trigger action.
type RunAction struct {
	Command Expr
}

// TimeTrigger is the result of parsing a periodic time trigger.
// It wraps a mutating statement (create, update, or delete) with a schedule interval.
type TimeTrigger struct {
	Interval DurationLiteral // e.g. {1, "hour"}, {1, "day"}
	Action   *Statement      // create, update, or delete (never select)
}

// Rule is the result of parsing a trigger definition.
// Exactly one variant is non-nil.
type Rule struct {
	Trigger     *Trigger
	TimeTrigger *TimeTrigger
}

// --- conditions ---

// Condition is the interface for all boolean condition nodes.
type Condition interface {
	conditionNode()
}

// BinaryCondition represents "<condition> and/or <condition>".
type BinaryCondition struct {
	Op    string // "and" or "or"
	Left  Condition
	Right Condition
}

// NotCondition represents "not <condition>".
type NotCondition struct {
	Inner Condition
}

// CompareExpr represents "<expr> <op> <expr>".
type CompareExpr struct {
	Left  Expr
	Op    string // "=", "!=", "<", ">", "<=", ">="
	Right Expr
}

// IsEmptyExpr represents "<expr> is [not] empty".
type IsEmptyExpr struct {
	Expr    Expr
	Negated bool // true = "is not empty"
}

// InExpr represents "<value> [not] in <collection>".
type InExpr struct {
	Value      Expr
	Collection Expr
	Negated    bool // true = "not in"
}

// QuantifierExpr represents "<expr> any/all <condition>".
type QuantifierExpr struct {
	Expr      Expr
	Kind      string // "any" or "all"
	Condition Condition
}

func (*BinaryCondition) conditionNode() {}
func (*NotCondition) conditionNode()    {}
func (*CompareExpr) conditionNode()     {}
func (*IsEmptyExpr) conditionNode()     {}
func (*InExpr) conditionNode()          {}
func (*QuantifierExpr) conditionNode()  {}

// --- expressions ---

// Expr is the interface for all expression nodes.
type Expr interface {
	exprNode()
}

// FieldRef represents a bare field name like "status" or "priority".
type FieldRef struct {
	Name string
}

// QualifiedRef represents "old.field" or "new.field".
type QualifiedRef struct {
	Qualifier string // "old" or "new"
	Name      string
}

// StringLiteral represents a double-quoted string value.
type StringLiteral struct {
	Value string
}

// IntLiteral represents an integer value.
type IntLiteral struct {
	Value int
}

// DateLiteral represents a YYYY-MM-DD date.
type DateLiteral struct {
	Value time.Time
}

// DurationLiteral represents a number+unit like "2day" or "1week".
type DurationLiteral struct {
	Value int
	Unit  string
}

// ListLiteral represents ["a", "b", ...].
type ListLiteral struct {
	Elements []Expr
}

// EmptyLiteral represents the "empty" keyword.
type EmptyLiteral struct{}

// FunctionCall represents "name(args...)".
type FunctionCall struct {
	Name string
	Args []Expr
}

// BinaryExpr represents "<expr> +/- <expr>".
type BinaryExpr struct {
	Op    string // "+" or "-"
	Left  Expr
	Right Expr
}

// BoolLiteral represents a bare true/false identifier lowered from FieldRef.
type BoolLiteral struct {
	Value bool
}

// SubQuery represents "select [where <condition>]" used inside count().
type SubQuery struct {
	Where Condition // nil = select all
}

func (*FieldRef) exprNode()        {}
func (*QualifiedRef) exprNode()    {}
func (*StringLiteral) exprNode()   {}
func (*IntLiteral) exprNode()      {}
func (*DateLiteral) exprNode()     {}
func (*DurationLiteral) exprNode() {}
func (*ListLiteral) exprNode()     {}
func (*EmptyLiteral) exprNode()    {}
func (*FunctionCall) exprNode()    {}
func (*BinaryExpr) exprNode()      {}
func (*BoolLiteral) exprNode()     {}
func (*SubQuery) exprNode()        {}

// --- order by ---

// OrderByClause represents a single sort criterion in "order by <field> [asc|desc]".
type OrderByClause struct {
	Field string // field name
	Desc  bool   // true = descending, false = ascending (default)
}

// --- assignments ---

// Assignment represents "field=value" in create/update statements.
type Assignment struct {
	Field string
	Value Expr
}

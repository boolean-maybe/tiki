package ruki

import "fmt"

// validate.go — structural validation and semantic type-checking.

// qualifierPolicy controls which old./new. qualifiers are allowed during validation.
type qualifierPolicy struct {
	allowOld bool
	allowNew bool
}

// no qualifiers allowed (standalone statements).
var noQualifiers = qualifierPolicy{}

func triggerQualifiers(event string) qualifierPolicy {
	switch event {
	case "create":
		return qualifierPolicy{allowNew: true}
	case "delete":
		return qualifierPolicy{allowOld: true}
	default: // "update"
		return qualifierPolicy{allowOld: true, allowNew: true}
	}
}

// known builtins and their return types.
var builtinFuncs = map[string]struct {
	returnType ValueType
	minArgs    int
	maxArgs    int
}{
	"count":     {ValueInt, 1, 1},
	"now":       {ValueTimestamp, 0, 0},
	"next_date": {ValueDate, 1, 1},
	"blocks":    {ValueListRef, 1, 1},
	"contains":  {ValueBool, 2, 2},
	"call":      {ValueString, 1, 1},
	"user":      {ValueString, 0, 0},
}

// --- structural validation ---

func (p *Parser) validateStatement(s *Statement) error {
	switch {
	case s.Create != nil:
		if len(s.Create.Assignments) == 0 {
			return fmt.Errorf("create must have at least one assignment")
		}
		return p.validateAssignments(s.Create.Assignments)
	case s.Update != nil:
		if len(s.Update.Set) == 0 {
			return fmt.Errorf("update must have at least one assignment in set")
		}
		if err := p.validateCondition(s.Update.Where); err != nil {
			return err
		}
		return p.validateAssignments(s.Update.Set)
	case s.Delete != nil:
		return p.validateCondition(s.Delete.Where)
	case s.Select != nil:
		if s.Select.Where != nil {
			if err := p.validateCondition(s.Select.Where); err != nil {
				return err
			}
		}
		return p.validateOrderBy(s.Select.OrderBy)
	default:
		return fmt.Errorf("empty statement")
	}
}

func (p *Parser) validateTrigger(t *Trigger) error {
	if t.Timing == "before" {
		if t.Action != nil || t.Run != nil {
			return fmt.Errorf("before-trigger must not have an action")
		}
		if t.Deny == nil {
			return fmt.Errorf("before-trigger must have deny")
		}
	}
	if t.Timing == "after" {
		if t.Deny != nil {
			return fmt.Errorf("after-trigger must not have deny")
		}
		if t.Action == nil && t.Run == nil {
			return fmt.Errorf("after-trigger must have an action")
		}
	}

	if t.Where != nil {
		if err := p.validateCondition(t.Where); err != nil {
			return err
		}
	}

	if t.Action != nil {
		if t.Action.Select != nil {
			return fmt.Errorf("trigger action must not be select")
		}
		if err := p.validateStatement(t.Action); err != nil {
			return err
		}
	}

	if t.Run != nil {
		typ, err := p.inferExprType(t.Run.Command)
		if err != nil {
			return fmt.Errorf("run command: %w", err)
		}
		if typ != ValueString {
			return fmt.Errorf("run command must be string, got %s", typeName(typ))
		}
	}

	return nil
}

func (p *Parser) validateAssignments(assignments []Assignment) error {
	seen := make(map[string]struct{}, len(assignments))
	for _, a := range assignments {
		if _, dup := seen[a.Field]; dup {
			return fmt.Errorf("duplicate assignment to field %q", a.Field)
		}
		seen[a.Field] = struct{}{}
		fs, ok := p.schema.Field(a.Field)
		if !ok {
			return fmt.Errorf("unknown field %q in assignment", a.Field)
		}
		rhsType, err := p.inferExprType(a.Value)
		if err != nil {
			return fmt.Errorf("field %q: %w", a.Field, err)
		}
		if err := p.checkAssignmentCompat(fs.Type, rhsType, a.Value); err != nil {
			return fmt.Errorf("field %q: %w", a.Field, err)
		}
	}
	return nil
}

// --- order by validation ---

func (p *Parser) validateOrderBy(clauses []OrderByClause) error {
	if len(clauses) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(clauses))
	for _, c := range clauses {
		if _, dup := seen[c.Field]; dup {
			return fmt.Errorf("duplicate field %q in order by", c.Field)
		}
		seen[c.Field] = struct{}{}
		fs, ok := p.schema.Field(c.Field)
		if !ok {
			return fmt.Errorf("unknown field %q in order by", c.Field)
		}
		if !isOrderableType(fs.Type) {
			return fmt.Errorf("cannot order by %s field %q", typeName(fs.Type), c.Field)
		}
	}
	return nil
}

func isOrderableType(t ValueType) bool {
	switch t {
	case ValueInt, ValueDate, ValueTimestamp, ValueDuration,
		ValueString, ValueStatus, ValueTaskType, ValueID, ValueRef:
		return true
	default:
		return false
	}
}

// --- condition validation with type-checking ---

func (p *Parser) validateCondition(c Condition) error {
	switch c := c.(type) {
	case *BinaryCondition:
		if err := p.validateCondition(c.Left); err != nil {
			return err
		}
		return p.validateCondition(c.Right)

	case *NotCondition:
		return p.validateCondition(c.Inner)

	case *CompareExpr:
		return p.validateCompare(c)

	case *IsEmptyExpr:
		_, err := p.inferExprType(c.Expr)
		return err

	case *InExpr:
		return p.validateIn(c)

	case *QuantifierExpr:
		return p.validateQuantifier(c)

	default:
		return fmt.Errorf("unknown condition type %T", c)
	}
}

func (p *Parser) validateCompare(c *CompareExpr) error {
	leftType, err := p.inferExprType(c.Left)
	if err != nil {
		return err
	}
	rightType, err := p.inferExprType(c.Right)
	if err != nil {
		return err
	}

	// resolve empty from context
	leftType, rightType = resolveEmptyPair(leftType, rightType)

	if !typesCompatible(leftType, rightType) {
		return fmt.Errorf("cannot compare %s %s %s", typeName(leftType), c.Op, typeName(rightType))
	}

	// reject cross-type comparisons involving enum fields,
	// unless the other side is a string literal (e.g. status = "done")
	if err := p.checkCompareCompat(leftType, rightType, c.Left, c.Right); err != nil {
		return err
	}

	// use the most specific type for operator and enum validation
	enumType := leftType
	if rightType == ValueStatus || rightType == ValueTaskType {
		enumType = rightType
	}

	if err := checkCompareOp(enumType, c.Op); err != nil {
		return err
	}

	return p.validateEnumLiterals(c.Left, c.Right, enumType)
}

func (p *Parser) validateIn(c *InExpr) error {
	valType, err := p.inferExprType(c.Value)
	if err != nil {
		return err
	}

	// infer collection type first — this validates list homogeneity
	collType, err := p.inferExprType(c.Collection)
	if err != nil {
		return err
	}

	// get the actual element type, checking literal elements directly
	elemType, err := p.inferListElementType(c.Collection)
	if err != nil {
		return err
	}

	if listElementType(collType) == -1 {
		return fmt.Errorf("%s is not a collection type; use contains() for substring checks", typeName(collType))
	}

	if !membershipCompatible(valType, elemType) {
		// allow string-like values in list literals whose elements are all string literals
		ll, isLiteral := c.Collection.(*ListLiteral)
		if !isLiteral || !isStringLike(valType) || !allStringLiterals(ll) {
			return fmt.Errorf("element type mismatch: %s in %s", typeName(valType), typeName(collType))
		}
	}

	return p.validateEnumListElements(c.Collection, valType)
}

func (p *Parser) validateQuantifier(q *QuantifierExpr) error {
	exprType, err := p.inferExprType(q.Expr)
	if err != nil {
		return err
	}
	if exprType != ValueListRef {
		return fmt.Errorf("quantifier %s requires list<ref>, got %s", q.Kind, typeName(exprType))
	}
	saved := p.qualifiers
	p.qualifiers = noQualifiers
	err = p.validateCondition(q.Condition)
	p.qualifiers = saved
	return err
}

// --- type inference ---

func (p *Parser) inferExprType(e Expr) (ValueType, error) {
	switch e := e.(type) {
	case *FieldRef:
		fs, ok := p.schema.Field(e.Name)
		if !ok {
			return 0, fmt.Errorf("unknown field %q", e.Name)
		}
		return fs.Type, nil

	case *QualifiedRef:
		if e.Qualifier == "old" && !p.qualifiers.allowOld {
			return 0, fmt.Errorf("old. qualifier is not valid in this context")
		}
		if e.Qualifier == "new" && !p.qualifiers.allowNew {
			return 0, fmt.Errorf("new. qualifier is not valid in this context")
		}
		fs, ok := p.schema.Field(e.Name)
		if !ok {
			return 0, fmt.Errorf("unknown field %q in %s.%s", e.Name, e.Qualifier, e.Name)
		}
		return fs.Type, nil

	case *StringLiteral:
		return ValueString, nil

	case *IntLiteral:
		return ValueInt, nil

	case *DateLiteral:
		return ValueDate, nil

	case *DurationLiteral:
		return ValueDuration, nil

	case *ListLiteral:
		return p.inferListType(e)

	case *EmptyLiteral:
		return -1, nil // sentinel: resolved from context

	case *FunctionCall:
		return p.inferFuncCallType(e)

	case *BinaryExpr:
		return p.inferBinaryExprType(e)

	case *SubQuery:
		return 0, fmt.Errorf("subquery is only valid as argument to count()")

	default:
		return 0, fmt.Errorf("unknown expression type %T", e)
	}
}

func (p *Parser) inferListType(l *ListLiteral) (ValueType, error) {
	if len(l.Elements) == 0 {
		return ValueListString, nil // default empty list type
	}
	firstType, err := p.inferExprType(l.Elements[0])
	if err != nil {
		return 0, err
	}
	for i := 1; i < len(l.Elements); i++ {
		t, err := p.inferExprType(l.Elements[i])
		if err != nil {
			return 0, err
		}
		if !typesCompatible(firstType, t) {
			return 0, fmt.Errorf("list elements must be the same type: got %s and %s", typeName(firstType), typeName(t))
		}
	}
	switch firstType {
	case ValueRef, ValueID:
		return ValueListRef, nil
	default:
		return ValueListString, nil
	}
}

// inferListElementType returns the element type of a list expression,
// checking literal elements directly when the list type enum is too coarse.
func (p *Parser) inferListElementType(e Expr) (ValueType, error) {
	if ll, ok := e.(*ListLiteral); ok && len(ll.Elements) > 0 {
		return p.inferExprType(ll.Elements[0])
	}
	collType, err := p.inferExprType(e)
	if err != nil {
		return 0, err
	}
	elem := listElementType(collType)
	if elem == -1 {
		return collType, nil // not a list type — return as-is for error reporting
	}
	return elem, nil
}

func (p *Parser) inferFuncCallType(fc *FunctionCall) (ValueType, error) {
	builtin, ok := builtinFuncs[fc.Name]
	if !ok {
		return 0, fmt.Errorf("unknown function %q", fc.Name)
	}
	if len(fc.Args) < builtin.minArgs || len(fc.Args) > builtin.maxArgs {
		if builtin.minArgs == builtin.maxArgs {
			return 0, fmt.Errorf("%s() expects %d argument(s), got %d", fc.Name, builtin.minArgs, len(fc.Args))
		}
		return 0, fmt.Errorf("%s() expects %d-%d arguments, got %d", fc.Name, builtin.minArgs, builtin.maxArgs, len(fc.Args))
	}

	// validate argument types for specific functions
	switch fc.Name {
	case "count":
		sq, ok := fc.Args[0].(*SubQuery)
		if !ok {
			return 0, fmt.Errorf("count() argument must be a select subquery")
		}
		if sq.Where != nil {
			if err := p.validateCondition(sq.Where); err != nil {
				return 0, fmt.Errorf("count() subquery: %w", err)
			}
		}
	case "blocks":
		argType, err := p.inferExprType(fc.Args[0])
		if err != nil {
			return 0, err
		}
		if argType != ValueID && argType != ValueRef && argType != ValueString {
			return 0, fmt.Errorf("blocks() argument must be an id or ref, got %s", typeName(argType))
		}
		if argType == ValueString {
			if _, ok := fc.Args[0].(*StringLiteral); !ok {
				return 0, fmt.Errorf("blocks() argument must be an id or ref, got %s", typeName(argType))
			}
		}
	case "contains":
		for i, arg := range fc.Args {
			t, err := p.inferExprType(arg)
			if err != nil {
				return 0, err
			}
			if t != ValueString {
				return 0, fmt.Errorf("contains() argument %d must be string, got %s", i+1, typeName(t))
			}
		}
	case "call":
		t, err := p.inferExprType(fc.Args[0])
		if err != nil {
			return 0, err
		}
		if t != ValueString {
			return 0, fmt.Errorf("call() argument must be string, got %s", typeName(t))
		}
	case "next_date":
		t, err := p.inferExprType(fc.Args[0])
		if err != nil {
			return 0, err
		}
		if t != ValueRecurrence {
			return 0, fmt.Errorf("next_date() argument must be recurrence, got %s", typeName(t))
		}
	}

	return builtin.returnType, nil
}

func (p *Parser) inferBinaryExprType(b *BinaryExpr) (ValueType, error) {
	leftType, err := p.inferExprType(b.Left)
	if err != nil {
		return 0, err
	}
	rightType, err := p.inferExprType(b.Right)
	if err != nil {
		return 0, err
	}

	leftType, rightType = resolveEmptyPair(leftType, rightType)

	switch b.Op {
	case "+":
		return p.inferPlusType(leftType, rightType, b.Right)
	case "-":
		return p.inferMinusType(leftType, rightType, b.Right)
	default:
		return 0, fmt.Errorf("unknown binary operator %q", b.Op)
	}
}

func isStringLike(t ValueType) bool {
	switch t {
	case ValueString, ValueStatus, ValueTaskType, ValueID, ValueRef:
		return true
	default:
		return false
	}
}

func (p *Parser) inferPlusType(left, right ValueType, rightExpr Expr) (ValueType, error) {
	switch {
	case isStringLike(left) && isStringLike(right):
		return ValueString, nil
	case left == ValueInt && right == ValueInt:
		return ValueInt, nil
	case left == ValueListString && (right == ValueString || right == ValueListString):
		return ValueListString, nil
	case left == ValueListRef && (isRefCompatible(right) || right == ValueListRef):
		return ValueListRef, nil
	case left == ValueListRef && right == ValueString:
		if _, ok := rightExpr.(*StringLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot add %s + %s", typeName(left), typeName(right))
	case left == ValueListRef && right == ValueListString:
		if _, ok := rightExpr.(*ListLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot add list<string> field to list<ref>")
	case left == ValueDate && right == ValueDuration:
		return ValueDate, nil
	case left == ValueTimestamp && right == ValueDuration:
		return ValueTimestamp, nil
	default:
		return 0, fmt.Errorf("cannot add %s + %s", typeName(left), typeName(right))
	}
}

func (p *Parser) inferMinusType(left, right ValueType, rightExpr Expr) (ValueType, error) {
	switch {
	case left == ValueListString && (right == ValueString || right == ValueListString):
		return ValueListString, nil
	case left == ValueListRef && (isRefCompatible(right) || right == ValueListRef):
		return ValueListRef, nil
	case left == ValueListRef && right == ValueString:
		if _, ok := rightExpr.(*StringLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot subtract %s - %s", typeName(left), typeName(right))
	case left == ValueListRef && right == ValueListString:
		if _, ok := rightExpr.(*ListLiteral); ok {
			return ValueListRef, nil
		}
		return 0, fmt.Errorf("cannot subtract list<string> field from list<ref>")
	case left == ValueInt && right == ValueInt:
		return ValueInt, nil
	case left == ValueDate && right == ValueDuration:
		return ValueDate, nil
	case left == ValueDate && right == ValueDate:
		return ValueDuration, nil
	case left == ValueTimestamp && right == ValueDuration:
		return ValueTimestamp, nil
	case left == ValueTimestamp && right == ValueTimestamp:
		return ValueDuration, nil
	default:
		return 0, fmt.Errorf("cannot subtract %s - %s", typeName(left), typeName(right))
	}
}

// --- enum literal validation ---

func (p *Parser) validateEnumLiterals(left, right Expr, resolvedType ValueType) error {
	if resolvedType == ValueStatus {
		if s, ok := right.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
				return fmt.Errorf("unknown status %q", s.Value)
			}
		}
		if s, ok := left.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
				return fmt.Errorf("unknown status %q", s.Value)
			}
		}
	}
	if resolvedType == ValueTaskType {
		if s, ok := right.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeType(s.Value); !valid {
				return fmt.Errorf("unknown type %q", s.Value)
			}
		}
		if s, ok := left.(*StringLiteral); ok {
			if _, valid := p.schema.NormalizeType(s.Value); !valid {
				return fmt.Errorf("unknown type %q", s.Value)
			}
		}
	}
	return nil
}

// validateEnumListElements checks string literals inside a list expression
// against the appropriate enum normalizer, based on the value type being checked.
func (p *Parser) validateEnumListElements(collection Expr, valType ValueType) error {
	ll, ok := collection.(*ListLiteral)
	if !ok {
		return nil
	}
	for _, elem := range ll.Elements {
		s, ok := elem.(*StringLiteral)
		if !ok {
			continue
		}
		if valType == ValueStatus {
			if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
				return fmt.Errorf("unknown status %q", s.Value)
			}
		}
		if valType == ValueTaskType {
			if _, valid := p.schema.NormalizeType(s.Value); !valid {
				return fmt.Errorf("unknown type %q", s.Value)
			}
		}
	}
	return nil
}

// --- assignment compatibility ---

func (p *Parser) checkAssignmentCompat(fieldType, rhsType ValueType, rhs Expr) error {
	// empty is assignable to anything
	if _, ok := rhs.(*EmptyLiteral); ok {
		return nil
	}
	if rhsType == -1 { // unresolved empty
		return nil
	}

	if typesCompatible(fieldType, rhsType) {
		// enum fields only accept same-type or string literals
		if (fieldType == ValueStatus || fieldType == ValueTaskType) && rhsType != fieldType {
			if _, ok := rhs.(*StringLiteral); !ok {
				return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
			}
		}
		// non-enum string-like fields reject enum-typed RHS
		if (fieldType == ValueString || fieldType == ValueID || fieldType == ValueRef) &&
			(rhsType == ValueStatus || rhsType == ValueTaskType) {
			return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
		}

		// list<string> field rejects list literals with non-string elements
		if fieldType == ValueListString {
			if ll, ok := rhs.(*ListLiteral); ok {
				for _, elem := range ll.Elements {
					elemType, err := p.inferExprType(elem)
					if err == nil && elemType != ValueString {
						if _, isLit := elem.(*StringLiteral); !isLit {
							return fmt.Errorf("cannot assign %s to list<string> field", typeName(elemType))
						}
					}
				}
			}
		}

		// validate enum values
		if fieldType == ValueStatus {
			if s, ok := rhs.(*StringLiteral); ok {
				if _, valid := p.schema.NormalizeStatus(s.Value); !valid {
					return fmt.Errorf("unknown status %q", s.Value)
				}
			}
		}
		if fieldType == ValueTaskType {
			if s, ok := rhs.(*StringLiteral); ok {
				if _, valid := p.schema.NormalizeType(s.Value); !valid {
					return fmt.Errorf("unknown type %q", s.Value)
				}
			}
		}
		return nil
	}

	// list<string> literal is assignable to list<ref>, but only if all elements are string literals
	if fieldType == ValueListRef && rhsType == ValueListString {
		if ll, ok := rhs.(*ListLiteral); ok && allStringLiterals(ll) {
			return nil
		}
	}

	return fmt.Errorf("cannot assign %s to %s field", typeName(rhsType), typeName(fieldType))
}

// --- type helpers ---

func typesCompatible(a, b ValueType) bool {
	if a == b {
		return true
	}
	if a == -1 || b == -1 { // unresolved empty
		return true
	}
	// string-like types are compatible with each other
	stringLike := map[ValueType]bool{
		ValueString:   true,
		ValueStatus:   true,
		ValueTaskType: true,
		ValueID:       true,
		ValueRef:      true,
	}
	return stringLike[a] && stringLike[b]
}

func isEnumType(t ValueType) bool {
	return t == ValueStatus || t == ValueTaskType
}

// allStringLiterals returns true if every element in the list is a *StringLiteral.
func allStringLiterals(ll *ListLiteral) bool {
	for _, elem := range ll.Elements {
		if _, ok := elem.(*StringLiteral); !ok {
			return false
		}
	}
	return true
}

// checkCompareCompat rejects nonsensical cross-type comparisons in WHERE clauses.
// e.g. status = title (enum vs string field) is rejected,
// but status = "done" (enum vs string literal) is allowed.
func (p *Parser) checkCompareCompat(leftType, rightType ValueType, left, right Expr) error {
	if isEnumType(leftType) && rightType != leftType {
		if err := checkEnumOperand(leftType, rightType, right); err != nil {
			return err
		}
	}
	if isEnumType(rightType) && leftType != rightType {
		if err := checkEnumOperand(rightType, leftType, left); err != nil {
			return err
		}
	}
	return nil
}

func checkEnumOperand(enumType, otherType ValueType, other Expr) error {
	if otherType == ValueString {
		if _, ok := other.(*StringLiteral); !ok {
			return fmt.Errorf("cannot compare %s with %s field", typeName(enumType), typeName(otherType))
		}
		return nil
	}
	return fmt.Errorf("cannot compare %s with %s", typeName(enumType), typeName(otherType))
}

// membershipCompatible checks strict type compatibility for in/not in
// expressions. Unlike typesCompatible, it does not treat all string-like
// types as interchangeable — only ID and Ref are interchangeable.
func membershipCompatible(a, b ValueType) bool {
	if a == b {
		return true
	}
	if a == -1 || b == -1 {
		return true
	}
	// ID and Ref are the same concept
	if (a == ValueID || a == ValueRef) && (b == ValueID || b == ValueRef) {
		return true
	}
	return false
}

// isRefCompatible returns true for types that can appear as operands
// in list<ref> add/remove operations.
func isRefCompatible(t ValueType) bool {
	switch t {
	case ValueRef, ValueID:
		return true
	default:
		return false
	}
}

func resolveEmptyPair(a, b ValueType) (ValueType, ValueType) {
	if a == -1 && b != -1 {
		a = b
	}
	if b == -1 && a != -1 {
		b = a
	}
	return a, b
}

func listElementType(t ValueType) ValueType {
	switch t {
	case ValueListString:
		return ValueString
	case ValueListRef:
		return ValueRef
	default:
		return -1
	}
}

func checkCompareOp(t ValueType, op string) error {
	switch op {
	case "=", "!=":
		return nil // all types support equality
	case "<", ">", "<=", ">=":
		switch t {
		case ValueInt, ValueDate, ValueTimestamp, ValueDuration:
			return nil
		default:
			return fmt.Errorf("operator %s not supported for %s", op, typeName(t))
		}
	default:
		return fmt.Errorf("unknown operator %q", op)
	}
}

func typeName(t ValueType) string {
	switch t {
	case ValueString:
		return "string"
	case ValueInt:
		return "int"
	case ValueDate:
		return "date"
	case ValueTimestamp:
		return "timestamp"
	case ValueDuration:
		return "duration"
	case ValueBool:
		return "bool"
	case ValueID:
		return "id"
	case ValueRef:
		return "ref"
	case ValueRecurrence:
		return "recurrence"
	case ValueListString:
		return "list<string>"
	case ValueListRef:
		return "list<ref>"
	case ValueStatus:
		return "status"
	case ValueTaskType:
		return "type"
	case -1:
		return "empty"
	default:
		return "unknown"
	}
}

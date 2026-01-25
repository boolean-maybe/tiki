package filter

import (
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/task"
)

// FilterExpr represents a filter expression that can be evaluated against a task
type FilterExpr interface {
	Evaluate(task *task.Task, now time.Time, currentUser string) bool
}

// BinaryExpr represents AND, OR operations
type BinaryExpr struct {
	Op    string // "AND", "OR"
	Left  FilterExpr
	Right FilterExpr
}

// Evaluate implements FilterExpr
func (b *BinaryExpr) Evaluate(task *task.Task, now time.Time, currentUser string) bool {
	switch strings.ToUpper(b.Op) {
	case "AND":
		return b.Left.Evaluate(task, now, currentUser) && b.Right.Evaluate(task, now, currentUser)
	case "OR":
		return b.Left.Evaluate(task, now, currentUser) || b.Right.Evaluate(task, now, currentUser)
	default:
		return false
	}
}

// UnaryExpr represents NOT operation
type UnaryExpr struct {
	Op   string // "NOT"
	Expr FilterExpr
}

// Evaluate implements FilterExpr
func (u *UnaryExpr) Evaluate(task *task.Task, now time.Time, currentUser string) bool {
	if strings.ToUpper(u.Op) == "NOT" {
		return !u.Expr.Evaluate(task, now, currentUser)
	}
	return false
}

// CompareExpr represents comparisons like status = 'ready' or Priority < 3
type CompareExpr struct {
	Field string      // "status", "type", "assignee", "priority", "points", "createdat", "updatedat", "tags"
	Op    string      // "=", "==", "!=", ">", "<", ">=", "<="
	Value interface{} // string, int, or TimeExpr
}

// InExpr represents IN/NOT IN operations like: tags IN ['ui', 'charts', 'viz']
type InExpr struct {
	Field  string        // "status", "type", "tags", etc.
	Not    bool          // true for NOT IN, false for IN
	Values []interface{} // List of values to check against (strings, ints, etc.)
}

// Evaluate implements FilterExpr for InExpr
func (i *InExpr) Evaluate(task *task.Task, now time.Time, currentUser string) bool {
	// Handle CURRENT_USER in the values list
	resolvedValues := make([]interface{}, len(i.Values))
	for idx, val := range i.Values {
		if strVal, ok := val.(string); ok && strings.ToUpper(strVal) == "CURRENT_USER" {
			resolvedValues[idx] = currentUser
		} else {
			resolvedValues[idx] = val
		}
	}

	// Special handling for tags (array field)
	if strings.ToLower(i.Field) == "tags" || strings.ToLower(i.Field) == "tag" {
		result := evaluateTagsInComparison(task.Tags, resolvedValues)
		if i.Not {
			return !result
		}
		return result
	}

	// For non-array fields, check if field value is in the list
	fieldValue := getTaskAttribute(task, i.Field)
	result := valueInList(fieldValue, resolvedValues)
	if i.Not {
		return !result
	}
	return result
}

// Evaluate implements FilterExpr for CompareExpr
func (c *CompareExpr) Evaluate(task *task.Task, now time.Time, currentUser string) bool {
	// Handle time expression comparisons (e.g., NOW - CreatedAt < 24hour)
	if c.Field == "time_expr" {
		return c.evaluateTimeExpr(task, now)
	}

	fieldValue := getTaskAttribute(task, c.Field)
	compareValue := c.Value

	// Handle CURRENT_USER special value
	if strVal, ok := compareValue.(string); ok && strings.ToUpper(strVal) == "CURRENT_USER" {
		compareValue = currentUser
	}

	// Handle TimeExpr (for NOW - CreatedAt type comparisons)
	if timeExpr, ok := compareValue.(*TimeExpr); ok {
		compareValue = timeExpr.Evaluate(task, now)
	}

	// Handle DurationValue
	if dv, ok := compareValue.(*DurationValue); ok {
		compareValue = dv.Duration
	}

	// Handle tags specially - check if tag is in the list
	if strings.ToLower(c.Field) == "tags" || strings.ToLower(c.Field) == "tag" {
		return evaluateTagComparison(task.Tags, c.Op, compareValue)
	}

	return compare(fieldValue, c.Op, compareValue)
}

// evaluateTimeExpr handles time expression comparisons like "NOW - CreatedAt < 24hour"
func (c *CompareExpr) evaluateTimeExpr(task *task.Task, now time.Time) bool {
	tec, ok := c.Value.(*timeExprCompare)
	if !ok {
		return false
	}

	leftValue := tec.left.Evaluate(task, now)
	rightValue := tec.right

	// Handle DurationValue
	if dv, ok := rightValue.(*DurationValue); ok {
		rightValue = dv.Duration
	}

	return compare(leftValue, c.Op, rightValue)
}

// TimeExpr represents time arithmetic like NOW - 24hour or NOW - CreatedAt
type TimeExpr struct {
	Base    string      // "NOW", "CreatedAt", "UpdatedAt"
	Op      string      // "+", "-"
	Operand interface{} // time.Duration or field name string
}

// Evaluate returns the computed time or duration value
func (t *TimeExpr) Evaluate(task *task.Task, now time.Time) interface{} {
	var baseTime time.Time

	switch strings.ToLower(t.Base) {
	case "now":
		baseTime = now
	case "createdat":
		baseTime = task.CreatedAt
	case "updatedat":
		baseTime = task.UpdatedAt
	default:
		return nil
	}

	if t.Op == "" {
		return baseTime
	}

	// Handle duration operand
	if dur, ok := t.Operand.(time.Duration); ok {
		if t.Op == "-" {
			return baseTime.Add(-dur)
		}
		return baseTime.Add(dur)
	}

	// Handle field name operand (e.g., NOW - CreatedAt returns duration)
	if fieldName, ok := t.Operand.(string); ok {
		var otherTime time.Time
		switch strings.ToLower(fieldName) {
		case "now":
			otherTime = now
		case "createdat":
			otherTime = task.CreatedAt
		case "updatedat":
			otherTime = task.UpdatedAt
		default:
			return nil
		}

		if t.Op == "-" {
			return baseTime.Sub(otherTime)
		}
		// Addition of times doesn't make sense, return nil
		return nil
	}

	return baseTime
}

// DurationValue represents a parsed duration for comparison
type DurationValue struct {
	Duration time.Duration
}

// timeExprCompare wraps a time expression comparison for evaluation
type timeExprCompare struct {
	left  *TimeExpr
	right interface{}
}

// getTaskAttribute returns the value of a task field by name
func getTaskAttribute(task *task.Task, field string) interface{} {
	switch strings.ToLower(field) {
	case "status":
		return string(task.Status)
	case "type":
		return string(task.Type)
	case "assignee":
		return task.Assignee
	case "priority":
		return task.Priority
	case "points":
		return task.Points
	case "createdat":
		return task.CreatedAt
	case "updatedat":
		return task.UpdatedAt
	case "tags":
		return task.Tags
	case "id":
		return task.ID
	case "title":
		return task.Title
	default:
		return nil
	}
}

// evaluateTagComparison checks if a tag matches the comparison
func evaluateTagComparison(tags []string, op string, value interface{}) bool {
	strVal, ok := value.(string)
	if !ok {
		return false
	}

	// Check if any tag matches
	found := false
	for _, tag := range tags {
		if strings.EqualFold(tag, strVal) {
			found = true
			break
		}
	}

	switch op {
	case "=", "==":
		return found
	case "!=":
		return !found
	default:
		return false
	}
}

// evaluateTagsInComparison checks if ANY task tag matches ANY value in the list
// Semantics: task.Tags ∩ values != ∅
func evaluateTagsInComparison(taskTags []string, values []interface{}) bool {
	for _, taskTag := range taskTags {
		for _, val := range values {
			if strVal, ok := val.(string); ok {
				if strings.EqualFold(taskTag, strVal) {
					return true
				}
			}
		}
	}
	return false
}

// valueInList checks if a single value exists in a list of values
func valueInList(fieldValue interface{}, values []interface{}) bool {
	for _, val := range values {
		// String comparison (case-insensitive)
		if fvStr, ok := fieldValue.(string); ok {
			if valStr, ok := val.(string); ok {
				if strings.EqualFold(fvStr, valStr) {
					return true
				}
			}
		}
		// Integer comparison
		if fvInt, ok := fieldValue.(int); ok {
			if valInt, ok := val.(int); ok {
				if fvInt == valInt {
					return true
				}
			}
		}
		// Direct equality for other types
		if fieldValue == val {
			return true
		}
	}
	return false
}

// compare compares two values using the given operator
func compare(left interface{}, op string, right interface{}) bool {
	// Normalize operator
	if op == "==" {
		op = "="
	}

	// String comparison
	if leftStr, ok := left.(string); ok {
		rightStr, ok := right.(string)
		if !ok {
			return false
		}
		return compareStrings(leftStr, op, rightStr)
	}

	// Integer comparison
	if leftInt, ok := left.(int); ok {
		rightInt, ok := right.(int)
		if !ok {
			return false
		}
		return compareInts(leftInt, op, rightInt)
	}

	// Time comparison
	if leftTime, ok := left.(time.Time); ok {
		if rightTime, ok := right.(time.Time); ok {
			return compareTimes(leftTime, op, rightTime)
		}
	}

	// Duration comparison
	if leftDur, ok := left.(time.Duration); ok {
		if rightDur, ok := right.(time.Duration); ok {
			return compareDurations(leftDur, op, rightDur)
		}
		// Compare duration with DurationValue
		if rightDurVal, ok := right.(*DurationValue); ok {
			return compareDurations(leftDur, op, rightDurVal.Duration)
		}
	}

	return false
}

func compareStrings(left, op, right string) bool {
	// Case-insensitive comparison
	left = strings.ToLower(left)
	right = strings.ToLower(right)

	switch op {
	case "=":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func compareInts(left int, op string, right int) bool {
	switch op {
	case "=":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

func compareTimes(left time.Time, op string, right time.Time) bool {
	switch op {
	case "=":
		return left.Equal(right)
	case "!=":
		return !left.Equal(right)
	case ">":
		return left.After(right)
	case "<":
		return left.Before(right)
	case ">=":
		return left.After(right) || left.Equal(right)
	case "<=":
		return left.Before(right) || left.Equal(right)
	default:
		return false
	}
}

func compareDurations(left time.Duration, op string, right time.Duration) bool {
	switch op {
	case "=":
		return left == right
	case "!=":
		return left != right
	case ">":
		return left > right
	case "<":
		return left < right
	case ">=":
		return left >= right
	case "<=":
		return left <= right
	default:
		return false
	}
}

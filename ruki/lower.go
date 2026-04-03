package ruki

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// lower.go converts participle grammar structs into clean AST types.

func lowerStatement(g *statementGrammar) (*Statement, error) {
	switch {
	case g.Select != nil:
		s, err := lowerSelect(g.Select)
		if err != nil {
			return nil, err
		}
		return &Statement{Select: s}, nil
	case g.Create != nil:
		s, err := lowerCreate(g.Create)
		if err != nil {
			return nil, err
		}
		return &Statement{Create: s}, nil
	case g.Update != nil:
		s, err := lowerUpdate(g.Update)
		if err != nil {
			return nil, err
		}
		return &Statement{Update: s}, nil
	case g.Delete != nil:
		s, err := lowerDelete(g.Delete)
		if err != nil {
			return nil, err
		}
		return &Statement{Delete: s}, nil
	default:
		return nil, fmt.Errorf("empty statement")
	}
}

func lowerSelect(g *selectGrammar) (*SelectStmt, error) {
	var where Condition
	if g.Where != nil {
		var err error
		where, err = lowerOrCond(g.Where)
		if err != nil {
			return nil, err
		}
	}
	orderBy := lowerOrderBy(g.OrderBy)
	return &SelectStmt{Where: where, OrderBy: orderBy}, nil
}

func lowerCreate(g *createGrammar) (*CreateStmt, error) {
	assignments, err := lowerAssignments(g.Assignments)
	if err != nil {
		return nil, err
	}
	return &CreateStmt{Assignments: assignments}, nil
}

func lowerUpdate(g *updateGrammar) (*UpdateStmt, error) {
	where, err := lowerOrCond(&g.Where)
	if err != nil {
		return nil, err
	}
	set, err := lowerAssignments(g.Set)
	if err != nil {
		return nil, err
	}
	return &UpdateStmt{Where: where, Set: set}, nil
}

func lowerDelete(g *deleteGrammar) (*DeleteStmt, error) {
	where, err := lowerOrCond(&g.Where)
	if err != nil {
		return nil, err
	}
	return &DeleteStmt{Where: where}, nil
}

func lowerAssignments(gs []assignmentGrammar) ([]Assignment, error) {
	result := make([]Assignment, len(gs))
	for i, g := range gs {
		val, err := lowerExpr(&g.Value)
		if err != nil {
			return nil, err
		}
		result[i] = Assignment{Field: g.Field, Value: val}
	}
	return result, nil
}

// --- trigger lowering ---

func lowerTrigger(g *triggerGrammar) (*Trigger, error) {
	t := &Trigger{
		Timing: g.Timing,
		Event:  g.Event,
	}

	if g.Where != nil {
		where, err := lowerOrCond(g.Where)
		if err != nil {
			return nil, err
		}
		t.Where = where
	}

	if g.Action != nil {
		if err := lowerTriggerAction(g.Action, t); err != nil {
			return nil, err
		}
	}

	if g.Deny != nil {
		msg := unquoteString(g.Deny.Message)
		t.Deny = &msg
	}

	return t, nil
}

func lowerTriggerAction(g *actionGrammar, t *Trigger) error {
	switch {
	case g.Run != nil:
		cmd, err := lowerExpr(&g.Run.Command)
		if err != nil {
			return err
		}
		t.Run = &RunAction{Command: cmd}
	case g.Create != nil:
		s, err := lowerCreate(g.Create)
		if err != nil {
			return err
		}
		t.Action = &Statement{Create: s}
	case g.Update != nil:
		s, err := lowerUpdate(g.Update)
		if err != nil {
			return err
		}
		t.Action = &Statement{Update: s}
	case g.Delete != nil:
		s, err := lowerDelete(g.Delete)
		if err != nil {
			return err
		}
		t.Action = &Statement{Delete: s}
	default:
		return fmt.Errorf("empty trigger action")
	}
	return nil
}

// --- condition lowering ---

func lowerOrCond(g *orCond) (Condition, error) {
	left, err := lowerAndCond(&g.Left)
	if err != nil {
		return nil, err
	}
	for _, r := range g.Right {
		right, err := lowerAndCond(&r)
		if err != nil {
			return nil, err
		}
		left = &BinaryCondition{Op: "or", Left: left, Right: right}
	}
	return left, nil
}

func lowerAndCond(g *andCond) (Condition, error) {
	left, err := lowerNotCond(&g.Left)
	if err != nil {
		return nil, err
	}
	for _, r := range g.Right {
		right, err := lowerNotCond(&r)
		if err != nil {
			return nil, err
		}
		left = &BinaryCondition{Op: "and", Left: left, Right: right}
	}
	return left, nil
}

func lowerNotCond(g *notCond) (Condition, error) {
	if g.Not != nil {
		inner, err := lowerNotCond(g.Not)
		if err != nil {
			return nil, err
		}
		return &NotCondition{Inner: inner}, nil
	}
	return lowerPrimaryCond(g.Primary)
}

func lowerPrimaryCond(g *primaryCond) (Condition, error) {
	if g.Paren != nil {
		return lowerOrCond(g.Paren)
	}
	return lowerExprCond(g.Expr)
}

func lowerExprCond(g *exprCond) (Condition, error) {
	left, err := lowerExpr(&g.Left)
	if err != nil {
		return nil, err
	}

	switch {
	case g.Compare != nil:
		right, err := lowerExpr(&g.Compare.Right)
		if err != nil {
			return nil, err
		}
		return &CompareExpr{Left: left, Op: g.Compare.Op, Right: right}, nil

	case g.IsEmpty != nil:
		return &IsEmptyExpr{Expr: left, Negated: false}, nil

	case g.IsNotEmpty != nil:
		return &IsEmptyExpr{Expr: left, Negated: true}, nil

	case g.In != nil:
		coll, err := lowerExpr(&g.In.Collection)
		if err != nil {
			return nil, err
		}
		return &InExpr{Value: left, Collection: coll, Negated: false}, nil

	case g.NotIn != nil:
		coll, err := lowerExpr(&g.NotIn.Collection)
		if err != nil {
			return nil, err
		}
		return &InExpr{Value: left, Collection: coll, Negated: true}, nil

	case g.Any != nil:
		cond, err := lowerPrimaryCond(&g.Any.Condition)
		if err != nil {
			return nil, err
		}
		return &QuantifierExpr{Expr: left, Kind: "any", Condition: cond}, nil

	case g.All != nil:
		cond, err := lowerPrimaryCond(&g.All.Condition)
		if err != nil {
			return nil, err
		}
		return &QuantifierExpr{Expr: left, Kind: "all", Condition: cond}, nil

	default:
		// bare expression used as condition — this is a parse error
		return nil, fmt.Errorf("expression used as condition without comparison operator")
	}
}

// --- expression lowering ---

func lowerExpr(g *exprGrammar) (Expr, error) {
	left, err := lowerUnary(&g.Left)
	if err != nil {
		return nil, err
	}
	for _, tail := range g.Tail {
		right, err := lowerUnary(&tail.Right)
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Op: tail.Op, Left: left, Right: right}
	}
	return left, nil
}

func lowerUnary(g *unaryExpr) (Expr, error) {
	switch {
	case g.FuncCall != nil:
		return lowerFuncCall(g.FuncCall)
	case g.SubQuery != nil:
		return lowerSubQuery(g.SubQuery)
	case g.QualRef != nil:
		return &QualifiedRef{Qualifier: g.QualRef.Qualifier, Name: g.QualRef.Name}, nil
	case g.ListLit != nil:
		return lowerListLit(g.ListLit)
	case g.StrLit != nil:
		return &StringLiteral{Value: unquoteString(*g.StrLit)}, nil
	case g.DateLit != nil:
		return parseDateLiteral(*g.DateLit)
	case g.DurLit != nil:
		return parseDurationLiteral(*g.DurLit)
	case g.IntLit != nil:
		return &IntLiteral{Value: *g.IntLit}, nil
	case g.Empty != nil:
		return &EmptyLiteral{}, nil
	case g.FieldRef != nil:
		return &FieldRef{Name: *g.FieldRef}, nil
	case g.Paren != nil:
		return lowerExpr(g.Paren)
	default:
		return nil, fmt.Errorf("empty expression")
	}
}

func lowerFuncCall(g *funcCallExpr) (Expr, error) {
	args := make([]Expr, len(g.Args))
	for i, a := range g.Args {
		arg, err := lowerExpr(&a)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}
	return &FunctionCall{Name: g.Name, Args: args}, nil
}

func lowerSubQuery(g *subQueryExpr) (Expr, error) {
	if g.OrderBy != nil {
		return nil, fmt.Errorf("order by is not valid inside a subquery")
	}
	var where Condition
	if g.Where != nil {
		var err error
		where, err = lowerOrCond(g.Where)
		if err != nil {
			return nil, err
		}
	}
	return &SubQuery{Where: where}, nil
}

func lowerListLit(g *listLitExpr) (Expr, error) {
	elems := make([]Expr, len(g.Elements))
	for i, e := range g.Elements {
		elem, err := lowerExpr(&e)
		if err != nil {
			return nil, err
		}
		elems[i] = elem
	}
	return &ListLiteral{Elements: elems}, nil
}

// --- order by lowering ---

func lowerOrderBy(g *orderByGrammar) []OrderByClause {
	if g == nil {
		return nil
	}
	clauses := make([]OrderByClause, 0, 1+len(g.Rest))
	clauses = append(clauses, lowerOrderByField(&g.First))
	for i := range g.Rest {
		clauses = append(clauses, lowerOrderByField(&g.Rest[i]))
	}
	return clauses
}

func lowerOrderByField(g *orderByField) OrderByClause {
	desc := g.Direction != nil && *g.Direction == "desc"
	return OrderByClause{Field: g.Field, Desc: desc}
}

// --- literal helpers ---

func unquoteString(s string) string {
	// strip surrounding quotes and unescape
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		unquoted, err := strconv.Unquote(s)
		if err == nil {
			return unquoted
		}
		// fallback: just strip quotes
		return s[1 : len(s)-1]
	}
	return s
}

func parseDateLiteral(s string) (Expr, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("invalid date literal %q: %w", s, err)
	}
	return &DateLiteral{Value: t}, nil
}

func parseDurationLiteral(s string) (Expr, error) {
	// find where digits end and unit begins
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i == len(s) {
		return nil, fmt.Errorf("invalid duration literal %q", s)
	}

	val, err := strconv.Atoi(s[:i])
	if err != nil {
		return nil, fmt.Errorf("invalid duration value in %q: %w", s, err)
	}

	unit := strings.TrimSuffix(s[i:], "s") // normalize "days" → "day"
	return &DurationLiteral{Value: val, Unit: unit}, nil
}

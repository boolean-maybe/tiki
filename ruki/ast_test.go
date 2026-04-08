package ruki

import "testing"

// TestConditionNodeInterface verifies all Condition implementors satisfy the interface.
func TestConditionNodeInterface(t *testing.T) {
	conditions := []Condition{
		&BinaryCondition{Op: "and"},
		&NotCondition{},
		&CompareExpr{Op: "="},
		&IsEmptyExpr{},
		&InExpr{},
		&QuantifierExpr{Kind: "any"},
	}

	for _, c := range conditions {
		c.conditionNode() // exercise marker method
	}

	if len(conditions) != 6 {
		t.Errorf("expected 6 condition types, got %d", len(conditions))
	}
}

// TestExprNodeInterface verifies all Expr implementors satisfy the interface.
func TestExprNodeInterface(t *testing.T) {
	exprs := []Expr{
		&FieldRef{Name: "status"},
		&QualifiedRef{Qualifier: "old", Name: "status"},
		&StringLiteral{Value: "hello"},
		&IntLiteral{Value: 42},
		&DateLiteral{},
		&DurationLiteral{Value: 1, Unit: "day"},
		&ListLiteral{},
		&EmptyLiteral{},
		&FunctionCall{Name: "now"},
		&BinaryExpr{Op: "+"},
		&SubQuery{},
	}

	for _, e := range exprs {
		e.exprNode() // exercise marker method
	}

	if len(exprs) != 11 {
		t.Errorf("expected 11 expr types, got %d", len(exprs))
	}
}

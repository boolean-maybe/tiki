package filter

import (
	"testing"
)

func validStatus(key string) bool {
	valid := map[string]bool{
		"backlog": true, "ready": true, "inProgress": true, "review": true, "done": true,
	}
	return valid[key]
}

func TestValidateFilterStatuses_ValidCompare(t *testing.T) {
	expr := &CompareExpr{Field: "status", Op: "=", Value: "ready"}
	if err := ValidateFilterStatuses(expr, validStatus); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateFilterStatuses_InvalidCompare(t *testing.T) {
	expr := &CompareExpr{Field: "status", Op: "=", Value: "bogus"}
	err := ValidateFilterStatuses(expr, validStatus)
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
	if got := err.Error(); got != `filter references unknown status "bogus"` {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestValidateFilterStatuses_ValidInExpr(t *testing.T) {
	expr := &InExpr{Field: "status", Values: []interface{}{"ready", "done"}}
	if err := ValidateFilterStatuses(expr, validStatus); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateFilterStatuses_InvalidInExpr(t *testing.T) {
	expr := &InExpr{Field: "status", Values: []interface{}{"ready", "unknown"}}
	err := ValidateFilterStatuses(expr, validStatus)
	if err == nil {
		t.Fatal("expected error for unknown status in IN list")
	}
}

func TestValidateFilterStatuses_NonStatusField(t *testing.T) {
	expr := &CompareExpr{Field: "type", Op: "=", Value: "anything"}
	if err := ValidateFilterStatuses(expr, validStatus); err != nil {
		t.Errorf("expected no error for non-status field, got: %v", err)
	}
}

func TestValidateFilterStatuses_BinaryExpr(t *testing.T) {
	expr := &BinaryExpr{
		Op:    "AND",
		Left:  &CompareExpr{Field: "status", Op: "=", Value: "ready"},
		Right: &CompareExpr{Field: "status", Op: "!=", Value: "bogus"},
	}
	err := ValidateFilterStatuses(expr, validStatus)
	if err == nil {
		t.Fatal("expected error for unknown status in right branch")
	}
}

func TestValidateFilterStatuses_UnaryExpr(t *testing.T) {
	expr := &UnaryExpr{
		Op:   "NOT",
		Expr: &CompareExpr{Field: "status", Op: "=", Value: "invalid"},
	}
	err := ValidateFilterStatuses(expr, validStatus)
	if err == nil {
		t.Fatal("expected error for unknown status in NOT expr")
	}
}

func TestValidateFilterStatuses_Nil(t *testing.T) {
	if err := ValidateFilterStatuses(nil, validStatus); err != nil {
		t.Errorf("expected no error for nil expr, got: %v", err)
	}
}

func TestValidateFilterStatuses_IntValue(t *testing.T) {
	// Status compared with int value — should be ignored (only string values checked)
	expr := &CompareExpr{Field: "status", Op: "=", Value: 42}
	if err := ValidateFilterStatuses(expr, validStatus); err != nil {
		t.Errorf("expected no error for non-string value, got: %v", err)
	}
}

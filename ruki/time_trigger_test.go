package ruki

import "testing"

func TestParseTimeTrigger_HappyPath(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name       string
		input      string
		wantValue  int
		wantUnit   string
		wantCreate bool
		wantUpdate bool
		wantDelete bool
	}{
		{
			"update stale tasks",
			`every 1hour update where status = "in_progress" and updatedAt < now() - 7day set status="backlog"`,
			1, "hour",
			false, true, false,
		},
		{
			"delete expired",
			`every 1day delete where status = "done" and updatedAt < now() - 30day`,
			1, "day",
			false, false, true,
		},
		{
			"create weekly review",
			`every 2week create title="weekly review" status="ready" priority=3`,
			2, "week",
			true, false, false,
		},
		{
			"plural duration",
			`every 3days delete where status = "cancelled"`,
			3, "day",
			false, false, true,
		},
		{
			"month interval",
			`every 1month delete where status = "done"`,
			1, "month",
			false, false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.ParseTimeTrigger(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if result.Interval.Value != tt.wantValue {
				t.Fatalf("expected interval value %d, got %d", tt.wantValue, result.Interval.Value)
			}
			if result.Interval.Unit != tt.wantUnit {
				t.Fatalf("expected interval unit %q, got %q", tt.wantUnit, result.Interval.Unit)
			}
			if tt.wantCreate && result.Action.Create == nil {
				t.Fatal("expected Create action")
			}
			if tt.wantUpdate && result.Action.Update == nil {
				t.Fatal("expected Update action")
			}
			if tt.wantDelete && result.Action.Delete == nil {
				t.Fatal("expected Delete action")
			}
		})
	}
}

func TestParseTimeTrigger_ASTVerification(t *testing.T) {
	p := newTestParser()

	input := `every 1hour update where status = "in_progress" set status="backlog"`
	tt, err := p.ParseTimeTrigger(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// verify interval
	if tt.Interval.Value != 1 || tt.Interval.Unit != "hour" {
		t.Fatalf("expected 1hour, got %d%s", tt.Interval.Value, tt.Interval.Unit)
	}

	// verify action is update with where and set
	if tt.Action.Update == nil {
		t.Fatal("expected Update action")
	}
	if tt.Action.Update.Where == nil {
		t.Fatal("expected Where condition")
	}
	if len(tt.Action.Update.Set) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(tt.Action.Update.Set))
	}
	if tt.Action.Update.Set[0].Field != "status" {
		t.Fatalf("expected assignment to status, got %s", tt.Action.Update.Set[0].Field)
	}
}

func TestParseTimeTrigger_Errors(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{
			"select not allowed",
			`every 1day select where status = "done"`,
		},
		{
			"run not allowed",
			`every 1hour run("echo hi")`,
		},
		{
			"qualifier old rejected",
			`every 1day update where old.status = "done" set status="backlog"`,
		},
		{
			"qualifier new rejected",
			`every 1day update where new.status = "done" set status="backlog"`,
		},
		{
			"unknown field",
			`every 1day update where foo = "bar" set status="done"`,
		},
		{
			"type mismatch",
			`every 1day create title="x" priority="high"`,
		},
		{
			"zero interval",
			`every 0day delete where status = "done"`,
		},
		{
			"missing statement",
			`every 1day`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseTimeTrigger(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

// --- ParseRule tests ---

func TestParseRule_EventTrigger(t *testing.T) {
	p := newTestParser()

	rule, err := p.ParseRule(`before update where new.status = "done" deny "blocked"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if rule.Trigger == nil {
		t.Fatal("expected event Trigger, got nil")
	}
	if rule.TimeTrigger != nil {
		t.Fatal("expected TimeTrigger to be nil")
	}
	if rule.Trigger.Timing != "before" || rule.Trigger.Event != "update" {
		t.Fatalf("expected before update, got %s %s", rule.Trigger.Timing, rule.Trigger.Event)
	}
}

func TestParseRule_TimeTrigger(t *testing.T) {
	p := newTestParser()

	rule, err := p.ParseRule(`every 1day delete where status = "done"`)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if rule.TimeTrigger == nil {
		t.Fatal("expected TimeTrigger, got nil")
	}
	if rule.Trigger != nil {
		t.Fatal("expected event Trigger to be nil")
	}
	if rule.TimeTrigger.Interval.Value != 1 || rule.TimeTrigger.Interval.Unit != "day" {
		t.Fatalf("expected 1day, got %d%s", rule.TimeTrigger.Interval.Value, rule.TimeTrigger.Interval.Unit)
	}
}

func TestParseRule_ParseError(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseRule(`not a valid rule at all`)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestParseRule_ValidationError(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{
			"event trigger validation: unknown field",
			`before update where new.foo = "bar" deny "no"`,
		},
		{
			"time trigger validation: zero interval",
			`every 0day delete where status = "done"`,
		},
		{
			"time trigger validation: qualifier rejected",
			`every 1day update where old.status = "done" set status="backlog"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseRule(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestValidateTimeTrigger_RejectsSelect(t *testing.T) {
	p := newTestParser()
	// construct a TimeTrigger with a Select action directly — the grammar prevents this,
	// but the validator should catch it as defense-in-depth
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action:   &Statement{Select: &SelectStmt{}},
	}
	err := p.validateTimeTrigger(tt)
	if err == nil {
		t.Fatal("expected error for select in time trigger")
	}
	if err.Error() != "time trigger action must not be select" {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- defense-in-depth: test internal lowering/validation with hand-crafted structs ---

func TestLowerRule_EmptyRule(t *testing.T) {
	_, err := lowerRule(&ruleGrammar{})
	if err == nil {
		t.Fatal("expected error for empty rule")
	}
}

func TestLowerRule_TimeTriggerLoweringError(t *testing.T) {
	// invalid duration triggers a lowering error in lowerTimeTrigger
	_, err := lowerRule(&ruleGrammar{
		TimeTrigger: &timeTriggerGrammar{Interval: "bad"},
	})
	if err == nil {
		t.Fatal("expected error for bad time trigger interval")
	}
}

func TestLowerRule_EventTriggerLoweringError(t *testing.T) {
	badDate := "bad-date"
	_, err := lowerRule(&ruleGrammar{
		Trigger: &triggerGrammar{
			Timing: "before",
			Event:  "update",
			Where: &orCond{Left: andCond{Left: notCond{
				Primary: &primaryCond{Expr: &exprCond{
					Left: exprGrammar{Left: unaryExpr{DateLit: &badDate}},
				}},
			}}},
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid event trigger")
	}
}

func TestLowerTimeTrigger_InvalidDuration(t *testing.T) {
	_, err := lowerTimeTrigger(&timeTriggerGrammar{Interval: "xyz"})
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestLowerTimeTrigger_EmptyAction(t *testing.T) {
	_, err := lowerTimeTrigger(&timeTriggerGrammar{Interval: "1day"})
	if err == nil {
		t.Fatal("expected error for empty action")
	}
}

func TestLowerTimeTrigger_CreateLoweringError(t *testing.T) {
	badDate := "bad-date"
	_, err := lowerTimeTrigger(&timeTriggerGrammar{
		Interval: "1day",
		Create: &createGrammar{Assignments: []assignmentGrammar{
			{Field: "title", Value: exprGrammar{Left: unaryExpr{DateLit: &badDate}}},
		}},
	})
	if err == nil {
		t.Fatal("expected error for create lowering failure")
	}
}

func TestLowerTimeTrigger_UpdateLoweringError(t *testing.T) {
	badDate := "bad-date"
	okStr := `"ok"`
	_, err := lowerTimeTrigger(&timeTriggerGrammar{
		Interval: "1day",
		Update: &updateGrammar{
			Where: orCond{Left: andCond{Left: notCond{
				Primary: &primaryCond{Expr: &exprCond{
					Left: exprGrammar{Left: unaryExpr{DateLit: &badDate}},
				}},
			}}},
			Set: []assignmentGrammar{
				{Field: "title", Value: exprGrammar{Left: unaryExpr{StrLit: &okStr}}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for update lowering failure")
	}
}

func TestLowerTimeTrigger_DeleteLoweringError(t *testing.T) {
	badDate := "bad-date"
	_, err := lowerTimeTrigger(&timeTriggerGrammar{
		Interval: "1day",
		Delete: &deleteGrammar{Where: orCond{Left: andCond{Left: notCond{
			Primary: &primaryCond{Expr: &exprCond{
				Left: exprGrammar{Left: unaryExpr{DateLit: &badDate}},
			}},
		}}}},
	})
	if err == nil {
		t.Fatal("expected error for delete lowering failure")
	}
}

func TestValidateRule_EmptyRule(t *testing.T) {
	p := newTestParser()
	err := p.validateRule(&Rule{})
	if err == nil {
		t.Fatal("expected error for empty rule")
	}
}

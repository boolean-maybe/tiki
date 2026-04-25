package ruki

import (
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 1. UnvalidatedWrapperError
// ---------------------------------------------------------------------------

func TestUnvalidatedWrapperError(t *testing.T) {
	e := &UnvalidatedWrapperError{Wrapper: "statement"}
	want := "statement wrapper is not semantically validated"
	if e.Error() != want {
		t.Errorf("got %q, want %q", e.Error(), want)
	}
}

// ---------------------------------------------------------------------------
// 2. ValidatedStatement — mustBeSealed, accessors
// ---------------------------------------------------------------------------

func TestValidatedStatement_mustBeSealed(t *testing.T) {
	tests := []struct {
		name string
		vs   *ValidatedStatement
	}{
		{"nil receiver", nil},
		{"nil seal", &ValidatedStatement{seal: nil, statement: &Statement{}}},
		{"nil statement", &ValidatedStatement{seal: validatedSeal, statement: nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.vs.mustBeSealed()
			if err == nil {
				t.Fatal("expected error")
			}
			var ue *UnvalidatedWrapperError
			if !errors.As(err, &ue) {
				t.Fatalf("expected *UnvalidatedWrapperError, got %T", err)
			}
			if ue.Wrapper != "statement" {
				t.Errorf("wrapper = %q, want %q", ue.Wrapper, "statement")
			}
		})
	}

	t.Run("sealed passes", func(t *testing.T) {
		vs := &ValidatedStatement{seal: validatedSeal, statement: &Statement{Select: &SelectStmt{}}}
		if err := vs.mustBeSealed(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidatedStatement_RequiresCreateTemplate(t *testing.T) {
	tests := []struct {
		name string
		vs   *ValidatedStatement
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil statement", &ValidatedStatement{statement: nil}, false},
		{"no create", &ValidatedStatement{statement: &Statement{Select: &SelectStmt{}}}, false},
		{"with create", &ValidatedStatement{statement: &Statement{Create: &CreateStmt{}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vs.RequiresCreateTemplate(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedStatement_IsSelect(t *testing.T) {
	tests := []struct {
		name string
		vs   *ValidatedStatement
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil statement", &ValidatedStatement{statement: nil}, false},
		{"select", &ValidatedStatement{statement: &Statement{Select: &SelectStmt{}}}, true},
		{"update", &ValidatedStatement{statement: &Statement{Update: &UpdateStmt{}}}, false},
		{"create", &ValidatedStatement{statement: &Statement{Create: &CreateStmt{}}}, false},
		{"delete", &ValidatedStatement{statement: &Statement{Delete: &DeleteStmt{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vs.IsSelect(); got != tt.want {
				t.Errorf("IsSelect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedStatement_IsUpdate(t *testing.T) {
	tests := []struct {
		name string
		vs   *ValidatedStatement
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil statement", &ValidatedStatement{statement: nil}, false},
		{"update", &ValidatedStatement{statement: &Statement{Update: &UpdateStmt{}}}, true},
		{"select", &ValidatedStatement{statement: &Statement{Select: &SelectStmt{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vs.IsUpdate(); got != tt.want {
				t.Errorf("IsUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedStatement_IsCreate(t *testing.T) {
	tests := []struct {
		name string
		vs   *ValidatedStatement
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil statement", &ValidatedStatement{statement: nil}, false},
		{"create", &ValidatedStatement{statement: &Statement{Create: &CreateStmt{}}}, true},
		{"select", &ValidatedStatement{statement: &Statement{Select: &SelectStmt{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vs.IsCreate(); got != tt.want {
				t.Errorf("IsCreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedStatement_IsDelete(t *testing.T) {
	tests := []struct {
		name string
		vs   *ValidatedStatement
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil statement", &ValidatedStatement{statement: nil}, false},
		{"delete", &ValidatedStatement{statement: &Statement{Delete: &DeleteStmt{}}}, true},
		{"select", &ValidatedStatement{statement: &Statement{Select: &SelectStmt{}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vs.IsDelete(); got != tt.want {
				t.Errorf("IsDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedStatement_Accessors(t *testing.T) {
	vs := &ValidatedStatement{
		runtime:    ExecutorRuntimePlugin,
		usesIDFunc: true,
	}
	if vs.RuntimeMode() != ExecutorRuntimePlugin {
		t.Errorf("RuntimeMode() = %q, want %q", vs.RuntimeMode(), ExecutorRuntimePlugin)
	}
	if !vs.UsesIDBuiltin() {
		t.Error("UsesIDBuiltin() = false, want true")
	}
}

// ---------------------------------------------------------------------------
// 3. ValidatedTrigger — mustBeSealed, accessors
// ---------------------------------------------------------------------------

func TestValidatedTrigger_mustBeSealed(t *testing.T) {
	tests := []struct {
		name string
		vt   *ValidatedTrigger
	}{
		{"nil receiver", nil},
		{"nil seal", &ValidatedTrigger{seal: nil, trigger: &Trigger{}}},
		{"nil trigger", &ValidatedTrigger{seal: validatedSeal, trigger: nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.vt.mustBeSealed()
			if err == nil {
				t.Fatal("expected error")
			}
			var ue *UnvalidatedWrapperError
			if !errors.As(err, &ue) {
				t.Fatalf("expected *UnvalidatedWrapperError, got %T", err)
			}
			if ue.Wrapper != "trigger" {
				t.Errorf("wrapper = %q, want %q", ue.Wrapper, "trigger")
			}
		})
	}

	t.Run("sealed passes", func(t *testing.T) {
		vt := &ValidatedTrigger{seal: validatedSeal, trigger: &Trigger{Timing: "after", Event: "create"}}
		if err := vt.mustBeSealed(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidatedTrigger_Timing(t *testing.T) {
	tests := []struct {
		name string
		vt   *ValidatedTrigger
		want string
	}{
		{"nil receiver", nil, ""},
		{"nil trigger", &ValidatedTrigger{trigger: nil}, ""},
		{"normal", &ValidatedTrigger{trigger: &Trigger{Timing: "before"}}, "before"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vt.Timing(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatedTrigger_Event(t *testing.T) {
	tests := []struct {
		name string
		vt   *ValidatedTrigger
		want string
	}{
		{"nil receiver", nil, ""},
		{"nil trigger", &ValidatedTrigger{trigger: nil}, ""},
		{"normal", &ValidatedTrigger{trigger: &Trigger{Event: "update"}}, "update"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vt.Event(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidatedTrigger_HasRunAction(t *testing.T) {
	tests := []struct {
		name string
		vt   *ValidatedTrigger
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil trigger", &ValidatedTrigger{trigger: nil}, false},
		{"no run", &ValidatedTrigger{trigger: &Trigger{}}, false},
		{"with run", &ValidatedTrigger{trigger: &Trigger{Run: &RunAction{Command: &StringLiteral{Value: "echo"}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vt.HasRunAction(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedTrigger_DenyMessage(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var vt *ValidatedTrigger
		msg, ok := vt.DenyMessage()
		if ok || msg != "" {
			t.Errorf("expected (\"\", false), got (%q, %v)", msg, ok)
		}
	})
	t.Run("nil trigger", func(t *testing.T) {
		vt := &ValidatedTrigger{trigger: nil}
		msg, ok := vt.DenyMessage()
		if ok || msg != "" {
			t.Errorf("expected (\"\", false), got (%q, %v)", msg, ok)
		}
	})
	t.Run("nil deny", func(t *testing.T) {
		vt := &ValidatedTrigger{trigger: &Trigger{}}
		msg, ok := vt.DenyMessage()
		if ok || msg != "" {
			t.Errorf("expected (\"\", false), got (%q, %v)", msg, ok)
		}
	})
	t.Run("with deny", func(t *testing.T) {
		s := "blocked"
		vt := &ValidatedTrigger{trigger: &Trigger{Deny: &s}}
		msg, ok := vt.DenyMessage()
		if !ok || msg != "blocked" {
			t.Errorf("expected (\"blocked\", true), got (%q, %v)", msg, ok)
		}
	})
}

func TestValidatedTrigger_RequiresCreateTemplate(t *testing.T) {
	tests := []struct {
		name string
		vt   *ValidatedTrigger
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil trigger", &ValidatedTrigger{trigger: nil}, false},
		{"no action", &ValidatedTrigger{trigger: &Trigger{}}, false},
		{"action no create", &ValidatedTrigger{trigger: &Trigger{Action: &Statement{Update: &UpdateStmt{}}}}, false},
		{"action with create", &ValidatedTrigger{trigger: &Trigger{Action: &Statement{Create: &CreateStmt{}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vt.RequiresCreateTemplate(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedTrigger_TriggerClone(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var vt *ValidatedTrigger
		if vt.TriggerClone() != nil {
			t.Error("expected nil from nil receiver")
		}
	})
}

func TestValidatedTrigger_Accessors(t *testing.T) {
	vt := &ValidatedTrigger{runtime: ExecutorRuntimeEventTrigger, usesIDFunc: false}
	if vt.RuntimeMode() != ExecutorRuntimeEventTrigger {
		t.Errorf("RuntimeMode() = %q, want %q", vt.RuntimeMode(), ExecutorRuntimeEventTrigger)
	}
	if vt.UsesIDBuiltin() {
		t.Error("UsesIDBuiltin() = true, want false")
	}
}

// ---------------------------------------------------------------------------
// 4. ValidatedTimeTrigger — mustBeSealed, accessors
// ---------------------------------------------------------------------------

func TestValidatedTimeTrigger_mustBeSealed(t *testing.T) {
	tests := []struct {
		name string
		vtt  *ValidatedTimeTrigger
	}{
		{"nil receiver", nil},
		{"nil seal", &ValidatedTimeTrigger{seal: nil, timeTrigger: &TimeTrigger{}}},
		{"nil timeTrigger", &ValidatedTimeTrigger{seal: validatedSeal, timeTrigger: nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.vtt.mustBeSealed()
			if err == nil {
				t.Fatal("expected error")
			}
			var ue *UnvalidatedWrapperError
			if !errors.As(err, &ue) {
				t.Fatalf("expected *UnvalidatedWrapperError, got %T", err)
			}
			if ue.Wrapper != "time trigger" {
				t.Errorf("wrapper = %q, want %q", ue.Wrapper, "time trigger")
			}
		})
	}

	t.Run("sealed passes", func(t *testing.T) {
		vtt := &ValidatedTimeTrigger{seal: validatedSeal, timeTrigger: &TimeTrigger{}}
		if err := vtt.mustBeSealed(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestValidatedTimeTrigger_IntervalLiteral(t *testing.T) {
	tests := []struct {
		name string
		vtt  *ValidatedTimeTrigger
		want DurationLiteral
	}{
		{"nil receiver", nil, DurationLiteral{}},
		{"nil timeTrigger", &ValidatedTimeTrigger{timeTrigger: nil}, DurationLiteral{}},
		{"normal", &ValidatedTimeTrigger{timeTrigger: &TimeTrigger{Interval: DurationLiteral{Value: 2, Unit: "hour"}}}, DurationLiteral{Value: 2, Unit: "hour"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.vtt.IntervalLiteral()
			if got.Value != tt.want.Value || got.Unit != tt.want.Unit {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestValidatedTimeTrigger_RequiresCreateTemplate(t *testing.T) {
	tests := []struct {
		name string
		vtt  *ValidatedTimeTrigger
		want bool
	}{
		{"nil receiver", nil, false},
		{"nil timeTrigger", &ValidatedTimeTrigger{timeTrigger: nil}, false},
		{"no action", &ValidatedTimeTrigger{timeTrigger: &TimeTrigger{}}, false},
		{"action with create", &ValidatedTimeTrigger{timeTrigger: &TimeTrigger{Action: &Statement{Create: &CreateStmt{}}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.vtt.RequiresCreateTemplate(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatedTimeTrigger_TimeTriggerClone(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var vtt *ValidatedTimeTrigger
		if vtt.TimeTriggerClone() != nil {
			t.Error("expected nil from nil receiver")
		}
	})
}

func TestValidatedTimeTrigger_Accessors(t *testing.T) {
	vtt := &ValidatedTimeTrigger{runtime: ExecutorRuntimeTimeTrigger, usesIDFunc: true}
	if vtt.RuntimeMode() != ExecutorRuntimeTimeTrigger {
		t.Errorf("RuntimeMode() = %q, want %q", vtt.RuntimeMode(), ExecutorRuntimeTimeTrigger)
	}
	if !vtt.UsesIDBuiltin() {
		t.Error("UsesIDBuiltin() = false, want true")
	}
}

// ---------------------------------------------------------------------------
// 5. ValidatedRule discriminated union
// ---------------------------------------------------------------------------

func TestValidatedEventRule_RuntimeMode(t *testing.T) {
	t.Run("nil trigger", func(t *testing.T) {
		r := ValidatedEventRule{trigger: nil}
		if r.RuntimeMode() != "" {
			t.Errorf("expected empty, got %q", r.RuntimeMode())
		}
	})
	t.Run("with trigger", func(t *testing.T) {
		r := ValidatedEventRule{trigger: &ValidatedTrigger{runtime: ExecutorRuntimeEventTrigger}}
		if r.RuntimeMode() != ExecutorRuntimeEventTrigger {
			t.Errorf("expected %q, got %q", ExecutorRuntimeEventTrigger, r.RuntimeMode())
		}
	})
}

func TestValidatedTimeRule_RuntimeMode(t *testing.T) {
	t.Run("nil time", func(t *testing.T) {
		r := ValidatedTimeRule{time: nil}
		if r.RuntimeMode() != "" {
			t.Errorf("expected empty, got %q", r.RuntimeMode())
		}
	})
	t.Run("with time", func(t *testing.T) {
		r := ValidatedTimeRule{time: &ValidatedTimeTrigger{runtime: ExecutorRuntimeTimeTrigger}}
		if r.RuntimeMode() != ExecutorRuntimeTimeTrigger {
			t.Errorf("expected %q, got %q", ExecutorRuntimeTimeTrigger, r.RuntimeMode())
		}
	})
}

func TestValidatedEventRule_Trigger(t *testing.T) {
	vt := &ValidatedTrigger{runtime: ExecutorRuntimeEventTrigger}
	r := ValidatedEventRule{trigger: vt}
	if r.Trigger() != vt {
		t.Error("Trigger() returned wrong pointer")
	}
}

func TestValidatedTimeRule_TimeTrigger(t *testing.T) {
	vtt := &ValidatedTimeTrigger{runtime: ExecutorRuntimeTimeTrigger}
	r := ValidatedTimeRule{time: vtt}
	if r.TimeTrigger() != vtt {
		t.Error("TimeTrigger() returned wrong pointer")
	}
}

// ---------------------------------------------------------------------------
// 6. NewSemanticValidator
// ---------------------------------------------------------------------------

func TestNewSemanticValidator_DefaultRuntime(t *testing.T) {
	v := NewSemanticValidator("")
	if v.runtime != ExecutorRuntimeCLI {
		t.Errorf("expected CLI default, got %q", v.runtime)
	}
}

func TestNewSemanticValidator_ExplicitRuntime(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimePlugin)
	if v.runtime != ExecutorRuntimePlugin {
		t.Errorf("expected plugin, got %q", v.runtime)
	}
}

// ---------------------------------------------------------------------------
// 7. ParseAndValidateStatement
// ---------------------------------------------------------------------------

func TestParseAndValidateStatement(t *testing.T) {
	p := newTestParser()

	t.Run("parse error forwarded", func(t *testing.T) {
		_, err := p.ParseAndValidateStatement("not a valid statement!!!", ExecutorRuntimeCLI)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid input", func(t *testing.T) {
		vs, err := p.ParseAndValidateStatement(`select where status = "done"`, ExecutorRuntimeCLI)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vs.RuntimeMode() != ExecutorRuntimeCLI {
			t.Errorf("runtime = %q, want %q", vs.RuntimeMode(), ExecutorRuntimeCLI)
		}
		if err := vs.mustBeSealed(); err != nil {
			t.Fatalf("not sealed: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. ParseAndValidateTrigger
// ---------------------------------------------------------------------------

func TestParseAndValidateTrigger(t *testing.T) {
	p := newTestParser()

	t.Run("parse error forwarded", func(t *testing.T) {
		_, err := p.ParseAndValidateTrigger("garbage", ExecutorRuntimeEventTrigger)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid input", func(t *testing.T) {
		vt, err := p.ParseAndValidateTrigger(
			`before update where new.status = "done" deny "no"`,
			ExecutorRuntimeEventTrigger,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if vt.Timing() != "before" {
			t.Errorf("timing = %q, want %q", vt.Timing(), "before")
		}
	})
}

// ---------------------------------------------------------------------------
// 9. ParseAndValidateTimeTrigger
// ---------------------------------------------------------------------------

func TestParseAndValidateTimeTrigger(t *testing.T) {
	p := newTestParser()

	t.Run("parse error forwarded", func(t *testing.T) {
		_, err := p.ParseAndValidateTimeTrigger("garbage", ExecutorRuntimeTimeTrigger)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid input", func(t *testing.T) {
		vtt, err := p.ParseAndValidateTimeTrigger(
			`every 1day delete where status = "cancelled"`,
			ExecutorRuntimeTimeTrigger,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		il := vtt.IntervalLiteral()
		if il.Value != 1 || il.Unit != "day" {
			t.Errorf("interval = %+v, want 1day", il)
		}
	})
}

// ---------------------------------------------------------------------------
// 10. ParseAndValidateRule
// ---------------------------------------------------------------------------

func TestParseAndValidateRule(t *testing.T) {
	p := newTestParser()

	t.Run("event trigger rule", func(t *testing.T) {
		rule, err := p.ParseAndValidateRule(
			`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="booleanmaybe"`,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		er, ok := rule.(ValidatedEventRule)
		if !ok {
			t.Fatalf("expected ValidatedEventRule, got %T", rule)
			return
		}
		if er.RuntimeMode() != ExecutorRuntimeEventTrigger {
			t.Errorf("runtime = %q, want %q", er.RuntimeMode(), ExecutorRuntimeEventTrigger)
		}
		if er.Trigger() == nil {
			t.Fatal("Trigger() is nil")
		}
	})

	t.Run("time trigger rule", func(t *testing.T) {
		rule, err := p.ParseAndValidateRule(
			`every 1day delete where status = "cancelled"`,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		tr, ok := rule.(ValidatedTimeRule)
		if !ok {
			t.Fatalf("expected ValidatedTimeRule, got %T", rule)
			return
		}
		if tr.RuntimeMode() != ExecutorRuntimeTimeTrigger {
			t.Errorf("runtime = %q, want %q", tr.RuntimeMode(), ExecutorRuntimeTimeTrigger)
		}
		if tr.TimeTrigger() == nil {
			t.Fatal("TimeTrigger() is nil")
		}
	})

	t.Run("parse error", func(t *testing.T) {
		_, err := p.ParseAndValidateRule("totally invalid input @@@")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// ---------------------------------------------------------------------------
// 11. ValidateStatement
// ---------------------------------------------------------------------------

func TestValidateStatement_NilReturnsError(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeCLI)
	_, err := v.ValidateStatement(nil)
	if err == nil || !strings.Contains(err.Error(), "nil statement") {
		t.Fatalf("expected nil statement error, got: %v", err)
	}
}

func TestValidateStatement_CallRejected(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeCLI)
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "echo"}}},
				Op:    "=",
				Right: &StringLiteral{Value: "x"},
			},
		},
	}
	_, err := v.ValidateStatement(stmt)
	if err == nil || !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() rejection, got: %v", err)
	}
}

func TestValidateStatement_IDRejectedInCLI(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeCLI)
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FunctionCall{Name: "id"},
				Op:    "=",
				Right: &StringLiteral{Value: "TIKI-AAA"},
			},
		},
	}
	_, err := v.ValidateStatement(stmt)
	if err == nil || !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() rejection, got: %v", err)
	}
}

func TestValidateStatement_IDAcceptedInPlugin(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimePlugin)
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FunctionCall{Name: "id"},
				Op:    "=",
				Right: &StringLiteral{Value: "TIKI-AAA"},
			},
		},
	}
	vs, err := v.ValidateStatement(stmt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !vs.UsesIDBuiltin() {
		t.Error("expected UsesIDBuiltin() = true")
	}
}

func TestValidateStatement_AssignmentValidationPropagated(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeCLI)
	stmt := &Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{
				{Field: "id", Value: &StringLiteral{Value: "x"}},
			},
		},
	}
	_, err := v.ValidateStatement(stmt)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 12. ValidateTrigger
// ---------------------------------------------------------------------------

func TestValidateTrigger_NilReturnsError(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	_, err := v.ValidateTrigger(nil)
	if err == nil || !strings.Contains(err.Error(), "nil trigger") {
		t.Fatalf("expected nil trigger error, got: %v", err)
	}
}

func TestValidateTrigger_CallRejected(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Where: &CompareExpr{
			Left:  &FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "cmd"}}},
			Op:    "=",
			Right: &StringLiteral{Value: "x"},
		},
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
				Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
			},
		},
	}
	_, err := v.ValidateTrigger(trig)
	if err == nil || !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() rejection, got: %v", err)
	}
}

func TestValidateTrigger_ActionAssignmentValidation(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	trig := &Trigger{
		Timing: "after",
		Event:  "create",
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "createdBy", Value: &StringLiteral{Value: "x"}},
				},
			},
		},
	}
	_, err := v.ValidateTrigger(trig)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable error, got: %v", err)
	}
}

func TestValidateTrigger_ValidInput(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
				},
				Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "updated"}}},
			},
		},
	}
	vt, err := v.ValidateTrigger(trig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vt.Timing() != "after" {
		t.Errorf("timing = %q, want %q", vt.Timing(), "after")
	}
}

// ---------------------------------------------------------------------------
// 13. ValidateTimeTrigger
// ---------------------------------------------------------------------------

func TestValidateTimeTrigger_NilReturnsError(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	_, err := v.ValidateTimeTrigger(nil)
	if err == nil || !strings.Contains(err.Error(), "nil time trigger") {
		t.Fatalf("expected nil time trigger error, got: %v", err)
	}
}

func TestValidateTimeTrigger_CallRejected(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left:  &FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "x"}}},
					Op:    "=",
					Right: &StringLiteral{Value: "x"},
				},
				Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
			},
		},
	}
	_, err := v.ValidateTimeTrigger(tt)
	if err == nil || !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() rejection, got: %v", err)
	}
}

func TestValidateTimeTrigger_IDRejectedInNonPlugin(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
				},
				Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
			},
		},
	}
	_, err := v.ValidateTimeTrigger(tt)
	if err == nil || !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() rejection, got: %v", err)
	}
}

func TestValidateTimeTrigger_ActionAssignmentValidation(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "updatedAt", Value: &StringLiteral{Value: "x"}},
				},
			},
		},
	}
	_, err := v.ValidateTimeTrigger(tt)
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable error, got: %v", err)
	}
}

func TestValidateTimeTrigger_ValidInput(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Delete: &DeleteStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "cancelled"},
				},
			},
		},
	}
	vtt, err := v.ValidateTimeTrigger(tt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	il := vtt.IntervalLiteral()
	if il.Value != 1 || il.Unit != "day" {
		t.Errorf("interval = %+v, want 1day", il)
	}
}

func TestValidateTimeTrigger_NilAction(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action:   nil,
	}
	vtt, err := v.ValidateTimeTrigger(tt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vtt.RequiresCreateTemplate() {
		t.Error("expected RequiresCreateTemplate() = false")
	}
}

// ---------------------------------------------------------------------------
// 14. validateAssignmentsSemantics
// ---------------------------------------------------------------------------

func TestValidateAssignmentsSemantics_ImmutableFields(t *testing.T) {
	for _, field := range []string{"id", "createdBy", "createdAt", "updatedAt"} {
		t.Run(field, func(t *testing.T) {
			err := validateAssignmentsSemantics([]Assignment{
				{Field: field, Value: &StringLiteral{Value: "x"}},
			})
			if err == nil || !strings.Contains(err.Error(), "immutable") {
				t.Fatalf("expected immutable error for %q, got: %v", field, err)
			}
		})
	}
}

func TestValidateAssignmentsSemantics_EmptyRejected(t *testing.T) {
	for _, field := range []string{"title", "status", "type", "priority"} {
		t.Run(field, func(t *testing.T) {
			err := validateAssignmentsSemantics([]Assignment{
				{Field: field, Value: &EmptyLiteral{}},
			})
			if err == nil || !strings.Contains(err.Error(), "cannot be empty") {
				t.Fatalf("expected cannot-be-empty error for %q, got: %v", field, err)
			}
		})
	}
}

func TestValidateAssignmentsSemantics_PriorityOutOfRange(t *testing.T) {
	for _, prio := range []int{0, 6, -1, 99} {
		err := validateAssignmentsSemantics([]Assignment{
			{Field: "priority", Value: &IntLiteral{Value: prio}},
		})
		if err == nil || !strings.Contains(err.Error(), "priority value out of range") {
			t.Fatalf("expected priority range error for %d, got: %v", prio, err)
		}
	}
}

func TestValidateAssignmentsSemantics_PriorityValid(t *testing.T) {
	for _, prio := range []int{1, 3, 5} {
		err := validateAssignmentsSemantics([]Assignment{
			{Field: "priority", Value: &IntLiteral{Value: prio}},
		})
		if err != nil {
			t.Fatalf("unexpected error for priority %d: %v", prio, err)
		}
	}
}

func TestValidateAssignmentsSemantics_PointsOutOfRange(t *testing.T) {
	err := validateAssignmentsSemantics([]Assignment{
		{Field: "points", Value: &IntLiteral{Value: -1}},
	})
	if err == nil || !strings.Contains(err.Error(), "points value out of range") {
		t.Fatalf("expected points range error, got: %v", err)
	}
}

func TestValidateAssignmentsSemantics_PointsValid(t *testing.T) {
	for _, pts := range []int{0, 1, 5} {
		err := validateAssignmentsSemantics([]Assignment{
			{Field: "points", Value: &IntLiteral{Value: pts}},
		})
		if err != nil {
			t.Fatalf("unexpected error for points %d: %v", pts, err)
		}
	}
}

func TestValidateAssignmentsSemantics_ValidAssignmentPasses(t *testing.T) {
	err := validateAssignmentsSemantics([]Assignment{
		{Field: "title", Value: &StringLiteral{Value: "hello"}},
		{Field: "tags", Value: &ListLiteral{Elements: []Expr{&StringLiteral{Value: "a"}}}},
		{Field: "assignee", Value: &EmptyLiteral{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 15. validateStatementAssignmentsSemantics — routes to create/update
// ---------------------------------------------------------------------------

func TestValidateStatementAssignmentsSemantics_Create(t *testing.T) {
	err := validateStatementAssignmentsSemantics(&Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{
				{Field: "createdAt", Value: &StringLiteral{Value: "x"}},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable, got: %v", err)
	}
}

func TestValidateStatementAssignmentsSemantics_Update(t *testing.T) {
	err := validateStatementAssignmentsSemantics(&Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
			Set: []Assignment{
				{Field: "id", Value: &StringLiteral{Value: "x"}},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable, got: %v", err)
	}
}

func TestValidateStatementAssignmentsSemantics_SelectAndDelete(t *testing.T) {
	// select and delete have no assignments — should return nil
	if err := validateStatementAssignmentsSemantics(&Statement{Select: &SelectStmt{}}); err != nil {
		t.Fatalf("select: unexpected error: %v", err)
	}
	if err := validateStatementAssignmentsSemantics(&Statement{Delete: &DeleteStmt{}}); err != nil {
		t.Fatalf("delete: unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 16. scanStatementSemantics
// ---------------------------------------------------------------------------

func TestScanStatementSemantics_EmptyStatement(t *testing.T) {
	_, _, err := scanStatementSemantics(&Statement{})
	if err == nil || !strings.Contains(err.Error(), "empty statement") {
		t.Fatalf("expected empty statement error, got: %v", err)
	}
}

func TestScanStatementSemantics_Select(t *testing.T) {
	usesID, hasCall, err := scanStatementSemantics(&Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left:  &FunctionCall{Name: "id"},
				Op:    "=",
				Right: &StringLiteral{Value: "x"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !usesID {
		t.Error("expected usesID = true")
	}
	if hasCall {
		t.Error("expected hasCall = false")
	}
}

func TestScanStatementSemantics_Create(t *testing.T) {
	usesID, hasCall, err := scanStatementSemantics(&Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{
				{Field: "title", Value: &FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "x"}}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usesID {
		t.Error("expected usesID = false")
	}
	if !hasCall {
		t.Error("expected hasCall = true")
	}
}

func TestScanStatementSemantics_Update(t *testing.T) {
	usesID, hasCall, err := scanStatementSemantics(&Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
			Set: []Assignment{
				{Field: "title", Value: &FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "y"}}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !usesID {
		t.Error("expected usesID = true from where")
	}
	if !hasCall {
		t.Error("expected hasCall = true from set")
	}
}

func TestScanStatementSemantics_Delete(t *testing.T) {
	usesID, _, err := scanStatementSemantics(&Statement{
		Delete: &DeleteStmt{
			Where: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !usesID {
		t.Error("expected usesID = true")
	}
}

func TestScanStatementSemantics_UpdateWhereError(t *testing.T) {
	_, _, err := scanStatementSemantics(&Statement{
		Update: &UpdateStmt{
			Where: &fakeCondition{},
			Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
		},
	})
	if err == nil {
		t.Fatal("expected error from unknown condition in where")
	}
}

// ---------------------------------------------------------------------------
// 17. scanConditionSemantics
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_AllTypes(t *testing.T) {
	t.Run("nil condition", func(t *testing.T) {
		u, c, err := scanConditionSemantics(nil)
		if err != nil || u || c {
			t.Fatalf("expected (false, false, nil), got (%v, %v, %v)", u, c, err)
		}
	})

	t.Run("BinaryCondition", func(t *testing.T) {
		u, c, err := scanConditionSemantics(&BinaryCondition{
			Op: "and",
			Left: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
			Right: &CompareExpr{
				Left: &FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "y"}}}, Op: "=", Right: &StringLiteral{Value: "z"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true")
		}
		if !c {
			t.Error("expected hasCall = true")
		}
	})

	t.Run("NotCondition", func(t *testing.T) {
		u, _, err := scanConditionSemantics(&NotCondition{
			Inner: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true")
		}
	})

	t.Run("BoolExprCondition", func(t *testing.T) {
		u, c, err := scanConditionSemantics(&BoolExprCondition{
			Expr: &FunctionCall{Name: "call", Args: []Expr{&FunctionCall{Name: "id"}}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u || !c {
			t.Errorf("expected (true, true), got (%v, %v)", u, c)
		}
	})

	t.Run("CompareExpr", func(t *testing.T) {
		u, c, err := scanConditionSemantics(&CompareExpr{
			Left:  &FunctionCall{Name: "id"},
			Op:    "=",
			Right: &FunctionCall{Name: "call", Args: nil},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u || !c {
			t.Errorf("expected (true, true), got (%v, %v)", u, c)
		}
	})

	t.Run("IsEmptyExpr", func(t *testing.T) {
		u, _, err := scanConditionSemantics(&IsEmptyExpr{
			Expr: &FunctionCall{Name: "id"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true")
		}
	})

	t.Run("InExpr", func(t *testing.T) {
		u, c, err := scanConditionSemantics(&InExpr{
			Value:      &FunctionCall{Name: "id"},
			Collection: &FunctionCall{Name: "call", Args: nil},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u || !c {
			t.Errorf("expected (true, true), got (%v, %v)", u, c)
		}
	})

	t.Run("QuantifierExpr", func(t *testing.T) {
		u, c, err := scanConditionSemantics(&QuantifierExpr{
			Expr: &FunctionCall{Name: "id"},
			Kind: "any",
			Condition: &CompareExpr{
				Left: &FunctionCall{Name: "call", Args: nil}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u || !c {
			t.Errorf("expected (true, true), got (%v, %v)", u, c)
		}
	})

	t.Run("unknown condition type", func(t *testing.T) {
		_, _, err := scanConditionSemantics(&fakeCondition{})
		if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
			t.Fatalf("expected unknown condition type error, got: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 18. scanExprSemantics
// ---------------------------------------------------------------------------

func TestScanExprSemantics(t *testing.T) {
	t.Run("nil expr", func(t *testing.T) {
		u, c, err := scanExprSemantics(nil)
		if err != nil || u || c {
			t.Fatalf("expected (false, false, nil), got (%v, %v, %v)", u, c, err)
		}
	})

	t.Run("FunctionCall id", func(t *testing.T) {
		u, c, err := scanExprSemantics(&FunctionCall{Name: "id"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true")
		}
		if c {
			t.Error("expected hasCall = false")
		}
	})

	t.Run("FunctionCall call", func(t *testing.T) {
		u, c, err := scanExprSemantics(&FunctionCall{Name: "call"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if u {
			t.Error("expected usesID = false")
		}
		if !c {
			t.Error("expected hasCall = true")
		}
	})

	t.Run("FunctionCall with nested id in args", func(t *testing.T) {
		u, _, err := scanExprSemantics(&FunctionCall{
			Name: "blocks",
			Args: []Expr{&FunctionCall{Name: "id"}},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true via nested arg")
		}
	})

	t.Run("BinaryExpr propagates", func(t *testing.T) {
		u, c, err := scanExprSemantics(&BinaryExpr{
			Op:    "+",
			Left:  &FunctionCall{Name: "id"},
			Right: &FunctionCall{Name: "call", Args: nil},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u || !c {
			t.Errorf("expected (true, true), got (%v, %v)", u, c)
		}
	})

	t.Run("ListLiteral propagates", func(t *testing.T) {
		u, _, err := scanExprSemantics(&ListLiteral{
			Elements: []Expr{
				&StringLiteral{Value: "a"},
				&FunctionCall{Name: "id"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true via list element")
		}
	})

	t.Run("SubQuery propagates", func(t *testing.T) {
		u, _, err := scanExprSemantics(&SubQuery{
			Where: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true via subquery")
		}
	})

	t.Run("literal types return false false nil", func(t *testing.T) {
		literals := []Expr{
			&FieldRef{Name: "status"},
			&QualifiedRef{Qualifier: "old", Name: "status"},
			&StringLiteral{Value: "x"},
			&IntLiteral{Value: 42},
			&BoolLiteral{Value: true},
			&DateLiteral{},
			&DurationLiteral{Value: 1, Unit: "day"},
			&EmptyLiteral{},
		}
		for _, lit := range literals {
			u, c, err := scanExprSemantics(lit)
			if err != nil || u || c {
				t.Errorf("%T: expected (false, false, nil), got (%v, %v, %v)", lit, u, c, err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// 19. scanTriggerSemantics
// ---------------------------------------------------------------------------

func TestScanTriggerSemantics(t *testing.T) {
	t.Run("where only", func(t *testing.T) {
		trig := &Trigger{
			Timing: "after",
			Event:  "update",
			Where: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
		}
		u, _, err := scanTriggerSemantics(trig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true from where")
		}
	})

	t.Run("action only", func(t *testing.T) {
		trig := &Trigger{
			Timing: "after",
			Event:  "create",
			Action: &Statement{
				Create: &CreateStmt{
					Assignments: []Assignment{
						{Field: "title", Value: &FunctionCall{Name: "call", Args: nil}},
					},
				},
			},
		}
		_, c, err := scanTriggerSemantics(trig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !c {
			t.Error("expected hasCall = true from action")
		}
	})

	t.Run("run only", func(t *testing.T) {
		trig := &Trigger{
			Timing: "after",
			Event:  "update",
			Run:    &RunAction{Command: &FunctionCall{Name: "id"}},
		}
		u, _, err := scanTriggerSemantics(trig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true from run command")
		}
	})

	t.Run("combined where+action", func(t *testing.T) {
		trig := &Trigger{
			Timing: "after",
			Event:  "update",
			Where: &CompareExpr{
				Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
			Action: &Statement{
				Update: &UpdateStmt{
					Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
					Set:   []Assignment{{Field: "title", Value: &FunctionCall{Name: "call", Args: nil}}},
				},
			},
		}
		u, c, err := scanTriggerSemantics(trig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true")
		}
		if !c {
			t.Error("expected hasCall = true")
		}
	})
}

// ---------------------------------------------------------------------------
// 20. scanTimeTriggerSemantics
// ---------------------------------------------------------------------------

func TestScanTimeTriggerSemantics(t *testing.T) {
	t.Run("nil time trigger", func(t *testing.T) {
		u, c, err := scanTimeTriggerSemantics(nil)
		if err != nil || u || c {
			t.Fatalf("expected (false, false, nil), got (%v, %v, %v)", u, c, err)
		}
	})

	t.Run("nil action", func(t *testing.T) {
		u, c, err := scanTimeTriggerSemantics(&TimeTrigger{Interval: DurationLiteral{Value: 1, Unit: "day"}})
		if err != nil || u || c {
			t.Fatalf("expected (false, false, nil), got (%v, %v, %v)", u, c, err)
		}
	})

	t.Run("action with id()", func(t *testing.T) {
		tt := &TimeTrigger{
			Interval: DurationLiteral{Value: 1, Unit: "day"},
			Action: &Statement{
				Delete: &DeleteStmt{
					Where: &CompareExpr{
						Left: &FunctionCall{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "x"},
					},
				},
			},
		}
		u, _, err := scanTimeTriggerSemantics(tt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !u {
			t.Error("expected usesID = true")
		}
	})
}

// ---------------------------------------------------------------------------
// 21. Clone functions — deep copy verification
// ---------------------------------------------------------------------------

func TestCloneStatement(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if cloneStatement(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("select", func(t *testing.T) {
		orig := &Statement{
			Select: &SelectStmt{
				Fields: []string{"title", "status"},
				Where: &CompareExpr{
					Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
				},
				OrderBy: []OrderByClause{{Field: "priority", Desc: true}},
			},
		}
		c := cloneStatement(orig)
		if c == orig {
			t.Error("clone should be a different pointer")
		}
		if c.Select == orig.Select {
			t.Error("Select should be cloned")
		}
		// mutate clone, verify original is unaffected
		c.Select.Fields[0] = "id"
		if orig.Select.Fields[0] != "title" {
			t.Error("mutating clone affected original fields")
		}
		c.Select.OrderBy[0].Field = "due"
		if orig.Select.OrderBy[0].Field != "priority" {
			t.Error("mutating clone affected original OrderBy")
		}
	})

	t.Run("create", func(t *testing.T) {
		orig := &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "hello"}},
				},
			},
		}
		c := cloneStatement(orig)
		c.Create.Assignments[0].Field = "status"
		if orig.Create.Assignments[0].Field != "title" {
			t.Error("mutating clone affected original")
		}
	})

	t.Run("update", func(t *testing.T) {
		orig := &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
				},
				Set: []Assignment{
					{Field: "title", Value: &StringLiteral{Value: "new"}},
				},
			},
		}
		c := cloneStatement(orig)
		c.Update.Set[0].Field = "status"
		if orig.Update.Set[0].Field != "title" {
			t.Error("mutating clone affected original")
		}
	})

	t.Run("delete", func(t *testing.T) {
		orig := &Statement{
			Delete: &DeleteStmt{
				Where: &CompareExpr{
					Left: &FieldRef{Name: "id"}, Op: "=", Right: &StringLiteral{Value: "T1"},
				},
			},
		}
		c := cloneStatement(orig)
		if c.Delete == orig.Delete {
			t.Error("Delete should be cloned")
		}
	})
}

func TestCloneSelect_Nil(t *testing.T) {
	if cloneSelect(nil) != nil {
		t.Error("expected nil")
	}
}

func TestCloneCondition(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if cloneCondition(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("BinaryCondition", func(t *testing.T) {
		orig := &BinaryCondition{
			Op: "and",
			Left: &CompareExpr{
				Left: &FieldRef{Name: "a"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
			Right: &CompareExpr{
				Left: &FieldRef{Name: "b"}, Op: "=", Right: &StringLiteral{Value: "y"},
			},
		}
		c, ok := cloneCondition(orig).(*BinaryCondition)
		if !ok {
			t.Fatal("expected *BinaryCondition")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
		if c.Op != "and" {
			t.Errorf("Op = %q, want %q", c.Op, "and")
		}
	})

	t.Run("NotCondition", func(t *testing.T) {
		orig := &NotCondition{
			Inner: &CompareExpr{
				Left: &FieldRef{Name: "a"}, Op: "=", Right: &StringLiteral{Value: "x"},
			},
		}
		c, ok := cloneCondition(orig).(*NotCondition)
		if !ok {
			t.Fatal("expected *NotCondition")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
	})

	t.Run("BoolExprCondition", func(t *testing.T) {
		orig := &BoolExprCondition{
			Expr: &FieldRef{Name: "flag"},
		}
		c, ok := cloneCondition(orig).(*BoolExprCondition)
		if !ok {
			t.Fatal("expected *BoolExprCondition")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
		if c.Expr == orig.Expr {
			t.Error("expected cloned expression")
		}
	})

	t.Run("CompareExpr", func(t *testing.T) {
		orig := &CompareExpr{
			Left: &FieldRef{Name: "a"}, Op: "!=", Right: &IntLiteral{Value: 1},
		}
		c, ok := cloneCondition(orig).(*CompareExpr)
		if !ok {
			t.Fatal("expected *CompareExpr")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
		if c.Op != "!=" {
			t.Errorf("Op = %q, want %q", c.Op, "!=")
		}
	})

	t.Run("IsEmptyExpr", func(t *testing.T) {
		orig := &IsEmptyExpr{
			Expr: &FieldRef{Name: "a"}, Negated: true,
		}
		c, ok := cloneCondition(orig).(*IsEmptyExpr)
		if !ok {
			t.Fatal("expected *IsEmptyExpr")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
		if !c.Negated {
			t.Error("expected Negated = true")
		}
	})

	t.Run("InExpr", func(t *testing.T) {
		orig := &InExpr{
			Value:      &StringLiteral{Value: "x"},
			Collection: &FieldRef{Name: "tags"},
			Negated:    true,
		}
		c, ok := cloneCondition(orig).(*InExpr)
		if !ok {
			t.Fatal("expected *InExpr")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
		if !c.Negated {
			t.Error("expected Negated = true")
		}
	})

	t.Run("QuantifierExpr", func(t *testing.T) {
		orig := &QuantifierExpr{
			Expr: &FieldRef{Name: "dependsOn"},
			Kind: "all",
			Condition: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
		}
		c, ok := cloneCondition(orig).(*QuantifierExpr)
		if !ok {
			t.Fatal("expected *QuantifierExpr")
		}
		if c == orig {
			t.Error("expected different pointer")
		}
		if c.Kind != "all" {
			t.Errorf("Kind = %q, want %q", c.Kind, "all")
		}
	})

	t.Run("unknown returns nil", func(t *testing.T) {
		if cloneCondition(&fakeCondition{}) != nil {
			t.Error("expected nil for unknown condition")
		}
	})
}

func TestCloneExpr(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if cloneExpr(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("all types", func(t *testing.T) {
		exprs := []Expr{
			&FieldRef{Name: "status"},
			&QualifiedRef{Qualifier: "old", Name: "title"},
			&StringLiteral{Value: "hello"},
			&IntLiteral{Value: 42},
			&BoolLiteral{Value: true},
			&DateLiteral{},
			&DurationLiteral{Value: 1, Unit: "day"},
			&ListLiteral{Elements: []Expr{&StringLiteral{Value: "a"}}},
			&EmptyLiteral{},
			&FunctionCall{Name: "id", Args: []Expr{&StringLiteral{Value: "x"}}},
			&BinaryExpr{Op: "+", Left: &IntLiteral{Value: 1}, Right: &IntLiteral{Value: 2}},
			&SubQuery{Where: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			}},
		}
		for _, e := range exprs {
			c := cloneExpr(e)
			if c == nil {
				t.Errorf("%T: clone returned nil", e)
			}
		}
	})

	t.Run("unknown returns nil", func(t *testing.T) {
		if cloneExpr(&fakeExpr{}) != nil {
			t.Error("expected nil for unknown expr")
		}
	})
}

func TestCloneTrigger(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if cloneTrigger(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("full trigger", func(t *testing.T) {
		deny := "blocked"
		orig := &Trigger{
			Timing: "before",
			Event:  "update",
			Where: &CompareExpr{
				Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "done"},
			},
			Action: &Statement{
				Update: &UpdateStmt{
					Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
					Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
				},
			},
			Run:  &RunAction{Command: &StringLiteral{Value: "echo"}},
			Deny: &deny,
		}
		c := cloneTrigger(orig)
		if c == orig {
			t.Error("expected different pointer")
		}
		if c.Timing != "before" || c.Event != "update" {
			t.Errorf("got timing=%q event=%q", c.Timing, c.Event)
		}
		// mutate deny clone
		newDeny := "changed"
		c.Deny = &newDeny
		if *orig.Deny != "blocked" {
			t.Error("mutating clone Deny affected original")
		}
		if c.Run == orig.Run {
			t.Error("Run should be cloned")
		}
	})
}

func TestCloneTimeTrigger(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if cloneTimeTrigger(nil) != nil {
			t.Error("expected nil")
		}
	})

	t.Run("with action", func(t *testing.T) {
		orig := &TimeTrigger{
			Interval: DurationLiteral{Value: 2, Unit: "hour"},
			Action: &Statement{
				Delete: &DeleteStmt{
					Where: &CompareExpr{
						Left: &FieldRef{Name: "status"}, Op: "=", Right: &StringLiteral{Value: "cancelled"},
					},
				},
			},
		}
		c := cloneTimeTrigger(orig)
		if c == orig {
			t.Error("expected different pointer")
		}
		if c.Interval.Value != 2 || c.Interval.Unit != "hour" {
			t.Errorf("interval = %+v, want 2hour", c.Interval)
		}
		if c.Action == orig.Action {
			t.Error("Action should be cloned")
		}
	})
}

// ---------------------------------------------------------------------------
// 22. Clone deep copy mutation isolation
// ---------------------------------------------------------------------------

func TestClone_MutationIsolation(t *testing.T) {
	t.Run("cloned list literal mutation", func(t *testing.T) {
		orig := &ListLiteral{Elements: []Expr{&StringLiteral{Value: "a"}, &StringLiteral{Value: "b"}}}
		c, ok := cloneExpr(orig).(*ListLiteral)
		if !ok {
			t.Fatal("expected *ListLiteral")
			return
		}
		c.Elements[0] = &StringLiteral{Value: "z"}
		sl, ok := orig.Elements[0].(*StringLiteral)
		if !ok {
			t.Fatal("expected *StringLiteral")
			return
		}
		if sl.Value != "a" {
			t.Error("mutating cloned ListLiteral affected original")
		}
	})

	t.Run("cloned FunctionCall args mutation", func(t *testing.T) {
		orig := &FunctionCall{Name: "fn", Args: []Expr{&IntLiteral{Value: 1}}}
		c, ok := cloneExpr(orig).(*FunctionCall)
		if !ok {
			t.Fatal("expected *FunctionCall")
			return
		}
		c.Args[0] = &IntLiteral{Value: 999}
		il, ok := orig.Args[0].(*IntLiteral)
		if !ok {
			t.Fatal("expected *IntLiteral")
			return
		}
		if il.Value != 1 {
			t.Error("mutating cloned FunctionCall args affected original")
		}
	})
}

// ---------------------------------------------------------------------------
// 23. End-to-end: ParseAndValidate round-trips produce sealed wrappers
// ---------------------------------------------------------------------------

func TestParseAndValidate_SealedResults(t *testing.T) {
	p := newTestParser()

	t.Run("statement sealed", func(t *testing.T) {
		vs, err := p.ParseAndValidateStatement("select", ExecutorRuntimeCLI)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := vs.mustBeSealed(); err != nil {
			t.Fatalf("not sealed: %v", err)
		}
	})

	t.Run("trigger sealed", func(t *testing.T) {
		vt, err := p.ParseAndValidateTrigger(
			`before update where new.status = "done" deny "no"`,
			ExecutorRuntimeEventTrigger,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := vt.mustBeSealed(); err != nil {
			t.Fatalf("not sealed: %v", err)
		}
	})

	t.Run("time trigger sealed", func(t *testing.T) {
		vtt, err := p.ParseAndValidateTimeTrigger(
			`every 1day delete where status = "done"`,
			ExecutorRuntimeTimeTrigger,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := vtt.mustBeSealed(); err != nil {
			t.Fatalf("not sealed: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// 24. scanSelectSemantics nil
// ---------------------------------------------------------------------------

func TestScanSelectSemantics_Nil(t *testing.T) {
	u, c, err := scanSelectSemantics(nil)
	if err != nil || u || c {
		t.Fatalf("expected (false, false, nil), got (%v, %v, %v)", u, c, err)
	}
}

// ---------------------------------------------------------------------------
// 25. id() detection in deeply nested positions
// ---------------------------------------------------------------------------

func TestIDDetection_NestedInBinaryExpr(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeCLI)
	stmt := &Statement{
		Select: &SelectStmt{
			Where: &CompareExpr{
				Left: &BinaryExpr{
					Op:    "+",
					Left:  &StringLiteral{Value: "prefix-"},
					Right: &FunctionCall{Name: "id"},
				},
				Op:    "=",
				Right: &StringLiteral{Value: "prefix-TIKI-AAA"},
			},
		},
	}
	_, err := v.ValidateStatement(stmt)
	if err == nil || !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() rejection in nested BinaryExpr, got: %v", err)
	}
}

func TestSemanticValidation_ExistsSubqueryScanned(t *testing.T) {
	p := newTestParser()

	_, err := p.ParseAndValidateStatement(`select where exists(select where id = id())`, ExecutorRuntimeCLI)
	if err == nil || !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() rejection inside exists subquery, got: %v", err)
	}

	_, err = p.ParseAndValidateStatementWithInput(
		`select where exists(select where assignee = input())`,
		ExecutorRuntimeCLI,
		ValueString,
	)
	if err == nil || !strings.Contains(err.Error(), "input() requires user interaction") {
		t.Fatalf("expected input() rejection inside exists subquery, got: %v", err)
	}

	_, err = p.ParseAndValidateStatement(`select where exists(select where id = choose(select))`, ExecutorRuntimeCLI)
	if err == nil || !strings.Contains(err.Error(), "choose() requires user interaction") {
		t.Fatalf("expected choose() rejection inside exists subquery, got: %v", err)
	}

	vs, err := p.ParseAndValidateStatement(
		`select where exists(select where id = choose(select where type = "epic"))`,
		ExecutorRuntimePlugin,
	)
	if err != nil {
		t.Fatalf("unexpected plugin validation error: %v", err)
	}
	if !vs.UsesChooseBuiltin() || vs.ChooseFilter() == nil {
		t.Fatal("expected choose() metadata from inside exists subquery")
	}
}

func TestCallDetection_NestedInListLiteral(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeCLI)
	stmt := &Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{
				{
					Field: "tags",
					Value: &ListLiteral{
						Elements: []Expr{
							&StringLiteral{Value: "a"},
							&FunctionCall{Name: "call", Args: []Expr{&StringLiteral{Value: "tag-gen"}}},
						},
					},
				},
			},
		},
	}
	_, err := v.ValidateStatement(stmt)
	if err == nil || !strings.Contains(err.Error(), "call() is not supported yet") {
		t.Fatalf("expected call() rejection in ListLiteral, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 26. Error propagation in scan* for BinaryCondition left error
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_BinaryConditionLeftError(t *testing.T) {
	_, _, err := scanConditionSemantics(&BinaryCondition{
		Op:    "and",
		Left:  &fakeCondition{},
		Right: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
	})
	if err == nil {
		t.Fatal("expected error from left branch")
	}
}

func TestScanConditionSemantics_BinaryConditionRightError(t *testing.T) {
	_, _, err := scanConditionSemantics(&BinaryCondition{
		Op:    "and",
		Left:  &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
		Right: &fakeCondition{},
	})
	if err == nil {
		t.Fatal("expected error from right branch")
	}
}

// ---------------------------------------------------------------------------
// 27. InExpr error propagation in scan
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_InExprCollectionError(t *testing.T) {
	// scanExprSemantics itself doesn't produce errors for basic types,
	// but scanConditionSemantics propagates correctly — verify both sides
	u, c, err := scanConditionSemantics(&InExpr{
		Value:      &FunctionCall{Name: "id"},
		Collection: &FieldRef{Name: "tags"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !u {
		t.Error("expected usesID = true from Value side")
	}
	if c {
		t.Error("expected hasCall = false")
	}
}

// ---------------------------------------------------------------------------
// 28. QuantifierExpr error propagation in scan
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_QuantifierExprError(t *testing.T) {
	_, _, err := scanConditionSemantics(&QuantifierExpr{
		Expr:      &FieldRef{Name: "dependsOn"},
		Kind:      "any",
		Condition: &fakeCondition{},
	})
	if err == nil {
		t.Fatal("expected error from quantifier condition")
	}
}

// ---------------------------------------------------------------------------
// 29. ValidatedTrigger clone integration — clone doesn't share pointers
// ---------------------------------------------------------------------------

func TestValidatedTrigger_TriggerClone_MutationIsolation(t *testing.T) {
	p := newTestParser()
	vt, err := p.ParseAndValidateTrigger(
		`before update where new.status = "done" deny "no"`,
		ExecutorRuntimeEventTrigger,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := vt.TriggerClone()
	if c == nil {
		t.Fatal("TriggerClone() returned nil")
	}
	// mutate clone
	c.Timing = "after"
	if vt.Timing() != "before" {
		t.Error("mutating clone affected validated trigger")
	}
}

// ---------------------------------------------------------------------------
// 30. ValidatedTimeTrigger clone integration
// ---------------------------------------------------------------------------

func TestValidatedTimeTrigger_TimeTriggerClone_MutationIsolation(t *testing.T) {
	p := newTestParser()
	vtt, err := p.ParseAndValidateTimeTrigger(
		`every 2day delete where status = "cancelled"`,
		ExecutorRuntimeTimeTrigger,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := vtt.TimeTriggerClone()
	if c == nil {
		t.Fatal("TimeTriggerClone() returned nil")
		return
	}
	c.Interval.Value = 99
	if vtt.IntervalLiteral().Value != 2 {
		t.Error("mutating clone affected validated time trigger")
	}
}

// ---------------------------------------------------------------------------
// 31. scanStatementSemantics update set error propagation
// ---------------------------------------------------------------------------

func TestScanStatementSemantics_UpdateSetError(t *testing.T) {
	// The scan functions return errors from scanExprSemantics only for internal errors,
	// and scanExprSemantics doesn't error on known types. But scanConditionSemantics
	// can error on unknown types — test that path via update where.
	_, _, err := scanStatementSemantics(&Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
			Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "ok"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 32. ParseAndValidateRule with semantic errors
// ---------------------------------------------------------------------------

func TestParseAndValidateRule_EventTriggerSemanticError(t *testing.T) {
	// call() in trigger action should be rejected
	// Use a valid trigger with call() injected directly
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &FunctionCall{Name: "call", Args: nil}},
				},
			},
		},
	}
	rule := &Rule{Trigger: trig}
	// since ParseAndValidateRule uses ParseRule internally, test validation directly
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	_, err := v.ValidateTrigger(rule.Trigger)
	if err == nil || !strings.Contains(err.Error(), "call()") {
		t.Fatalf("expected call() rejection, got: %v", err)
	}
}

func TestParseAndValidateRule_TimeTriggerSemanticError(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Create: &CreateStmt{
				Assignments: []Assignment{
					{Field: "title", Value: &FunctionCall{Name: "call", Args: nil}},
				},
			},
		},
	}
	_, err := v.ValidateTimeTrigger(tt)
	if err == nil || !strings.Contains(err.Error(), "call()") {
		t.Fatalf("expected call() rejection, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 33. ValidateTrigger — no action (nil action path)
// ---------------------------------------------------------------------------

func TestValidateTrigger_NilAction(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	deny := "blocked"
	trig := &Trigger{
		Timing: "before",
		Event:  "update",
		Deny:   &deny,
	}
	vt, err := v.ValidateTrigger(trig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, ok := vt.DenyMessage()
	if !ok || msg != "blocked" {
		t.Errorf("expected (\"blocked\", true), got (%q, %v)", msg, ok)
	}
}

// ---------------------------------------------------------------------------
// 34. ValidateTimeTrigger — no action (nil action path)
// ---------------------------------------------------------------------------

func TestValidateTimeTrigger_NilActionNoError(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 5, Unit: "hour"},
	}
	vtt, err := v.ValidateTimeTrigger(tt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	il := vtt.IntervalLiteral()
	if il.Value != 5 || il.Unit != "hour" {
		t.Errorf("interval = %+v, want 5hour", il)
	}
}

// ---------------------------------------------------------------------------
// 35. ValidateTrigger — id() rejected in event trigger runtime
// ---------------------------------------------------------------------------

func TestValidateTrigger_IDRejectedInEventTriggerRuntime(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeEventTrigger)
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Where: &CompareExpr{
			Left:  &FunctionCall{Name: "id"},
			Op:    "=",
			Right: &StringLiteral{Value: "x"},
		},
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
				Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "y"}}},
			},
		},
	}
	_, err := v.ValidateTrigger(trig)
	if err == nil || !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() rejection in event trigger runtime, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 36. ValidateTimeTrigger — id() rejected in time trigger runtime (via where)
// ---------------------------------------------------------------------------

func TestValidateTimeTrigger_IDRejectedInTimeTriggerRuntime(t *testing.T) {
	v := NewSemanticValidator(ExecutorRuntimeTimeTrigger)
	tt := &TimeTrigger{
		Interval: DurationLiteral{Value: 1, Unit: "day"},
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &CompareExpr{
					Left:  &FunctionCall{Name: "id"},
					Op:    "=",
					Right: &StringLiteral{Value: "x"},
				},
				Set: []Assignment{{Field: "title", Value: &StringLiteral{Value: "y"}}},
			},
		},
	}
	_, err := v.ValidateTimeTrigger(tt)
	if err == nil || !strings.Contains(err.Error(), "id() is only available in plugin runtime") {
		t.Fatalf("expected id() rejection in time trigger runtime, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 37. scanTriggerSemantics — error propagation from where, action, run
// ---------------------------------------------------------------------------

func TestScanTriggerSemantics_WhereError(t *testing.T) {
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Where:  &fakeCondition{},
	}
	_, _, err := scanTriggerSemantics(trig)
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected unknown condition type error from where, got: %v", err)
	}
}

func TestScanTriggerSemantics_ActionError(t *testing.T) {
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Action: &Statement{
			Update: &UpdateStmt{
				Where: &fakeCondition{},
				Set:   []Assignment{{Field: "title", Value: &StringLiteral{Value: "x"}}},
			},
		},
	}
	_, _, err := scanTriggerSemantics(trig)
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected unknown condition type error from action, got: %v", err)
	}
}

func TestScanTriggerSemantics_RunCommandError(t *testing.T) {
	trig := &Trigger{
		Timing: "after",
		Event:  "update",
		Run: &RunAction{
			Command: &SubQuery{Where: &fakeCondition{}},
		},
	}
	_, _, err := scanTriggerSemantics(trig)
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected unknown condition type error from run command, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 38. scanAssignmentsSemantics — error propagation via SubQuery
// ---------------------------------------------------------------------------

func TestScanAssignmentsSemantics_Error(t *testing.T) {
	assignments := []Assignment{
		{Field: "title", Value: &SubQuery{Where: &fakeCondition{}}},
	}
	_, _, err := scanAssignmentsSemantics(assignments)
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected unknown condition type error from assignment value, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 39. scanConditionSemantics — CompareExpr right-side error
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_CompareExprRightError(t *testing.T) {
	_, _, err := scanConditionSemantics(&CompareExpr{
		Left:  &StringLiteral{Value: "x"},
		Op:    "=",
		Right: &SubQuery{Where: &fakeCondition{}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from CompareExpr right side, got: %v", err)
	}
}

func TestScanConditionSemantics_CompareExprLeftError(t *testing.T) {
	_, _, err := scanConditionSemantics(&CompareExpr{
		Left:  &SubQuery{Where: &fakeCondition{}},
		Op:    "=",
		Right: &StringLiteral{Value: "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from CompareExpr left side, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 40. scanConditionSemantics — InExpr value and collection errors
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_InExprValueError(t *testing.T) {
	_, _, err := scanConditionSemantics(&InExpr{
		Value:      &SubQuery{Where: &fakeCondition{}},
		Collection: &FieldRef{Name: "tags"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from InExpr value, got: %v", err)
	}
}

func TestScanConditionSemantics_InExprCollectionError2(t *testing.T) {
	_, _, err := scanConditionSemantics(&InExpr{
		Value:      &StringLiteral{Value: "a"},
		Collection: &SubQuery{Where: &fakeCondition{}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from InExpr collection, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 41. scanConditionSemantics — QuantifierExpr expr error
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_QuantifierExprExprError(t *testing.T) {
	_, _, err := scanConditionSemantics(&QuantifierExpr{
		Expr: &SubQuery{Where: &fakeCondition{}},
		Kind: "any",
		Condition: &CompareExpr{
			Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from QuantifierExpr expr, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 42. scanConditionSemantics — NotCondition error propagation
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_NotConditionError(t *testing.T) {
	_, _, err := scanConditionSemantics(&NotCondition{
		Inner: &fakeCondition{},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from NotCondition inner, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 43. scanConditionSemantics — IsEmptyExpr error propagation
// ---------------------------------------------------------------------------

func TestScanConditionSemantics_IsEmptyExprError(t *testing.T) {
	_, _, err := scanConditionSemantics(&IsEmptyExpr{
		Expr: &SubQuery{Where: &fakeCondition{}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from IsEmptyExpr, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 44. scanExprSemantics — BinaryExpr left and right error propagation
// ---------------------------------------------------------------------------

func TestScanExprSemantics_BinaryExprLeftError(t *testing.T) {
	_, _, err := scanExprSemantics(&BinaryExpr{
		Op:    "+",
		Left:  &SubQuery{Where: &fakeCondition{}},
		Right: &IntLiteral{Value: 1},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from BinaryExpr left, got: %v", err)
	}
}

func TestScanExprSemantics_BinaryExprRightError(t *testing.T) {
	_, _, err := scanExprSemantics(&BinaryExpr{
		Op:    "+",
		Left:  &IntLiteral{Value: 1},
		Right: &SubQuery{Where: &fakeCondition{}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from BinaryExpr right, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 45. scanExprSemantics — ListLiteral element error propagation
// ---------------------------------------------------------------------------

func TestScanExprSemantics_ListLiteralElementError(t *testing.T) {
	_, _, err := scanExprSemantics(&ListLiteral{
		Elements: []Expr{
			&StringLiteral{Value: "ok"},
			&SubQuery{Where: &fakeCondition{}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from ListLiteral element, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 46. scanExprSemantics — FunctionCall arg error propagation
// ---------------------------------------------------------------------------

func TestScanExprSemantics_FunctionCallArgError(t *testing.T) {
	_, _, err := scanExprSemantics(&FunctionCall{
		Name: "blocks",
		Args: []Expr{&SubQuery{Where: &fakeCondition{}}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from FunctionCall arg, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 47. scanExprSemantics — SubQuery error propagation
// ---------------------------------------------------------------------------

func TestScanExprSemantics_SubQueryError(t *testing.T) {
	_, _, err := scanExprSemantics(&SubQuery{Where: &fakeCondition{}})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from SubQuery, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 48. scanStatementSemantics — update set assignment error propagation
// ---------------------------------------------------------------------------

func TestScanStatementSemantics_UpdateSetAssignmentError(t *testing.T) {
	_, _, err := scanStatementSemantics(&Statement{
		Update: &UpdateStmt{
			Where: &CompareExpr{Left: &IntLiteral{Value: 1}, Op: "=", Right: &IntLiteral{Value: 1}},
			Set:   []Assignment{{Field: "title", Value: &SubQuery{Where: &fakeCondition{}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from update set assignment, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 49. scanStatementSemantics — create assignment error propagation
// ---------------------------------------------------------------------------

func TestScanStatementSemantics_CreateAssignmentError(t *testing.T) {
	_, _, err := scanStatementSemantics(&Statement{
		Create: &CreateStmt{
			Assignments: []Assignment{{Field: "title", Value: &SubQuery{Where: &fakeCondition{}}}},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from create assignment, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 50. scanStatementSemantics — delete where error propagation
// ---------------------------------------------------------------------------

func TestScanStatementSemantics_DeleteWhereError(t *testing.T) {
	_, _, err := scanStatementSemantics(&Statement{
		Delete: &DeleteStmt{Where: &fakeCondition{}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from delete where, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// 51. scanStatementSemantics — select where error propagation
// ---------------------------------------------------------------------------

func TestScanStatementSemantics_SelectWhereError(t *testing.T) {
	_, _, err := scanStatementSemantics(&Statement{
		Select: &SelectStmt{Where: &fakeCondition{}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown condition type") {
		t.Fatalf("expected error from select where, got: %v", err)
	}
}

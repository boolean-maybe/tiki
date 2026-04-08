package ruki

import "testing"

func TestParseTrigger_BeforeDeny(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
		event string
	}{
		{
			"block completion with open deps",
			`before update where new.status = "done" and new.dependsOn any status != "done" deny "cannot complete task with open dependencies"`,
			"update",
		},
		{
			"deny delete high priority",
			`before delete where old.priority <= 2 deny "cannot delete high priority tasks"`,
			"delete",
		},
		{
			"require description for high priority",
			`before update where new.priority <= 2 and new.description is empty deny "high priority tasks need a description"`,
			"update",
		},
		{
			"require description for stories",
			`before create where new.type = "story" and new.description is empty deny "stories must have a description"`,
			"create",
		},
		{
			"prevent skipping review",
			`before update where old.status = "in progress" and new.status = "done" deny "tasks must go through review before completion"`,
			"update",
		},
		{
			"protect high priority from demotion",
			`before update where old.priority = 1 and old.status = "in progress" and new.priority > 1 deny "cannot demote priority of active critical tasks"`,
			"update",
		},
		{
			"no empty epics",
			`before update where new.status = "done" and new.type = "epic" and blocks(new.id) is empty deny "epic has no dependencies"`,
			"update",
		},
		{
			"WIP limit",
			`before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit reached for this assignee"`,
			"update",
		},
		{
			"points required before start",
			`before update where new.status = "in progress" and new.points = 0 deny "tasks must be estimated before starting work"`,
			"update",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trig, err := p.ParseTrigger(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if trig.Timing != "before" {
				t.Fatalf("expected before, got %s", trig.Timing)
			}
			if trig.Event != tt.event {
				t.Fatalf("expected %s, got %s", tt.event, trig.Event)
			}
			if trig.Deny == nil {
				t.Fatal("expected Deny, got nil")
			}
			if trig.Action != nil {
				t.Fatal("expected nil Action in before-trigger")
			}
		})
	}
}

func TestParseTrigger_AfterAction(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name       string
		input      string
		event      string
		wantCreate bool
		wantUpdate bool
		wantDelete bool
		wantRun    bool
	}{
		{
			"recurring task create next",
			`after update where new.status = "done" and old.recurrence is not empty create title=old.title priority=old.priority tags=old.tags recurrence=old.recurrence due=next_date(old.recurrence) status="ready"`,
			"update",
			true, false, false, false,
		},
		{
			"recurring task clear recurrence",
			`after update where new.status = "done" and old.recurrence is not empty update where id = old.id set recurrence=empty`,
			"update",
			false, true, false, false,
		},
		{
			"auto assign urgent",
			`after create where new.priority <= 2 and new.assignee is empty update where id = new.id set assignee="booleanmaybe"`,
			"create",
			false, true, false, false,
		},
		{
			"cascade epic completion",
			`after update where new.status = "done" update where id in blocks(old.id) and type = "epic" and dependsOn all status = "done" set status="done"`,
			"update",
			false, true, false, false,
		},
		{
			"reopen epic on regression",
			`after update where old.status = "done" and new.status != "done" update where id in blocks(old.id) and type = "epic" and status = "done" set status="in progress"`,
			"update",
			false, true, false, false,
		},
		{
			"auto tag bugs",
			`after create where new.type = "bug" update where id = new.id set tags=new.tags + ["needs-triage"]`,
			"create",
			false, true, false, false,
		},
		{
			"propagate cancellation",
			`after update where new.status = "cancelled" update where id in blocks(old.id) and status in ["backlog", "ready"] set status="cancelled"`,
			"update",
			false, true, false, false,
		},
		{
			"unblock on last blocker",
			`after update where new.status = "done" update where old.id in dependsOn and dependsOn all status = "done" and status = "backlog" set status="ready"`,
			"update",
			false, true, false, false,
		},
		{
			"cleanup on delete",
			`after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`,
			"delete",
			false, true, false, false,
		},
		{
			"auto delete stale",
			`after update where new.status = "done" and old.updatedAt < now() - 2day delete where id = old.id`,
			"update",
			false, false, true, false,
		},
		{
			"run action",
			`after update where new.status = "in progress" and "claude" in new.tags run("claude -p 'implement tiki " + old.id + "'")`,
			"update",
			false, false, false, true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trig, err := p.ParseTrigger(tt.input)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if trig.Timing != "after" {
				t.Fatalf("expected after, got %s", trig.Timing)
			}
			if trig.Event != tt.event {
				t.Fatalf("expected %s, got %s", tt.event, trig.Event)
			}
			if trig.Deny != nil {
				t.Fatal("expected nil Deny in after-trigger")
			}

			if tt.wantRun {
				if trig.Run == nil {
					t.Fatal("expected Run action, got nil")
				}
			} else {
				if trig.Action == nil {
					t.Fatal("expected Action, got nil")
				}
				if tt.wantCreate && trig.Action.Create == nil {
					t.Fatal("expected Create action")
				}
				if tt.wantUpdate && trig.Action.Update == nil {
					t.Fatal("expected Update action")
				}
				if tt.wantDelete && trig.Action.Delete == nil {
					t.Fatal("expected Delete action")
				}
			}
		})
	}
}

func TestParseTrigger_StructuralErrors(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{
			"before with action",
			`before update where new.status = "done" update where id = old.id set status="done"`,
		},
		{
			"after with deny",
			`after update where new.status = "done" deny "no"`,
		},
		{
			"before without deny",
			`before update where new.status = "done"`,
		},
		{
			"after without action",
			`after update where new.status = "done"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseTrigger(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestParseTrigger_QualifiedRefsInWhere(t *testing.T) {
	p := newTestParser()

	input := `before update where old.status = "in progress" and new.status = "done" deny "skip"`
	trig, err := p.ParseTrigger(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	bc, ok := trig.Where.(*BinaryCondition)
	if !ok {
		t.Fatalf("expected BinaryCondition, got %T", trig.Where)
	}

	// check left side has old.status
	leftCmp, ok := bc.Left.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr on left, got %T", bc.Left)
	}
	qr, ok := leftCmp.Left.(*QualifiedRef)
	if !ok {
		t.Fatalf("expected QualifiedRef, got %T", leftCmp.Left)
	}
	if qr.Qualifier != "old" || qr.Name != "status" {
		t.Fatalf("expected old.status, got %s.%s", qr.Qualifier, qr.Name)
	}

	// check right side has new.status
	rightCmp, ok := bc.Right.(*CompareExpr)
	if !ok {
		t.Fatalf("expected CompareExpr on right, got %T", bc.Right)
	}
	qr2, ok := rightCmp.Left.(*QualifiedRef)
	if !ok {
		t.Fatalf("expected QualifiedRef, got %T", rightCmp.Left)
	}
	if qr2.Qualifier != "new" || qr2.Name != "status" {
		t.Fatalf("expected new.status, got %s.%s", qr2.Qualifier, qr2.Name)
	}
}

func TestParseTrigger_NoWhereGuard(t *testing.T) {
	p := newTestParser()

	input := `after delete update where old.id in dependsOn set dependsOn=dependsOn - [old.id]`
	trig, err := p.ParseTrigger(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if trig.Where != nil {
		t.Fatal("expected nil Where guard")
	}
	if trig.Action == nil || trig.Action.Update == nil {
		t.Fatal("expected Update action")
	}
}

func TestParseTrigger_BareFieldInGuard_Rejected(t *testing.T) {
	p := newTestParser()

	tests := []struct {
		name  string
		input string
	}{
		{
			"bare field in comparison",
			`before update where status = "done" deny "no"`,
		},
		{
			"bare field in quantifier collection",
			`before update where dependsOn any status = "done" deny "no"`,
		},
		{
			"bare field in is empty",
			`before create where description is empty deny "need description"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseTrigger(tt.input)
			if err == nil {
				t.Fatal("expected error for bare field in trigger guard")
			}
		})
	}
}

func TestParseTrigger_BareFieldInsideQuantifier_Allowed(t *testing.T) {
	p := newTestParser()

	// bare status inside quantifier body is OK (zone 3), even within a trigger guard
	input := `before update where new.status = "done" and new.dependsOn all status != "done" deny "open deps"`
	_, err := p.ParseTrigger(input)
	if err != nil {
		t.Fatalf("expected success for bare field inside quantifier body: %v", err)
	}
}

func TestParseTrigger_BareFieldInsideSubquery_Allowed(t *testing.T) {
	p := newTestParser()

	// bare fields inside count(select where ...) are OK (zone 4), qualifiers also OK
	input := `before update where new.status = "in progress" and count(select where assignee = new.assignee and status = "in progress") >= 3 deny "WIP limit"`
	_, err := p.ParseTrigger(input)
	if err != nil {
		t.Fatalf("expected success for bare field inside subquery: %v", err)
	}
}

func TestParseTrigger_QualifierInQuantifierBody_Rejected(t *testing.T) {
	p := newTestParser()

	// qualifiers inside quantifier bodies are forbidden (zone 3)
	input := `before update where new.dependsOn all old.status = "done" deny "no"`
	_, err := p.ParseTrigger(input)
	if err == nil {
		t.Fatal("expected error for qualifier inside quantifier body")
	}
}

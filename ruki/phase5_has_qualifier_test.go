package ruki

import (
	"strings"
	"testing"
)

// These tests lock in that has() routes its argument through the same
// qualifier-policy rules as ordinary field references. Before the fix,
// validateHasFuncCall only checked that the field name existed, so
// qualified forms valid only in plugin or trigger contexts silently
// parsed in CLI selects and either evaluated false or errored at runtime.

// --- CLI context rejects trigger-only qualifiers ---

func TestPhase5_Has_RejectsOldQualifierInCLI(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select where has(old.status)`)
	if err == nil {
		t.Fatal("expected parse error: has(old.status) in CLI context")
	}
	if !strings.Contains(err.Error(), "old.") {
		t.Fatalf("expected qualifier-policy error mentioning old., got: %v", err)
	}
}

func TestPhase5_Has_RejectsNewQualifierInCLI(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseStatement(`select where has(new.status)`)
	if err == nil {
		t.Fatal("expected parse error: has(new.status) in CLI context")
	}
	if !strings.Contains(err.Error(), "new.") {
		t.Fatalf("expected qualifier-policy error mentioning new., got: %v", err)
	}
}

// --- CLI context rejects plugin-only qualifiers ---
//
// The `target.` qualifier parses cleanly (the parser allows it so plugin
// actions can use it), and its runtime-mode enforcement runs at semantic
// validation. Route through ParseAndValidateStatement with
// ExecutorRuntimeCLI to exercise the full pipeline the CLI uses.

func TestPhase5_Has_RejectsTargetQualifierInCLI(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseAndValidateStatement(`select where has(target.status)`, ExecutorRuntimeCLI)
	if err == nil {
		t.Fatal("expected semantic validation error: has(target.status) outside plugin runtime")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Fatalf("expected target-qualifier error, got: %v", err)
	}
}

// --- trigger guard rejects bare refs inside has() ---
//
// Bare field refs are rejected in trigger guards (the guard has no
// "current task"; refs must be qualified with old./new.). has() must
// honor that rule too — otherwise bare has(status) would silently
// evaluate against an ambiguous sentinel.

func TestPhase5_Has_RejectsBareRefInTriggerGuard(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseTrigger(`before update where has(status) deny "x"`)
	if err == nil {
		t.Fatal("expected parse error: bare has(status) in trigger guard")
	}
	if !strings.Contains(err.Error(), "bare field") {
		t.Fatalf("expected bare-field error, got: %v", err)
	}
}

// --- trigger guard accepts has(new.X) / has(old.X) ---
//
// Positive-path regression guard so the policy tightening doesn't
// accidentally block the valid qualified forms.

func TestPhase5_Has_AcceptsQualifiedRefInTriggerGuard(t *testing.T) {
	p := newTestParser()
	if _, err := p.ParseTrigger(`before update where has(old.status) deny "x"`); err != nil {
		t.Fatalf("has(old.status) should parse in trigger guard: %v", err)
	}
	if _, err := p.ParseTrigger(`before update where not has(new.assignee) deny "x"`); err != nil {
		t.Fatalf("has(new.assignee) should parse in trigger guard: %v", err)
	}
}

// --- unknown field still rejected at validation time ---
//
// The schema check is independent of qualifier policy; both must pass.

func TestPhase5_Has_RejectsUnknownFieldInQualifiedForm(t *testing.T) {
	p := newTestParser()
	_, err := p.ParseTrigger(`before update where has(new.nosuchfield) deny "x"`)
	if err == nil {
		t.Fatal("expected parse error: unknown field in has(new.nosuchfield)")
	}
}

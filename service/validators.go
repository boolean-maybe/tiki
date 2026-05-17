package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/boolean-maybe/tiki/document"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// RegisterFieldValidators registers standard field validators with the gate.
// Every validator runs on every create and update. Workflow-field validators
// treat an absent value as success, matching the presence-aware contract.
func RegisterFieldValidators(g *TikiMutationGate) {
	for _, fn := range AllTikiValidators() {
		wrapped := wrapTikiFieldValidator(fn)
		g.OnCreate(wrapped)
		g.OnUpdate(wrapped)
	}
}

func wrapTikiFieldValidator(fn func(*tikipkg.Tiki) string) MutationValidator {
	return func(old, new *tikipkg.Tiki, _ []*tikipkg.Tiki) *Rejection {
		t := new
		if t == nil {
			t = old // delete case
		}
		if msg := fn(t); msg != "" {
			return &Rejection{Reason: msg}
		}
		return nil
	}
}

// AllTikiValidators returns the list of mutation-gate validators. Identity
// invariants (title required, length cap) are system-level. Per-field
// validation is driven by the loaded workflow catalog: every workflow-
// declared field is checked against its declared type. Workflows that omit
// fields naturally skip validation for those names.
func AllTikiValidators() []func(*tikipkg.Tiki) string {
	return []func(*tikipkg.Tiki) string{
		validateTikiTitle,
		validateTikiWorkflowFields,
	}
}

func validateTikiTitle(tk *tikipkg.Tiki) string {
	title := strings.TrimSpace(tk.Title)
	if title == "" {
		return "title is required"
	}
	const maxTitleLength = 200
	if len(title) > maxTitleLength {
		return fmt.Sprintf("title exceeds maximum length of %d characters", maxTitleLength)
	}
	return ""
}

// validateTikiWorkflowFields walks every workflow-declared field and rejects
// values that don't match the declared type. Absent fields pass (presence-
// aware contract); fields not declared in workflow.yaml are not checked here
// — they round-trip as unknown.
func validateTikiWorkflowFields(tk *tikipkg.Tiki) string {
	if tk == nil {
		return ""
	}
	for _, fd := range workflow.WorkflowFields() {
		raw, present := tk.Get(fd.Name)
		if !present {
			continue
		}
		if msg := validateWorkflowFieldValue(fd, raw); msg != "" {
			return msg
		}
	}
	return ""
}

// validateWorkflowFieldValue returns a human error message if raw doesn't
// satisfy the declared type for fd, or "" if it does.
func validateWorkflowFieldValue(fd workflow.FieldDef, raw interface{}) string {
	switch fd.Type {
	case workflow.TypeString:
		if _, ok := raw.(string); !ok {
			return fmt.Sprintf("%s field has wrong type (expected string)", fd.Name)
		}

	case workflow.TypeInt:
		if _, ok := coerceIntValue(raw); !ok {
			return fmt.Sprintf("%s field has wrong type (expected integer)", fd.Name)
		}

	case workflow.TypeBool:
		if _, ok := raw.(bool); !ok {
			return fmt.Sprintf("%s field has wrong type (expected boolean)", fd.Name)
		}

	case workflow.TypeEnum:
		s, ok := raw.(string)
		if !ok {
			return fmt.Sprintf("%s field has wrong type (expected string)", fd.Name)
		}
		if s == "" {
			return ""
		}
		if !fd.IsValidEnum(s) {
			return fmt.Sprintf("invalid %s value: %s", fd.Name, s)
		}

	case workflow.TypeListString:
		if _, ok := coerceStringListValue(raw); !ok {
			return fmt.Sprintf("%s field has wrong type (expected list of strings)", fd.Name)
		}

	case workflow.TypeListRef:
		ss, ok := coerceStringListValue(raw)
		if !ok {
			return fmt.Sprintf("%s field has wrong type (expected list of refs)", fd.Name)
		}
		for _, dep := range ss {
			if !document.IsValidID(dep) {
				return fmt.Sprintf("invalid document ID format: %s (expected %d uppercase alphanumeric chars)",
					dep, document.IDLength)
			}
		}

	case workflow.TypeDate, workflow.TypeTimestamp:
		if _, ok := raw.(time.Time); !ok {
			return fmt.Sprintf("%s field has wrong type (expected date)", fd.Name)
		}

	case workflow.TypeRef, workflow.TypeID:
		s, ok := raw.(string)
		if !ok {
			return fmt.Sprintf("%s field has wrong type (expected ref string)", fd.Name)
		}
		if s != "" && !document.IsValidID(strings.ToUpper(strings.TrimSpace(s))) {
			return fmt.Sprintf("invalid %s reference %q", fd.Name, s)
		}

	case workflow.TypeRecurrence, workflow.TypeDuration:
		if _, ok := raw.(string); !ok {
			return fmt.Sprintf("%s field has wrong type (expected string)", fd.Name)
		}
	}
	return ""
}

func coerceIntValue(raw interface{}) (int, bool) {
	switch n := raw.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		if n == float64(int64(n)) {
			return int(n), true
		}
	}
	return 0, false
}

func coerceStringListValue(raw interface{}) ([]string, bool) {
	switch v := raw.(type) {
	case []string:
		return v, true
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	}
	return nil, false
}

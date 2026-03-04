package filter

import "fmt"

// ValidateFilterStatuses walks a FilterExpr AST and checks that any status
// references (in CompareExpr and InExpr nodes where Field == "status") are
// valid according to the provided validator function.
// Returns the first invalid status found with a descriptive error, or nil.
func ValidateFilterStatuses(expr FilterExpr, validStatus func(string) bool) error {
	if expr == nil {
		return nil
	}

	switch e := expr.(type) {
	case *BinaryExpr:
		if err := ValidateFilterStatuses(e.Left, validStatus); err != nil {
			return err
		}
		return ValidateFilterStatuses(e.Right, validStatus)

	case *UnaryExpr:
		return ValidateFilterStatuses(e.Expr, validStatus)

	case *CompareExpr:
		if e.Field != "status" {
			return nil
		}
		if strVal, ok := e.Value.(string); ok {
			if !validStatus(strVal) {
				return fmt.Errorf("filter references unknown status %q", strVal)
			}
		}
		return nil

	case *InExpr:
		if e.Field != "status" {
			return nil
		}
		for _, val := range e.Values {
			if strVal, ok := val.(string); ok {
				if !validStatus(strVal) {
					return fmt.Errorf("filter references unknown status %q", strVal)
				}
			}
		}
		return nil

	default:
		return nil
	}
}

package fieldmeta

import (
	"testing"

	"github.com/boolean-maybe/tiki/workflow"
)

func TestForValueType(t *testing.T) {
	tests := []struct {
		name string
		in   workflow.ValueType
		want SemanticType
	}{
		{"enum", workflow.TypeEnum, SemanticEnum},
		{"text", workflow.TypeString, SemanticText},
		{"user", workflow.TypeUser, SemanticUser},
		{"integer", workflow.TypeInt, SemanticInteger},
		{"boolean", workflow.TypeBool, SemanticBoolean},
		{"date", workflow.TypeDate, SemanticDate},
		{"datetime", workflow.TypeTimestamp, SemanticDateTime},
		{"recurrence", workflow.TypeRecurrence, SemanticRecurrence},
		{"string list", workflow.TypeListString, SemanticStringList},
		{"tiki id list", workflow.TypeListRef, SemanticTikiIDList},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ForValueType(tt.in); got != tt.want {
				t.Fatalf("ForValueType(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

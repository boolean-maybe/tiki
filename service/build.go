package service

// BuildGate creates a TikiMutationGate with standard field validators registered.
// Call SetStore() on the returned gate after store initialization.
func BuildGate() *TikiMutationGate {
	gate := NewTikiMutationGate()
	RegisterFieldValidators(gate)
	return gate
}

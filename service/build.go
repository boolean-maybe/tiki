package service

// BuildGate creates a TaskMutationGate with standard field validators registered.
// Call SetStore() on the returned gate after store initialization.
func BuildGate() *TaskMutationGate {
	gate := NewTaskMutationGate()
	RegisterFieldValidators(gate)
	return gate
}

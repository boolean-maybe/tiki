package task

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDependsOnValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
		wantErr  bool
	}{
		// Valid scenarios
		{
			name:     "empty dependsOn (omitted)",
			yaml:     "other: value",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "empty array",
			yaml:     "dependsOn: []",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "single dependency",
			yaml:     "dependsOn: [TIKI-ABC123]",
			expected: []string{"TIKI-ABC123"},
			wantErr:  false,
		},
		{
			name:     "multiple dependencies",
			yaml:     "dependsOn:\n  - TIKI-ABC123\n  - TIKI-DEF456\n  - TIKI-GHI789",
			expected: []string{"TIKI-ABC123", "TIKI-DEF456", "TIKI-GHI789"},
			wantErr:  false,
		},
		{
			name:     "lowercase IDs uppercased",
			yaml:     "dependsOn:\n  - tiki-abc123\n  - tiki-def456",
			expected: []string{"TIKI-ABC123", "TIKI-DEF456"},
			wantErr:  false,
		},
		{
			name:     "mixed case IDs uppercased",
			yaml:     "dependsOn: [Tiki-Abc123]",
			expected: []string{"TIKI-ABC123"},
			wantErr:  false,
		},
		{
			name:     "filter empty strings",
			yaml:     "dependsOn: [TIKI-ABC123, '', TIKI-DEF456]",
			expected: []string{"TIKI-ABC123", "TIKI-DEF456"},
			wantErr:  false,
		},
		{
			name:     "filter whitespace-only strings",
			yaml:     "dependsOn: [TIKI-ABC123, '  ', TIKI-DEF456]",
			expected: []string{"TIKI-ABC123", "TIKI-DEF456"},
			wantErr:  false,
		},

		// Invalid scenarios - should default to empty with no error
		{
			name:     "scalar string instead of list",
			yaml:     "dependsOn: not-a-list",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "number instead of list",
			yaml:     "dependsOn: 123",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "boolean instead of list",
			yaml:     "dependsOn: true",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "object instead of list",
			yaml:     "dependsOn:\n  key: value",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "null value",
			yaml:     "dependsOn: null",
			expected: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				DependsOn DependsOnValue `yaml:"dependsOn,omitempty"`
			}

			var result testStruct
			err := yaml.Unmarshal([]byte(tt.yaml), &result)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got := result.DependsOn.ToStringSlice()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("UnmarshalYAML() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestDependsOnValue_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		deps     DependsOnValue
		expected string
	}{
		{
			name:     "empty dependsOn",
			deps:     DependsOnValue([]string{}),
			expected: " []\n",
		},
		{
			name:     "single dependency",
			deps:     DependsOnValue([]string{"TIKI-ABC123"}),
			expected: "\n    - TIKI-ABC123\n",
		},
		{
			name:     "multiple dependencies",
			deps:     DependsOnValue([]string{"TIKI-ABC123", "TIKI-DEF456", "TIKI-GHI789"}),
			expected: "\n    - TIKI-ABC123\n    - TIKI-DEF456\n    - TIKI-GHI789\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				DependsOn DependsOnValue `yaml:"dependsOn"`
			}

			input := testStruct{DependsOn: tt.deps}
			got, err := yaml.Marshal(input)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}

			expected := "dependsOn:" + tt.expected
			if string(got) != expected {
				t.Errorf("MarshalYAML() got = %q, expected %q", string(got), expected)
			}
		})
	}
}

func TestDependsOnValue_ToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		deps     DependsOnValue
		expected []string
	}{
		{
			name:     "nil dependsOn",
			deps:     nil,
			expected: []string{},
		},
		{
			name:     "empty dependsOn",
			deps:     DependsOnValue([]string{}),
			expected: []string{},
		},
		{
			name:     "single dependency",
			deps:     DependsOnValue([]string{"TIKI-ABC123"}),
			expected: []string{"TIKI-ABC123"},
		},
		{
			name:     "multiple dependencies",
			deps:     DependsOnValue([]string{"TIKI-ABC123", "TIKI-DEF456"}),
			expected: []string{"TIKI-ABC123", "TIKI-DEF456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.deps.ToStringSlice()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ToStringSlice() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestFindBlockedTasks(t *testing.T) {
	taskA := &Task{ID: "TIKI-AAA", Title: "Task A", DependsOn: []string{}}
	taskB := &Task{ID: "TIKI-BBB", Title: "Task B", DependsOn: []string{"TIKI-AAA"}}
	taskC := &Task{ID: "TIKI-CCC", Title: "Task C", DependsOn: []string{"TIKI-AAA", "TIKI-BBB"}}
	taskD := &Task{ID: "TIKI-DDD", Title: "Task D", DependsOn: []string{"TIKI-CCC"}}

	allTasks := []*Task{taskA, taskB, taskC, taskD}

	tests := []struct {
		name    string
		taskID  string
		wantIDs []string
	}{
		{
			name:    "task with two blockers",
			taskID:  "TIKI-AAA",
			wantIDs: []string{"TIKI-BBB", "TIKI-CCC"},
		},
		{
			name:    "task with one blocker",
			taskID:  "TIKI-BBB",
			wantIDs: []string{"TIKI-CCC"},
		},
		{
			name:    "task with one blocker (chain)",
			taskID:  "TIKI-CCC",
			wantIDs: []string{"TIKI-DDD"},
		},
		{
			name:    "task blocking nothing",
			taskID:  "TIKI-DDD",
			wantIDs: nil,
		},
		{
			name:    "non-existent task ID",
			taskID:  "TIKI-ZZZ",
			wantIDs: nil,
		},
		{
			name:    "case insensitive lookup",
			taskID:  "tiki-aaa",
			wantIDs: []string{"TIKI-BBB", "TIKI-CCC"},
		},
		{
			name:    "empty task list",
			taskID:  "TIKI-AAA",
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := allTasks
			if tt.name == "empty task list" {
				input = nil
			}
			got := FindBlockedTasks(input, tt.taskID)

			var gotIDs []string
			for _, task := range got {
				gotIDs = append(gotIDs, task.ID)
			}

			if len(gotIDs) != len(tt.wantIDs) {
				t.Errorf("FindBlockedTasks() returned %d tasks %v, want %d tasks %v", len(gotIDs), gotIDs, len(tt.wantIDs), tt.wantIDs)
				return
			}

			wantSet := make(map[string]bool, len(tt.wantIDs))
			for _, id := range tt.wantIDs {
				wantSet[id] = true
			}
			for _, id := range gotIDs {
				if !wantSet[id] {
					t.Errorf("FindBlockedTasks() returned unexpected task %s", id)
				}
			}
		})
	}
}

func TestDependsOnValue_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		deps []string
	}{
		{
			name: "empty dependsOn",
			deps: []string{},
		},
		{
			name: "single dependency",
			deps: []string{"TIKI-ABC123"},
		},
		{
			name: "multiple dependencies",
			deps: []string{"TIKI-ABC123", "TIKI-DEF456", "TIKI-GHI789"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				DependsOn DependsOnValue `yaml:"dependsOn"`
			}

			// Marshal
			input := testStruct{DependsOn: DependsOnValue(tt.deps)}
			yamlBytes, err := yaml.Marshal(input)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Unmarshal
			var output testStruct
			err = yaml.Unmarshal(yamlBytes, &output)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			// Compare
			got := output.DependsOn.ToStringSlice()
			if !reflect.DeepEqual(got, tt.deps) {
				t.Errorf("Round trip failed: got = %v, expected %v", got, tt.deps)
			}
		})
	}
}

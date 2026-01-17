package task

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPriorityValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected int
		wantErr  bool
	}{
		{
			name:     "integer value",
			yaml:     "priority: 3",
			expected: 3,
			wantErr:  false,
		},
		{
			name:     "high priority word",
			yaml:     "priority: high",
			expected: PriorityHigh, // 1
			wantErr:  false,
		},
		{
			name:     "medium-high priority word",
			yaml:     "priority: medium-high",
			expected: PriorityMediumHigh, // 2
			wantErr:  false,
		},
		{
			name:     "high-medium priority word (alias)",
			yaml:     "priority: high-medium",
			expected: PriorityMediumHigh, // 2
			wantErr:  false,
		},
		{
			name:     "medium priority word",
			yaml:     "priority: medium",
			expected: PriorityMedium, // 3
			wantErr:  false,
		},
		{
			name:     "medium-low priority word",
			yaml:     "priority: medium-low",
			expected: PriorityMediumLow, // 4
			wantErr:  false,
		},
		{
			name:     "low priority word",
			yaml:     "priority: low",
			expected: PriorityLow, // 5
			wantErr:  false,
		},
		{
			name:     "uppercase word",
			yaml:     "priority: HIGH",
			expected: PriorityHigh, // 1
			wantErr:  false,
		},
		{
			name:     "mixed case word",
			yaml:     "priority: Medium-High",
			expected: PriorityMediumHigh, // 2
			wantErr:  false,
		},
		{
			name:     "underscore separator",
			yaml:     "priority: medium_high",
			expected: PriorityMediumHigh, // 2
			wantErr:  false,
		},
		{
			name:     "space separator",
			yaml:     "priority: medium high",
			expected: PriorityMediumHigh, // 2
			wantErr:  false,
		},
		{
			name:     "numeric string in range",
			yaml:     "priority: \"4\"",
			expected: 4,
			wantErr:  false,
		},
		{
			name:     "invalid word defaults to medium",
			yaml:     "priority: invalid",
			expected: PriorityMedium, // 3
			wantErr:  false,
		},
		{
			name:     "boolean defaults to medium",
			yaml:     "priority: true",
			expected: PriorityMedium, // 3
			wantErr:  false,
		},
		{
			name:     "empty string defaults to medium",
			yaml:     "priority: \"\"",
			expected: PriorityMedium, // 3
			wantErr:  false,
		},
		{
			name:     "whitespace-only defaults to medium",
			yaml:     "priority: \"  \"",
			expected: PriorityMedium, // 3
			wantErr:  false,
		},
		// Clamping tests
		{
			name:     "zero clamps to 1",
			yaml:     "priority: 0",
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "negative clamps to 1",
			yaml:     "priority: -5",
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "above max clamps to 5",
			yaml:     "priority: 10",
			expected: 5,
			wantErr:  false,
		},
		{
			name:     "numeric string zero clamps to 1",
			yaml:     "priority: \"0\"",
			expected: 1,
			wantErr:  false,
		},
		{
			name:     "numeric string above max clamps to 5",
			yaml:     "priority: \"99\"",
			expected: 5,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data struct {
				Priority PriorityValue `yaml:"priority"`
			}

			err := yaml.Unmarshal([]byte(tt.yaml), &data)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && int(data.Priority) != tt.expected {
				t.Errorf("UnmarshalYAML() = %d, want %d", data.Priority, tt.expected)
			}
		})
	}
}

func TestPriorityValue_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		priority PriorityValue
		expected string
	}{
		{
			name:     "high priority",
			priority: PriorityValue(PriorityHigh),
			expected: "priority: 1\n",
		},
		{
			name:     "medium priority",
			priority: PriorityValue(PriorityMedium),
			expected: "priority: 3\n",
		},
		{
			name:     "low priority",
			priority: PriorityValue(PriorityLow),
			expected: "priority: 5\n",
		},
		{
			name:     "medium-high priority",
			priority: PriorityValue(PriorityMediumHigh),
			expected: "priority: 2\n",
		},
		{
			name:     "medium-low priority",
			priority: PriorityValue(PriorityMediumLow),
			expected: "priority: 4\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := struct {
				Priority PriorityValue `yaml:"priority"`
			}{
				Priority: tt.priority,
			}

			result, err := yaml.Marshal(&data)
			if err != nil {
				t.Errorf("MarshalYAML() error = %v", err)
				return
			}

			if string(result) != tt.expected {
				t.Errorf("MarshalYAML() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		expected string
	}{
		{
			name:     "high priority (1)",
			priority: PriorityHigh, // 1
			expected: "🔴",
		},
		{
			name:     "medium-high priority (2)",
			priority: PriorityMediumHigh, // 2
			expected: "🟠",
		},
		{
			name:     "medium priority (3)",
			priority: PriorityMedium, // 3
			expected: "🟡",
		},
		{
			name:     "medium-low priority (4)",
			priority: PriorityMediumLow, // 4
			expected: "🔵",
		},
		{
			name:     "low priority (5)",
			priority: PriorityLow, // 5
			expected: "🟢",
		},
		{
			name:     "below min shows high",
			priority: 0,
			expected: "🔴",
		},
		{
			name:     "above max shows low",
			priority: 99,
			expected: "🟢",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PriorityLabel(tt.priority)
			if result != tt.expected {
				t.Errorf("PriorityLabel(%d) = %q, want %q", tt.priority, result, tt.expected)
			}
		})
	}
}

func TestNormalizePriority(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "high word",
			input:    "high",
			expected: PriorityHigh, // 1
		},
		{
			name:     "medium-high word",
			input:    "medium-high",
			expected: PriorityMediumHigh, // 2
		},
		{
			name:     "medium word",
			input:    "medium",
			expected: PriorityMedium, // 3
		},
		{
			name:     "low word",
			input:    "low",
			expected: PriorityLow, // 5
		},
		{
			name:     "uppercase",
			input:    "HIGH",
			expected: PriorityHigh, // 1
		},
		{
			name:     "mixed case",
			input:    "Medium-High",
			expected: PriorityMediumHigh, // 2
		},
		{
			name:     "underscore separator",
			input:    "medium_high",
			expected: PriorityMediumHigh, // 2
		},
		{
			name:     "space separator",
			input:    "medium high",
			expected: PriorityMediumHigh, // 2
		},
		{
			name:     "numeric string in range",
			input:    "4",
			expected: 4,
		},
		{
			name:     "numeric string zero clamps to 1",
			input:    "0",
			expected: 1,
		},
		{
			name:     "numeric string above max clamps to 5",
			input:    "10",
			expected: 5,
		},
		{
			name:     "invalid word",
			input:    "invalid",
			expected: -1,
		},
		{
			name:     "empty string",
			input:    "",
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePriority(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizePriority(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

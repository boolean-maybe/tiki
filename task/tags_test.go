package task

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTagsValue_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
		wantErr  bool
	}{
		// Valid scenarios
		{
			name:     "empty tags (omitted)",
			yaml:     "other: value",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "empty array",
			yaml:     "tags: []",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "single tag",
			yaml:     "tags: [frontend]",
			expected: []string{"frontend"},
			wantErr:  false,
		},
		{
			name:     "multiple tags",
			yaml:     "tags:\n  - frontend\n  - backend\n  - urgent",
			expected: []string{"frontend", "backend", "urgent"},
			wantErr:  false,
		},
		{
			name:     "tags with hyphens",
			yaml:     "tags: [tech-debt, bug-fix]",
			expected: []string{"tech-debt", "bug-fix"},
			wantErr:  false,
		},
		{
			name:     "tags with spaces (quoted)",
			yaml:     `tags: ["label with spaces", "another label"]`,
			expected: []string{"label with spaces", "another label"},
			wantErr:  false,
		},
		{
			name:     "tags with whitespace",
			yaml:     "tags:\n  - frontend\n  -  backend  \n  - urgent",
			expected: []string{"frontend", "backend", "urgent"},
			wantErr:  false,
		},
		{
			name:     "filter empty strings",
			yaml:     "tags: [frontend, '', backend]",
			expected: []string{"frontend", "backend"},
			wantErr:  false,
		},
		{
			name:     "filter whitespace-only strings",
			yaml:     "tags: [frontend, '  ', backend]",
			expected: []string{"frontend", "backend"},
			wantErr:  false,
		},

		// Invalid scenarios - should default to empty with no error
		{
			name:     "scalar string instead of list",
			yaml:     "tags: not-a-list",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "number instead of list",
			yaml:     "tags: 123",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "boolean instead of list",
			yaml:     "tags: true",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "object instead of list",
			yaml:     "tags:\n  key: value",
			expected: []string{},
			wantErr:  false,
		},
		{
			name:     "null value",
			yaml:     "tags: null",
			expected: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Tags TagsValue `yaml:"tags,omitempty"`
			}

			var result testStruct
			err := yaml.Unmarshal([]byte(tt.yaml), &result)

			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			got := result.Tags.ToStringSlice()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("UnmarshalYAML() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestTagsValue_MarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		tags     TagsValue
		expected string
	}{
		{
			name:     "empty tags",
			tags:     TagsValue([]string{}),
			expected: " []\n",
		},
		{
			name:     "single tag",
			tags:     TagsValue([]string{"frontend"}),
			expected: "\n    - frontend\n",
		},
		{
			name:     "multiple tags",
			tags:     TagsValue([]string{"frontend", "backend", "urgent"}),
			expected: "\n    - frontend\n    - backend\n    - urgent\n",
		},
		{
			name:     "tags with special characters",
			tags:     TagsValue([]string{"tech-debt", "bug-fix"}),
			expected: "\n    - tech-debt\n    - bug-fix\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Tags TagsValue `yaml:"tags"`
			}

			input := testStruct{Tags: tt.tags}
			got, err := yaml.Marshal(input)
			if err != nil {
				t.Fatalf("MarshalYAML() error = %v", err)
			}

			expected := "tags:" + tt.expected
			if string(got) != expected {
				t.Errorf("MarshalYAML() got = %q, expected %q", string(got), expected)
			}
		})
	}
}

func TestTagsValue_ToStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		tags     TagsValue
		expected []string
	}{
		{
			name:     "nil tags",
			tags:     nil,
			expected: []string{},
		},
		{
			name:     "empty tags",
			tags:     TagsValue([]string{}),
			expected: []string{},
		},
		{
			name:     "single tag",
			tags:     TagsValue([]string{"frontend"}),
			expected: []string{"frontend"},
		},
		{
			name:     "multiple tags",
			tags:     TagsValue([]string{"frontend", "backend", "urgent"}),
			expected: []string{"frontend", "backend", "urgent"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tags.ToStringSlice()
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ToStringSlice() got = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestTagsValue_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tags []string
	}{
		{
			name: "empty tags",
			tags: []string{},
		},
		{
			name: "single tag",
			tags: []string{"frontend"},
		},
		{
			name: "multiple tags",
			tags: []string{"frontend", "backend", "urgent"},
		},
		{
			name: "tags with special characters",
			tags: []string{"tech-debt", "bug-fix", "high-priority"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type testStruct struct {
				Tags TagsValue `yaml:"tags"`
			}

			// Marshal
			input := testStruct{Tags: TagsValue(tt.tags)}
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
			got := output.Tags.ToStringSlice()
			if !reflect.DeepEqual(got, tt.tags) {
				t.Errorf("Round trip failed: got = %v, expected %v", got, tt.tags)
			}
		})
	}
}

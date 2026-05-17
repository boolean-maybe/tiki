package store

import (
	"testing"

	"github.com/boolean-maybe/tiki/internal/teststatuses"
)

func init() {
	teststatuses.Init()
}

func TestParseFrontmatter(t *testing.T) {
	tests := []struct {
		name                string
		input               string
		expectedFrontmatter string
		expectedBody        string
		expectError         bool
	}{
		{
			name: "valid frontmatter with all fields",
			input: `---
id: ABC123
title: Test Tiki
type: story
status: todo
---
Tiki description here`,
			expectedFrontmatter: `id: ABC123
title: Test Tiki
type: story
status: todo`,
			expectedBody: "Tiki description here",
			expectError:  false,
		},
		{
			name: "valid frontmatter with body containing markdown",
			input: `---
id: ABC123
title: Bug Fix
type: bug
status: in_progress
---
## Description
This is a **bold** bug.`,
			expectedFrontmatter: `id: ABC123
title: Bug Fix
type: bug
status: in_progress`,
			expectedBody: `## Description
This is a **bold** bug.`,
			expectError: false,
		},
		{
			name: "missing closing delimiter",
			input: `---
id: ABC123
title: Incomplete
status: todo
This should fail`,
			expectedFrontmatter: "",
			expectedBody:        "",
			expectError:         true,
		},
		{
			name:                "no frontmatter - plain markdown",
			input:               "Just plain text without frontmatter",
			expectedFrontmatter: "",
			expectedBody:        "Just plain text without frontmatter",
			expectError:         false,
		},
		{
			name: "empty frontmatter",
			input: `---
id: ABC123
---
Body text here`,
			expectedFrontmatter: "id: ABC123",
			expectedBody:        "Body text here",
			expectError:         false,
		},
		{
			name: "frontmatter with extra whitespace",
			input: `---
id: ABC123
title: Whitespace Test
---

Body with leading newline`,
			expectedFrontmatter: `id: ABC123
title: Whitespace Test`,
			expectedBody: "\nBody with leading newline",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frontmatter, body, err := ParseFrontmatter(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if frontmatter != tt.expectedFrontmatter {
				t.Errorf("frontmatter = %q, want %q", frontmatter, tt.expectedFrontmatter)
			}

			if body != tt.expectedBody {
				t.Errorf("body = %q, want %q", body, tt.expectedBody)
			}
		})
	}
}

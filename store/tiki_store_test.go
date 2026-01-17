package store

import (
	"testing"

	taskpkg "github.com/boolean-maybe/tiki/task"
)

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
id: TIKI-1
title: Test Task
type: story
status: todo
---
Task description here`,
			expectedFrontmatter: `id: TIKI-1
title: Test Task
type: story
status: todo`,
			expectedBody: "Task description here",
			expectError:  false,
		},
		{
			name: "valid frontmatter with body containing markdown",
			input: `---
id: TIKI-2
title: Bug Fix
type: bug
status: in_progress
---
## Description
This is a **bold** bug.`,
			expectedFrontmatter: `id: TIKI-2
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
id: TIKI-3
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
---
Body text here`,
			expectedFrontmatter: "",
			expectedBody:        "Body text here",
			expectError:         false,
		},
		{
			name: "frontmatter with extra whitespace",
			input: `---
id: TIKI-4
title: Whitespace Test
---

Body with leading newline`,
			expectedFrontmatter: `id: TIKI-4
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

func TestMapStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected taskpkg.Status
	}{
		// Valid statuses - exact match
		{name: "backlog", input: "backlog", expected: taskpkg.StatusBacklog},
		{name: "todo", input: "todo", expected: taskpkg.StatusTodo},
		{name: "ready", input: "ready", expected: taskpkg.StatusReady},
		{name: "in_progress", input: "in_progress", expected: taskpkg.StatusInProgress},
		{name: "waiting", input: "waiting", expected: taskpkg.StatusWaiting},
		{name: "blocked", input: "blocked", expected: taskpkg.StatusBlocked},
		{name: "review", input: "review", expected: taskpkg.StatusReview},
		{name: "done", input: "done", expected: taskpkg.StatusDone},

		// Case variations
		{name: "BACKLOG uppercase", input: "BACKLOG", expected: taskpkg.StatusBacklog},
		{name: "ToDo mixed case", input: "ToDo", expected: taskpkg.StatusTodo},
		{name: "DONE uppercase", input: "DONE", expected: taskpkg.StatusDone},

		// Aliases and variants
		{name: "open -> todo", input: "open", expected: taskpkg.StatusTodo},
		{name: "in process -> in_progress", input: "in process", expected: taskpkg.StatusInProgress},
		{name: "closed -> done", input: "closed", expected: taskpkg.StatusDone},
		{name: "completed -> done", input: "completed", expected: taskpkg.StatusDone},

		// in_progress variations
		{name: "in-progress hyphenated", input: "in-progress", expected: taskpkg.StatusInProgress},
		{name: "inprogress no separator", input: "inprogress", expected: taskpkg.StatusInProgress},
		{name: "in progress spaces", input: "in progress", expected: taskpkg.StatusInProgress},
		{name: "In-Progress mixed case", input: "In-Progress", expected: taskpkg.StatusInProgress},

		// review variations
		{name: "in_review", input: "in_review", expected: taskpkg.StatusReview},
		{name: "in review", input: "in review", expected: taskpkg.StatusReview},

		// Unknown status defaults to backlog
		{name: "unknown status", input: "unknown", expected: taskpkg.StatusBacklog},
		{name: "empty string", input: "", expected: taskpkg.StatusBacklog},
		{name: "random text", input: "foobar", expected: taskpkg.StatusBacklog},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := taskpkg.MapStatus(tt.input)
			if result != tt.expected {
				t.Errorf("mapStatus(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

package store

import (
	"testing"

	"github.com/boolean-maybe/tiki/config"
	taskpkg "github.com/boolean-maybe/tiki/task"
	"github.com/boolean-maybe/tiki/workflow"
)

func init() {
	// set up the default status registry for tests.
	config.ResetStatusRegistry([]workflow.StatusDef{
		{Key: "backlog", Label: "Backlog", Emoji: "📥", Default: true},
		{Key: "ready", Label: "Ready", Emoji: "📋", Active: true},
		{Key: "inProgress", Label: "In Progress", Emoji: "⚙️", Active: true},
		{Key: "review", Label: "Review", Emoji: "👀", Active: true},
		{Key: "done", Label: "Done", Emoji: "✅", Done: true},
	})
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
title: Test Task
type: story
status: todo
---
Task description here`,
			expectedFrontmatter: `id: ABC123
title: Test Task
type: story
status: todo`,
			expectedBody: "Task description here",
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

func TestMapStatus(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected taskpkg.Status
	}{
		// Valid statuses - exact match
		{name: "backlog", input: "backlog", expected: taskpkg.StatusBacklog},
		{name: "ready", input: "ready", expected: taskpkg.StatusReady},
		{name: "inProgress", input: "inProgress", expected: taskpkg.StatusInProgress},
		{name: "review", input: "review", expected: taskpkg.StatusReview},
		{name: "done", input: "done", expected: taskpkg.StatusDone},

		// Case variations (normalization still works)
		{name: "BACKLOG uppercase", input: "BACKLOG", expected: taskpkg.StatusBacklog},
		{name: "DONE uppercase", input: "DONE", expected: taskpkg.StatusDone},

		// Separator normalization (hyphens/spaces → underscores)
		{name: "in-progress hyphenated", input: "in-progress", expected: taskpkg.StatusInProgress},
		{name: "in progress spaces", input: "in progress", expected: taskpkg.StatusInProgress},
		{name: "In-Progress mixed case", input: "In-Progress", expected: taskpkg.StatusInProgress},

		// Unknown status defaults to configured default (backlog)
		{name: "unknown status", input: "unknown", expected: taskpkg.StatusBacklog},
		{name: "empty string", input: "", expected: taskpkg.StatusBacklog},
		{name: "random text", input: "foobar", expected: taskpkg.StatusBacklog},
		// Aliases no longer supported — these now map to default
		{name: "todo (no alias)", input: "todo", expected: taskpkg.StatusBacklog},
		{name: "closed (no alias)", input: "closed", expected: taskpkg.StatusBacklog},
		{name: "completed (no alias)", input: "completed", expected: taskpkg.StatusBacklog},
		{name: "open (no alias)", input: "open", expected: taskpkg.StatusBacklog},
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

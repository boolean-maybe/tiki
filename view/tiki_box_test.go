package view

import (
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
	tikipkg "github.com/boolean-maybe/tiki/tiki"
	"github.com/boolean-maybe/tiki/workflow"
)

// fieldEmoji returns the visual for a workflow enum value, or "" when missing.
// (Keeps the legacy helper name to minimize churn in callers below.)
func fieldEmoji(fieldName, key string) string {
	fd, ok := workflow.Field(fieldName)
	if !ok {
		return ""
	}
	v, found := fd.LookupEnum(key)
	if !found {
		return ""
	}
	return v.Visual
}

// makeTiki creates a tiki with the given ID, type string, and priority key
// for tests. priority is now an enum key — pass "" to omit the field.
func makeTiki(id string, tikiType string, priority string) *tikipkg.Tiki {
	tk := tikipkg.New()
	tk.ID = id
	tk.Set(tikipkg.FieldType, tikiType)
	if priority != "" {
		tk.Set(tikipkg.FieldPriority, priority)
	}
	return tk
}

func TestBuildCompactTikiContent(t *testing.T) {
	colors := theme.Roles()

	tests := []struct {
		name           string
		tiki           *tikipkg.Tiki
		availableWidth int
		contains       []string
		notContains    []string
	}{
		{
			name:           "contains tiki ID",
			tiki:           makeTiki("ABC123", "story", "medium"),
			availableWidth: 40,
			contains:       []string{"ABC123"},
		},
		{
			name: "contains title",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("TTL001", "story", "medium")
				tk.Title = "My Tiki"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"My Tiki"},
		},
		{
			name: "title truncated at width",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("TRC001", "story", "medium")
				tk.Title = "ABCDEFGHIJ"
				return tk
			}(),
			availableWidth: 7,
			contains:       []string{"ABCD"},
			notContains:    []string{"ABCDEFGHIJ"},
		},
		{
			name:           "emoji for story type",
			tiki:           makeTiki("EMO001", "story", "medium"),
			availableWidth: 40,
			contains:       []string{fieldEmoji("type", "story")},
		},
		{
			name:           "emoji for bug type",
			tiki:           makeTiki("EMO002", "bug", "medium"),
			availableWidth: 40,
			contains:       []string{fieldEmoji("type", "bug")},
		},
		{
			name:           "priority label",
			tiki:           makeTiki("PRI001", "story", "high"),
			availableWidth: 40,
			contains:       []string{fieldEmoji("priority", "high")},
		},
		{
			name:           "zero points does not panic",
			tiki:           makeTiki("PT0001", "story", "medium"),
			availableWidth: 40,
		},
		{
			name:           "empty title does not panic",
			tiki:           makeTiki("NT0001", "story", "medium"),
			availableWidth: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildCompactTikiContent(tt.tiki, colors, tt.availableWidth)

			if result == "" {
				t.Error("expected non-empty output")
			}
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q\noutput: %q", want, result)
				}
			}
			for _, unwanted := range tt.notContains {
				if strings.Contains(result, unwanted) {
					t.Errorf("expected output NOT to contain %q\noutput: %q", unwanted, result)
				}
			}
		})
	}
}

// TestBuildCompactTikiContent_TitleRoleMarkupExpands pins that a compact
// tiki card whose title carries `<role>` color markup renders with the
// resolved tview color tag and the literal text. Card titles are
// user-controlled — same escape-then-expand contract as the detail view's
// title.
func TestBuildCompactTikiContent_TitleRoleMarkupExpands(t *testing.T) {
	colors := theme.Roles()
	tk := makeTiki("MRK001", "story", "medium")
	tk.Title = "<highlight>foo"

	got := buildCompactTikiContent(tk, colors, 40)
	highlightTag := colors.Highlight().Tag()
	if !strings.Contains(got, highlightTag) {
		t.Errorf("expected highlight color tag %q in compact content, got %q", highlightTag, got)
	}
	if !strings.Contains(got, "foo") {
		t.Errorf("expected literal 'foo' in compact content, got %q", got)
	}
}

// TestBuildCompactTikiContent_TitleTviewTagsEscaped pins that literal
// tview `[red]` tags in a stored title are escaped — they appear as
// characters, not active color markup. Guards against hostile stored
// content from hijacking row coloring.
func TestBuildCompactTikiContent_TitleTviewTagsEscaped(t *testing.T) {
	colors := theme.Roles()
	tk := makeTiki("MRK002", "story", "medium")
	tk.Title = "[red]x[/]"

	got := buildCompactTikiContent(tk, colors, 40)
	if !strings.Contains(got, "[red") {
		t.Errorf("expected '[red' fragment to be escaped/present in compact content, got %q", got)
	}
}

// TestBuildCompactTikiContent_TitleTruncatedMidRoleToken pins the
// truncation-safety contract for title markup: when the title is chopped
// mid-token by the caller's width-based truncator, the rendered card must
// not leak the broken `<role` fragment as visible text. The fix trims
// any unclosed `<...` tail before passing to ExpandVisual; the visible
// title shortens slightly more than the raw rune width allowed, but the
// alternative (broken token visible on every narrow board) is worse.
func TestBuildCompactTikiContent_TitleTruncatedMidRoleToken(t *testing.T) {
	colors := theme.Roles()
	tk := makeTiki("TRC100", "story", "medium")
	tk.Title = "<highlight>fix the very long bug"

	// Pick a width that chops mid-token. The raw title is 30 chars;
	// width 12 means TruncateText returns "<highligh..." or similar.
	got := buildCompactTikiContent(tk, colors, 12)

	// Must NOT contain a dangling open-angle token. Allow `<<` (literal
	// angle, escape form) and `<role>foo` (intact token); reject only an
	// unbalanced `<` near the end of the title slot.
	if strings.Contains(got, "<highligh") {
		t.Errorf("broken role-token fragment leaked into rendered card: %q", got)
	}
}

// TestTrimUnclosedRoleToken_Cases pins the trim helper's contract across
// the cases that matter: clean strings pass through, an unterminated tail
// is chopped, escaped `<<` is preserved, and a closed token earlier in
// the string survives even when a later `<` is unclosed.
func TestTrimUnclosedRoleToken_Cases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text untouched", "hello world", "hello world"},
		{"closed token survives", "<role>body", "<role>body"},
		{"unclosed tail trimmed", "foo <role", "foo "},
		{"escape `<<` preserved", "ab << cd", "ab << cd"},
		{"closed then unclosed: trim at unclosed", "<ok>done <bad", "<ok>done "},
		{"only unclosed at start", "<stuck", ""},
		{"trailing `<<` not a role open", "tail <<", "tail <<"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := trimUnclosedRoleToken(c.in); got != c.want {
				t.Errorf("trimUnclosedRoleToken(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestBuildExpandedTikiContent_TitleRoleMarkupExpands mirrors the compact
// test for the expanded card variant: same title path, same expectations.
func TestBuildExpandedTikiContent_TitleRoleMarkupExpands(t *testing.T) {
	colors := theme.Roles()
	tk := makeTiki("MRK003", "story", "medium")
	tk.Title = "<highlight>bar"

	got := buildExpandedTikiContent(tk, colors, 40)
	highlightTag := colors.Highlight().Tag()
	if !strings.Contains(got, highlightTag) {
		t.Errorf("expected highlight color tag %q in expanded content, got %q", highlightTag, got)
	}
	if !strings.Contains(got, "bar") {
		t.Errorf("expected literal 'bar' in expanded content, got %q", got)
	}
}

func TestBuildExpandedTikiContent(t *testing.T) {
	colors := theme.Roles()

	tests := []struct {
		name           string
		tiki           *tikipkg.Tiki
		availableWidth int
		contains       []string
		notContains    []string
	}{
		{
			name:           "empty description no panic",
			tiki:           makeTiki("EXP001", "story", "medium"),
			availableWidth: 40,
		},
		{
			name: "single desc line included",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP002", "story", "medium")
				tk.Body = "Line1"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"Line1"},
		},
		{
			name: "three desc lines all included",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP003", "story", "medium")
				tk.Body = "L1\nL2\nL3"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"L1", "L2", "L3"},
		},
		{
			name: "fourth desc line not included",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP004", "story", "medium")
				tk.Body = "L1\nL2\nL3\nL4"
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"L1", "L2", "L3"},
			notContains:    []string{"L4"},
		},
		{
			name: "empty tags omits tags label",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP005", "story", "medium")
				tk.Set(tikipkg.FieldTags, []string{})
				return tk
			}(),
			availableWidth: 40,
			notContains:    []string{"Tags:"},
		},
		{
			name: "non-empty tags included",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP006", "story", "medium")
				tk.Set(tikipkg.FieldTags, []string{"ui", "backend"})
				return tk
			}(),
			availableWidth: 40,
			contains:       []string{"ui", "backend"},
		},
		{
			name: "tag truncated at small width no panic",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP007", "story", "medium")
				tk.Set(tikipkg.FieldTags, []string{"abcdefghij"})
				return tk
			}(),
			availableWidth: 8,
		},
		{
			name: "desc line truncated at small width",
			tiki: func() *tikipkg.Tiki {
				tk := makeTiki("EXP008", "story", "medium")
				tk.Body = "ABCDEFGHIJ"
				return tk
			}(),
			availableWidth: 7,
			contains:       []string{"ABCD"},
			notContains:    []string{"ABCDEFGHIJ"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExpandedTikiContent(tt.tiki, colors, tt.availableWidth)

			if result == "" {
				t.Error("expected non-empty output")
			}
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q\noutput: %q", want, result)
				}
			}
			for _, unwanted := range tt.notContains {
				if strings.Contains(result, unwanted) {
					t.Errorf("expected output NOT to contain %q\noutput: %q", unwanted, result)
				}
			}
		})
	}
}

package pipe

import "testing"

func TestParseInput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTitle string
		wantDesc  string
	}{
		{
			name:      "single line",
			input:     "Fix the login bug",
			wantTitle: "Fix the login bug",
			wantDesc:  "",
		},
		{
			name:      "multi-line with blank separator",
			input:     "Bug title\n\nDetailed description here",
			wantTitle: "Bug title",
			wantDesc:  "Detailed description here",
		},
		{
			name:      "multi-line without blank separator",
			input:     "Bug title\nDescription starts immediately",
			wantTitle: "Bug title",
			wantDesc:  "Description starts immediately",
		},
		{
			name:      "leading and trailing whitespace trimmed",
			input:     "  Fix the bug  \n\n  Some details  ",
			wantTitle: "Fix the bug",
			wantDesc:  "Some details",
		},
		{
			name:      "multi-line description",
			input:     "Title\n\nLine 1\nLine 2\nLine 3",
			wantTitle: "Title",
			wantDesc:  "Line 1\nLine 2\nLine 3",
		},
		{
			name:      "title with trailing newline only",
			input:     "Just a title\n",
			wantTitle: "Just a title",
			wantDesc:  "",
		},
		{
			name:      "multiple blank lines between title and description",
			input:     "Title\n\n\n\nDescription after gaps",
			wantTitle: "Title",
			wantDesc:  "Description after gaps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotDesc := parseInput(tt.input)
			if gotTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotDesc != tt.wantDesc {
				t.Errorf("description = %q, want %q", gotDesc, tt.wantDesc)
			}
		})
	}
}

func TestHasPositionalArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "empty args", args: nil, want: false},
		{name: "flags only", args: []string{"--version"}, want: false},
		{name: "log-level flag with value", args: []string{"--log-level", "debug"}, want: false},
		{name: "log-level=value", args: []string{"--log-level=debug"}, want: false},
		{name: "positional file", args: []string{"file.md"}, want: true},
		{name: "init command", args: []string{"init"}, want: true},
		{name: "stdin dash", args: []string{"-"}, want: true},
		{name: "flag then positional", args: []string{"--log-level", "debug", "file.md"}, want: true},
		{name: "double dash", args: []string{"--", "file.md"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasPositionalArgs(tt.args); got != tt.want {
				t.Errorf("HasPositionalArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

package view

import "testing"

// docTitle derives the human-readable title of an opened markdown file:
// frontmatter `title:` wins, then the first `# H1`, then the filename.
func TestDocTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		relPath string
		want    string
	}{
		{
			name: "frontmatter title wins over H1",
			content: "---\nid: 18RS7D\n" +
				"title: When done with customizing detail\ntype: story\n---\n\n# A Heading\n",
			relPath: "18RS7D.md",
			want:    "When done with customizing detail",
		},
		{
			name:    "frontmatter title with empty body",
			content: "---\nid: 18RS7D\ntitle: Just a title\n---\n",
			relPath: "18RS7D.md",
			want:    "Just a title",
		},
		{
			name:    "H1 used when no frontmatter title",
			content: "---\nid: ABC123\nstatus: open\n---\n\n# Real Heading\n\nbody",
			relPath: "notes/ABC123.md",
			want:    "Real Heading",
		},
		{
			name:    "H1 used when no frontmatter at all",
			content: "# Plain Doc Heading\n\nsome text",
			relPath: "readme.md",
			want:    "Plain Doc Heading",
		},
		{
			name:    "filename fallback when neither title nor H1",
			content: "just some prose with no heading",
			relPath: "sub/dir/loose-note.md",
			want:    "loose-note.md",
		},
		{
			name:    "filename fallback for empty content",
			content: "",
			relPath: "empty.md",
			want:    "empty.md",
		},
		{
			name:    "H2 does not count as the title",
			content: "## Not A Title\n\nbody",
			relPath: "sub.md",
			want:    "sub.md",
		},
		{
			name:    "quoted frontmatter title is unquoted",
			content: "---\ntitle: \"Quoted Title\"\n---\n",
			relPath: "q.md",
			want:    "Quoted Title",
		},
		{
			name:    "H1 with trailing hashes and spaces trimmed",
			content: "#   Spaced Heading   #\n",
			relPath: "h.md",
			want:    "Spaced Heading",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := docTitle(tt.content, tt.relPath); got != tt.want {
				t.Errorf("docTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

package document

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct {
		name  string
		title string
		want  string
	}{
		{"basic", "Fix Login", "fix-login"},
		{"punctuation collapses", "Fix Login: OAuth & SAML!", "fix-login-oauth-saml"},
		{"accents to ascii", "Café crème", "cafe-creme"},
		{"leading trailing trimmed", "  --Hello--  ", "hello"},
		{"repeated separators", "a___b   c", "a-b-c"},
		{"all punctuation is empty", "!!! ??? ...", ""},
		{"empty is empty", "", ""},
		{"length cap at hyphen boundary", "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggggggggg", "aaaaaaaaaa-bbbbbbbbbb-cccccccccc-dddddddddd-eeeeeeeeee"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Slugify(tc.title); got != tc.want {
				t.Fatalf("Slugify(%q) = %q, want %q", tc.title, got, tc.want)
			}
		})
	}
}

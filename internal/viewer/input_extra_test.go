package viewer

import (
	"errors"
	"testing"
)

// --- GitHub URL variants ---

func TestParseViewerInputGitHubBlobURL(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"github.com/owner/repo/blob/main/docs/README.md"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	if spec.Kind != InputGitHub {
		t.Fatalf("expected github input, got %s", spec.Kind)
	}
	want := "https://raw.githubusercontent.com/owner/repo/main/docs/README.md"
	if len(spec.Candidates) != 1 || spec.Candidates[0] != want {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
}

func TestParseViewerInputGitHubTreeURL(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"github.com/owner/repo/tree/v1.2.3"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	// tree with no file path defaults to README.md
	want := "https://raw.githubusercontent.com/owner/repo/v1.2.3/README.md"
	if len(spec.Candidates) != 1 || spec.Candidates[0] != want {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
}

func TestParseViewerInputGitHubHTTPS(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"https://github.com/owner/repo"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	// no explicit ref → two fallback candidates
	if len(spec.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d: %v", len(spec.Candidates), spec.Candidates)
	}
}

func TestParseViewerInputGitHubDotGitStripped(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"github.com/owner/repo.git"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	for _, c := range spec.Candidates {
		if len(c) > 0 && c[len(c)-4:] == ".git" {
			t.Fatalf(".git suffix not stripped from candidate: %s", c)
		}
	}
	// candidates should reference "repo" not "repo.git"
	want0 := "https://raw.githubusercontent.com/owner/repo/main/README.md"
	if len(spec.Candidates) < 1 || spec.Candidates[0] != want0 {
		t.Fatalf("unexpected first candidate: %v", spec.Candidates)
	}
}

func TestParseViewerInputGitHubSingleSegmentFallsThrough(t *testing.T) {
	// only one path segment — not a valid github repo URL
	spec, ok, err := ParseViewerInput(
		[]string{"github.com/owner"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode (treated as file path)")
	}
	if spec.Kind != InputFile {
		t.Fatalf("single-segment github path should fall through to file, got %s", spec.Kind)
	}
}

// --- GitLab URL variants ---

func TestParseViewerInputGitLabBasic(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"gitlab.com/group/repo"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	if spec.Kind != InputGitLab {
		t.Fatalf("expected gitlab input, got %s", spec.Kind)
	}
	if len(spec.Candidates) != 2 {
		t.Fatalf("expected 2 fallback candidates, got %d", len(spec.Candidates))
	}
}

func TestParseViewerInputGitLabBlobURL(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"gitlab.com/group/repo/-/blob/main/README.md"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	want := "https://gitlab.com/group/repo/-/raw/main/README.md"
	if len(spec.Candidates) != 1 || spec.Candidates[0] != want {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
}

func TestParseViewerInputGitLabRawWithSubgroup(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"gitlab.com/group/subgroup/repo/-/raw/v2/docs/guide.md"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	want := "https://gitlab.com/group/subgroup/repo/-/raw/v2/docs/guide.md"
	if len(spec.Candidates) != 1 || spec.Candidates[0] != want {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
}

func TestParseViewerInputGitLabHTTPS(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"https://gitlab.com/owner/repo"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode")
	}
	if spec.Kind != InputGitLab {
		t.Fatalf("expected gitlab input, got %s", spec.Kind)
	}
}

func TestParseViewerInputGitLabSingleSegmentFallsThrough(t *testing.T) {
	spec, ok, err := ParseViewerInput(
		[]string{"gitlab.com/singlegroup"},
		map[string]struct{}{},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected viewer mode (file path fallthrough)")
	}
	if spec.Kind != InputFile {
		t.Fatalf("single-segment gitlab path should fall through to file, got %s", spec.Kind)
	}
}

// --- collectPositionalArgs edge cases ---

func TestCollectPositionalArgsDoubleDashTerminator(t *testing.T) {
	got, err := collectPositionalArgs([]string{"--", "file.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "file.md" {
		t.Fatalf("expected [file.md], got %v", got)
	}
}

func TestCollectPositionalArgsLogLevelInlineForm(t *testing.T) {
	got, err := collectPositionalArgs([]string{"--log-level=debug", "file.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "file.md" {
		t.Fatalf("expected [file.md], got %v", got)
	}
}

func TestCollectPositionalArgsVersionFlag(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		got, err := collectPositionalArgs([]string{flag, "file.md"})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", flag, err)
		}
		if len(got) != 1 || got[0] != "file.md" {
			t.Fatalf("%s: expected [file.md], got %v", flag, got)
		}
	}
}

func TestCollectPositionalArgsHelpFlag(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		got, err := collectPositionalArgs([]string{flag, "file.md"})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", flag, err)
		}
		if len(got) != 1 || got[0] != "file.md" {
			t.Fatalf("%s: expected [file.md], got %v", flag, got)
		}
	}
}

func TestCollectPositionalArgsLogLevelInvalidValueNotSkipped(t *testing.T) {
	// --log-level followed by non-log-level value — next arg should NOT be skipped
	got, err := collectPositionalArgs([]string{"--log-level", "notavalidlevel", "file.md"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "notavalidlevel" is not a valid log level so it is treated as a positional arg
	if len(got) != 2 {
		t.Fatalf("expected 2 positional args, got %v", got)
	}
}

// --- internal helpers ---

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"/a/b/", []string{"a", "b"}},
		{"a/b/c", []string{"a", "b", "c"}},
		{"/", nil},
	}
	for _, tt := range tests {
		got := splitPath(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitPath(%q) = %v, want %v", tt.input, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestTrimGitSuffix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"repo.git", "repo"},
		{"repo", "repo"},
		{"my.repo.git", "my.repo"},
		{"", ""},
	}
	for _, tt := range tests {
		got := trimGitSuffix(tt.input)
		if got != tt.want {
			t.Errorf("trimGitSuffix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseRefAndPath(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		wantRef  string
		wantPath string
	}{
		{"empty segments", []string{}, "", ""},
		{"blob with ref and path", []string{"blob", "main", "a/b.md"}, "main", "a/b.md"},
		{"tree with ref only", []string{"tree", "v1"}, "v1", ""},
		{"non-blob-tree segments joined as path", []string{"a", "b"}, "", "a/b"},
		{"single non-keyword segment", []string{"docs"}, "", "docs"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRef, gotPath := parseRefAndPath(tt.segments)
			if gotRef != tt.wantRef || gotPath != tt.wantPath {
				t.Errorf("parseRefAndPath(%v) = (%q, %q), want (%q, %q)",
					tt.segments, gotRef, gotPath, tt.wantRef, tt.wantPath)
			}
		})
	}
}

func TestParseGitLabSegments(t *testing.T) {
	tests := []struct {
		name          string
		segments      []string
		wantRepoIndex int
		wantRef       string
		wantPath      string
	}{
		{
			name:          "no dash returns last index as repo",
			segments:      []string{"group", "repo"},
			wantRepoIndex: 1,
			wantRef:       "",
			wantPath:      "",
		},
		{
			name:          "dash at index 0 invalid",
			segments:      []string{"-", "blob", "main"},
			wantRepoIndex: -1,
			wantRef:       "",
			wantPath:      "",
		},
		{
			name:          "standard blob layout",
			segments:      []string{"group", "repo", "-", "blob", "main", "README.md"},
			wantRepoIndex: 1,
			wantRef:       "main",
			wantPath:      "README.md",
		},
		{
			name:          "subgroup namespace",
			segments:      []string{"group", "subgroup", "repo", "-", "raw", "v2", "docs/guide.md"},
			wantRepoIndex: 2,
			wantRef:       "v2",
			wantPath:      "docs/guide.md",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotRef, gotPath := parseGitLabSegments(tt.segments)
			if gotIndex != tt.wantRepoIndex || gotRef != tt.wantRef || gotPath != tt.wantPath {
				t.Errorf("parseGitLabSegments(%v) = (%d, %q, %q), want (%d, %q, %q)",
					tt.segments, gotIndex, gotRef, gotPath,
					tt.wantRepoIndex, tt.wantRef, tt.wantPath)
			}
		})
	}
}

func TestCollectPositionalArgsUnknownFlag(t *testing.T) {
	_, err := collectPositionalArgs([]string{"--unknown-flag"})
	if !errors.Is(err, ErrUnknownFlag) {
		t.Fatalf("expected ErrUnknownFlag, got %v", err)
	}
}

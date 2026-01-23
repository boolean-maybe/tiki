package viewer

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestParseViewerInputFile(t *testing.T) {
	spec, ok, err := ParseViewerInput([]string{"README.md"}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected viewer mode")
	}
	if spec.Kind != InputFile {
		t.Fatalf("expected file input, got %s", spec.Kind)
	}
	abs, err := filepath.Abs("README.md")
	if err != nil {
		t.Fatalf("abs path error: %v", err)
	}
	if len(spec.Candidates) != 1 || spec.Candidates[0] != abs {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
	if len(spec.SearchRoots) != 1 || spec.SearchRoots[0] != filepath.Dir(abs) {
		t.Fatalf("unexpected search roots: %v", spec.SearchRoots)
	}
}

func TestParseViewerInputStdin(t *testing.T) {
	spec, ok, err := ParseViewerInput([]string{"-"}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected viewer mode")
	}
	if spec.Kind != InputStdin {
		t.Fatalf("expected stdin input, got %s", spec.Kind)
	}
}

func TestParseViewerInputURL(t *testing.T) {
	spec, ok, err := ParseViewerInput([]string{"https://host.tld/file.md"}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected viewer mode")
	}
	if spec.Kind != InputURL {
		t.Fatalf("expected url input, got %s", spec.Kind)
	}
	if len(spec.Candidates) != 1 || spec.Candidates[0] != "https://host.tld/file.md" {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
}

func TestParseViewerInputGitHub(t *testing.T) {
	spec, ok, err := ParseViewerInput([]string{"github.com/boolean-maybe/tiki"}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected viewer mode")
	}
	if spec.Kind != InputGitHub {
		t.Fatalf("expected github input, got %s", spec.Kind)
	}
	expected := []string{
		"https://raw.githubusercontent.com/boolean-maybe/tiki/main/README.md",
		"https://raw.githubusercontent.com/boolean-maybe/tiki/master/README.md",
	}
	if len(spec.Candidates) != len(expected) {
		t.Fatalf("unexpected candidates: %v", spec.Candidates)
	}
	for i, candidate := range expected {
		if spec.Candidates[i] != candidate {
			t.Fatalf("unexpected candidate at %d: %s", i, spec.Candidates[i])
		}
	}
}

func TestParseViewerInputMultiple(t *testing.T) {
	_, ok, err := ParseViewerInput([]string{"README.md", "OTHER.md"}, map[string]struct{}{})
	if !errors.Is(err, ErrMultipleInputs) {
		t.Fatalf("expected multiple input error, got %v", err)
	}
	if ok {
		t.Fatalf("expected viewer mode to be false")
	}
}

func TestParseViewerInputFlags(t *testing.T) {
	spec, ok, err := ParseViewerInput([]string{"--log-level", "warn", "README.md"}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected viewer mode")
	}
	if spec.Kind != InputFile {
		t.Fatalf("expected file input, got %s", spec.Kind)
	}
}

func TestParseViewerInputLogLevelMissingValue(t *testing.T) {
	spec, ok, err := ParseViewerInput([]string{"--log-level", "README.md"}, map[string]struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected viewer mode")
	}
	if spec.Kind != InputFile {
		t.Fatalf("expected file input, got %s", spec.Kind)
	}
}

func TestParseViewerInputReserved(t *testing.T) {
	_, ok, err := ParseViewerInput([]string{"status"}, map[string]struct{}{"status": {}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected viewer mode to be false")
	}
}

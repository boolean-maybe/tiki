package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
)

// TestBootstrap_AllDigitIDSurvivesLoad verifies the M3 fix end-to-end: when
// the random-id generator happens to emit an all-digit id (e.g. "000001"),
// the sample writer must quote it in the YAML frontmatter so the next strict
// load parses it as a string, not an integer that drops leading zeros.
//
// Without the fix, yaml.v3 decodes `id: 000001` as int 1, FrontmatterID
// stringifies as "1", and ValidateID rejects it — the sample file is
// effectively unloadable by the very store that wrote it.
func TestBootstrap_AllDigitIDSurvivesLoad(t *testing.T) {
	repoDir := setupInitTest(t)

	// Force the generator to emit an all-digit id for the first sample and
	// recognizable letter ids for the rest (so repeated calls don't collide).
	call := 0
	prev := config.GenerateRandomIDForTest
	config.GenerateRandomIDForTest = func() string {
		call++
		if call == 1 {
			return "000001"
		}
		// fall back to fresh ids for subsequent samples.
		return document.NewID()
	}
	t.Cleanup(func() { config.GenerateRandomIDForTest = prev })

	code := runInit([]string{repoDir, "-n", "--samples"})
	if code != exitOK {
		t.Fatalf("runInit exit = %d, want %d", code, exitOK)
	}

	tikiDir := filepath.Join(repoDir, ".doc", "tiki")
	target := filepath.Join(tikiDir, "000001.md")
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("sample for all-digit id not written at expected path: %v", err)
	}

	// Must be quoted — guards against yaml.v3 serializing numerically.
	if !strings.Contains(string(content), `id: "000001"`) {
		t.Errorf("all-digit id was not quoted in frontmatter:\n%s", content)
	}

	// Round-trip via the document parser: id must come back as "000001".
	parsed, err := document.ParseFrontmatter(string(content))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	id, ok := document.FrontmatterID(parsed.Map)
	if !ok {
		t.Fatal("id missing after round-trip — numeric-id regression")
	}
	if id != "000001" {
		t.Errorf("id after round-trip = %q, want %q — leading zeros dropped", id, "000001")
	}
	if !document.IsValidID(id) {
		t.Errorf("round-tripped id %q fails strict validation", id)
	}
}

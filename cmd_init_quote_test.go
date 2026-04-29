package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boolean-maybe/tiki/config"
	"github.com/boolean-maybe/tiki/document"
	"github.com/boolean-maybe/tiki/store/tikistore"
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

	// Phase 2: samples land directly under .doc/, not .doc/tiki/.
	docDir := filepath.Join(repoDir, ".doc")
	target := filepath.Join(docDir, "000001.md")
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

// TestRunInit_FreshInitLoadsWithNoDiagnostics is the Phase 2 regression
// gate for project initialization: after `tiki init`, opening the freshly
// populated `.doc/` through the strict store must yield zero load
// diagnostics. This catches the case where a new bundled file type — like
// the plain-markdown doki templates that ship as `config/index.md` and
// `config/linked.md` — gets written without a frontmatter id and then
// fails the very loader that just initialized the project.
//
// Without this test the hole is invisible: init "succeeds", the next TUI
// launch silently logs rejections, and the user sees empty views. The
// assertion here is symmetric with the demo-loads-cleanly test — if ever
// a new bundled `.md` gets added to init without an id, both this test
// and demo loading would fail.
func TestRunInit_FreshInitLoadsWithNoDiagnostics(t *testing.T) {
	repoDir := setupInitTest(t)

	code := runInit([]string{repoDir, "-n", "--samples"})
	if code != exitOK {
		t.Fatalf("runInit exit = %d, want %d", code, exitOK)
	}

	docRoot := filepath.Join(repoDir, ".doc")
	store, err := tikistore.NewTikiStore(docRoot)
	if err != nil {
		t.Fatalf("NewTikiStore on initialized project: %v", err)
	}

	diag := store.LoadDiagnostics()
	if diag != nil && diag.HasIssues() {
		t.Fatalf("fresh init surfaced load diagnostics: %s", diag.Summary())
	}
}

package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/task"
)

// CreateTestTask creates a markdown task file with YAML frontmatter
func CreateTestTask(dir, id, title string, status task.Status, taskType task.Type) error {
	return CreateTestTaskWithDeps(dir, id, title, status, taskType, nil)
}

// CreateTestTaskWithDeps creates a markdown task file with optional dependsOn
// IDs in the frontmatter. Accepts either bare ("ABC123") or legacy-looking
// ("TIKI-ABC123") ids; the legacy prefix is stripped so the written file
// conforms to the Phase 1 strict-load contract (bare id in frontmatter,
// filename is <id>.md).
func CreateTestTaskWithDeps(dir, id, title string, status task.Status, taskType task.Type, dependsOn []string) error {
	bareID := normalizeBareID(id)
	filename := bareID + ".md"
	filePath := filepath.Join(dir, filename)

	depsYAML := ""
	if len(dependsOn) > 0 {
		depsYAML = "dependsOn:\n"
		for _, dep := range dependsOn {
			depsYAML += fmt.Sprintf("  - %s\n", normalizeBareID(dep))
		}
	}

	// Quote the id so YAML preserves leading zeros in all-numeric ids like
	// "000001". Without quotes, yaml.v3 decodes them as integers and loses
	// the padding — which then fails the strict-format validator at load.
	content := fmt.Sprintf(`---
id: %q
title: %s
type: %s
status: %s
priority: 3
points: 1
%s---
%s
`, bareID, title, taskType, status, depsYAML, title)

	return os.WriteFile(filePath, []byte(content), 0o644)
}

// ID returns the bare 6-character document id corresponding to a test-friendly
// shorthand like "TIKI-1" or "ABC". Accepts TIKI- prefix, any case, and any
// length ≤ 6, padding with leading zeros. Use this in both CreateTestTask
// calls and in GetTask/test assertions so keys match end-to-end.
func ID(raw string) string {
	return normalizeBareID(raw)
}

// normalizeBareID accepts a variety of test-friendly shorthand — bare ids,
// the pre-unification "TIKI-ABC123" form, hyphenated labels like "DELETE-1"
// — and returns a canonical bare 6-character id. The helpers are forgiving
// so we don't have to update every test call site in lockstep; production
// code remains strict.
//
// Rules, applied in order:
//  1. trim whitespace
//  2. strip leading "TIKI-" (case-insensitive)
//  3. strip any remaining hyphens ("DELETE-1" → "DELETE1")
//  4. upper-case
//  5. right-pad with 0 to 6 characters; truncate if longer
func normalizeBareID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(strings.ToUpper(id), "TIKI-") {
		id = id[len("TIKI-"):]
	}
	id = strings.ReplaceAll(id, "-", "")
	id = strings.ToUpper(id)
	const wantLen = 6
	if len(id) < wantLen {
		id = strings.Repeat("0", wantLen-len(id)) + id
	}
	if len(id) > wantLen {
		id = id[:wantLen]
	}
	return id
}

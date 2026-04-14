package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const settingsFile = ".claude/settings.local.json"

func TestEnsureSkillPermissions_CreatesFile(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := ensureSkillPermissions(settingsFile, []string{"tiki", "doki"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readSettings(t)
	allow := getAllow(t, got)

	if len(allow) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(allow), allow)
	}
	if allow[0] != "Skill(tiki)" || allow[1] != "Skill(doki)" {
		t.Errorf("unexpected entries: %v", allow)
	}
}

func TestEnsureSkillPermissions_MergesExisting(t *testing.T) {
	t.Chdir(t.TempDir())

	writeSettings(t, map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Bash(go test:*)"},
		},
	})

	if err := ensureSkillPermissions(settingsFile, []string{"tiki"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readSettings(t)
	allow := getAllow(t, got)

	if len(allow) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(allow), allow)
	}
	if allow[0] != "Bash(go test:*)" {
		t.Errorf("existing entry clobbered: %v", allow)
	}
	if allow[1] != "Skill(tiki)" {
		t.Errorf("new entry missing: %v", allow)
	}
}

func TestEnsureSkillPermissions_AlreadyPresent(t *testing.T) {
	t.Chdir(t.TempDir())

	writeSettings(t, map[string]any{
		"permissions": map[string]any{
			"allow": []any{"Skill(tiki)", "Skill(doki)"},
		},
	})

	before, _ := os.ReadFile(settingsFile)
	if err := ensureSkillPermissions(settingsFile, []string{"tiki", "doki"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after, _ := os.ReadFile(settingsFile)

	if string(before) != string(after) {
		t.Error("file was modified despite all skills already present")
	}
}

func TestEnsureSkillPermissions_PreservesOtherKeys(t *testing.T) {
	t.Chdir(t.TempDir())

	writeSettings(t, map[string]any{
		"outputStyle": "Explanatory",
		"permissions": map[string]any{
			"allow": []any{"Bash(go test:*)"},
		},
	})

	if err := ensureSkillPermissions(settingsFile, []string{"tiki"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readSettings(t)
	style, ok := got["outputStyle"].(string)
	if !ok || style != "Explanatory" {
		t.Errorf("outputStyle not preserved: got %v", got["outputStyle"])
	}
}

func TestEnsureSkillPermissions_EmptyPermissions(t *testing.T) {
	t.Chdir(t.TempDir())

	writeSettings(t, map[string]any{
		"permissions": map[string]any{},
	})

	if err := ensureSkillPermissions(settingsFile, []string{"tiki"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readSettings(t)
	allow := getAllow(t, got)
	if len(allow) != 1 || allow[0] != "Skill(tiki)" {
		t.Errorf("unexpected entries: %v", allow)
	}
}

func TestEnsureSkillPermissions_MalformedJSON(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.MkdirAll(filepath.Dir(settingsFile), 0750); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G306: 0644 is appropriate for test fixture files
	if err := os.WriteFile(settingsFile, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	err := ensureSkillPermissions(settingsFile, []string{"tiki"})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

// helpers

func readSettings(t *testing.T) map[string]any {
	t.Helper()
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		t.Fatalf("reading %s: %v", settingsFile, err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing %s: %v", settingsFile, err)
	}
	return m
}

func getAllow(t *testing.T, settings map[string]any) []string {
	t.Helper()
	perms, _ := settings["permissions"].(map[string]any)
	if perms == nil {
		t.Fatal("missing permissions key")
	}
	rawAllow, _ := perms["allow"].([]any)
	if rawAllow == nil {
		t.Fatal("missing allow key")
	}
	allow := make([]string, len(rawAllow))
	for i, v := range rawAllow {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("allow[%d] is not a string: %v", i, v)
		}
		allow[i] = s
	}
	return allow
}

func writeSettings(t *testing.T, settings map[string]any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(settingsFile), 0750); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // G306: 0644 is appropriate for test fixture files
	if err := os.WriteFile(settingsFile, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

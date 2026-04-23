package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSemVer(t *testing.T) {
	tests := []struct {
		input string
		want  SemVer
		ok    bool
	}{
		{"0.5.2", SemVer{0, 5, 2}, true},
		{"v1.2.3", SemVer{1, 2, 3}, true},
		{"V1.2.3", SemVer{1, 2, 3}, true},
		{"0.0.0", SemVer{0, 0, 0}, true},
		{"10.20.30", SemVer{10, 20, 30}, true},

		// suffix stripping (git describe, prerelease, build metadata)
		{"0.5.2-3-gabc123", SemVer{0, 5, 2}, true},
		{"0.5.2-dirty", SemVer{0, 5, 2}, true},
		{"v0.6.0-rc1", SemVer{0, 6, 0}, true},
		{"1.0.0+build.42", SemVer{1, 0, 0}, true},
		{"0.5.2-3-gabc123-dirty", SemVer{0, 5, 2}, true},

		// invalid
		{"dev", SemVer{}, false},
		{"", SemVer{}, false},
		{"1.2", SemVer{}, false},
		{"1.2.3.4", SemVer{}, false},
		{"-1.2.3", SemVer{}, false},
		{"1.2.three", SemVer{}, false},
		{"abc123f", SemVer{}, false},
		{"v", SemVer{}, false},
		{"...", SemVer{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseSemVer(tt.input)
			if ok != tt.ok {
				t.Fatalf("ParseSemVer(%q): ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Errorf("ParseSemVer(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareSemVer(t *testing.T) {
	tests := []struct {
		name string
		a, b SemVer
		want int
	}{
		{"equal", SemVer{0, 5, 2}, SemVer{0, 5, 2}, 0},
		{"patch less", SemVer{0, 5, 2}, SemVer{0, 5, 3}, -1},
		{"patch greater", SemVer{0, 5, 3}, SemVer{0, 5, 2}, 1},
		{"minor less", SemVer{0, 5, 0}, SemVer{0, 6, 0}, -1},
		{"minor greater", SemVer{0, 6, 0}, SemVer{0, 5, 0}, 1},
		{"major less", SemVer{0, 99, 99}, SemVer{1, 0, 0}, -1},
		{"major greater", SemVer{1, 0, 0}, SemVer{0, 99, 99}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSemVer(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareSemVer(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func withAppVersion(t *testing.T, v string) {
	t.Helper()
	orig := Version
	Version = v
	t.Cleanup(func() { Version = orig })
}

func TestCheckWorkflowVersionCompatibility(t *testing.T) {
	tests := []struct {
		name            string
		appVersion      string
		workflowVersion string
		wantErr         bool
		errContains     string
	}{
		{"same version", "0.5.2", "0.5.2", false, ""},
		{"older workflow", "0.5.2", "0.5.1", false, ""},
		{"newer workflow", "0.5.2", "0.6.0", true, "newer than tiki"},
		{"newer workflow major", "0.5.2", "1.0.0", true, "newer than tiki"},
		{"dev app skips", "dev", "99.0.0", false, ""},
		{"commit hash app skips", "abc123f", "0.5.2", false, ""},
		{"empty workflow skips", "0.5.2", "", false, ""},
		{"unparseable workflow skips", "0.5.2", "draft", false, ""},
		{"git describe app rejects newer", "0.5.2-3-gabc123", "0.6.0", true, "newer than tiki"},
		{"git describe app accepts same", "0.5.2-3-gabc123", "0.5.2", false, ""},
		{"prerelease app accepts same", "0.6.0-rc1", "0.6.0", false, ""},
		{"prerelease app rejects newer", "0.6.0-rc1", "0.7.0", true, "newer than tiki"},
		{"dirty app accepts same", "0.5.2-dirty", "0.5.2", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAppVersion(t, tt.appVersion)

			err := CheckWorkflowVersionCompatibility(tt.workflowVersion)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

const validWorkflowWithVersion = `version: 99.0.0
statuses:
  - key: open
    label: Open
    default: true
  - key: closed
    label: Closed
    done: true
types:
  - key: story
    label: Story
views:
  plugins:
    - name: board
      lanes:
        - name: Open
          filter: select where status = "open"
`

func TestLoadWorkflowRegistries_RejectsNewerVersion(t *testing.T) {
	cwdDir := setupLoadRegistryTest(t)
	withAppVersion(t, "0.5.2")

	if err := os.WriteFile(filepath.Join(cwdDir, "workflow.yaml"), []byte(validWorkflowWithVersion), 0644); err != nil {
		t.Fatal(err)
	}

	err := LoadWorkflowRegistries()
	if err == nil {
		t.Fatal("expected version error, got nil")
	}
	if !contains(err.Error(), "newer than tiki") {
		t.Errorf("error %q does not mention version mismatch", err.Error())
	}
}

func TestLoadWorkflowRegistries_NoFilesErrorFromStatusRegistry(t *testing.T) {
	_ = setupLoadRegistryTest(t)
	withAppVersion(t, "0.5.2")

	err := LoadWorkflowRegistries()
	if err == nil {
		t.Fatal("expected error when no workflow files found")
	}
	if !contains(err.Error(), "no workflow.yaml found") {
		t.Errorf("expected missing-file error from LoadStatusRegistry, got: %v", err)
	}
}

func TestLoadRegistriesFromFile_RejectsNewerVersion(t *testing.T) {
	withAppVersion(t, "0.5.2")

	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	if err := os.WriteFile(path, []byte(validWorkflowWithVersion), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, _, err := LoadRegistriesFromFile(path)
	if err == nil {
		t.Fatal("expected version error, got nil")
	}
	if !contains(err.Error(), "newer than tiki") {
		t.Errorf("error %q does not mention version mismatch", err.Error())
	}
}

func TestCheckFileVersionCompatibility(t *testing.T) {
	withAppVersion(t, "0.5.2")

	t.Run("compatible", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "workflow.yaml")
		if err := os.WriteFile(path, []byte("version: 0.5.2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := CheckFileVersionCompatibility(path); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("incompatible", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "workflow.yaml")
		if err := os.WriteFile(path, []byte("version: 99.0.0\n"), 0644); err != nil {
			t.Fatal(err)
		}
		err := CheckFileVersionCompatibility(path)
		if err == nil {
			t.Fatal("expected version error, got nil")
		}
		if !contains(err.Error(), "newer than tiki") {
			t.Errorf("error %q does not mention version mismatch", err.Error())
		}
	})
}

package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPluginFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create test files in different locations
	currentDir := tmpDir
	testFile := "test-plugin.yaml"
	testFilePath := filepath.Join(currentDir, testFile)

	// Create the test file
	if err := os.WriteFile(testFilePath, []byte("name: test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Change to temp directory for testing
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	tests := []struct {
		name      string
		filename  string
		wantPath  string
		wantFound bool
	}{
		{
			name:      "absolute path",
			filename:  testFilePath,
			wantPath:  testFilePath,
			wantFound: true,
		},
		{
			name:      "relative path in current dir",
			filename:  testFile,
			wantPath:  testFile,
			wantFound: true,
		},
		{
			name:      "non-existent file",
			filename:  "nonexistent.yaml",
			wantPath:  "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findPluginFile(tt.filename)

			if tt.wantFound {
				if got == "" {
					t.Errorf("findPluginFile(%q) = empty, want non-empty path",
						tt.filename)
				}
				// Verify the file exists at the returned path
				if _, err := os.Stat(got); err != nil {
					t.Errorf("findPluginFile returned path %q that doesn't exist: %v",
						got, err)
				}
			} else {
				if got != "" {
					t.Errorf("findPluginFile(%q) = %q, want empty string",
						tt.filename, got)
				}
			}
		})
	}
}

func TestFindPluginFile_SearchOrder(t *testing.T) {
	// Create temporary directories
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	//nolint:gosec // G301: test directory permissions
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create test files in different locations with same name
	testFile := "plugin.yaml"
	currentFile := filepath.Join(tmpDir, testFile)
	subFile := filepath.Join(subDir, testFile)

	// Create files
	if err := os.WriteFile(currentFile, []byte("current"), 0644); err != nil {
		t.Fatalf("Failed to create current file: %v", err)
	}
	if err := os.WriteFile(subFile, []byte("sub"), 0644); err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Change to temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test that current directory is preferred in search order
	got := findPluginFile(testFile)
	if got == "" {
		t.Fatal("findPluginFile returned empty path")
	}

	// Read the file to verify which one was found
	content, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("Failed to read found file: %v", err)
	}

	// Should find the one in current directory first
	if string(content) != "current" {
		t.Errorf("findPluginFile found wrong file: got content %q, want %q",
			string(content), "current")
	}
}

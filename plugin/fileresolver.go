package plugin

import (
	"os"
	"path/filepath"
)

// findPluginFile searches for the plugin file in various locations
func findPluginFile(filename string, baseDir string) string {
	// List of directories to search
	searchPaths := []string{
		filename,                         // As provided (absolute or relative)
		filepath.Join(".", filename),     // Current directory
		filepath.Join(baseDir, filename), // Binary directory
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

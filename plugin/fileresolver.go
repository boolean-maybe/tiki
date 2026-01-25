package plugin

import (
	"os"
	"path/filepath"

	"github.com/boolean-maybe/tiki/config"
)

// findPluginFile searches for the plugin file in various locations
// Search order: absolute path → project config dir → user config dir
func findPluginFile(filename string) string {
	// If filename is absolute, try it directly
	if filepath.IsAbs(filename) {
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
		return ""
	}

	// Get search paths from PathManager
	// Search order: project config dir → user config dir
	searchPaths := config.GetPluginSearchPaths()

	// Build full list of paths to check
	var paths []string
	paths = append(paths, filename) // Try as-is first (relative to cwd)

	for _, dir := range searchPaths {
		paths = append(paths, filepath.Join(dir, filename))
	}

	// Search for the file
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

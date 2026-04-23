package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// SemVer holds the major.minor.patch components of a semantic version.
type SemVer struct {
	Major int
	Minor int
	Patch int
}

// ParseSemVer parses a version string into its major.minor.patch components.
// Strips a leading "v"/"V" prefix and everything after the first "-" or "+"
// (prerelease/build metadata) before parsing. Returns false if the string
// is not a valid semver core (e.g. "dev", commit hashes, empty string).
func ParseSemVer(s string) (SemVer, bool) {
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return SemVer{}, false
	}

	nums := [3]int{}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return SemVer{}, false
		}
		nums[i] = n
	}

	return SemVer{Major: nums[0], Minor: nums[1], Patch: nums[2]}, true
}

// CompareSemVer returns -1 if a < b, 0 if a == b, +1 if a > b.
func CompareSemVer(a, b SemVer) int {
	switch {
	case a.Major != b.Major:
		return cmpInt(a.Major, b.Major)
	case a.Minor != b.Minor:
		return cmpInt(a.Minor, b.Minor)
	default:
		return cmpInt(a.Patch, b.Patch)
	}
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

// CheckWorkflowVersionCompatibility checks whether the given workflow version
// is compatible with the running application version. Returns an error when
// the workflow version is strictly newer than the app version (major.minor.patch).
// Skips the check when either version is not valid semver.
func CheckWorkflowVersionCompatibility(workflowVersion string) error {
	appVer, ok := ParseSemVer(Version)
	if !ok {
		return nil
	}

	if workflowVersion == "" {
		return nil
	}

	wfVer, ok := ParseSemVer(workflowVersion)
	if !ok {
		return nil
	}

	if CompareSemVer(wfVer, appVer) > 0 {
		return fmt.Errorf(
			"workflow.yaml version %s is newer than tiki v%d.%d.%d — upgrade tiki to open this workflow",
			workflowVersion, appVer.Major, appVer.Minor, appVer.Patch,
		)
	}

	return nil
}

type workflowVersionData struct {
	Version string `yaml:"version"`
}

func readWorkflowVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	var v workflowVersionData
	if err := yaml.Unmarshal(data, &v); err != nil {
		return "", fmt.Errorf("parsing %s: %w", path, err)
	}

	return v.Version, nil
}

// checkWorkflowFileVersion is the discovery-based version check called by
// LoadWorkflowRegistries. Returns nil when no workflow files are found —
// the missing-file error belongs to LoadStatusRegistry.
func checkWorkflowFileVersion() error {
	files := FindRegistryWorkflowFiles()
	if len(files) == 0 {
		return nil
	}

	version, err := readWorkflowVersion(files[0])
	if err != nil {
		return err
	}

	return CheckWorkflowVersionCompatibility(version)
}

// CheckFileVersionCompatibility checks whether the workflow file at the given
// path has a version compatible with the running application. Used by callers
// that load from an explicit path rather than discovery.
func CheckFileVersionCompatibility(path string) error {
	version, err := readWorkflowVersion(path)
	if err != nil {
		return err
	}

	return CheckWorkflowVersionCompatibility(version)
}

package viewer

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// viewer input parsing: decides whether to run the markdown viewer and resolves a
// single positional argument into candidate sources. accepted inputs: "-" for
// stdin, file paths, http(s) urls, and github/gitlab repo or file links.

// InputKind identifies the source type for viewer content resolution.
type InputKind string

const (
	InputStdin  InputKind = "stdin"
	InputFile   InputKind = "file"
	InputURL    InputKind = "url"
	InputGitHub InputKind = "github"
	InputGitLab InputKind = "gitlab"
)

// InputSpec carries the resolved input plus candidate URLs/paths to try.
type InputSpec struct {
	Kind        InputKind
	Raw         string
	Candidates  []string
	SearchRoots []string
}

// ErrMultipleInputs is returned when more than one input is provided.
var ErrMultipleInputs = errors.New("multiple input arguments provided")

// ParseViewerInput resolves a single input to decide if viewer mode should run.
func ParseViewerInput(args []string, reservedCommands map[string]struct{}) (InputSpec, bool, error) {
	positional := collectPositionalArgs(args)
	if len(positional) == 0 {
		return InputSpec{}, false, nil
	}
	if len(positional) > 1 {
		return InputSpec{}, false, ErrMultipleInputs
	}

	raw := positional[0]
	if _, reserved := reservedCommands[raw]; reserved {
		return InputSpec{}, false, nil
	}

	spec, err := buildInputSpec(raw)
	if err != nil {
		return InputSpec{}, false, err
	}
	return spec, true, nil
}

// collectPositionalArgs strips known flags and keeps the single viewer input.
func collectPositionalArgs(args []string) []string {
	var positional []string
	skipNext := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if skipNext {
			skipNext = false
			continue
		}

		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}

		if arg == "-" {
			positional = append(positional, arg)
			continue
		}

		if strings.HasPrefix(arg, "-") {
			if arg == "--log-level" {
				if i+1 < len(args) && isValidLogLevel(args[i+1]) {
					skipNext = true
				}
				continue
			}
			if strings.HasPrefix(arg, "--log-level=") {
				continue
			}
			if arg == "-v" || arg == "--version" {
				continue
			}
			continue
		}

		positional = append(positional, arg)
	}

	return positional
}

// isValidLogLevel matches supported logger levels for flag parsing.
func isValidLogLevel(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "info", "warn", "warning", "error":
		return true
	default:
		return false
	}
}

// buildInputSpec classifies the raw input and builds candidate sources.
func buildInputSpec(raw string) (InputSpec, error) {
	if raw == "-" {
		return InputSpec{
			Kind: InputStdin,
			Raw:  raw,
		}, nil
	}

	if spec, ok := parseGitHub(raw); ok {
		return spec, nil
	}
	if spec, ok := parseGitLab(raw); ok {
		return spec, nil
	}
	if spec, ok := parseHTTPURL(raw); ok {
		return spec, nil
	}

	absPath, err := filepath.Abs(raw)
	if err != nil {
		return InputSpec{}, fmt.Errorf("resolve file path: %w", err)
	}

	return InputSpec{
		Kind:        InputFile,
		Raw:         raw,
		Candidates:  []string{absPath},
		SearchRoots: []string{filepath.Dir(absPath)},
	}, nil
}

// parseHTTPURL detects explicit http(s) links.
func parseHTTPURL(raw string) (InputSpec, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return InputSpec{}, false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return InputSpec{}, false
	}
	if parsed.Host == "" {
		return InputSpec{}, false
	}

	return InputSpec{
		Kind:       InputURL,
		Raw:        raw,
		Candidates: []string{raw},
	}, true
}

// parseGitHub maps GitHub inputs to raw content URLs.
func parseGitHub(raw string) (InputSpec, bool) {
	pathPart, ok := extractHostPath(raw, "github.com")
	if !ok {
		return InputSpec{}, false
	}

	segments := splitPath(pathPart)
	if len(segments) < 2 {
		return InputSpec{}, false
	}

	owner := segments[0]
	repo := trimGitSuffix(segments[1])
	ref, filePath := parseRefAndPath(segments[2:])
	if filePath == "" {
		filePath = "README.md"
	}

	if ref != "" {
		rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, filePath)
		return InputSpec{
			Kind:       InputGitHub,
			Raw:        raw,
			Candidates: []string{rawURL},
		}, true
	}

	return InputSpec{
		Kind: InputGitHub,
		Raw:  raw,
		Candidates: []string{
			fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", owner, repo, filePath),
			fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/%s", owner, repo, filePath),
		},
	}, true
}

// parseGitLab maps GitLab inputs to raw content URLs.
func parseGitLab(raw string) (InputSpec, bool) {
	pathPart, ok := extractHostPath(raw, "gitlab.com")
	if !ok {
		return InputSpec{}, false
	}

	segments := splitPath(pathPart)
	if len(segments) < 2 {
		return InputSpec{}, false
	}

	repoIndex, ref, filePath := parseGitLabSegments(segments)
	if repoIndex < 1 {
		return InputSpec{}, false
	}

	namespace := strings.Join(segments[:repoIndex], "/")
	repo := trimGitSuffix(segments[repoIndex])
	if filePath == "" {
		filePath = "README.md"
	}

	if ref != "" {
		rawURL := fmt.Sprintf("https://gitlab.com/%s/%s/-/raw/%s/%s", namespace, repo, ref, filePath)
		return InputSpec{
			Kind:       InputGitLab,
			Raw:        raw,
			Candidates: []string{rawURL},
		}, true
	}

	return InputSpec{
		Kind: InputGitLab,
		Raw:  raw,
		Candidates: []string{
			fmt.Sprintf("https://gitlab.com/%s/%s/-/raw/main/%s", namespace, repo, filePath),
			fmt.Sprintf("https://gitlab.com/%s/%s/-/raw/master/%s", namespace, repo, filePath),
		},
	}, true
}

// extractHostPath normalizes host inputs to repo path segments.
func extractHostPath(raw, host string) (string, bool) {
	if strings.HasPrefix(raw, host+"/") {
		return strings.TrimPrefix(raw, host+"/"), true
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if parsed.Host != host {
		return "", false
	}
	return strings.TrimPrefix(parsed.Path, "/"), true
}

// splitPath tokenizes path parts without leading/trailing slashes.
func splitPath(pathPart string) []string {
	clean := strings.Trim(pathPart, "/")
	if clean == "" {
		return nil
	}
	return strings.Split(clean, "/")
}

// trimGitSuffix removes optional .git suffix on repo names.
func trimGitSuffix(repo string) string {
	return strings.TrimSuffix(repo, ".git")
}

// parseRefAndPath pulls a ref and file path from github blob/tree URLs.
func parseRefAndPath(segments []string) (string, string) {
	if len(segments) == 0 {
		return "", ""
	}
	if segments[0] == "blob" || segments[0] == "tree" {
		if len(segments) >= 2 {
			ref := segments[1]
			filePath := strings.Join(segments[2:], "/")
			return ref, filePath
		}
		return "", ""
	}
	return "", path.Join(segments...)
}

// parseGitLabSegments parses namespace/repo/-/blob|tree|raw layout.
func parseGitLabSegments(segments []string) (int, string, string) {
	dashIndex := -1
	for i, seg := range segments {
		if seg == "-" {
			dashIndex = i
			break
		}
	}

	if dashIndex == -1 {
		return len(segments) - 1, "", ""
	}

	if dashIndex < 1 {
		return -1, "", ""
	}

	ref := ""
	filePath := ""
	if len(segments) > dashIndex+2 && (segments[dashIndex+1] == "blob" || segments[dashIndex+1] == "tree" || segments[dashIndex+1] == "raw") {
		ref = segments[dashIndex+2]
		if len(segments) > dashIndex+3 {
			filePath = strings.Join(segments[dashIndex+3:], "/")
		}
	} else if len(segments) > dashIndex+1 {
		filePath = strings.Join(segments[dashIndex+1:], "/")
	}

	return dashIndex - 1, ref, filePath
}

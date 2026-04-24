package dokiindex

import (
	"bufio"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reIndexNav         = regexp.MustCompile(`<!--\s*INDEX_NAV\s*-->`)
	reInclude          = regexp.MustCompile(`<!--\s*INCLUDE:(.+?)\s*-->`)
	reIncludeRecursive = regexp.MustCompile(`<!--\s*INCLUDE_RECURSIVE:(.+?)\s*-->`)
	reMdLink           = regexp.MustCompile(`(!?\[[^\]]*\])\(([^)]+)\)`)
)

// InjectTags processes INDEX_NAV, INCLUDE, and INCLUDE_RECURSIVE HTML comment tags
// in content, returning the result. filePath must be the absolute path of the file
// whose content is being processed; it is used as the base for resolving relative paths.
// Returns content unchanged when filePath is empty or no comment tags are present.
func InjectTags(content, filePath string) string {
	if filePath == "" || !strings.Contains(content, "<!--") {
		return content
	}
	return process(content, filePath, map[string]bool{filePath: true})
}

func process(content, filePath string, visited map[string]bool) string {
	baseDir := filepath.Dir(filePath)

	content = reIndexNav.ReplaceAllStringFunc(content, func(_ string) string {
		nav := buildIndexNav(baseDir)
		slog.Debug("dokiindex: injected INDEX_NAV", "file", filePath, "entries", strings.Count(nav, "\n"))
		return nav
	})

	content = reIncludeRecursive.ReplaceAllStringFunc(content, func(match string) string {
		m := reIncludeRecursive.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		target := filepath.Join(baseDir, strings.TrimSpace(m[1]))
		if visited[target] {
			slog.Warn("dokiindex: cycle detected, skipping INCLUDE_RECURSIVE", "target", target, "from", filePath)
			return ""
		}
		data, err := os.ReadFile(target)
		if err != nil {
			slog.Warn("dokiindex: INCLUDE_RECURSIVE target not found", "target", target, "from", filePath)
			return ""
		}
		newVisited := make(map[string]bool, len(visited)+1)
		for k := range visited {
			newVisited[k] = true
		}
		newVisited[target] = true
		embedded := process(string(data), target, newVisited)
		return rewriteLinks(embedded, baseDir, filepath.Dir(target))
	})

	content = reInclude.ReplaceAllStringFunc(content, func(match string) string {
		m := reInclude.FindStringSubmatch(match)
		if len(m) < 2 {
			return match
		}
		target := filepath.Join(baseDir, strings.TrimSpace(m[1]))
		data, err := os.ReadFile(target)
		if err != nil {
			slog.Warn("dokiindex: INCLUDE target not found", "target", target, "from", filePath)
			return ""
		}
		return rewriteLinks(string(data), baseDir, filepath.Dir(target))
	})

	return content
}

// buildIndexNav walks baseDir recursively and returns a nested markdown list
// of all index.md files found in subdirectories, using the H1 heading as the
// link label and falling back to the directory name.
func buildIndexNav(baseDir string) string {
	type entry struct {
		relPath string
		depth   int
		label   string
	}

	var entries []entry

	filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck
		if err != nil || !d.IsDir() || path == baseDir {
			return nil
		}
		indexPath := filepath.Join(path, "index.md")
		if _, err := os.Stat(indexPath); err != nil {
			return nil
		}
		rel, _ := filepath.Rel(baseDir, indexPath)
		slashRel := filepath.ToSlash(rel)
		depth := strings.Count(filepath.ToSlash(filepath.Dir(rel)), "/")
		label := extractH1(indexPath)
		if label == "" {
			label = filepath.Base(path)
		}
		entries = append(entries, entry{relPath: slashRel, depth: depth, label: label})
		return nil
	})

	var sb strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&sb, "- [%s%s](%s)\n", strings.Repeat("  ", e.depth), e.label, e.relPath)
	}
	return sb.String()
}

func extractH1(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// rewriteLinks rewrites relative markdown links in content from paths relative
// to includedDir to paths relative to includingDir, so links remain valid after
// the content is embedded in a file at a different location.
func rewriteLinks(content, includingDir, includedDir string) string {
	if includingDir == includedDir {
		return content
	}
	return reMdLink.ReplaceAllStringFunc(content, func(match string) string {
		m := reMdLink.FindStringSubmatch(match)
		if len(m) < 3 {
			return match
		}
		linkText, url := m[1], m[2]
		if isAbsoluteURL(url) {
			return match
		}
		urlPath, fragment := splitFragment(url)
		if urlPath == "" {
			return match
		}
		abs := filepath.Join(includedDir, urlPath)
		rel, err := filepath.Rel(includingDir, abs)
		if err != nil {
			return match
		}
		return fmt.Sprintf("%s(%s%s)", linkText, filepath.ToSlash(rel), fragment)
	})
}

func isAbsoluteURL(url string) bool {
	return strings.HasPrefix(url, "http://") ||
		strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "/") ||
		strings.HasPrefix(url, "#")
}

func splitFragment(url string) (path, fragment string) {
	if idx := strings.Index(url, "#"); idx >= 0 {
		return url[:idx], url[idx:]
	}
	return url, ""
}

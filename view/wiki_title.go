package view

import (
	"path/filepath"
	"strings"

	"github.com/boolean-maybe/tiki/store"
	"gopkg.in/yaml.v3"
)

// docTitle derives the human-readable title of an opened markdown file from its
// content, falling back to the base filename. Resolution order:
//
//  1. YAML frontmatter `title:` (how tikis and titled docs carry their name);
//  2. the first `# H1` heading (plain docs without frontmatter);
//  3. the base filename (untitled loose notes).
//
// content is the raw file text (frontmatter intact), as fetched by the wiki
// provider for a `DocumentPath` view.
func docTitle(content, relPath string) string {
	if t := frontmatterTitle(content); t != "" {
		return t
	}
	if t := firstH1(content); t != "" {
		return t
	}
	return filepath.Base(relPath)
}

// frontmatterTitle returns the `title:` value from the document's YAML
// frontmatter, or "" when absent or unparseable. Reuses store.ParseFrontmatter
// + yaml so quoting and embedded colons are handled the same way the loader
// handles them (see store/tikistore/tiki_bridge.go).
func frontmatterTitle(content string) string {
	fm, _, err := store.ParseFrontmatter(content)
	if err != nil || fm == "" {
		return ""
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(fm), &m); err != nil {
		return ""
	}
	title, _ := m["title"].(string)
	return strings.TrimSpace(title)
}

// firstH1 returns the text of the first level-1 ATX heading (`# ...`) in body,
// or "" when there is none. `##` and deeper headings do not count.
func firstH1(content string) string {
	_, body, err := store.ParseFrontmatter(content)
	if err != nil {
		body = content
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "# ") {
			continue
		}
		heading := strings.TrimPrefix(trimmed, "# ")
		// closing hashes (`# Title #`) are optional per CommonMark; strip them
		heading = strings.TrimRight(heading, "#")
		return strings.TrimSpace(heading)
	}
	return ""
}

package store

import (
	"fmt"
	"strings"
)

// ParseFrontmatter extracts YAML frontmatter and body from markdown content
func ParseFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	// find closing ---
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return "", content, fmt.Errorf("no closing frontmatter delimiter")
	}

	frontmatter = strings.TrimSpace(rest[:idx])
	body = strings.TrimPrefix(rest[idx+4:], "\n")

	return frontmatter, body, nil
}

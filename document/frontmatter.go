package document

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FrontmatterSplit is the raw result of separating YAML frontmatter from the
// document body.
//
//   - HadDelimiters is true when the document contained a matched pair of
//     --- fences. This is the authoritative "had a frontmatter block" signal
//     and is independent of whether the block was empty.
//   - Frontmatter is the text between the fences (trimmed). Empty when the
//     block was empty or absent — callers must consult HadDelimiters to
//     distinguish "empty block" from "no block".
//   - Body is the content after the closing delimiter with the immediately
//     following newline consumed.
type FrontmatterSplit struct {
	Frontmatter   string
	Body          string
	HadDelimiters bool
}

// SplitFrontmatter extracts YAML frontmatter and body from markdown content
// with byte-for-byte body preservation. When the content does not start with
// "---" (optionally preceded by a UTF-8 BOM), Frontmatter is empty and Body
// is the full content as-is — leading/trailing blank lines included.
//
// The closing delimiter consumes exactly one newline after "\n---" so that
// a document written as:
//
//	---\nid: X\n---\n\nhello\n
//
// round-trips with Body == "\nhello\n". Trailing content beyond the closing
// delimiter (whitespace, additional newlines, body text) is preserved
// verbatim.
// utf8BOM is EF BB BF; we strip it from the head of a document so a leading
// BOM does not hide the frontmatter delimiter and so the body does not carry
// phantom characters.
const utf8BOM = "\uFEFF"

func SplitFrontmatter(content string) (FrontmatterSplit, error) {
	rest := strings.TrimPrefix(content, utf8BOM)

	if !strings.HasPrefix(rest, "---") {
		return FrontmatterSplit{Body: rest}, nil
	}

	// opening "---" must be followed by a newline to be treated as a
	// delimiter; `--- foo` is not frontmatter.
	afterOpen := rest[3:]
	if !strings.HasPrefix(afterOpen, "\n") && afterOpen != "" {
		return FrontmatterSplit{Body: rest}, nil
	}

	// advance past the newline after the opening delimiter (if any) so the
	// first frontmatter line is the first yaml line.
	afterOpen = strings.TrimPrefix(afterOpen, "\n")

	idx := strings.Index(afterOpen, "\n---")
	if idx == -1 {
		// Special-case: `---\n---...` means the opening line's trailing
		// newline was consumed and the next line is the closing fence. After
		// TrimPrefix above, afterOpen begins with "---" when that happens.
		if strings.HasPrefix(afterOpen, "---") {
			bodyStart := len("---")
			if bodyStart < len(afterOpen) && afterOpen[bodyStart] == '\n' {
				bodyStart++
			}
			return FrontmatterSplit{
				Frontmatter:   "",
				Body:          afterOpen[bodyStart:],
				HadDelimiters: true,
			}, nil
		}
		return FrontmatterSplit{Body: rest}, fmt.Errorf("no closing frontmatter delimiter")
	}

	frontmatter := afterOpen[:idx]
	// consume the "\n---" delimiter and exactly one newline after it so the
	// body starts on the following line; additional blank lines are body.
	bodyStart := idx + len("\n---")
	if bodyStart < len(afterOpen) && afterOpen[bodyStart] == '\n' {
		bodyStart++
	}
	body := afterOpen[bodyStart:]

	return FrontmatterSplit{
		Frontmatter:   strings.TrimRight(frontmatter, "\n"),
		Body:          body,
		HadDelimiters: true,
	}, nil
}

// ParsedFrontmatter is the decoded frontmatter as a generic map plus a flag
// recording whether the source document had a frontmatter block at all.
//
//   - HasFrontmatter is true when the document contained a matched pair of
//     --- fences, even if the block was empty. Distinguishes "empty block"
//     from "plain markdown"; the former is an error during strict-load
//     validation (it looks intentional but carries no id), the latter is a
//     legitimate plain doc.
//   - "Has parsed keys" is a different question; callers who want that
//     should check len(result.Map) > 0.
type ParsedFrontmatter struct {
	Map            map[string]interface{}
	Body           string
	HasFrontmatter bool
	RawFrontmatter string
}

// ParseFrontmatter splits and YAML-decodes the frontmatter in one step.
// Returns an empty Map (non-nil) when the document has no frontmatter, so
// callers can always range over it safely.
func ParseFrontmatter(content string) (ParsedFrontmatter, error) {
	split, err := SplitFrontmatter(content)
	if err != nil {
		return ParsedFrontmatter{Map: map[string]interface{}{}, Body: split.Body}, err
	}

	result := ParsedFrontmatter{
		Map:            map[string]interface{}{},
		Body:           split.Body,
		HasFrontmatter: split.HadDelimiters,
		RawFrontmatter: split.Frontmatter,
	}

	if !result.HasFrontmatter || split.Frontmatter == "" {
		return result, nil
	}

	if err := yaml.Unmarshal([]byte(split.Frontmatter), &result.Map); err != nil {
		return result, fmt.Errorf("parsing yaml: %w", err)
	}
	if result.Map == nil {
		// yaml.Unmarshal of an empty-ish document can leave Map nil; keep the
		// non-nil invariant for callers.
		result.Map = map[string]interface{}{}
	}
	return result, nil
}

// FrontmatterID extracts the id field from a decoded frontmatter map, if any.
// Returns the raw string and a presence flag. Callers decide whether to
// normalize or validate.
//
// YAML parses numeric-looking ids like `000001` as integers, so this function
// accepts numeric values and stringifies them. Letter-containing or quoted
// ids come through as strings.
func FrontmatterID(fm map[string]interface{}) (string, bool) {
	if fm == nil {
		return "", false
	}
	raw, ok := fm["id"]
	if !ok {
		return "", false
	}
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v), true
	case int:
		return fmt.Sprintf("%d", v), true
	case int64:
		return fmt.Sprintf("%d", v), true
	case float64:
		// yaml.v3 sometimes decodes large ints as float; only accept whole values.
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return "", false
	default:
		return "", false
	}
}

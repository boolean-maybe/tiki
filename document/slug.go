package document

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// maxSlugLen caps a generated slug so filenames stay well under the 255-byte
// filesystem name limit. truncation happens at a hyphen boundary so words are
// not cut mid-way.
const maxSlugLen = 60

// asciiFold transliterates accented latin letters to their ASCII base form by
// decomposing to NFD and dropping the combining marks (café -> cafe).
var asciiFold = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// Slugify converts a title into a lowercase-kebab ASCII filename stem. it
// lowercases, transliterates accents to ASCII, replaces every run of
// non-[a-z0-9] with a single hyphen, trims and collapses hyphens, and caps the
// length at a hyphen boundary. a title that slugs to nothing returns "".
func Slugify(title string) string {
	folded, _, err := transform.String(asciiFold, title)
	if err != nil {
		folded = title
	}
	folded = strings.ToLower(folded)

	var b strings.Builder
	lastHyphen := true // true so a leading separator run produces no leading hyphen
	for _, r := range folded {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	return capAtHyphen(slug)
}

// capAtHyphen truncates slug to at most maxSlugLen runes, cutting at the last
// hyphen boundary so a word is never split. a slug with no hyphen under the cap
// is returned whole; one longer than the cap with no internal hyphen is hard-cut.
func capAtHyphen(slug string) string {
	if len(slug) <= maxSlugLen {
		return slug
	}
	cut := slug[:maxSlugLen]
	if i := strings.LastIndexByte(cut, '-'); i > 0 {
		return cut[:i]
	}
	return cut
}

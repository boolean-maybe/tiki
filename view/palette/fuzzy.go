package palette

import "unicode"

// fuzzyMatch performs a case-insensitive subsequence match of query against text.
// Returns (matched, score) where lower score is better.
// Score = firstMatchPos + spanLength, where spanLength = lastMatchIdx - firstMatchIdx.
func fuzzyMatch(query, text string) (bool, int) {
	if query == "" {
		return true, 0
	}

	queryRunes := []rune(query)
	textRunes := []rune(text)
	qi := 0
	firstMatch := -1
	lastMatch := -1

	for ti := 0; ti < len(textRunes) && qi < len(queryRunes); ti++ {
		if unicode.ToLower(textRunes[ti]) == unicode.ToLower(queryRunes[qi]) {
			if firstMatch == -1 {
				firstMatch = ti
			}
			lastMatch = ti
			qi++
		}
	}

	if qi < len(queryRunes) {
		return false, 0
	}

	return true, firstMatch + (lastMatch - firstMatch)
}

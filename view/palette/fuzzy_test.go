package palette

import (
	"testing"
)

func TestFuzzyMatch_EmptyQuery(t *testing.T) {
	matched, score := fuzzyMatch("", "anything")
	if !matched {
		t.Error("empty query should match everything")
	}
	if score != 0 {
		t.Errorf("empty query score should be 0, got %d", score)
	}
}

func TestFuzzyMatch_ExactMatch(t *testing.T) {
	matched, score := fuzzyMatch("Save", "Save")
	if !matched {
		t.Error("exact match should succeed")
	}
	if score != 3 {
		t.Errorf("expected score 3 (pos=0, span=3), got %d", score)
	}
}

func TestFuzzyMatch_PrefixMatch(t *testing.T) {
	matched, score := fuzzyMatch("Sav", "Save Task")
	if !matched {
		t.Error("prefix match should succeed")
	}
	if score != 2 {
		t.Errorf("expected score 2 (pos=0, span=2), got %d", score)
	}
}

func TestFuzzyMatch_SubsequenceMatch(t *testing.T) {
	matched, score := fuzzyMatch("TH", "Toggle Header")
	if !matched {
		t.Error("subsequence match should succeed")
	}
	// T at 0, H at 7 → score = 0 + 7 = 7
	if score != 7 {
		t.Errorf("expected score 7, got %d", score)
	}
}

func TestFuzzyMatch_CaseInsensitive(t *testing.T) {
	matched, _ := fuzzyMatch("save", "Save Task")
	if !matched {
		t.Error("case-insensitive match should succeed")
	}
}

func TestFuzzyMatch_NoMatch(t *testing.T) {
	matched, _ := fuzzyMatch("xyz", "Save Task")
	if matched {
		t.Error("non-matching query should not match")
	}
}

func TestFuzzyMatch_ScoreOrdering(t *testing.T) {
	// "se" matches "Search" (S=0, e=1 → score=1) better than "Save Edit" (S=0, e=5 → score=5)
	_, scoreSearch := fuzzyMatch("se", "Search")
	_, scoreSaveEdit := fuzzyMatch("se", "Save Edit")

	if scoreSearch >= scoreSaveEdit {
		t.Errorf("'Search' should score better than 'Save Edit' for 'se': %d vs %d", scoreSearch, scoreSaveEdit)
	}
}

func TestFuzzyMatch_SingleChar(t *testing.T) {
	matched, score := fuzzyMatch("q", "Quit")
	if !matched {
		t.Error("single char should match")
	}
	// q at position 0 → score = 0 + 0 = 0
	if score != 0 {
		t.Errorf("single char at start should score 0, got %d", score)
	}
}

package component

import (
	"reflect"
	"testing"

	"github.com/boolean-maybe/tiki/theme"
)

func TestNewWordList(t *testing.T) {
	words := []string{"hello", "world", "test"}
	wl := NewWordList(words)

	if wl == nil {
		t.Fatal("NewWordList returned nil")
		return
	}

	if !reflect.DeepEqual(wl.words, words) {
		t.Errorf("Expected words %v, got %v", words, wl.words)
	}

	// colors should come from the active theme — compare hex strings because
	// Role is an interface and direct == comparison is not the contract.
	roles := theme.Roles()
	if wl.fgColor.Hex() != roles.TextSecondary().Hex() {
		t.Errorf("Expected fg color %s, got %s", roles.TextSecondary().Hex(), wl.fgColor.Hex())
	}

	if wl.bgColor.Hex() != roles.SurfaceSelection().Hex() {
		t.Errorf("Expected bg color %s, got %s", roles.SurfaceSelection().Hex(), wl.bgColor.Hex())
	}
}

func TestSetWords(t *testing.T) {
	wl := NewWordList([]string{"initial"})
	newWords := []string{"updated", "words"}

	result := wl.SetWords(newWords)

	if result != wl {
		t.Error("SetWords should return self for chaining")
	}

	if !reflect.DeepEqual(wl.words, newWords) {
		t.Errorf("Expected words %v, got %v", newWords, wl.words)
	}
}

func TestGetWords(t *testing.T) {
	words := []string{"get", "these", "words"}
	wl := NewWordList(words)

	retrieved := wl.GetWords()

	if !reflect.DeepEqual(retrieved, words) {
		t.Errorf("Expected %v, got %v", words, retrieved)
	}
}

func TestSetColors(t *testing.T) {
	wl := NewWordList([]string{"test"})
	roles := theme.Roles()
	fg := roles.StatusDanger()
	bg := roles.StatusOk()

	result := wl.SetColors(fg, bg)

	if result != wl {
		t.Error("SetColors should return self for chaining")
	}

	if wl.fgColor.Hex() != fg.Hex() {
		t.Errorf("Expected fg color %s, got %s", fg.Hex(), wl.fgColor.Hex())
	}

	if wl.bgColor.Hex() != bg.Hex() {
		t.Errorf("Expected bg color %s, got %s", bg.Hex(), wl.bgColor.Hex())
	}
}

func TestWrapWords_EmptyList(t *testing.T) {
	wl := NewWordList([]string{})
	lines := wl.WrapWords(80)

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for empty word list, got %d", len(lines))
	}
}

func TestWrapWords_ZeroWidth(t *testing.T) {
	wl := NewWordList([]string{"test"})
	lines := wl.WrapWords(0)

	if len(lines) != 0 {
		t.Errorf("Expected 0 lines for zero width, got %d", len(lines))
	}
}

func TestWrapWords_SingleWord(t *testing.T) {
	wl := NewWordList([]string{"hello"})
	lines := wl.WrapWords(80)

	expected := []string{"hello"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_MultipleWordsSingleLine(t *testing.T) {
	wl := NewWordList([]string{"hello", "world", "test"})
	lines := wl.WrapWords(80)

	expected := []string{"hello world test"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_MultipleWordsMultipleLines(t *testing.T) {
	wl := NewWordList([]string{"hello", "world", "this", "is", "a", "test"})
	lines := wl.WrapWords(15)

	expected := []string{
		"hello world",
		"this is a test",
	}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_ExactFit(t *testing.T) {
	wl := NewWordList([]string{"hello", "world"})
	lines := wl.WrapWords(11) // exactly "hello world"

	expected := []string{"hello world"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_WordTooLong(t *testing.T) {
	wl := NewWordList([]string{"hello", "superlongword", "test"})
	lines := wl.WrapWords(10)

	// "superlongword" is 13 chars, exceeds width of 10
	// it should still appear on its own line
	expected := []string{
		"hello",
		"superlongword",
		"test",
	}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_WrapBoundary(t *testing.T) {
	wl := NewWordList([]string{"one", "two", "three", "four"})
	lines := wl.WrapWords(10)

	// "one two" = 7 chars (fits)
	// "three" = 5 chars, "one two three" = 13 chars (won't fit, needs new line)
	// "three four" = 10 chars (exact fit)
	expected := []string{
		"one two",
		"three four",
	}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_SingleCharacterWords(t *testing.T) {
	wl := NewWordList([]string{"a", "b", "c", "d", "e"})
	lines := wl.WrapWords(5)

	// "a b c" = 5 chars (exact fit)
	// "d e" = 3 chars
	expected := []string{
		"a b c",
		"d e",
	}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_PreserveWordOrder(t *testing.T) {
	wl := NewWordList([]string{"first", "second", "third", "fourth", "fifth"})
	lines := wl.WrapWords(15)

	expected := []string{
		"first second",
		"third fourth",
		"fifth",
	}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_VeryNarrowWidth(t *testing.T) {
	wl := NewWordList([]string{"a", "b", "c"})
	lines := wl.WrapWords(1)

	// each word gets its own line
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_EmptyStringsInList(t *testing.T) {
	wl := NewWordList([]string{"hello", "", "world"})
	lines := wl.WrapWords(20)

	// empty strings should be treated as zero-width words
	expected := []string{"hello  world"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

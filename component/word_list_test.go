package component

import (
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewWordList(t *testing.T) {
	words := []string{"hello", "world", "test"}
	wl := NewWordList(words)

	if wl == nil {
		t.Fatal("NewWordList returned nil")
	}

	if !reflect.DeepEqual(wl.words, words) {
		t.Errorf("Expected words %v, got %v", words, wl.words)
	}

	if wl.fgColor != tcell.ColorCornflowerBlue {
		t.Errorf("Expected fg color CornflowerBlue, got %v", wl.fgColor)
	}

	if wl.bgColor != tcell.ColorNavy {
		t.Errorf("Expected bg color Navy, got %v", wl.bgColor)
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
	fg := tcell.ColorRed
	bg := tcell.ColorGreen

	result := wl.SetColors(fg, bg)

	if result != wl {
		t.Error("SetColors should return self for chaining")
	}

	if wl.fgColor != fg {
		t.Errorf("Expected fg color %v, got %v", fg, wl.fgColor)
	}

	if wl.bgColor != bg {
		t.Errorf("Expected bg color %v, got %v", bg, wl.bgColor)
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
	lines := wl.WrapWords(11) // Exactly "hello world"

	expected := []string{"hello world"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_WordTooLong(t *testing.T) {
	wl := NewWordList([]string{"hello", "superlongword", "test"})
	lines := wl.WrapWords(10)

	// "superlongword" is 13 chars, exceeds width of 10
	// It should still appear on its own line
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

	// Each word gets its own line
	expected := []string{"a", "b", "c"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

func TestWrapWords_EmptyStringsInList(t *testing.T) {
	wl := NewWordList([]string{"hello", "", "world"})
	lines := wl.WrapWords(20)

	// Empty strings should be treated as zero-width words
	expected := []string{"hello  world"}
	if !reflect.DeepEqual(lines, expected) {
		t.Errorf("Expected %v, got %v", expected, lines)
	}
}

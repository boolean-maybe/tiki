package model

import "testing"

func TestMarkdownTreeConfig_SelectFiresAndHides(t *testing.T) {
	cfg := NewMarkdownTreeConfig()
	cfg.SetVisible(true)
	var got string
	cfg.SetOnSelect(func(p string) { got = p })
	cfg.Select("docs/a.md")
	if got != "docs/a.md" {
		t.Fatalf("onSelect got %q", got)
	}
	if cfg.IsVisible() {
		t.Fatal("expected hidden after Select")
	}
}

// TestMarkdownTreeConfig_SelectHidesBeforeOnSelect pins the ordering contract
// the focus wiring depends on: the visibility listener (which restores the
// pre-overlay focus) must run BEFORE onSelect (which pushes a new view and
// focuses it). If onSelect ran first, the visibility listener would fire
// afterwards and clobber the freshly-pushed view's focus with the stale
// pre-overlay primitive.
func TestMarkdownTreeConfig_SelectHidesBeforeOnSelect(t *testing.T) {
	cfg := NewMarkdownTreeConfig()
	cfg.SetVisible(true)

	var order []string
	cfg.AddListener(func() {
		if !cfg.IsVisible() {
			order = append(order, "hide")
		}
	})
	cfg.SetOnSelect(func(string) { order = append(order, "select") })

	cfg.Select("docs/a.md")

	if len(order) != 2 || order[0] != "hide" || order[1] != "select" {
		t.Fatalf("event order = %v, want [hide select] (overlay must hide before onSelect pushes+focuses)", order)
	}
}

func TestMarkdownTreeConfig_CancelFiresAndHides(t *testing.T) {
	cfg := NewMarkdownTreeConfig()
	cfg.SetVisible(true)
	called := false
	cfg.SetOnCancel(func() { called = true })
	cfg.Cancel()
	if !called || cfg.IsVisible() {
		t.Fatalf("cancel: called=%v visible=%v", called, cfg.IsVisible())
	}
}

func TestMarkdownTreeConfig_ListenerFiresOnChange(t *testing.T) {
	cfg := NewMarkdownTreeConfig()
	n := 0
	cfg.AddListener(func() { n++ })
	cfg.SetVisible(true)
	cfg.SetVisible(true) // no change -> no fire
	cfg.SetVisible(false)
	if n != 2 {
		t.Fatalf("listener fired %d times, want 2", n)
	}
}

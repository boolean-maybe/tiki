package gridlayout

import "testing"

func TestParseSizing(t *testing.T) {
	cases := []struct {
		in   string
		want Sizing
		err  bool
	}{
		{in: "", want: Sizing{Mode: SizeAuto}},                      // bare field -> auto, uncapped
		{in: "12", want: Sizing{Mode: SizeFixed, Min: 12, Max: 12}}, // fixed N
		{in: "fr", want: Sizing{Mode: SizeGrow, Weight: 1}},         // default weight 1
		{in: "2fr", want: Sizing{Mode: SizeGrow, Weight: 2}},
		{in: "auto", want: Sizing{Mode: SizeAuto}},
		{in: "auto..20", want: Sizing{Mode: SizeAuto, Max: 20}},
		{in: "8..", want: Sizing{Mode: SizeAuto, Min: 8, MinSet: true}},
		{in: "8..20", want: Sizing{Mode: SizeAuto, Min: 8, Max: 20, MinSet: true}},
		{in: "fr..40", want: Sizing{Mode: SizeGrow, Weight: 1, Max: 40}},
		{in: "0..fr", want: Sizing{Mode: SizeGrow, Weight: 1, Min: 0, MinSet: true}},
		{in: "garbage..nope", err: true},
		{in: "0", err: true}, // fixed 0 is meaningless, keep today's positive-int rule
	}
	for _, c := range cases {
		got, err := ParseSizing(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseSizing(%q): expected error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSizing(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSizing(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestSpecTypesHaveSizing(t *testing.T) {
	// Compile-time shape assertion: these fields must exist with these types.
	var fc FieldCell
	fc.Sizing = Sizing{Mode: SizeFixed, Min: 5, Max: 5}
	fc.HideWhenEmpty = true
	var seg Segment
	seg.Sizing = Sizing{Mode: SizeGrow, Weight: 1}
	var a Anchor
	a.Sizing = Sizing{Mode: SizeAuto}
	a.HideWhenEmpty = true
	if fc.Sizing.Min != 5 || !fc.HideWhenEmpty || a.Sizing.Mode != SizeAuto ||
		seg.Sizing.Mode != SizeGrow || !a.HideWhenEmpty {
		t.Fatal("field wiring sanity check failed")
	}
}

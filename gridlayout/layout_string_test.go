package gridlayout

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitLayoutString_SingleCellPerRow(t *testing.T) {
	in := "type.visual + \" \" + id\n<highlight>title\n\"priority \" + priority.visual\n"
	want := [][]string{
		{`type.visual + " " + id`},
		{`<highlight>title`},
		{`"priority " + priority.visual`},
	}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSplitLayoutString_MultiCellPipe(t *testing.T) {
	in := `"Status:" | status.label | "Tags:" | tags` + "\n"
	want := [][]string{
		{`"Status:"`, `status.label`, `"Tags:"`, `tags`},
	}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSplitLayoutString_TrimsCellWhitespace(t *testing.T) {
	in := "  a  |  b  |  c  \n"
	want := [][]string{{"a", "b", "c"}}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSplitLayoutString_QuotedPipeIsLiteral(t *testing.T) {
	in := `"a|b" | c` + "\n"
	want := [][]string{{`"a|b"`, "c"}}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSplitLayoutString_PreservesInnerQuotedWhitespace(t *testing.T) {
	in := `"  points "` + "\n"
	want := [][]string{{`"  points "`}}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSplitLayoutString_SkipsBlankLines(t *testing.T) {
	in := "a\n\n   \nb\n"
	want := [][]string{{"a"}, {"b"}}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestSplitLayoutString_SkipsHashComments(t *testing.T) {
	in := "# header comment\na\n  # indented comment\nb\n"
	want := [][]string{{"a"}, {"b"}}
	got := splitLayoutString(in)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestParseLayout_SingleColumnTikiBox(t *testing.T) {
	in := `type.visual + " " + id
<highlight>title
"priority " + priority.visual + "  points " + points.visual
`
	spec, err := ParseLayout(in)
	if err != nil {
		t.Fatalf("ParseLayout: %v", err)
	}
	if spec.Rows != 3 {
		t.Errorf("Rows: got %d, want 3", spec.Rows)
	}
	if spec.Cols != 1 {
		t.Errorf("Cols: got %d, want 1", spec.Cols)
	}
}

func TestParseLayout_MultiColumnWithSpans(t *testing.T) {
	in := `<highlight>title | -- | -- | --
_ | _ | _ | _
"Status:" | status.label + " " + status.visual | "Tags:" | tags
"Type:" | type.label + " " + type.visual | ^ | ^
`
	spec, err := ParseLayout(in)
	if err != nil {
		t.Fatalf("ParseLayout: %v", err)
	}
	if spec.Rows != 4 {
		t.Errorf("Rows: got %d, want 4", spec.Rows)
	}
	if spec.Cols != 4 {
		t.Errorf("Cols: got %d, want 4", spec.Cols)
	}
}

func TestParseLayout_EmptyError(t *testing.T) {
	cases := []string{"", "   ", "\n\n", "# only comment\n"}
	for _, in := range cases {
		_, err := ParseLayout(in)
		if err == nil {
			t.Errorf("ParseLayout(%q): want error, got nil", in)
			continue
		}
		if !strings.Contains(err.Error(), "empty") {
			t.Errorf("ParseLayout(%q): want 'empty' in error, got %v", in, err)
		}
	}
}

func TestParseLayout_RaggedRowsError(t *testing.T) {
	in := `a | b | c
d | e
`
	_, err := ParseLayout(in)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

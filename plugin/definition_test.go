package plugin

import (
	"testing"

	"github.com/boolean-maybe/tiki/gridlayout"
)

func TestWorkflowPluginHasLayoutField(t *testing.T) {
	p := WorkflowPlugin{Layout: gridlayout.GridSpec{Rows: 3, Cols: 1}}
	if p.Layout.Rows != 3 {
		t.Fatalf("Layout.Rows = %d, want 3", p.Layout.Rows)
	}
}

func TestDetailPluginHasLayoutField(t *testing.T) {
	p := DetailPlugin{Layout: gridlayout.GridSpec{Rows: 5}}
	if p.Layout.Rows != 5 {
		t.Fatalf("Layout.Rows = %d, want 5", p.Layout.Rows)
	}
}

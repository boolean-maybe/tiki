package plugin

import "testing"

func TestDetailModeValues(t *testing.T) {
	cases := []struct {
		mode DetailMode
		want string
	}{
		{DetailModeView, "view"},
		{DetailModeEdit, "edit"},
		{DetailModeNew, "new"},
		{DetailModeEditDesc, "edit-desc"},
	}
	for _, tc := range cases {
		if string(tc.mode) != tc.want {
			t.Errorf("DetailMode %q = %q, want %q", tc.mode, string(tc.mode), tc.want)
		}
	}
}

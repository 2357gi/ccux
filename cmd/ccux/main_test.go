package main

import "testing"

func TestPaneIDFromSelection(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"normal", "%18\t… Working  sre-toolbox ...\n", "%18"},
		{"no trailing newline", "%9\trow", "%9"},
		{"empty (aborted)", "", ""},
		{"only newline", "\n", ""},
	}
	for _, tt := range tests {
		if got := paneIDFromSelection(tt.in); got != tt.want {
			t.Errorf("paneIDFromSelection(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

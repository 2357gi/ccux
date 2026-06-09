package main

import "testing"

func TestDispWidth(t *testing.T) {
	tests := []struct {
		s    string
		want int
	}{
		{"abc", 3},
		{"", 0},
		{"あいう", 6},   // 3 wide runes
		{"a あ b", 6}, // a,sp,あ,sp,b = 1+1+2+1+1
		{"日本語", 6},
	}
	for _, tt := range tests {
		if got := dispWidth(tt.s); got != tt.want {
			t.Errorf("dispWidth(%q) = %d, want %d", tt.s, got, tt.want)
		}
	}
}

func TestPad(t *testing.T) {
	if got := pad("ab", 5); got != "ab   " {
		t.Errorf("pad ascii = %q, want %q", got, "ab   ")
	}
	// wide string: "あ" is width 2, so pad to 5 -> 3 spaces
	if got := pad("あ", 5); got != "あ   " {
		t.Errorf("pad wide = %q, want %q", got, "あ   ")
	}
	// already wider than target: returned unchanged
	if got := pad("abcdef", 3); got != "abcdef" {
		t.Errorf("pad over = %q, want unchanged", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q, want hello", got)
	}
	if got := truncate("hello world", 5); got != "hell…" {
		t.Errorf("truncate ascii = %q, want hell…", got)
	}
	// wide runes: each is width 2; max width 5 -> two wide runes (4) + "…" = 5
	if got := truncate("あいうえお", 5); got != "ああ…" && got != "あい…" {
		// expect first two wide runes then ellipsis
		if got != "あい…" {
			t.Errorf("truncate wide = %q, want あい…", got)
		}
	}
}

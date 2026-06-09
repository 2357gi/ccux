package main

import "strings"

// dispWidth returns the terminal display width of s, counting East-Asian wide
// runes (CJK, kana, hangul, fullwidth forms) as 2 cells and everything else as 1.
func dispWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F: // Hangul Jamo
		return true
	case r >= 0x2E80 && r <= 0x303E: // CJK radicals, Kangxi, symbols & punct
		return true
	case r >= 0x3041 && r <= 0x33FF: // kana, CJK symbols, etc.
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK ext A
		return true
	case r >= 0x4E00 && r <= 0x9FFF: // CJK unified
		return true
	case r >= 0xA000 && r <= 0xA4CF: // Yi
		return true
	case r >= 0xAC00 && r <= 0xD7A3: // Hangul syllables
		return true
	case r >= 0xF900 && r <= 0xFAFF: // CJK compat ideographs
		return true
	case r >= 0xFF00 && r <= 0xFF60: // fullwidth forms
		return true
	case r >= 0xFFE0 && r <= 0xFFE6: // fullwidth signs
		return true
	case r >= 0x20000 && r <= 0x3FFFD: // CJK ext B+ / supplementary ideographs
		return true
	}
	return false
}

// pad right-pads s with spaces to display width w. If s is already at least w
// wide it is returned unchanged.
func pad(s string, w int) string {
	d := dispWidth(s)
	if d >= w {
		return s
	}
	return s + strings.Repeat(" ", w-d)
}

// truncate shortens s so its display width is at most maxWidth, appending an
// ellipsis when it had to cut.
func truncate(s string, maxWidth int) string {
	if dispWidth(s) <= maxWidth {
		return s
	}
	if maxWidth <= 1 {
		return "…"
	}
	limit := maxWidth - 1 // leave room for the ellipsis
	w := 0
	var b strings.Builder
	for _, r := range s {
		rw := 1
		if isWide(r) {
			rw = 2
		}
		if w+rw > limit {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

package status

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		captured string
		want     Kind
	}{
		{
			name:     "working with esc to interrupt",
			captured: "some output\n✻ Cogitating… (12s · ↑ 1.2k tokens · esc to interrupt)\n",
			want:     Working,
		},
		{
			name:     "working with spinner and Running",
			captured: "⠹ Running… (3s)\n",
			want:     Working,
		},
		{
			name: "waiting for permission decision",
			captured: "Do you want to proceed?\n" +
				"❯ 1. Yes\n  2. Yes, and don't ask again\n  3. No, and tell Claude what to do differently (esc)\n",
			want: Waiting,
		},
		{
			name:     "waiting with y/n prompt",
			captured: "Run this command? (y/n)\n",
			want:     Waiting,
		},
		{
			name: "idle empty prompt box",
			captured: "╭──────────────────────────────╮\n" +
				"│ >                            │\n" +
				"╰──────────────────────────────╯\n  ? for shortcuts\n",
			want: Idle,
		},
		{
			name:     "empty capture is unknown",
			captured: "   \n\n",
			want:     Unknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.captured).Kind; got != tt.want {
				t.Errorf("Classify().Kind = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClassifyWaitingBeatsIdleBox(t *testing.T) {
	// A permission prompt is rendered inside the same box chrome as the idle
	// prompt; the decision options must win.
	captured := "╭────────────────────────────╮\n" +
		"│ Do you want to make this edit?\n" +
		"│ ❯ 1. Yes                      │\n" +
		"│   2. No                       │\n" +
		"╰────────────────────────────╯\n"
	if got := Classify(captured).Kind; got != Waiting {
		t.Errorf("Classify().Kind = %v, want Waiting", got)
	}
}

func TestStatusHasLabelAndGlyph(t *testing.T) {
	for _, k := range []Kind{Unknown, Idle, Working, Waiting} {
		s := statusFor(k)
		if s.Label == "" {
			t.Errorf("Kind %v has empty Label", k)
		}
		if s.Glyph == "" {
			t.Errorf("Kind %v has empty Glyph", k)
		}
	}
}

func TestContextLeft(t *testing.T) {
	tests := []struct {
		captured string
		want     string
	}{
		{"Context left until auto-compact: 23%\n", "23%"},
		{"foo 99% bar\nbaz 12% qux\n", "12%"}, // last percentage wins
		{"no percentage here\n", ""},
	}
	for _, tt := range tests {
		if got := ContextLeft(tt.captured); got != tt.want {
			t.Errorf("ContextLeft(%q) = %q, want %q", tt.captured, got, tt.want)
		}
	}
}

package status

import "testing"

// Real-world pane captures (visible screen only). These encode the bugs where
// idle sessions were misclassified as Working because a middle-dot "·" in the
// status line and the "✻" in a completed-summary line were treated as spinners.

const idleCompleted = `  ここまでの作業をまとめました。次の手順に進めます。
✻ Churned for 1m 3s
───────────────────────────────
❯
───────────────────────────────
  [Using Opus 4.8 (1M context)] | ~/proj | Context: 33,410/1,000,000 (3.3%)
  ⏵⏵ auto mode on (shift+tab to cycle) · ← for agents`

const idlePlainDot = `  通常のテキスト応答です。
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle) · PR #4 · ← for agents`

const idleSurvey = `● How is Claude doing this session? (optional)
  1: Bad    2: Fine   3: Good   0: Dismiss
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle)`

const workingTimer = `⏺ Bash(go test ./...)
  ⎿  Running…
✽ Incubating… (24s · ↓ 732 tokens)
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle)`

const workingEsc = `✽ Cogitating… (12s · ↑ 1.2k tokens · esc to interrupt)
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle)`

// Long-running work shows the elapsed time as "(5m 53s …)" rather than "(353s)".
const workingMinutes = `  ⎿  Running…
· Forging… (5m 53s · ↓ 23.3k tokens)
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle)`

// A bare running-tool marker with no elapsed timer on its line.
const workingRunning = `⏺ Bash(sleep 5)
  ⎿  Running…
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle)`

// A *completed* session: the summary verb varies (Worked/Churned/Cooked/…) and
// reuses the ✻ glyph, but there is no live token-flow parenthetical.
const idleWorkedFor = `  作業が完了しました。次に進めます。
✻ Worked for 9m 8s
───────────────────────────────
❯
───────────────────────────────
  ⏵⏵ auto mode on (shift+tab to cycle)`

// An idle pane whose *visible conversation* (well above the footer) happens to
// contain the literal strings "esc to interrupt" and "⎿ Running…". Only the
// live region near the footer should be classified.
const idleScrollbackHasWorkingWords = `  ログ: ここで esc to interrupt の話をした
  ⎿  Running… という文字列も本文に出てくる
  段落3
  段落4
  段落5
  段落6
  段落7
  段落8
  段落9 説明おわり
✻ Worked for 9m 8s
───────────────────────────────
❯
───────────────────────────────
  [Using Opus 4.8 (1M context)] | ~/proj | Context: 50,000/1,000,000 (5.0%)
  ⏵⏵ auto mode on (shift+tab to cycle)`

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		captured string
		want     Kind
	}{
		{"active spinner with timer", workingTimer, Working},
		{"esc to interrupt", workingEsc, Working},
		{"long-running minutes timer", workingMinutes, Working},
		{"bare running-tool marker", workingRunning, Working},
		{"spinner with token flow", "⠹ Forging… (3s · ↓ 120 tokens)\n❯ \n  ⏵⏵ auto mode on\n", Working},
		{"completed summary line is idle (not working)", idleCompleted, Idle},
		{"completed 'Worked for' verb is idle", idleWorkedFor, Idle},
		{"working words in scrollback are ignored", idleScrollbackHasWorkingWords, Idle},
		{"status line middle-dot is idle (not working)", idlePlainDot, Idle},
		{"optional survey is idle (not waiting/working)", idleSurvey, Idle},
		{
			name: "permission decision is waiting",
			captured: "Do you want to proceed?\n" +
				"❯ 1. Yes\n  2. Yes, and don't ask again\n  3. No (esc)\n",
			want: Waiting,
		},
		{"y/n confirm is waiting", "Run this command? (y/n)\n", Waiting},
		{
			name: "idle empty prompt box",
			captured: "╭──────────────────────────────╮\n" +
				"│ >                            │\n" +
				"╰──────────────────────────────╯\n  ? for shortcuts\n",
			want: Idle,
		},
		{"empty capture is unknown", "   \n\n", Unknown},
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

func TestFromState(t *testing.T) {
	cases := map[string]Kind{"working": Working, "waiting": Waiting, "idle": Idle}
	for s, want := range cases {
		got, ok := FromState(s)
		if !ok || got.Kind != want {
			t.Errorf("FromState(%q) = (%v, %v), want kind %v", s, got.Kind, ok, want)
		}
	}
	if _, ok := FromState("bogus"); ok {
		t.Error("FromState(bogus) ok = true, want false")
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

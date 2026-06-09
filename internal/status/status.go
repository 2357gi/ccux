// Package status classifies the live state of a Claude Code session from the
// text captured off its tmux pane.
//
// Only the "live region" at the very bottom of the pane is examined — the lines
// just above and including Claude's footer ("⏵⏵ auto mode on …"). This is where
// the spinner, the input box, and any decision menu live. Looking higher up
// would let the *displayed conversation* (which may quote "esc to interrupt",
// "⎿ Running…", an elapsed timer, etc.) trigger false positives.
package status

import (
	"regexp"
	"strings"
)

// Kind is the coarse state of a session.
type Kind int

const (
	// Unknown means the pane content didn't match any known signal.
	Unknown Kind = iota
	// Idle means Claude is waiting for the user to type.
	Idle
	// Working means Claude is actively thinking or running a tool.
	Working
	// Waiting means Claude is blocked on a user decision (permission/confirm).
	Waiting
)

// Status is a classified Kind decorated with a label and glyph for display.
type Status struct {
	Kind  Kind
	Label string
	Glyph string
}

// FromState maps a hook-reported state string ("working"/"waiting"/"idle") to a
// Status. ok is false for an unrecognized state.
func FromState(s string) (Status, bool) {
	switch s {
	case "working":
		return statusFor(Working), true
	case "waiting":
		return statusFor(Waiting), true
	case "idle":
		return statusFor(Idle), true
	}
	return Status{}, false
}

func statusFor(k Kind) Status {
	switch k {
	case Idle:
		return Status{Idle, "Idle", "●"}
	case Working:
		return Status{Working, "Working", "…"}
	case Waiting:
		return Status{Waiting, "Waiting", "▲"}
	default:
		return Status{Unknown, "Active", "○"}
	}
}

// regionLines is how many non-blank lines above Claude's footer make up the
// live region we classify.
const regionLines = 10

var (
	// Claude's bottom chrome, used both to anchor the live region and to detect
	// the interactive (idle) prompt.
	footerRe = regexp.MustCompile(`⏵⏵|auto mode|for shortcuts|new task\?`)

	// A pending decision needs the user: a numbered menu whose selected line is
	// marked with ❯, or an explicit (y/n) confirmation. This is distinct from the
	// optional post-session survey ("1: Bad  2: Fine"), which uses ":".
	decisionRe = regexp.MustCompile(`❯\s*1\.|\(y/n\)|\by/n\b`)
	wantRe     = regexp.MustCompile(`(?i)do you want to`)

	// Active-work signals, deliberately specific to Claude's own UI so they never
	// match a *completed* summary line ("✻ Worked for 9m 8s" — note the verb
	// varies: Worked/Churned/Cooked/…) nor prose:
	//   - "esc to interrupt"
	//   - the running-tool marker "⎿  Running…"
	//   - the spinner's live counter: a parenthetical with a token-flow arrow,
	//     e.g. "(3m 23s · ↓ 13.2k tokens)". The ↑/↓ inside parens is the tell;
	//     a bare "(5m 30s)" in prose has none.
	workingRe = regexp.MustCompile(`esc to interrupt|⎿\s*Running|\([^)\n]*[↑↓]`)

	// An interactive prompt — present whenever Claude sits at the REPL.
	promptRe = regexp.MustCompile(`(?m)^\s*(│\s*)?[>❯]`)

	pctRe = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?%`)
)

// Classify inspects captured pane text and returns the session's Status. Pass
// the visible screen; Classify narrows it to the live region itself.
func Classify(captured string) Status {
	region := liveRegion(captured)
	if strings.TrimSpace(region) == "" {
		return statusFor(Unknown)
	}

	// 1. A pending decision needs the user — highest priority, since it can be
	//    drawn inside the same box chrome as an idle prompt.
	if decisionRe.MatchString(region) || (wantRe.MatchString(region) && hasNumberedOption(region)) {
		return statusFor(Waiting)
	}

	// 2. Actively working: a live progress signal.
	if workingRe.MatchString(region) {
		return statusFor(Working)
	}

	// 3. Otherwise, if the interactive prompt / footer is present, Claude is idle.
	if promptRe.MatchString(region) || footerRe.MatchString(region) {
		return statusFor(Idle)
	}

	return statusFor(Unknown)
}

// liveRegion returns the bottom slice of the pane to classify: up to regionLines
// non-blank lines ending at Claude's footer (or, if no footer is found, the last
// non-blank lines of the capture).
func liveRegion(captured string) string {
	lines := strings.Split(strings.TrimRight(captured, "\n"), "\n")

	anchor := len(lines) - 1
	for i := len(lines) - 1; i >= 0; i-- {
		if footerRe.MatchString(lines[i]) {
			anchor = i
			break
		}
	}

	var region []string
	nonblank := 0
	for i := anchor; i >= 0 && nonblank < regionLines; i-- {
		region = append([]string{lines[i]}, region...)
		if strings.TrimSpace(lines[i]) != "" {
			nonblank++
		}
	}
	return strings.Join(region, "\n")
}

func hasNumberedOption(s string) bool {
	return strings.Contains(s, "1.") || strings.Contains(s, "1)")
}

// ContextLeft returns the last percentage shown in the pane (Claude's
// "context left until auto-compact" indicator), or "" if none is present.
func ContextLeft(captured string) string {
	matches := pctRe.FindAllString(captured, -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

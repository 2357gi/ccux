// Package status classifies the live state of a Claude Code session from the
// text captured off its tmux pane.
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
	// Idle means Claude is waiting for the user to type (empty prompt).
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

// spinnerRunes are the braille/asterisk glyphs Claude animates while busy.
const spinnerRunes = "✽✻✶✳✢·*⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

var (
	workingRe = regexp.MustCompile(`esc to interrupt|to interrupt\)`)
	// A decision prompt: numbered options whose selected line is marked with ❯,
	// or an explicit (y/n) confirmation.
	decisionRe = regexp.MustCompile(`❯\s*1\.|\(y/n\)|\by/n\b`)
	wantRe     = regexp.MustCompile(`(?i)do you want to`)
	// An empty input prompt line, e.g. "│ > " possibly inside box chrome.
	idlePromptRe = regexp.MustCompile(`(?m)^\s*(│\s*)?[>❯]\s*│?\s*$`)
	pctRe        = regexp.MustCompile(`[0-9]+(?:\.[0-9]+)?%`)
)

// Classify inspects captured pane text and returns the session's Status.
func Classify(captured string) Status {
	if strings.TrimSpace(captured) == "" {
		return statusFor(Unknown)
	}

	// 1. A pending decision needs the user — highest priority, since it can be
	//    drawn inside the same box chrome as an idle prompt.
	if decisionRe.MatchString(captured) || (wantRe.MatchString(captured) && hasNumberedOption(captured)) {
		return statusFor(Waiting)
	}

	// 2. Actively working: the interrupt hint or an animated spinner.
	if workingRe.MatchString(captured) || hasSpinner(captured) {
		return statusFor(Working)
	}

	// 3. An empty prompt box means Claude is idle awaiting input.
	if idlePromptRe.MatchString(captured) {
		return statusFor(Idle)
	}

	return statusFor(Unknown)
}

func hasSpinner(s string) bool {
	return strings.ContainsAny(s, spinnerRunes)
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

// Command ccux browses Claude Code sessions running across tmux panes, shows
// each session's status and a recap, and jumps to the selected pane.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/2357gi/ccux/internal/session"
	"github.com/2357gi/ccux/internal/status"
	"github.com/2357gi/ccux/internal/tmux"
)

const version = "0.1.0"

const usage = `ccux - browse Claude Code sessions across tmux panes and jump to one

Usage:
  ccux                 interactive picker (fzf) -> jump to the chosen pane
                       (ctrl-r refreshes the table; preview is always live)
  ccux list            print the session table to stdout
  ccux feed            print the fzf input lines (used by the ctrl-r reload)
  ccux preview <pane>  print the status + recap for one pane (used by fzf)
  ccux jump <pane>     switch the tmux client to the given pane
  ccux version         print the version
`

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}

	var err error
	switch cmd {
	case "":
		err = runInteractive()
	case "list":
		err = runList()
	case "feed":
		err = runFeed()
	case "preview":
		if len(args) < 2 {
			err = fmt.Errorf("preview needs a pane id")
			break
		}
		err = runPreview(args[1])
	case "jump":
		if len(args) < 2 {
			err = fmt.Errorf("jump needs a pane id")
			break
		}
		err = tmux.Jump(args[1])
	case "version", "-v", "--version":
		fmt.Println("ccux", version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "ccux:", err)
		os.Exit(1)
	}
}

// runInteractive collects sessions, lets the user pick one with fzf (with a
// live recap preview), and jumps to the chosen pane.
//
// The session table is a snapshot taken when fzf launches; press ctrl-r to
// reload it (re-running `ccux feed`). The preview pane re-runs `ccux preview`
// on every selection move, so the recap and the live pane tail there are always
// current.
func runInteractive() error {
	input, n, err := feedLines()
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Println("No Claude Code sessions found in tmux panes.")
		fmt.Println("Start one with `claude` inside a tmux pane.")
		return nil
	}

	self, err := os.Executable()
	if err != nil {
		self = "ccux"
	}
	q := "'" + self + "'"

	header := fmt.Sprintf("  %s  %s  %s  %s   (ctrl-r: refresh)",
		pad("STATUS", 10), pad("PROJECT", 20), pad("BRANCH", 16), "RECAP")

	fzf := exec.Command("fzf",
		"--ansi",
		"--delimiter", "\t",
		"--with-nth", "2",
		"--no-sort",
		"--reverse",
		"--header", header,
		"--prompt", "Claude Sessions > ",
		"--preview", q+" preview {1}",
		"--preview-window", "right:55%:wrap",
		"--bind", "ctrl-r:reload("+q+" feed)",
		"--color", "pointer:75,marker:75,prompt:75,info:240,header:245,hl:75,hl+:75",
	)
	fzf.Stderr = os.Stderr
	fzf.Stdin = strings.NewReader(input)

	out, err := fzf.Output()
	if err != nil {
		// fzf exits non-zero when the user aborts (Esc/Ctrl-C); that's not an error.
		return nil
	}
	paneID := paneIDFromSelection(string(out))
	if paneID == "" {
		return nil
	}
	return tmux.Jump(paneID)
}

// feedLines builds the tab-separated fzf input for every session: field 1 is
// the hidden pane id (for preview/jump), field 2 is the colored display row. It
// also returns the number of sessions found.
func feedLines() (string, int, error) {
	sessions, err := session.Collect()
	if err != nil {
		return "", 0, err
	}
	var b strings.Builder
	for _, s := range sessions {
		b.WriteString(s.PaneID)
		b.WriteByte('\t')
		b.WriteString(formatRow(s))
		b.WriteByte('\n')
	}
	return b.String(), len(sessions), nil
}

// runFeed prints the fzf input lines. Used as the ctrl-r reload source so the
// picker can refresh the session table without restarting.
func runFeed() error {
	input, _, err := feedLines()
	if err != nil {
		return err
	}
	fmt.Print(input)
	return nil
}

// paneIDFromSelection extracts the hidden pane-id (field 1) from a line fzf
// emitted on selection, or "" when nothing was selected.
func paneIDFromSelection(out string) string {
	selected := strings.TrimRight(out, "\n")
	if selected == "" {
		return ""
	}
	return strings.SplitN(selected, "\t", 2)[0]
}

// runList prints the session table to stdout for inspection.
func runList() error {
	sessions, err := session.Collect()
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Println("No Claude Code sessions found in tmux panes.")
		return nil
	}
	fmt.Printf("  %s  %s  %s  %s  %s\n",
		pad("TARGET", 18), pad("STATUS", 10), pad("PROJECT", 20), pad("BRANCH", 16), "RECAP")
	for _, s := range sessions {
		fmt.Printf("  %s  %s\n", pad(s.Target, 18), formatRow(s))
	}
	return nil
}

// runPreview prints the status and recap for a single pane, plus a tail of the
// live pane content. Used as the fzf preview command.
func runPreview(paneID string) error {
	s, ok, err := findSession(paneID)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Println("session not found for pane", paneID)
		return nil
	}

	bold := func(s string) string { return "\x1b[1m" + s + "\x1b[0m" }
	dim := func(s string) string { return "\x1b[2m" + s + "\x1b[0m" }

	fmt.Printf("%s %s\n", colorStatus(s.Status), bold(s.Project))
	if s.GitBranch != "" {
		fmt.Printf("%s %s\n", dim("branch"), s.GitBranch)
	}
	fmt.Printf("%s %s\n", dim("pane  "), s.Target)
	if s.Context != "" {
		fmt.Printf("%s %s\n", dim("ctx   "), s.Context)
	}
	if id := shortID(s.SessionID); id != "" {
		fmt.Printf("%s %s\n", dim("sess  "), id)
	}
	if s.Title != "" {
		fmt.Printf("\n%s %s\n", dim("▌"), bold(s.Title))
	}
	if s.Question != "" {
		// the pending question Claude is blocked on — highlight it
		fmt.Printf("\n%s\n%s\n", "\x1b[1;33m❓ asking you\x1b[0m", indent(s.Question))
	}
	if s.LastPrompt != "" {
		fmt.Printf("\n%s\n%s\n", dim("you ›"), indent(truncate(s.LastPrompt, 400)))
	}
	if s.LastReply != "" {
		fmt.Printf("\n%s\n%s\n", dim("claude ›"), indent(truncate(s.LastReply, 400)))
	}

	if captured, err := tmux.Capture(paneID, 40); err == nil {
		fmt.Printf("\n%s\n%s\n", dim("──── live pane ────"), strings.TrimRight(captured, "\n"))
	}
	return nil
}

// findSession collects all sessions and returns the one for paneID.
func findSession(paneID string) (session.Session, bool, error) {
	sessions, err := session.Collect()
	if err != nil {
		return session.Session{}, false, err
	}
	for _, s := range sessions {
		if s.PaneID == paneID {
			return s, true, nil
		}
	}
	return session.Session{}, false, nil
}

// formatRow renders one session as a colored, fixed-width display row (without
// the leading pane-id field).
func formatRow(s Session) string {
	// A pending question is the most relevant thing to show; otherwise fall back
	// to the AI title, then the last prompt.
	recap := s.Question
	if recap == "" {
		recap = s.Title
	}
	if recap == "" {
		recap = s.LastPrompt
	}
	if recap == "" {
		recap = "(no recap yet)"
	}
	ctx := s.Context
	return fmt.Sprintf("%s  %s  %s  %s%s",
		colorStatus(s.Status),
		pad(truncate(s.Project, 20), 20),
		pad(truncate(s.GitBranch, 16), 16),
		ctxField(ctx),
		truncate(recap, 80),
	)
}

func ctxField(ctx string) string {
	if ctx == "" {
		return pad("", 6)
	}
	return pad(ctx, 6)
}

// Session is a local alias so this file reads cleanly.
type Session = session.Session

func colorStatus(st status.Status) string {
	var code string
	switch st.Kind {
	case status.Waiting:
		code = "1;33" // bold yellow: needs you
	case status.Working:
		code = "36" // cyan
	case status.Idle:
		code = "32" // green
	default:
		code = "2" // dim
	}
	label := fmt.Sprintf("%s %s", st.Glyph, st.Label)
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", code, pad(label, 10))
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}

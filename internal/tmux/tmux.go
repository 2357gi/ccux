// Package tmux wraps the tmux commands ccux needs: enumerating panes, capturing
// their visible content, and switching the client to a pane.
package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Format is the tmux -F format string matching ParsePanes' tab-separated layout.
const Format = "#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}\t#{pane_pid}\t#{pane_current_command}\t#{pane_current_path}\t#{pane_activity}"

// Pane is a single tmux pane.
type Pane struct {
	ID          string // unique pane id, e.g. "%9"
	SessionName string
	WindowIndex string
	PaneIndex   string
	PID         int // pane_pid (the pane's top process, usually the shell)
	Command     string
	Path        string
	Activity    int64 // pane_activity: unix time of the pane's last output
}

// Target returns the "session:window.pane" address tmux commands accept.
func (p Pane) Target() string {
	return fmt.Sprintf("%s:%s.%s", p.SessionName, p.WindowIndex, p.PaneIndex)
}

// ParsePanes parses the tab-separated output produced by `tmux list-panes -F Format`.
func ParsePanes(out string) []Pane {
	var panes []Pane
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		pid, _ := strconv.Atoi(f[4])
		var activity int64
		if len(f) >= 8 {
			activity, _ = strconv.ParseInt(f[7], 10, 64)
		}
		panes = append(panes, Pane{
			ID:          f[0],
			SessionName: f[1],
			WindowIndex: f[2],
			PaneIndex:   f[3],
			PID:         pid,
			Command:     f[5],
			Path:        f[6],
			Activity:    activity,
		})
	}
	return panes
}

// ListPanes returns every pane across all sessions.
func ListPanes() ([]Pane, error) {
	out, err := exec.Command("tmux", "list-panes", "-a", "-F", Format).Output()
	if err != nil {
		return nil, err
	}
	return ParsePanes(string(out)), nil
}

// Capture returns the visible (and up to `scrollback` lines above) content of a
// pane as plain text.
func Capture(paneID string, scrollback int) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", paneID, "-p", "-S", "-"+strconv.Itoa(scrollback)).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CaptureVisible returns only the currently visible screen of a pane (no
// scrollback). This is what status classification should use: the current state
// is at the bottom of the screen, and scrollback would carry stale lines.
func CaptureVisible(paneID string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", paneID, "-p").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Jump focuses the given pane: switch the client to its session, then select
// the window and pane.
func Jump(paneID string) error {
	for _, args := range [][]string{
		{"switch-client", "-t", paneID},
		{"select-window", "-t", paneID},
		{"select-pane", "-t", paneID},
	} {
		if err := exec.Command("tmux", args...).Run(); err != nil {
			return fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
		}
	}
	return nil
}

// InsideTmux reports whether ccux itself is running inside a tmux client.
func InsideTmux() bool {
	return strings.TrimSpace(os.Getenv("TMUX")) != ""
}

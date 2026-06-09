// Package proc discovers running Claude Code processes and maps them to the
// tmux pane they live in via the TMUX_PANE environment variable.
package proc

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ClaudeProc is a running `claude` process together with the tmux pane it
// belongs to.
type ClaudeProc struct {
	PID    int
	PaneID string // tmux pane id, e.g. "%9"
}

var tmuxPaneRe = regexp.MustCompile(`TMUX_PANE=(%[0-9]+)`)

// ParsePsEnv parses `ps -E ... -o pid=,command=` output. Each line begins with a
// PID followed by the process argv and its environment. Only processes whose
// environment exposes a TMUX_PANE are returned (a claude not inside tmux can't
// be jumped to).
func ParsePsEnv(out string) []ClaudeProc {
	var procs []ClaudeProc
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		m := tmuxPaneRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		procs = append(procs, ClaudeProc{PID: pid, PaneID: m[1]})
	}
	return procs
}

// ByPane indexes procs by their tmux pane id.
func ByPane(procs []ClaudeProc) map[string]ClaudeProc {
	m := make(map[string]ClaudeProc, len(procs))
	for _, p := range procs {
		m[p.PaneID] = p
	}
	return m
}

// List finds every running claude process that lives inside a tmux pane.
func List() ([]ClaudeProc, error) {
	pids := claudePIDs()
	if len(pids) == 0 {
		return nil, nil
	}
	out, err := exec.Command("ps", "-E", "-ww", "-p", strings.Join(pids, ","), "-o", "pid=,command=").Output()
	if err != nil {
		return nil, err
	}
	return ParsePsEnv(string(out)), nil
}

// ParseClaudePIDs parses `ps -axo pid=,comm=` output and returns the PIDs of
// processes whose executable base name is "claude". The comm column may be a
// full path (and may contain spaces for app bundles), so everything after the
// PID is treated as the command and reduced to its base name.
//
// We use this rather than `pgrep -x claude`: pgrep was observed to miss genuine
// claude processes, whereas `ps -o comm` reports them all consistently.
func ParseClaudePIDs(out string) []int {
	var pids []int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.IndexAny(line, " \t")
		if i < 0 {
			continue
		}
		pid, err := strconv.Atoi(line[:i])
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(line[i+1:])
		if filepath.Base(comm) == "claude" {
			pids = append(pids, pid)
		}
	}
	return pids
}

// claudePIDs returns the PIDs of every running claude process. Returns nil (not
// an error) when none are running.
func claudePIDs() []string {
	out, err := exec.Command("ps", "-axo", "pid=,comm=").Output()
	if err != nil {
		return nil
	}
	var pids []string
	for _, pid := range ParseClaudePIDs(string(out)) {
		pids = append(pids, strconv.Itoa(pid))
	}
	return pids
}

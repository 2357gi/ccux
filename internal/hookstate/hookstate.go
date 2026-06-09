// Package hookstate persists the live state of a Claude Code session as written
// by ccux's hook handler, keyed by tmux pane. This lets ccux report status from
// authoritative Claude Code events instead of scraping the rendered pane.
package hookstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Record is the per-pane state file content.
type Record struct {
	State     string `json:"state"` // "working" | "waiting" | "idle"
	SessionID string `json:"sessionId"`
	Pane      string `json:"pane"`
	Unix      int64  `json:"unix"` // when the state was written
}

// Dir is the directory holding per-pane state files.
func Dir(home string) string {
	return filepath.Join(home, ".claude", "ccux-state")
}

// FileName maps a tmux pane id ("%9") to its state file name ("9.json").
func FileName(pane string) string {
	return strings.TrimPrefix(pane, "%") + ".json"
}

func path(home, pane string) string {
	return filepath.Join(Dir(home), FileName(pane))
}

// ValidState reports whether s is a state ccux understands.
func ValidState(s string) bool {
	switch s {
	case "working", "waiting", "idle":
		return true
	}
	return false
}

// Write persists r for the given pane (no-op if pane is empty).
func Write(home, pane string, r Record) error {
	if pane == "" {
		return nil
	}
	if err := os.MkdirAll(Dir(home), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(path(home, pane), b, 0o644)
}

// Read returns the stored record for a pane, or ok=false if none exists.
func Read(home, pane string) (Record, bool) {
	b, err := os.ReadFile(path(home, pane))
	if err != nil {
		return Record{}, false
	}
	var r Record
	if json.Unmarshal(b, &r) != nil || !ValidState(r.State) {
		return Record{}, false
	}
	return r, true
}

// Clear removes the state file for a pane (used on SessionEnd).
func Clear(home, pane string) error {
	if pane == "" {
		return nil
	}
	err := os.Remove(path(home, pane))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

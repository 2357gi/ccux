// Package session assembles a cross-pane view of every Claude Code session
// running under tmux, merging tmux pane data, the claude process map, live pane
// status, and the on-disk transcript recap.
package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/2357gi/ccux/internal/proc"
	"github.com/2357gi/ccux/internal/status"
	"github.com/2357gi/ccux/internal/tmux"
	"github.com/2357gi/ccux/internal/transcript"
)

// Session is the merged view of one Claude Code session in a tmux pane.
type Session struct {
	PaneID    string
	Target    string // session:window.pane
	Project   string // basename of the working directory
	Cwd       string
	GitBranch string
	Status    status.Status
	Context   string // context-left percentage, e.g. "23%"

	SessionID  string
	Title      string // AI-generated title
	LastPrompt string
	LastReply  string

	Activity       int64 // pane_activity, for sorting
	TranscriptPath string
}

// Collect gathers every claude session currently running inside tmux, sorted
// for display (attention-worthy first).
func Collect() ([]Session, error) {
	panes, err := tmux.ListPanes()
	if err != nil {
		return nil, err
	}
	procs, err := proc.List()
	if err != nil {
		return nil, err
	}
	byPane := proc.ByPane(procs)

	home, _ := os.UserHomeDir()

	// keep only panes that actually host a claude process
	var claudePanes []tmux.Pane
	for _, p := range panes {
		if _, ok := byPane[p.ID]; ok {
			claudePanes = append(claudePanes, p)
		}
	}

	transcripts := resolveTranscripts(home, claudePanes)

	var sessions []Session
	for _, p := range claudePanes {
		s := Session{
			PaneID:   p.ID,
			Target:   p.Target(),
			Project:  filepath.Base(p.Path),
			Cwd:      p.Path,
			Activity: p.Activity,
		}
		if captured, err := tmux.CaptureVisible(p.ID); err == nil {
			s.Status = status.Classify(captured)
			s.Context = status.ContextLeft(captured)
		}

		if tp := transcripts[p.ID]; tp != "" {
			s.TranscriptPath = tp
			if info, err := parseTranscriptFile(tp); err == nil {
				s.SessionID = info.SessionID
				s.Title = info.AITitle
				s.LastPrompt = info.LastUserPrompt
				s.LastReply = info.LastAssistantText
				if info.GitBranch != "" {
					s.GitBranch = info.GitBranch
				}
			}
		}
		sessions = append(sessions, s)
	}

	sortSessions(sessions)
	return sessions, nil
}

// resolveTranscripts maps each claude pane to its transcript file, grouping
// panes by working directory so panes that share a cwd get distinct files.
func resolveTranscripts(home string, panes []tmux.Pane) map[string]string {
	byCwd := make(map[string][]tmux.Pane)
	for _, p := range panes {
		byCwd[p.Path] = append(byCwd[p.Path], p)
	}

	out := make(map[string]string)
	for cwd, group := range byCwd {
		files := listTranscripts(transcript.ProjectPath(home, cwd))
		refs := make([]paneRef, len(group))
		for i, p := range group {
			refs[i] = paneRef{ID: p.ID, Activity: p.Activity}
		}
		for paneID, path := range assignTranscripts(refs, files) {
			out[paneID] = path
		}
	}
	return out
}

// listTranscripts returns the .jsonl transcript files directly in dir.
func listTranscripts(dir string) []fileRef {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []fileRef
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileRef{
			Path:    filepath.Join(dir, e.Name()),
			ModUnix: info.ModTime().Unix(),
		})
	}
	return files
}

func parseTranscriptFile(path string) (transcript.Info, error) {
	f, err := os.Open(path)
	if err != nil {
		return transcript.Info{}, err
	}
	defer f.Close()
	return transcript.Parse(f)
}

// sortSessions orders sessions so the ones that want attention surface first:
// Waiting, then Working, then Idle/others; within a kind, most recently active.
func sortSessions(s []Session) {
	rank := func(k status.Kind) int {
		switch k {
		case status.Waiting:
			return 0
		case status.Working:
			return 1
		case status.Idle:
			return 2
		default:
			return 3
		}
	}
	sort.SliceStable(s, func(i, j int) bool {
		ri, rj := rank(s[i].Status.Kind), rank(s[j].Status.Kind)
		if ri != rj {
			return ri < rj
		}
		return s[i].Activity > s[j].Activity
	})
}

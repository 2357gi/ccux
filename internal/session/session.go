// Package session assembles a cross-pane view of every Claude Code session
// running under tmux, merging tmux pane data, the claude process map, live pane
// status, and the on-disk transcript recap.
package session

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/2357gi/ccux/internal/hookstate"
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
	Question   string // pending AskUserQuestion, if the session is asking one

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

	// hook-reported state (authoritative, keyed by pane) takes precedence over
	// scraping the rendered pane. It also pins the exact session id, which
	// resolves the recap transcript even when several claudes share a cwd.
	records := make(map[string]hookstate.Record)
	sessionIDs := make(map[string]string)
	for _, p := range claudePanes {
		if rec, ok := hookstate.Read(home, p.ID); ok {
			records[p.ID] = rec
			if rec.SessionID != "" {
				sessionIDs[p.ID] = rec.SessionID
			}
		}
	}

	transcripts := resolveTranscripts(home, claudePanes, sessionIDs)

	var sessions []Session
	for _, p := range claudePanes {
		s := Session{
			PaneID:   p.ID,
			Target:   p.Target(),
			Project:  filepath.Base(p.Path),
			Cwd:      p.Path,
			Activity: p.Activity,
		}

		captured, _ := tmux.CaptureVisible(p.ID)
		s.Context = status.ContextLeft(captured)

		rec, hooked := records[p.ID]
		if hooked {
			if st, ok := status.FromState(rec.State); ok {
				s.Status = st
			} else {
				hooked = false
			}
		}
		if !hooked {
			s.Status = status.Classify(captured)
		}

		if tp := transcripts[p.ID]; tp != "" {
			s.TranscriptPath = tp
			if info, err := parseTranscriptFile(tp); err == nil {
				s.SessionID = info.SessionID
				s.Title = info.AITitle
				s.LastPrompt = info.LastUserPrompt
				s.LastReply = info.LastAssistantText
				s.Question = info.Question
				if info.GitBranch != "" {
					s.GitBranch = info.GitBranch
				}
			}
		}

		// Without hook state, a transcript-detected pending question is the most
		// reliable "needs you" signal.
		if !hooked && s.Question != "" {
			if st, ok := status.FromState("waiting"); ok {
				s.Status = st
			}
		}
		sessions = append(sessions, s)
	}

	sortSessions(sessions)
	return sessions, nil
}

// resolveTranscripts maps each claude pane to its transcript file. When a pane's
// session id is known (from hook state) the transcript is pinned exactly;
// remaining panes are matched within their working directory by recency.
func resolveTranscripts(home string, panes []tmux.Pane, sessionIDs map[string]string) map[string]string {
	out := make(map[string]string)

	byCwd := make(map[string][]tmux.Pane)
	for _, p := range panes {
		if sid := sessionIDs[p.ID]; sid != "" {
			path := filepath.Join(transcript.ProjectPath(home, p.Path), sid+".jsonl")
			if _, err := os.Stat(path); err == nil {
				out[p.ID] = path
				continue
			}
		}
		byCwd[p.Path] = append(byCwd[p.Path], p)
	}

	for cwd, group := range byCwd {
		files := listTranscripts(transcript.ProjectPath(home, cwd))
		// exclude transcripts already pinned to a pane in this cwd
		var avail []fileRef
		for _, f := range files {
			if !pinned(out, f.Path) {
				avail = append(avail, f)
			}
		}
		refs := make([]paneRef, len(group))
		for i, p := range group {
			refs[i] = paneRef{ID: p.ID, Activity: p.Activity}
		}
		for paneID, path := range assignTranscripts(refs, avail) {
			out[paneID] = path
		}
	}
	return out
}

func pinned(out map[string]string, path string) bool {
	for _, v := range out {
		if v == path {
			return true
		}
	}
	return false
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

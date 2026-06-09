package session

import "testing"

func TestAssignTranscripts_SinglePaneGetsNewest(t *testing.T) {
	panes := []paneRef{{ID: "%9", Activity: 100}}
	files := []fileRef{
		{Path: "/p/old.jsonl", ModUnix: 10},
		{Path: "/p/new.jsonl", ModUnix: 50},
	}
	got := assignTranscripts(panes, files)
	if got["%9"] != "/p/new.jsonl" {
		t.Errorf("single pane got %q, want /p/new.jsonl", got["%9"])
	}
}

func TestAssignTranscripts_MultiPaneDistinctByActivity(t *testing.T) {
	// Two claude panes in the same cwd. The more-recently-active pane should be
	// paired with the more-recently-modified transcript, and assignments must be
	// distinct.
	panes := []paneRef{
		{ID: "%a", Activity: 200}, // most recently active
		{ID: "%b", Activity: 100},
	}
	files := []fileRef{
		{Path: "/p/2.jsonl", ModUnix: 20},
		{Path: "/p/3.jsonl", ModUnix: 30}, // newest
		{Path: "/p/1.jsonl", ModUnix: 10},
	}
	got := assignTranscripts(panes, files)
	if got["%a"] != "/p/3.jsonl" {
		t.Errorf("%%a got %q, want /p/3.jsonl (newest -> most active)", got["%a"])
	}
	if got["%b"] != "/p/2.jsonl" {
		t.Errorf("%%b got %q, want /p/2.jsonl", got["%b"])
	}
}

func TestAssignTranscripts_MorePanesThanFiles(t *testing.T) {
	panes := []paneRef{{ID: "%a", Activity: 200}, {ID: "%b", Activity: 100}}
	files := []fileRef{{Path: "/p/only.jsonl", ModUnix: 5}}
	got := assignTranscripts(panes, files)
	if got["%a"] != "/p/only.jsonl" {
		t.Errorf("%%a got %q, want /p/only.jsonl", got["%a"])
	}
	if _, ok := got["%b"]; ok {
		t.Errorf("%%b should be unassigned, got %q", got["%b"])
	}
}

func TestAssignTranscripts_NoFiles(t *testing.T) {
	got := assignTranscripts([]paneRef{{ID: "%a"}}, nil)
	if len(got) != 0 {
		t.Errorf("want empty map, got %+v", got)
	}
}

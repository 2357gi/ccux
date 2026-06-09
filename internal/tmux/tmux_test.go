package tmux

import "testing"

func TestParsePanes(t *testing.T) {
	// Fields are tab-separated (see Format): id, session, window, pane, pid, cmd, path, activity
	out := "%9\tdotfiles\t0\t1\t51664\t2.1.169\t/Users/kento_ogi/dotfiles\t1700000123\n" +
		"%17\tkanban-guesser\t0\t2\t69903\tzsh\t/Users/kento_ogi/src/github.com/nealle/kanban-guesser\t1700000050\n" +
		"%18\tsre-toolbox\t1\t1\t96436\t2.1.169\t/Users/kento_ogi/src/with space/repo\t1700000200\n"

	panes := ParsePanes(out)
	if len(panes) != 3 {
		t.Fatalf("got %d panes, want 3", len(panes))
	}

	p := panes[0]
	if p.ID != "%9" {
		t.Errorf("ID = %q, want %%9", p.ID)
	}
	if p.SessionName != "dotfiles" {
		t.Errorf("SessionName = %q, want dotfiles", p.SessionName)
	}
	if p.PID != 51664 {
		t.Errorf("PID = %d, want 51664", p.PID)
	}
	if p.Command != "2.1.169" {
		t.Errorf("Command = %q, want 2.1.169", p.Command)
	}
	if p.Path != "/Users/kento_ogi/dotfiles" {
		t.Errorf("Path = %q", p.Path)
	}
	if p.Target() != "dotfiles:0.1" {
		t.Errorf("Target() = %q, want dotfiles:0.1", p.Target())
	}
	if p.Activity != 1700000123 {
		t.Errorf("Activity = %d, want 1700000123", p.Activity)
	}

	// path with spaces must survive
	if panes[2].Path != "/Users/kento_ogi/src/with space/repo" {
		t.Errorf("Path with space = %q", panes[2].Path)
	}
}

func TestParsePanesIgnoresBlankAndShortLines(t *testing.T) {
	out := "\n%1\tdotfiles\t0\t0\t1\tzsh\t/tmp\n\ngarbage-without-tabs\n"
	panes := ParsePanes(out)
	if len(panes) != 1 {
		t.Fatalf("got %d panes, want 1", len(panes))
	}
	if panes[0].ID != "%1" {
		t.Errorf("ID = %q, want %%1", panes[0].ID)
	}
}

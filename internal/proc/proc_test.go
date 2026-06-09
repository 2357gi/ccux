package proc

import "testing"

func TestParsePsEnv(t *testing.T) {
	// `ps -E -ww -p <pids> -o pid=,command=` style output: right-padded pid,
	// then argv, then the environment (which contains TMUX_PANE for tmux panes).
	out := "  63138 claude TMUX_PANE=%9 PATH=/x _=/Users/me/.local/bin/claude\n" +
		"60801 claude SOMEVAR=1 TMUX_PANE=%14 OTHER=2\n" +
		"99999 claude PATH=/x NO_PANE=1\n" // no TMUX_PANE -> excluded

	got := ParsePsEnv(out)
	if len(got) != 2 {
		t.Fatalf("got %d procs, want 2 (the one without TMUX_PANE is dropped): %+v", len(got), got)
	}

	if got[0].PID != 63138 || got[0].PaneID != "%9" {
		t.Errorf("got[0] = %+v, want {PID:63138 PaneID:%%9}", got[0])
	}
	if got[1].PID != 60801 || got[1].PaneID != "%14" {
		t.Errorf("got[1] = %+v, want {PID:60801 PaneID:%%14}", got[1])
	}
}

func TestParsePsEnvIgnoresBlankAndBadLines(t *testing.T) {
	out := "\nnot-a-pid claude TMUX_PANE=%1\n  42 claude TMUX_PANE=%2\n"
	got := ParsePsEnv(out)
	if len(got) != 1 {
		t.Fatalf("got %d, want 1: %+v", len(got), got)
	}
	if got[0].PID != 42 || got[0].PaneID != "%2" {
		t.Errorf("got %+v, want {42 %%2}", got[0])
	}
}

func TestParseClaudePIDs(t *testing.T) {
	// `ps -axo pid=,comm=` output: pid then the executable path/name (which may
	// contain spaces for app bundles). We match on the base name "claude".
	out := "96572 claude\n" +
		"    1 /sbin/launchd\n" +
		"   42 /Applications/Google Chrome.app/Contents/MacOS/Google Chrome\n" +
		"   63138 claude\n" +
		"  777 /Users/me/.local/share/claude/versions/2.1.169/claude\n" + // base is "claude"
		"\n"
	got := ParseClaudePIDs(out)
	want := []int{96572, 63138, 777}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %d, want %d (full: %v)", i, got[i], want[i], got)
		}
	}
}

func TestByPane(t *testing.T) {
	procs := []ClaudeProc{{PID: 1, PaneID: "%9"}, {PID: 2, PaneID: "%14"}}
	m := ByPane(procs)
	if m["%9"].PID != 1 {
		t.Errorf("ByPane[%%9].PID = %d, want 1", m["%9"].PID)
	}
	if m["%14"].PID != 2 {
		t.Errorf("ByPane[%%14].PID = %d, want 2", m["%14"].PID)
	}
	if _, ok := m["%99"]; ok {
		t.Error("ByPane should not contain unknown pane")
	}
}

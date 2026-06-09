package session

import "sort"

// paneRef is the minimal pane info needed to pair panes with transcripts.
type paneRef struct {
	ID       string
	Activity int64 // tmux pane_activity (unix time of last output)
}

// fileRef is a transcript file candidate.
type fileRef struct {
	Path    string
	ModUnix int64
}

// assignTranscripts pairs panes that share a working directory with the
// transcript files in that directory. The most-recently-active pane is matched
// to the most-recently-modified transcript, and so on, so each pane gets a
// distinct file. With a single claude pane in a cwd this simply picks the newest
// transcript. Extra panes (more panes than files) are left unassigned.
func assignTranscripts(panes []paneRef, files []fileRef) map[string]string {
	out := make(map[string]string)
	if len(panes) == 0 || len(files) == 0 {
		return out
	}

	ps := append([]paneRef(nil), panes...)
	sort.SliceStable(ps, func(i, j int) bool {
		if ps[i].Activity != ps[j].Activity {
			return ps[i].Activity > ps[j].Activity
		}
		return ps[i].ID < ps[j].ID
	})

	fs := append([]fileRef(nil), files...)
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].ModUnix != fs[j].ModUnix {
			return fs[i].ModUnix > fs[j].ModUnix
		}
		return fs[i].Path < fs[j].Path
	})

	for i := range ps {
		if i >= len(fs) {
			break
		}
		out[ps[i].ID] = fs[i].Path
	}
	return out
}

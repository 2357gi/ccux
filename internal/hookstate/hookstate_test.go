package hookstate

import "testing"

func TestFileName(t *testing.T) {
	if got := FileName("%9"); got != "9.json" {
		t.Errorf("FileName(%%9) = %q, want 9.json", got)
	}
	if got := FileName("%123"); got != "123.json" {
		t.Errorf("FileName(%%123) = %q, want 123.json", got)
	}
}

func TestValidState(t *testing.T) {
	for _, s := range []string{"working", "waiting", "idle"} {
		if !ValidState(s) {
			t.Errorf("ValidState(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "busy", "end", "WORKING"} {
		if ValidState(s) {
			t.Errorf("ValidState(%q) = true, want false", s)
		}
	}
}

func TestWriteReadClear(t *testing.T) {
	home := t.TempDir()

	if _, ok := Read(home, "%9"); ok {
		t.Fatal("Read on empty dir should be ok=false")
	}

	rec := Record{State: "waiting", SessionID: "sess-1", Pane: "%9", Unix: 1700000000}
	if err := Write(home, "%9", rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, ok := Read(home, "%9")
	if !ok {
		t.Fatal("Read after Write: ok=false")
	}
	if got.State != "waiting" || got.SessionID != "sess-1" {
		t.Errorf("Read = %+v, want state=waiting sessionId=sess-1", got)
	}

	if err := Clear(home, "%9"); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, ok := Read(home, "%9"); ok {
		t.Error("Read after Clear should be ok=false")
	}
	// Clearing a non-existent file is not an error.
	if err := Clear(home, "%9"); err != nil {
		t.Errorf("Clear on missing file: %v", err)
	}
}

func TestReadRejectsInvalidState(t *testing.T) {
	home := t.TempDir()
	// directly write a record with an unknown state
	if err := Write(home, "%1", Record{State: "bogus", Pane: "%1"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, ok := Read(home, "%1"); ok {
		t.Error("Read should reject an unknown state")
	}
}

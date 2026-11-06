package cmd

import (
	"testing"
)

const openIssueLine = `{"id":"bd-1","title":"x","status":"open","priority":2,"issue_type":"task","created_at":"2026-11-01T00:00:00Z","updated_at":"2026-11-01T00:00:00Z"}`

func TestCloseRecordsClosedBy(t *testing.T) {
	_, s := cleanupTestStore(t, openIssueLine)
	t.Setenv("BD_ACTOR", "Andy/claude")
	prevReason := closeReason
	closeReason = "done"
	t.Cleanup(func() { closeReason = prevReason })

	captureStdout(t, func() {
		if err := runClose(closeCmd, []string{"bd-1"}); err != nil {
			t.Fatalf("runClose: %v", err)
		}
	})

	if got := s.Get("bd-1").ClosedBy; got != "Andy/claude" {
		t.Errorf("ClosedBy = %q, want %q", got, "Andy/claude")
	}
}

func TestUpdateStatusStampsAndClearsClosedBy(t *testing.T) {
	_, s := cleanupTestStore(t, openIssueLine)
	t.Setenv("BD_ACTOR", "Andy")
	prevStatus, prevPriority := updateStatus, updatePriority
	updatePriority = -1
	t.Cleanup(func() { updateStatus, updatePriority = prevStatus, prevPriority })

	update := func(status string) {
		updateStatus = status
		captureStdout(t, func() {
			if err := runUpdate(updateCmd, []string{"bd-1"}); err != nil {
				t.Fatalf("runUpdate %s: %v", status, err)
			}
		})
	}

	update("closed")
	if got := s.Get("bd-1").ClosedBy; got != "Andy" {
		t.Errorf("after close: ClosedBy = %q, want %q", got, "Andy")
	}

	update("open")
	if got := s.Get("bd-1").ClosedBy; got != "" {
		t.Errorf("after reopen: ClosedBy = %q, want empty", got)
	}
}

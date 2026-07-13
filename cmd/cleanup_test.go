package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/store"
)

// cleanupTestStore writes a temp .beads dir with the given issue lines,
// loads it into the package-level st, and resets every cleanup flag var
// (plus st and output.JSONMode) once the test ends so tests don't leak
// state into one another.
func cleanupTestStore(t *testing.T, lines ...string) (dir string, s *store.Store) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("issue-prefix: bd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "issues.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := store.Load(dir)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}

	prevSt, prevDryRun, prevNoArchive, prevYes, prevOlderThan, prevJSON := st, cleanupDryRun, cleanupNoArchive, cleanupYes, cleanupOlderThan, output.JSONMode
	prevIsInteractive := isInteractive
	st = s
	cleanupDryRun, cleanupNoArchive, cleanupYes, cleanupOlderThan = false, false, false, 0
	output.JSONMode = false

	t.Cleanup(func() {
		st, cleanupDryRun, cleanupNoArchive, cleanupYes, cleanupOlderThan, output.JSONMode = prevSt, prevDryRun, prevNoArchive, prevYes, prevOlderThan, prevJSON
		isInteractive = prevIsInteractive
	})

	return dir, s
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	f()
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

const closedIssueA = `{"id":"bd-aaa","title":"Closed A","status":"closed","priority":2,` +
	`"issue_type":"task","created_at":"2026-01-01T00:00:00Z",` +
	`"updated_at":"2026-01-01T00:00:00Z","closed_at":"2026-01-02T00:00:00Z"}`
const closedIssueB = `{"id":"bd-bbb","title":"Closed B","status":"closed","priority":2,` +
	`"issue_type":"task","created_at":"2026-01-01T00:00:00Z",` +
	`"updated_at":"2026-01-01T00:00:00Z","closed_at":"2026-01-02T00:00:00Z"}`
const openIssueC = `{"id":"bd-ccc","title":"Open C","status":"open","priority":2,` +
	`"issue_type":"task","created_at":"2026-01-01T00:00:00Z",` +
	`"updated_at":"2026-01-01T00:00:00Z"}`

// Background workers and hooks must never silently wipe issues: when stdin
// isn't a terminal, cleanup must refuse to delete unless --yes was passed
// explicitly, and it must leave issues.jsonl byte-for-byte untouched.
func TestCleanupOffTTYWithoutYesAborts(t *testing.T) {
	dir, _ := cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)
	isInteractive = func() bool { return false }

	before, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	err = runCleanup(cleanupCmd, nil)
	if err == nil {
		t.Fatal("expected runCleanup to return an error when stdin is not a terminal and --yes is not set")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("expected error to mention --yes, got: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("issues.jsonl was modified despite the abort:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	if _, err := os.Stat(filepath.Join(dir, ".cleanup-backups")); !os.IsNotExist(err) {
		t.Errorf("expected no backup directory to be created on abort, got err=%v", err)
	}
}

// findCleanupBackup returns the single backup file under
// dir/.beads-style/.cleanup-backups, failing the test if there isn't exactly
// one.
func findCleanupBackup(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".cleanup-backups"))
	if err != nil {
		t.Fatalf("reading .cleanup-backups: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 backup file, got %d: %v", len(entries), entries)
	}
	return filepath.Join(dir, ".cleanup-backups", entries[0].Name())
}

// Before any cleanup deletion, bd must write a full, content-complete copy of
// the pre-cleanup issues.jsonl under .beads/.cleanup-backups/ named
// issues-<RFC3339-timestamp>.jsonl -- this is the guard against the 2026-06-24
// incident where a chained cleanup wiped 516 closed issues with no way back.
func TestCleanupWritesContentCompleteBackupBeforeDeleting(t *testing.T) {
	dir, _ := cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)

	before, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	cleanupYes = true
	if err := runCleanup(cleanupCmd, nil); err != nil {
		t.Fatalf("runCleanup: %v", err)
	}

	backupFile := findCleanupBackup(t, dir)
	if !strings.HasPrefix(filepath.Base(backupFile), "issues-") || !strings.HasSuffix(backupFile, ".jsonl") {
		t.Errorf("expected backup file named issues-<timestamp>.jsonl, got %s", backupFile)
	}
	ts := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(backupFile), "issues-"), ".jsonl")
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("backup filename timestamp %q is not RFC3339: %v", ts, err)
	}

	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupContent) != string(before) {
		t.Errorf("backup content does not match pre-cleanup issues.jsonl:\nwant:\n%s\ngot:\n%s", before, backupContent)
	}
}

// This is the exact shape of the 2026-06-24 incident: --no-archive was meant
// to skip the user-facing archive.jsonl, not the safety net. The backup must
// still be written even though nothing gets archived.
func TestCleanupNoArchiveStillWritesBackup(t *testing.T) {
	dir, _ := cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)

	before, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	cleanupYes = true
	cleanupNoArchive = true
	if err := runCleanup(cleanupCmd, nil); err != nil {
		t.Fatalf("runCleanup: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "archive.jsonl")); !os.IsNotExist(err) {
		t.Errorf("expected no archive.jsonl under --no-archive, stat err=%v", err)
	}

	backupFile := findCleanupBackup(t, dir)
	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(backupContent) != string(before) {
		t.Errorf("backup content does not match pre-cleanup issues.jsonl under --no-archive:\nwant:\n%s\ngot:\n%s", before, backupContent)
	}
}

// The count in the confirmation/result message must match how many issues
// actually got deleted, for both the archived and --no-archive phrasings.
func TestCleanupDeleteCountMessage(t *testing.T) {
	cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)
	cleanupYes = true

	out := captureStdout(t, func() {
		if err := runCleanup(cleanupCmd, nil); err != nil {
			t.Fatalf("runCleanup: %v", err)
		}
	})
	if !strings.Contains(out, "Archived and deleted 2 closed issue(s)") {
		t.Errorf("expected archived-and-deleted message with count 2, got: %q", out)
	}

	// Second run: nothing left to clean, count must read 0.
	out = captureStdout(t, func() {
		if err := runCleanup(cleanupCmd, nil); err != nil {
			t.Fatalf("runCleanup: %v", err)
		}
	})
	if !strings.Contains(out, "0 closed issue(s)") {
		t.Errorf("expected 0-count message on second run, got: %q", out)
	}
}

// --dry-run must keep working, and must now also report where the backup
// would land, without touching the filesystem at all.
func TestCleanupDryRunReportsBackupPath(t *testing.T) {
	dir, _ := cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)
	cleanupDryRun = true

	out := captureStdout(t, func() {
		if err := runCleanup(cleanupCmd, nil); err != nil {
			t.Fatalf("runCleanup: %v", err)
		}
	})

	if !strings.Contains(out, "Would archive and delete 2 closed issue(s)") {
		t.Errorf("expected dry-run count message, got: %q", out)
	}
	wantPrefix := filepath.Join(dir, ".cleanup-backups", "issues-")
	if !strings.Contains(out, wantPrefix) {
		t.Errorf("expected dry-run output to mention backup path under %s, got: %q", wantPrefix, out)
	}

	if _, err := os.Stat(filepath.Join(dir, ".cleanup-backups")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not actually create a backup, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "archive.jsonl")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not actually archive, stat err=%v", err)
	}
}

// On a TTY without --yes, cleanup must print the count and backup path and
// require an explicit y/N answer -- declining must leave issues.jsonl
// untouched and skip the backup entirely, exactly like the off-TTY abort.
func TestCleanupInteractiveConfirmDeclineAborts(t *testing.T) {
	dir, _ := cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)
	isInteractive = func() bool { return true }
	cleanupCmd.SetIn(strings.NewReader("n\n"))
	t.Cleanup(func() { cleanupCmd.SetIn(nil) })

	before, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	out := captureStdout(t, func() {
		if err := runCleanup(cleanupCmd, nil); err != nil {
			t.Fatalf("runCleanup: %v", err)
		}
	})
	if !strings.Contains(out, "2") {
		t.Errorf("expected prompt to mention the count of 2, got: %q", out)
	}
	if !strings.Contains(out, filepath.Join(dir, ".cleanup-backups", "issues-")) {
		t.Errorf("expected prompt to mention the backup path, got: %q", out)
	}

	after, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("issues.jsonl was modified despite declining the prompt")
	}
	if _, err := os.Stat(filepath.Join(dir, ".cleanup-backups")); !os.IsNotExist(err) {
		t.Errorf("expected no backup to be written when the prompt is declined, stat err=%v", err)
	}
}

// Confirming with "y" on a TTY proceeds exactly like --yes would.
func TestCleanupInteractiveConfirmYesProceeds(t *testing.T) {
	dir, _ := cleanupTestStore(t, closedIssueA, closedIssueB, openIssueC)
	isInteractive = func() bool { return true }
	cleanupCmd.SetIn(strings.NewReader("y\n"))
	t.Cleanup(func() { cleanupCmd.SetIn(nil) })

	out := captureStdout(t, func() {
		if err := runCleanup(cleanupCmd, nil); err != nil {
			t.Fatalf("runCleanup: %v", err)
		}
	})
	if !strings.Contains(out, "Archived and deleted 2 closed issue(s)") {
		t.Errorf("expected deletion to proceed after confirming, got: %q", out)
	}
	findCleanupBackup(t, dir)
}

// /dev/null is a character device, same as a real terminal, so a TTY check
// that only looks at os.ModeCharDevice mistakes it for interactive. That is
// exactly what a background worker or hook does when it redirects stdin from
// /dev/null (or closes it) -- the single most common non-interactive case --
// so this test exercises the real isInteractive implementation, not the
// test-injected override the other tests use.
func TestIsInteractiveFalseForDevNull(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()

	oldStdin := os.Stdin
	os.Stdin = null
	t.Cleanup(func() { os.Stdin = oldStdin })

	if isInteractive() {
		t.Error("isInteractive() = true for /dev/null, want false")
	}
}

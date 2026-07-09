package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tempBeadsDir writes a .beads directory containing the given jsonl lines.
func tempBeadsDir(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("issue-prefix: tui\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "issues.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Save rewrites every line of the file, not just the modified issue. Commenting
// on issue A must therefore not strip unmodelled fields from issue B.
func TestSavePreservesUnknownFieldsOnUntouchedIssues(t *testing.T) {
	touched := `{"id":"tui-aaa","title":"Touched","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-02-10T13:12:00Z",` +
		`"updated_at":"2026-02-10T13:12:00Z"}`
	untouched := `{"id":"tui-tjf","title":"Theme system","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-02-10T13:12:00Z",` +
		`"updated_at":"2026-02-10T13:12:00Z",` +
		`"design":"ch <- x && y","notes":"keep me"}`

	dir := tempBeadsDir(t, touched, untouched)

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.AddComment("tui-aaa", "hello", "andy"); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	// The bytes on disk must not be HTML-escaped, and json.Unmarshal would hide
	// that by decoding < back to '<'. Check the raw text as well.
	if !strings.Contains(string(raw), `ch <- x && y`) {
		t.Errorf("design HTML-escaped or lost on disk:\n%s", raw)
	}

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var got map[string]any
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("re-parse %q: %v", line, err)
		}
		if got["id"] != "tui-tjf" {
			continue
		}
		found = true
		if got["design"] != "ch <- x && y" {
			t.Errorf("design = %v, want %q", got["design"], "ch <- x && y")
		}
		if got["notes"] != "keep me" {
			t.Errorf("notes = %v, want %q", got["notes"], "keep me")
		}
	}
	if !found {
		t.Fatalf("tui-tjf missing from saved file:\n%s", raw)
	}
}

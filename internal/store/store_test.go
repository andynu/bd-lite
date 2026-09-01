package store

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bd-lite/internal/types"
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

// Dependency.metadata and unmodelled comment keys must survive the same
// whole-file rewrite as unmodelled top-level Issue keys do: Save re-encodes
// every issue, including the nested Dependencies and Comments slices of
// issues untouched by the current mutation.
func TestSavePreservesUnknownFieldsInNestedStructs(t *testing.T) {
	touched := `{"id":"tui-aaa","title":"Touched","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-02-10T13:12:00Z",` +
		`"updated_at":"2026-02-10T13:12:00Z"}`
	untouched := `{"id":"tui-tjf","title":"Theme system","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-02-10T13:12:00Z",` +
		`"updated_at":"2026-02-10T13:12:00Z",` +
		`"dependencies":[{"issue_id":"tui-tjf","depends_on_id":"tui-aaa",` +
		`"type":"blocks","created_at":"2026-02-10T13:12:00Z","metadata":"{}"}],` +
		`"comments":[{"id":1,"issue_id":"tui-tjf","author":"andy","text":"note",` +
		`"created_at":"2026-02-10T13:12:00Z","reactions":["+1"]}]}`

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

		deps, _ := got["dependencies"].([]any)
		if len(deps) != 1 {
			t.Fatalf("expected 1 dependency, got %v", deps)
		}
		dep, _ := deps[0].(map[string]any)
		if dep["metadata"] != "{}" {
			t.Errorf("dependency metadata = %v, want %q", dep["metadata"], "{}")
		}

		comments, _ := got["comments"].([]any)
		if len(comments) != 1 {
			t.Fatalf("expected 1 comment, got %v", comments)
		}
		comment, _ := comments[0].(map[string]any)
		reactions, _ := comment["reactions"].([]any)
		if len(reactions) != 1 || reactions[0] != "+1" {
			t.Errorf("comment reactions = %v, want [\"+1\"]", comment["reactions"])
		}
	}
	if !found {
		t.Fatalf("tui-tjf missing from saved file:\n%s", raw)
	}
}

// savedIDs returns the ids in the order Save wrote them.
func savedIDs(t *testing.T, dir string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var got struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		ids = append(ids, got.ID)
	}
	return ids
}

// issueLine builds a minimal valid issue with the given id and created_by.
func issueLine(id, createdBy string) string {
	by := ""
	if createdBy != "" {
		by = `"created_by":"` + createdBy + `",`
	}
	return `{"id":"` + id + `","title":"` + id + `","status":"open","priority":2,` +
		`"issue_type":"task",` + by + `"created_at":"2026-02-10T13:00:00Z",` +
		`"updated_at":"2026-02-10T13:00:00Z"}`
}

// File order is by (created_by, id), not by created_at. Two people's issues
// end up in different regions of the file, so a concurrent bd create by each
// of them lands at different insertion points instead of both racing to
// append at the same spot.
func TestSaveOrdersByAuthorThenID(t *testing.T) {
	dir := tempBeadsDir(t,
		issueLine("tui-ccc", "scott"),
		issueLine("tui-aaa", "andy"),
		issueLine("tui-bbb", "andy"),
		issueLine("tui-ddd", ""), // unset created_by sorts first, as ""
	)

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := savedIDs(t, dir)
	want := []string{"tui-ddd", "tui-aaa", "tui-bbb", "tui-ccc"}
	if !slices.Equal(got, want) {
		t.Errorf("file order = %v, want %v", got, want)
	}
}

// Two issues by the same author still need a total order between them: id
// breaks the tie. Without it, a shared created_by block would be left to
// sort.Slice, which is not stable, and issues within the block would
// reshuffle on every write.
func TestSaveIsDeterministicWithinAnAuthorBlock(t *testing.T) {
	var lines []string
	for _, id := range []string{"tui-aaa", "tui-bbb", "tui-ccc", "tui-ddd", "tui-eee", "tui-fff"} {
		lines = append(lines, issueLine(id, "andy"))
	}
	dir := tempBeadsDir(t, lines...)

	var first []byte
	for i := 0; i < 50; i++ {
		s, err := Load(dir)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if err := s.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}
		raw, err := os.ReadFile(filepath.Join(dir, "issues.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = raw
			continue
		}
		if !bytes.Equal(raw, first) {
			t.Fatalf("write %d differs from the first write:\n got %s\nwant %s", i, raw, first)
		}
	}
}

// cleanup builds its archive batch by ranging over AllIssues (map order), so
// archive.jsonl is written through this path too and needs the same order.
func TestSaveToFileOrdersByAuthorThenID(t *testing.T) {
	dir := tempBeadsDir(t, issueLine("tui-aaa", "andy"))
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	unordered := []*types.Issue{
		{ID: "tui-bbb", CreatedBy: "scott"},
		{ID: "tui-aaa", CreatedBy: "andy"},
		{ID: "tui-ccc", CreatedBy: "andy"},
	}
	path := filepath.Join(dir, "archive.jsonl")
	if err := s.SaveToFile(path, unordered); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var got struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, got.ID)
	}
	want := []string{"tui-aaa", "tui-ccc", "tui-bbb"}
	if !slices.Equal(ids, want) {
		t.Errorf("archive order = %v, want %v", ids, want)
	}
}

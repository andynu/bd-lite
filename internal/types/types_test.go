package types

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// encodeLikeStore mirrors store.Save: an Encoder with HTML escaping disabled.
// json.Marshal cannot be used here, because it re-escapes a Marshaler's output.
func encodeLikeStore(t *testing.T, v any) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return strings.TrimRight(buf.String(), "\n")
}

const issueWithUnknownKeys = `{"id":"tui-tjf","title":"Theme system",` +
	`"status":"open","priority":2,"issue_type":"task",` +
	`"created_at":"2026-02-10T13:12:00Z","updated_at":"2026-02-10T13:12:00Z",` +
	`"design":"ch <- x && y","notes":"keep me","estimated_minutes":90}`

// bd-lite models 14 JSON keys; upstream beads writes more. A key bd-lite does not
// model must survive an unmarshal/marshal cycle untouched, because store.Save
// rewrites every line of the file on any write.
func TestIssueRoundTripPreservesUnknownKeys(t *testing.T) {
	var issue Issue
	if err := json.Unmarshal([]byte(issueWithUnknownKeys), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if issue.Title != "Theme system" {
		t.Errorf("known field lost: Title = %q", issue.Title)
	}
	for _, k := range []string{"design", "notes", "estimated_minutes"} {
		if _, ok := issue.Extra[k]; !ok {
			t.Errorf("unknown key %q not captured into Extra (have %v)", k, issue.Extra)
		}
	}
	if _, ok := issue.Extra["title"]; ok {
		t.Error("known key \"title\" leaked into Extra")
	}

	out := encodeLikeStore(t, &issue)

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if got["design"] != "ch <- x && y" {
		t.Errorf("design = %v, want %q", got["design"], "ch <- x && y")
	}
	if got["notes"] != "keep me" {
		t.Errorf("notes = %v, want %q", got["notes"], "keep me")
	}
	if got["estimated_minutes"] != float64(90) {
		t.Errorf("estimated_minutes = %v, want 90", got["estimated_minutes"])
	}
	if got["title"] != "Theme system" {
		t.Errorf("title = %v, want %q", got["title"], "Theme system")
	}
}

// A Marshaler's output is not un-escaped by the outer encoder, so MarshalJSON
// must not escape in the first place.
func TestIssueRoundTripDoesNotHTMLEscape(t *testing.T) {
	var issue Issue
	if err := json.Unmarshal([]byte(issueWithUnknownKeys), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	out := encodeLikeStore(t, &issue)

	if !strings.Contains(out, `ch <- x && y`) {
		t.Errorf("design HTML-escaped on round trip:\n%s", out)
	}
	// Guard the specific escapes json.Marshal would have introduced. Asserting on
	// "<" would be vacuous: the correct output contains a literal "<".
	for _, esc := range []string{`\u003c`, `\u003e`, `\u0026`} {
		if strings.Contains(out, esc) {
			t.Errorf("output contains HTML escape %s:\n%s", esc, out)
		}
	}
}

// An issue with no unknown keys must not detour through a map, so its keys keep
// struct order and existing jsonl lines do not churn.
func TestIssueWithoutExtraKeepsStructKeyOrder(t *testing.T) {
	var issue Issue
	line := `{"id":"bd-lite-aaa","title":"Plain","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-07-09T12:00:00Z",` +
		`"updated_at":"2026-07-09T12:00:00Z"}`
	if err := json.Unmarshal([]byte(line), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Extra != nil {
		t.Errorf("Extra should be nil for a plain issue, got %v", issue.Extra)
	}

	out := encodeLikeStore(t, &issue)

	if out != line {
		t.Errorf("plain issue did not round-trip byte-identically\n got: %s\nwant: %s", out, line)
	}

	// "title" precedes "status" in struct order but follows it alphabetically,
	// so this pair distinguishes the fast path from a map-ordered encoding.
	// (An "id" vs "title" check cannot: id precedes title under both orders.)
	if strings.Index(out, `"title"`) > strings.Index(out, `"status"`) {
		t.Errorf("keys emitted in map order, not struct order:\n%s", out)
	}
}

// A known field must always win over an Extra entry of the same name. This
// branch is unreachable via UnmarshalJSON, which deletes known keys from
// Extra before storing it, so it needs a hand-built Issue to exercise.
func TestIssueMarshalKnownFieldWinsOverExtra(t *testing.T) {
	issue := Issue{
		ID:     "bd-lite-bbb",
		Title:  "Real title",
		Status: StatusOpen,
		Extra: map[string]json.RawMessage{
			"title": json.RawMessage(`"from extra"`),
		},
	}

	out := encodeLikeStore(t, &issue)

	if n := strings.Count(out, `"title"`); n != 1 {
		t.Errorf("expected \"title\" key exactly once, appeared %d times:\n%s", n, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if got["title"] != "Real title" {
		t.Errorf("title = %v, want struct field %q to win over Extra", got["title"], "Real title")
	}
}

// Unmarshalling a line with no unknown keys into a previously-populated Issue
// must clear Extra, not leave the prior contents behind.
func TestIssueUnmarshalResetsExtra(t *testing.T) {
	var issue Issue
	if err := json.Unmarshal([]byte(issueWithUnknownKeys), &issue); err != nil {
		t.Fatalf("unmarshal with unknown keys: %v", err)
	}
	if len(issue.Extra) == 0 {
		t.Fatalf("expected Extra to be populated as a precondition, got %v", issue.Extra)
	}

	plainLine := `{"id":"bd-lite-ccc","title":"Plain","status":"open","priority":2,` +
		`"issue_type":"task","created_at":"2026-07-09T12:00:00Z",` +
		`"updated_at":"2026-07-09T12:00:00Z"}`
	if err := json.Unmarshal([]byte(plainLine), &issue); err != nil {
		t.Fatalf("unmarshal plain line: %v", err)
	}
	if issue.Extra != nil {
		t.Errorf("expected Extra reset to nil, got %v", issue.Extra)
	}
}

// Upstream beads writes a "metadata" key on dependencies that bd-lite does not
// model. It must survive a round trip like Issue's unknown keys do.
func TestDependencyRoundTripPreservesUnknownKeys(t *testing.T) {
	const depWithUnknownKey = `{"issue_id":"tui-a","depends_on_id":"tui-b",` +
		`"type":"blocks","created_at":"2026-02-10T13:12:00Z","metadata":"{}"}`

	var dep Dependency
	if err := json.Unmarshal([]byte(depWithUnknownKey), &dep); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dep.IssueID != "tui-a" {
		t.Errorf("known field lost: IssueID = %q", dep.IssueID)
	}
	if _, ok := dep.Extra["metadata"]; !ok {
		t.Errorf("unknown key %q not captured into Extra (have %v)", "metadata", dep.Extra)
	}
	if _, ok := dep.Extra["issue_id"]; ok {
		t.Error("known key \"issue_id\" leaked into Extra")
	}

	out := encodeLikeStore(t, &dep)

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if got["metadata"] != "{}" {
		t.Errorf("metadata = %v, want %q", got["metadata"], "{}")
	}
	if got["issue_id"] != "tui-a" {
		t.Errorf("issue_id = %v, want %q", got["issue_id"], "tui-a")
	}
}

// A comment's unknown keys must round-trip too, even though no unmodelled
// comment keys were found in the sample beads-tui data — the same data-loss
// mechanism applies as soon as upstream adds one.
func TestCommentRoundTripPreservesUnknownKeys(t *testing.T) {
	const commentWithUnknownKey = `{"id":7,"issue_id":"tui-a","author":"andy",` +
		`"text":"hello","created_at":"2026-02-10T13:12:00Z","reactions":["+1"]}`

	var c Comment
	if err := json.Unmarshal([]byte(commentWithUnknownKey), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Text != "hello" {
		t.Errorf("known field lost: Text = %q", c.Text)
	}
	if _, ok := c.Extra["reactions"]; !ok {
		t.Errorf("unknown key %q not captured into Extra (have %v)", "reactions", c.Extra)
	}
	if _, ok := c.Extra["text"]; ok {
		t.Error("known key \"text\" leaked into Extra")
	}

	out := encodeLikeStore(t, &c)

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if reactions, _ := got["reactions"].([]any); len(reactions) != 1 || reactions[0] != "+1" {
		t.Errorf("reactions = %v, want [\"+1\"]", got["reactions"])
	}
	if got["text"] != "hello" {
		t.Errorf("text = %v, want %q", got["text"], "hello")
	}
}

// A Dependency's unknown value carrying Go-source-like text with < and & must
// survive nested inside an Issue's MarshalJSON, which takes the Extra-empty
// fast path itself while its Dependencies slice does not. This confirms
// escaping-safety is not just a top-level Issue property.
func TestNestedExtraDoesNotHTMLEscape(t *testing.T) {
	issue := Issue{
		ID:     "tui-a",
		Title:  "Has a risky dependency",
		Status: StatusOpen,
		Dependencies: []*Dependency{
			{
				IssueID:     "tui-a",
				DependsOnID: "tui-b",
				Type:        DepBlocks,
				Extra: map[string]json.RawMessage{
					"note": json.RawMessage(`"ch <- x && y"`),
				},
			},
		},
	}
	if issue.Extra != nil {
		t.Fatalf("precondition: Issue.Extra must be empty to exercise the fast path, got %v", issue.Extra)
	}

	out := encodeLikeStore(t, &issue)

	if !strings.Contains(out, `ch <- x && y`) {
		t.Errorf("nested note HTML-escaped or lost:\n%s", out)
	}
	// Guard the specific escapes json.Marshal would have introduced. Asserting
	// on "<" would be vacuous: the correct output contains a literal "<".
	for _, esc := range []string{"\\u003c", "\\u003e", "\\u0026"} {
		if strings.Contains(out, esc) {
			t.Errorf("output contains HTML escape %s:\n%s", esc, out)
		}
	}
}

// A dependency with no unknown keys must not detour through a map, so its
// keys keep struct order. issue_id precedes depends_on_id in struct order but
// follows it alphabetically, so this pair distinguishes the fast path from a
// map-ordered encoding.
func TestDependencyWithoutExtraKeepsStructKeyOrder(t *testing.T) {
	var dep Dependency
	line := `{"issue_id":"tui-a","depends_on_id":"tui-b","type":"blocks",` +
		`"created_at":"2026-07-09T12:00:00Z"}`
	if err := json.Unmarshal([]byte(line), &dep); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dep.Extra != nil {
		t.Errorf("Extra should be nil for a plain dependency, got %v", dep.Extra)
	}

	out := encodeLikeStore(t, &dep)

	if out != line {
		t.Errorf("plain dependency did not round-trip byte-identically\n got: %s\nwant: %s", out, line)
	}
	if strings.Index(out, `"issue_id"`) > strings.Index(out, `"depends_on_id"`) {
		t.Errorf("keys emitted in map order, not struct order:\n%s", out)
	}
}

// A known field on a nested struct must always win over an Extra entry of the
// same name, mirroring the Issue-level guarantee.
func TestDependencyMarshalKnownFieldWinsOverExtra(t *testing.T) {
	dep := Dependency{
		IssueID:     "tui-a",
		DependsOnID: "tui-b",
		Type:        DepBlocks,
		Extra: map[string]json.RawMessage{
			"issue_id": json.RawMessage(`"from extra"`),
		},
	}

	out := encodeLikeStore(t, &dep)

	if n := strings.Count(out, `"issue_id"`); n != 1 {
		t.Errorf("expected \"issue_id\" key exactly once, appeared %d times:\n%s", n, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if got["issue_id"] != "tui-a" {
		t.Errorf("issue_id = %v, want struct field %q to win over Extra", got["issue_id"], "tui-a")
	}
}

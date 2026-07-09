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

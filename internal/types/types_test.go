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

	idAt, titleAt := strings.Index(out, `"id"`), strings.Index(out, `"title"`)
	if idAt == -1 || titleAt == -1 || idAt > titleAt {
		t.Errorf("expected struct order (id before title), got:\n%s", out)
	}
}

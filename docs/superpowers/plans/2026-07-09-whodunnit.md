# Whodunnit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record which human created each bead as `created_by`, without destroying the fields bd-lite does not model, and show issue age in `bd list`.

**Architecture:** `internal/types` gains a custom JSON marshaller that round-trips unknown keys verbatim, closing a data-loss bug that would otherwise be triggered by the very act of filing the beads-tui ticket. A new `internal/actor` package resolves one identity string from `$BD_ACTOR`, `git config user.name`, or `$USER`, and `cmd/create.go` plus `cmd/comment.go` both stamp it. `internal/output` gains the `by <name>` suffix and wires up the orphaned `Age()` helper as a list column.

**Tech Stack:** Go 1.26.0, `github.com/spf13/cobra` v1.9.1, stdlib `encoding/json` and `reflect`. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-07-09-whodunnit-design.md`

## Global Constraints

- Add no new module dependencies. `go.mod` requires only `github.com/spf13/cobra`.
- Before every commit run `go build ./... && go test ./...`. Both must pass. Never commit a red build.
- Delete files with `trash`, never `rm`.
- bd issue IDs go in the **commit body footer**, never the subject line.
- Mark the bd issue `in_progress` before starting its task: `bd update <id> -s in_progress`.
- **Do not run any `bd` write command inside `~/projects/beads-tui` until Task 1 is committed.** Doing so rewrites all 164 lines of that repo's `issues.jsonl` and destroys `design` on 6 issues and `notes` on 5. Task 5 is gated on Task 1 for exactly this reason.
- `created_by` is optional permanently. Absent means absent. Nothing backfills it, and `bd show` must render an issue lacking it byte-identically to today.

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `internal/types/types.go` | Modify | `Issue` struct; `Extra` passthrough map; custom `MarshalJSON`/`UnmarshalJSON`; `CreatedBy` field |
| `internal/types/types_test.go` | Create | Unknown-key capture and re-emission at the type level |
| `internal/store/store_test.go` | Create | Load → mutate → Save round trip preserves unknown keys on disk |
| `internal/actor/actor.go` | Create | Resolve the creator identity string. One exported function. |
| `internal/actor/actor_test.go` | Create | The four resolution branches |
| `cmd/create.go` | Modify | Stamp `CreatedBy` on new issues |
| `cmd/comment.go` | Modify | Stamp the same identity as comment author |
| `internal/output/output.go` | Modify | `by <name>` suffix on Created line; age column; `formatAge` |
| `internal/output/output_test.go` | Modify | Suffix present/absent; `formatAge` table; column renders |

`internal/store/store.go` needs **no change**. `loadFromFile` already calls `json.Unmarshal` into a `types.Issue`, and `Save`/`SaveToFile` already call `enc.Encode(issue)` on a `*types.Issue`. Both pick up the custom marshallers automatically, because `*Issue`'s method set includes `Issue`'s value-receiver `MarshalJSON`.

---

## Task 1: Preserve unknown JSONL fields (bd-lite-g0m)

Closes the P0. Must land before Task 5.

**Files:**
- Modify: `internal/types/types.go`
- Create: `internal/types/types_test.go`
- Create: `internal/store/store_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `types.Issue.Extra map[string]json.RawMessage` (tagged `json:"-"`, populated by `UnmarshalJSON`, never read by application code); `func (i *Issue) UnmarshalJSON([]byte) error`; `func (i Issue) MarshalJSON() ([]byte, error)`; unexported `knownIssueKeys map[string]struct{}` and `marshalNoEscape(any) ([]byte, error)`.

### Why `marshalNoEscape` exists

`json.Marshal` always HTML-escapes `<`, `>`, and `&` in strings. `store.Save` sets `enc.SetEscapeHTML(false)`, but that cannot undo escaping already applied inside a `MarshalJSON` implementation: `encoding/json`'s `compact()` only ever escapes, never unescapes. Verified:

```
# throwaway program, not committed; output reproduced verbatim

json.Marshal(map):    {"design":"ch \u003c- x \u0026\u0026 y"}
marshalNoEscape(map): {"design":"ch <- x && y"}
  outer(escapeHTML=false) -> {"design":"ch \u003c- x \u0026\u0026 y"}
  outer(escapeHTML=false) -> {"design":"ch <- x && y"}
```

The third line is the point. The outer encoder passes already-escaped bytes through
unchanged, so escaping done inside `MarshalJSON` is permanent. beads-tui's `tui-tjf`
stores a Go code block in `design`; a naive `json.Marshal` inside `MarshalJSON` would
silently rewrite it.

### The bug is measured, not theorized

Running the **current** `bd` against a scratch copy of beads-tui's tracker, one
`bd comment` on an unrelated issue:

```
issues: 164 -> 164
  DESTROYED 'created_by' on 1 issue(s): tui-6f1
  DESTROYED 'design' on 6 issue(s): tui-qxy.1, tui-qxy.2, tui-qxy.4, tui-qxy.5, tui-qxy.7, tui-tjf
  DESTROYED 'notes' on 5 issue(s): tui-1oj, tui-hxu, tui-qxy, tui-qxy.3, tui-qxy.8
```

The same probe against a prototype of the Step 4 and Step 5 code:

```
issues: 164 -> 164
LOST FIELDS: none
tui-tjf design identical: True (1815 chars)
no \u003c escapes on disk: True
comment landed: True
```

Step 9 reproduces this check. Treat it as the acceptance criterion for this task.

- [ ] **Step 1: Claim the issue**

```bash
bd update bd-lite-g0m -s in_progress
```

- [ ] **Step 2: Write the failing type-level test**

Create `internal/types/types_test.go`:

```go
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
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
go test ./internal/types/ -run TestIssue -v
```

Expected: FAIL. `TestIssueRoundTripPreservesUnknownKeys` fails at `issue.Extra` with a compile error, `undefined: issue.Extra`.

- [ ] **Step 4: Add the `Extra` field to `Issue`**

In `internal/types/types.go`, add the field as the last member of the struct, after `Comments`:

```go
	Comments     []*Comment    `json:"comments,omitempty"`

	// Extra carries JSONL keys this build of bd-lite does not model. Upstream
	// beads writes design, notes, acceptance_criteria and others; a bd-lite
	// write rewrites every line of the file, so anything not round-tripped here
	// is destroyed. Populated by UnmarshalJSON, re-emitted by MarshalJSON,
	// never read by application code.
	Extra map[string]json.RawMessage `json:"-"`
}
```

Update the import block at the top of the file:

```go
import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)
```

- [ ] **Step 5: Implement the marshallers**

Append to `internal/types/types.go`, below `Validate`:

```go
// knownIssueKeys is the set of JSON keys Issue models directly. It is derived
// from the struct tags rather than hand-listed, so adding a field cannot leave
// a stale duplicate behind in Extra.
var knownIssueKeys = func() map[string]struct{} {
	t := reflect.TypeOf(Issue{})
	keys := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			keys[name] = struct{}{}
		}
	}
	return keys
}()

// marshalNoEscape encodes v without HTML-escaping < > and &. json.Marshal always
// escapes them, and an outer encoder's SetEscapeHTML(false) cannot undo it: the
// compact pass only ever escapes. Go source stored in an upstream "design" field
// would otherwise come back as "ch <- x".
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// UnmarshalJSON decodes the known fields and stashes every other key in Extra.
func (i *Issue) UnmarshalJSON(data []byte) error {
	type plain Issue // a defined type inherits no methods, so this cannot recurse
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*i = Issue(p)

	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return err
	}
	for k := range all {
		if _, known := knownIssueKeys[k]; known {
			delete(all, k)
		}
	}
	if len(all) > 0 {
		i.Extra = all
	}
	return nil
}

// MarshalJSON emits the known fields, then merges Extra back in.
func (i Issue) MarshalJSON() ([]byte, error) {
	type plain Issue
	b, err := marshalNoEscape(plain(i))
	if err != nil {
		return nil, err
	}
	if len(i.Extra) == 0 {
		return b, nil // no detour through a map; struct key order survives
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range i.Extra {
		if _, exists := m[k]; !exists { // a known key always wins
			m[k] = v
		}
	}
	return marshalNoEscape(m)
}
```

- [ ] **Step 6: Run the type-level tests to verify they pass**

```bash
go test ./internal/types/ -run TestIssue -v
```

Expected: PASS, three tests.

- [ ] **Step 7: Write the failing store-level regression test**

This is the test that actually pins the bug: a write to *one* issue must not damage *another*.

Create `internal/store/store_test.go`:

```go
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
```

- [ ] **Step 8: Run the store test to verify it passes**

It exercises code written in Step 5, so it should pass immediately. Run it to confirm the wiring, since `store.go` was not modified:

```bash
go test ./internal/store/ -run TestSavePreservesUnknownFields -v
```

Expected: PASS.

To see it fail as a sanity check, `git stash` the `types.go` change and rerun. Expected then: FAIL with `design HTML-escaped or lost on disk`. Restore with `git stash pop`.

- [ ] **Step 9: Verify against the real beads-tui file (canary, read-only)**

Prove the fix on the actual 164-issue file without writing to it:

```bash
cd /tmp && trash -f canary 2>/dev/null; mkdir -p canary/.beads
cp ~/projects/beads-tui/.beads/issues.jsonl canary/.beads/
printf 'issue-prefix: tui\n' > canary/.beads/config.yaml
cd ~/projects/bd-lite && go build -o /tmp/canary/bd . && cd /tmp/canary
cp .beads/issues.jsonl /tmp/canary/before.jsonl
BEADS_DIR=/tmp/canary/.beads ./bd comment tui-6f1 "canary" >/dev/null
python3 - <<'PY'
import json
before = {json.loads(l)['id']: json.loads(l) for l in open('/tmp/canary/before.jsonl')}
after  = {json.loads(l)['id']: json.loads(l) for l in open('/tmp/canary/.beads/issues.jsonl')}
lost = [(i, k) for i, b in before.items() for k in b
        if k not in after.get(i, {}) ]
print("issues before/after:", len(before), len(after))
print("LOST FIELDS:", lost or "none")
PY
```

Expected: `issues before/after: 164 164` and `LOST FIELDS: none`.

Then clean up: `trash /tmp/canary`.

- [ ] **Step 10: Full build and test**

```bash
go build ./... && go test ./...
```

Expected: all packages `ok` or `[no test files]`.

- [ ] **Step 11: Commit and close**

```bash
bd close bd-lite-g0m -r "types.Issue now round-trips unknown JSONL keys via Extra map; marshalNoEscape prevents HTML re-escaping of design/notes"
git add internal/types/ internal/store/store_test.go .beads/issues.jsonl
git commit -F - <<'EOF'
Preserve unknown JSONL fields across the load/save round trip

store.Save() re-encodes every issue from types.Issue, so any JSON key
absent from that struct was destroyed on the next write of the file,
not just of that issue. Against beads-tui's tracker that was design on
6 issues and notes on 5.

Unknown keys now land in Issue.Extra and are merged back on marshal.
The known-key set is derived by reflecting over the struct tags so it
cannot drift as fields are added.

MarshalJSON encodes through marshalNoEscape rather than json.Marshal.
An outer SetEscapeHTML(false) cannot undo escaping applied inside a
Marshaler, and upstream design fields hold Go source containing <- and &&.

bd-lite-g0m
EOF
```

---

## Task 2: The `internal/actor` package (bd-lite-gh8, part 1)

**Files:**
- Create: `internal/actor/actor.go`
- Create: `internal/actor/actor_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func actor.Name() string`. Returns the creator identity, or `""` when nothing resolves. Callers must treat `""` as "omit the field".

- [ ] **Step 1: Claim the issue**

```bash
bd update bd-lite-gh8 -s in_progress
```

- [ ] **Step 2: Write the failing test**

Create `internal/actor/actor_test.go`:

```go
package actor

import "testing"

// stubGit replaces the git lookup so tests never shell out or depend on the
// developer's real git config.
func stubGit(t *testing.T, name string) {
	t.Helper()
	orig := gitUserName
	gitUserName = func() string { return name }
	t.Cleanup(func() { gitUserName = orig })
}

func TestName(t *testing.T) {
	tests := []struct {
		name     string
		bdActor  string
		gitName  string
		user     string
		want     string
	}{
		{"BD_ACTOR wins over git and USER", "ci-bot", "Andy Nutter-Upham", "andy", "ci-bot"},
		{"git wins over USER", "", "Andy Nutter-Upham", "andy", "Andy Nutter-Upham"},
		{"USER is the last resort", "", "", "andy", "andy"},
		{"empty when nothing resolves", "", "", "", ""},
		{"BD_ACTOR is trimmed", "  ci-bot  ", "Andy Nutter-Upham", "andy", "ci-bot"},
		{"blank BD_ACTOR falls through", "   ", "Andy Nutter-Upham", "andy", "Andy Nutter-Upham"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BD_ACTOR", tt.bdActor)
			t.Setenv("USER", tt.user)
			stubGit(t, tt.gitName)

			if got := Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run the test to verify it fails**

```bash
go test ./internal/actor/ -v
```

Expected: FAIL. The package does not exist: `no Go files in .../internal/actor`.

- [ ] **Step 4: Write the implementation**

Create `internal/actor/actor.go`:

```go
// Package actor resolves the identity recorded as the creator of new records.
package actor

import (
	"os"
	"os/exec"
	"strings"
)

// gitUserName is a package variable so tests can substitute it without building
// a temporary git repository.
var gitUserName = func() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Name returns the identity to stamp on issues and comments this process
// creates: $BD_ACTOR, else git config user.name, else $USER. An empty result
// means the caller should omit the field rather than store a placeholder.
//
// Call this from write paths only. It forks a git subprocess, so it must not be
// hoisted into rootCmd.PersistentPreRunE, where bd list and bd show would pay
// for it on every invocation.
func Name() string {
	if a := strings.TrimSpace(os.Getenv("BD_ACTOR")); a != "" {
		return a
	}
	if n := gitUserName(); n != "" {
		return n
	}
	return strings.TrimSpace(os.Getenv("USER"))
}
```

- [ ] **Step 5: Run the test to verify it passes**

```bash
go test ./internal/actor/ -v
```

Expected: PASS, six subtests.

- [ ] **Step 6: Full build and test, then commit**

```bash
go build ./... && go test ./...
git add internal/actor/
git commit -F - <<'EOF'
Add internal/actor to resolve the creator identity

Resolution is $BD_ACTOR, then git config user.name, then $USER, and an
empty result means the caller omits the field rather than storing a
placeholder. This is upstream's chain minus the --actor flag, the
$BEADS_ACTOR alias, and the literal "unknown" sentinel.

gitUserName is a package var so tests substitute it rather than
building a temp git repo.

bd-lite-gh8
EOF
```

---

## Task 3: Stamp and display `created_by` (bd-lite-gh8, part 2)

**Files:**
- Modify: `internal/types/types.go` (one field)
- Modify: `cmd/create.go:43-53`
- Modify: `cmd/comment.go:1-35`
- Modify: `internal/output/output.go:58`
- Modify: `internal/output/output_test.go`

**Interfaces:**
- Consumes: `actor.Name() string` from Task 2. `types.Issue.Extra` from Task 1 (indirectly: `knownIssueKeys` picks up the new tag by reflection, no edit needed).
- Produces: `types.Issue.CreatedBy string`, tagged `json:"created_by,omitempty"`.

- [ ] **Step 1: Write the failing output test**

Append to `internal/output/output_test.go`:

```go
// created_by is optional forever. An issue that has it shows who; an issue that
// lacks it must render exactly as it did before the field existed.
func TestPrintIssueCreatedBySuffix(t *testing.T) {
	now := time.Date(2026, 7, 9, 12, 7, 0, 0, time.UTC)
	mk := func(createdBy string) *types.Issue {
		return &types.Issue{
			ID: "bd-lite-x1y", Title: "Record whodunnit",
			Status: types.StatusOpen, IssueType: types.TypeFeature,
			CreatedBy: createdBy,
			CreatedAt: now, UpdatedAt: now,
		}
	}

	with := captureStdout(t, func() { PrintIssue(mk("Andy Nutter-Upham")) })
	if !strings.Contains(with, "Created:  2026-07-09 12:07 by Andy Nutter-Upham") {
		t.Errorf("expected creator suffix on the Created line, got:\n%s", with)
	}

	without := captureStdout(t, func() { PrintIssue(mk("")) })
	if !strings.Contains(without, "Created:  2026-07-09 12:07\n") {
		t.Errorf("expected unchanged Created line when created_by is absent, got:\n%s", without)
	}
	if strings.Contains(without, " by ") {
		t.Errorf("rendered a creator suffix for an issue that has none:\n%s", without)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

```bash
go test ./internal/output/ -run TestPrintIssueCreatedBy -v
```

Expected: FAIL, compile error `unknown field CreatedBy in struct literal of type types.Issue`.

- [ ] **Step 3: Add the field**

In `internal/types/types.go`, insert between `Assignee` and `CreatedAt`:

```go
	Assignee     string        `json:"assignee,omitempty"`
	CreatedBy    string        `json:"created_by,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
```

No change to `knownIssueKeys`; it reflects over the tags.

No change to `Validate`. The field is optional in every state.

- [ ] **Step 4: Render it in `bd show`**

In `internal/output/output.go`, replace line 58:

```go
	fmt.Printf("  Created:  %s\n", issue.CreatedAt.Format("2006-01-02 15:04"))
```

with:

```go
	created := issue.CreatedAt.Format("2006-01-02 15:04")
	if issue.CreatedBy != "" {
		created += " by " + issue.CreatedBy
	}
	fmt.Printf("  Created:  %s\n", created)
```

- [ ] **Step 5: Run the output test to verify it passes**

```bash
go test ./internal/output/ -run TestPrintIssueCreatedBy -v
```

Expected: PASS.

- [ ] **Step 6: Stamp it on create**

In `cmd/create.go`, add the import and the field. The import block becomes:

```go
import (
	"fmt"
	"strings"
	"time"

	"bd-lite/internal/actor"
	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)
```

and the struct literal at line 43 gains one line, after `Assignee`:

```go
	issue := &types.Issue{
		Title:       args[0],
		Description: createDescription,
		Status:      types.StatusOpen,
		Priority:    createPriority,
		IssueType:   types.IssueType(createType),
		Assignee:    createAssignee,
		CreatedBy:   actor.Name(),
		Labels:      createLabels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
```

- [ ] **Step 7: Unify the comment author**

Replace the whole of `cmd/comment.go` with:

```go
package cmd

import (
	"bd-lite/internal/actor"
	"bd-lite/internal/output"

	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment <id> <text>",
	Short: "Add a comment to an issue",
	Args:  cobra.ExactArgs(2),
	RunE:  runComment,
}

func init() {
	rootCmd.AddCommand(commentCmd)
}

func runComment(cmd *cobra.Command, args []string) error {
	id, err := st.ResolveID(args[0])
	if err != nil {
		return err
	}

	if err := st.AddComment(id, args[1], actor.Name()); err != nil {
		return err
	}

	if err := saveStore(); err != nil {
		return err
	}

	issue := st.Get(id)
	// Return the newly added comment (last in the list), matching beads behavior
	comment := issue.Comments[len(issue.Comments)-1]
	output.PrintComment(comment)
	return nil
}
```

The `os/user` import is gone. New comments now record the same identity string as `created_by` instead of the OS username.

- [ ] **Step 8: Verify end to end in a scratch repo**

Prove both halves of the contract: the field renders when present, and its absence
leaves the line exactly as it was.

```bash
go build -o /tmp/bdw . || exit 1
DEMO=$(mktemp -d) && cd "$DEMO" && /tmp/bdw init --prefix demo

BD_ACTOR="Ada Lovelace" /tmp/bdw create "First bead"
ID=$(/tmp/bdw list --json | python3 -c 'import sys,json; print(json.load(sys.stdin)[0]["id"])')
/tmp/bdw show "$ID"
```

Expected: the `Created:` line ends with ` by Ada Lovelace`.

Now strip the field back out, exactly as a pre-whodunnit file would look, and confirm
the line reverts:

```bash
python3 - <<'PY'
import json, pathlib
p = pathlib.Path('.beads/issues.jsonl')
rows = [json.loads(l) for l in p.read_text().splitlines() if l.strip()]
for r in rows:
    r.pop('created_by', None)
p.write_text(''.join(json.dumps(r) + '\n' for r in rows))
PY
/tmp/bdw show "$ID"
```

Expected: a `Created:` line with no ` by ` suffix, and no error.

Also confirm `bd comment` now stamps the same identity:

```bash
BD_ACTOR="Ada Lovelace" /tmp/bdw comment "$ID" "hello"
```

Expected output: `[<timestamp>] Ada Lovelace: hello`

Clean up:

```bash
cd /home/andy/projects/bd-lite && trash "$DEMO" && trash /tmp/bdw
```

- [ ] **Step 9: Full build and test, then commit and close**

```bash
go build ./... && go test ./...
bd close bd-lite-gh8 -r "created_by stamped on create via internal/actor; comment author unified to the same helper; bd show suffixes the Created line and is unchanged when the field is absent"
git add internal/types/types.go internal/output/ cmd/create.go cmd/comment.go .beads/issues.jsonl
git commit -F - <<'EOF'
Record created_by on new issues and comments

Adopts upstream beads' created_by wire format rather than inventing a
field. cmd/comment.go moves from the OS username to the same resolver,
so one person no longer appears in a single file under two spellings.
Existing comments keep their stored value.

bd show suffixes its existing Created line and renders an issue lacking
the field byte-identically to before. The field is optional
permanently; nothing backfills it.

bd-lite-gh8
EOF
```

---

## Task 4: Age column in `bd list` and `bd ready` (bd-lite-03t)

Independent of Tasks 2 and 3, but edits the same file. Land it after them, not beside them.

**Files:**
- Modify: `internal/output/output.go:76-82` (`PrintIssueList`) and `:250-261` (`Age`)
- Modify: `internal/output/output_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `func formatAge(d time.Duration) string` (unexported). `Age(t time.Time) string` keeps its existing exported signature and becomes a one-line wrapper.

- [ ] **Step 1: Claim the issue**

```bash
bd update bd-lite-03t -s in_progress
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/output/output_test.go`:

```go
// Age() is a clock read wrapped around pure formatting. Test the pure part
// against fixed durations so the assertions cannot drift with wall time.
func TestFormatAge(t *testing.T) {
	const day = 24 * time.Hour
	tests := []struct {
		d    time.Duration
		want string
	}{
		{-5 * time.Minute, "0m"}, // clock skew, or a hand-edited future timestamp
		{0, "0m"},
		{59 * time.Minute, "59m"},
		{time.Hour, "1h"},
		{23 * time.Hour, "23h"},
		{day, "1d"},
		{89 * day, "89d"},
		{90 * day, "3mo"},
		{364 * day, "12mo"},
		{365 * day, "1y"},
		{730 * day, "2y"},
	}
	for _, tt := range tests {
		if got := formatAge(tt.d); got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// The column is 4 wide because "12mo" is the widest value formatAge can emit:
// months roll over to years at 365 days.
func TestFormatAgeNeverExceedsFourChars(t *testing.T) {
	const day = 24 * time.Hour
	for d := time.Duration(0); d < 800*day; d += 7 * time.Hour {
		if got := formatAge(d); len(got) > 4 {
			t.Fatalf("formatAge(%v) = %q, wider than the 4-char column", d, got)
		}
	}
}

func TestPrintIssueListShowsAge(t *testing.T) {
	issue := &types.Issue{
		ID: "bd-lite-c3d", Title: "Fix dep tree direction",
		Status: types.StatusOpen, Priority: 1, IssueType: types.TypeBug,
		CreatedAt: time.Now().Add(-12 * 24 * time.Hour),
	}

	out := captureStdout(t, func() { PrintIssueList([]*types.Issue{issue}) })

	if !strings.Contains(out, "12d") {
		t.Errorf("expected a 12d age column, got:\n%s", out)
	}
	// The age sits between the type and the title, not after the title.
	agePos, titlePos := strings.Index(out, "12d"), strings.Index(out, "Fix dep tree")
	if agePos == -1 || titlePos == -1 || agePos > titlePos {
		t.Errorf("expected age before title, got:\n%s", out)
	}
}
```

- [ ] **Step 3: Run them to verify they fail**

```bash
go test ./internal/output/ -run 'TestFormatAge|TestPrintIssueListShowsAge' -v
```

Expected: FAIL, compile error `undefined: formatAge`.

- [ ] **Step 4: Split `Age` and extend it**

In `internal/output/output.go`, replace the whole `Age` function (lines 250-261):

```go
// Age returns a human-readable age string.
func Age(t time.Time) string { return formatAge(time.Since(t)) }

// formatAge renders a duration in the narrowest useful unit. It tops out at
// years so a stale backlog item reads "2y" rather than "731d", and never
// exceeds four characters, which is the width of the bd list age column.
func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0 // clock skew, or a hand-edited future timestamp
	}
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 90*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy", int(d.Hours()/24/365))
	}
}
```

- [ ] **Step 5: Add the column**

In `internal/output/output.go`, inside `PrintIssueList`, replace:

```go
	for _, issue := range issues {
		status := statusIcon(issue.Status)
		fmt.Printf("%s %s  P%d  %-12s  %s\n",
			status, issue.ID, issue.Priority, issue.IssueType, issue.Title)
	}
```

with:

```go
	for _, issue := range issues {
		status := statusIcon(issue.Status)
		fmt.Printf("%s %s  P%d  %-12s %4s  %s\n",
			status, issue.ID, issue.Priority, issue.IssueType,
			Age(issue.CreatedAt), issue.Title)
	}
```

`store.go:93` already sorts issues by `CreatedAt`, so the column reads monotonically down the list. `bd ready` shares this printer and gets the column too.

- [ ] **Step 6: Run the tests to verify they pass**

```bash
go test ./internal/output/ -v
```

Expected: PASS, including the pre-existing dependency-wording and tree tests.

- [ ] **Step 7: Eyeball the real output**

```bash
go build -o /tmp/bdw . && /tmp/bdw list --all
```

Expected shape, with the age right-aligned in a 4-wide column between type and title:

```
[x] bd-lite-bi9  P2  feature       9d  bd show: resolve bare suffix code when unambiguous
[ ] bd-lite-g0m  P0  bug           0m  store.Save() drops unknown JSONL fields ...
```

- [ ] **Step 8: Full build and test, then commit and close**

```bash
go build ./... && go test ./...
bd close bd-lite-03t -r "Age column wired into PrintIssueList; Age() split into a clock read plus pure formatAge, extended through mo/y, negatives clamped"
git add internal/output/ .beads/issues.jsonl
git commit -F - <<'EOF'
Show issue age in bd list and bd ready

output.Age() had been defined and called from nowhere. Wire it into
PrintIssueList as a right-aligned 4-wide column between type and title;
12mo is the widest value it can emit.

Split the clock read out of the formatting so the boundaries can be
tested against fixed durations, extend it through months and years so a
year-old backlog item no longer reads "731d", and clamp negative
durations to zero.

This changes default bd list output for anything parsing it.

bd-lite-03t
EOF
```

---

## Task 5: File the beads-tui sister ticket

**Gated on Task 1.** Filing this ticket is itself a `bd` write inside `~/projects/beads-tui`, which rewrites all 164 lines of that repo's `issues.jsonl`. Until Task 1 is committed and the `bd` on `$PATH` is rebuilt from it, that write destroys `design` on 6 issues and `notes` on 5.

**Files:** none in this repo. Writes `~/projects/beads-tui/.beads/issues.jsonl`.

**Interfaces:**
- Consumes: `types.Issue.Extra` from Task 1, by way of the installed `bd` binary.

- [ ] **Step 1: Reinstall `bd` from the fixed source**

```bash
cd ~/projects/bd-lite && ./install.sh && which bd
```

- [ ] **Step 2: Confirm the installed binary preserves unknown keys**

Do not skip this. It is the guard between you and 11 damaged issues.

```bash
cd /tmp && trash -f guard 2>/dev/null; mkdir -p guard/.beads
cp ~/projects/beads-tui/.beads/issues.jsonl guard/.beads/
printf 'issue-prefix: tui\n' > guard/.beads/config.yaml
cp guard/.beads/issues.jsonl guard/before.jsonl
BEADS_DIR=/tmp/guard/.beads bd comment tui-6f1 "guard check" >/dev/null
python3 - <<'PY'
import json
before = {json.loads(l)['id']: json.loads(l) for l in open('/tmp/guard/before.jsonl')}
after  = {json.loads(l)['id']: json.loads(l) for l in open('/tmp/guard/.beads/issues.jsonl')}
lost = sorted({k for i, b in before.items() for k in b if k not in after.get(i, {})})
print("issues:", len(before), "->", len(after))
print("LOST FIELDS:", lost or "none")
PY
```

Expected: `issues: 164 -> 164` and `LOST FIELDS: none`.

**If any field is listed as lost, stop.** Task 1 did not do its job. Do not proceed.

Clean up: `trash /tmp/guard`.

- [ ] **Step 3: File the ticket**

```bash
cd ~/projects/beads-tui && bd create "Surface created_by in the issue detail pane" \
  -t feature -p 2 \
  -d "bd-lite now records created_by on issue create (bd-lite-gh8), adopting upstream beads' wire format. beads-tui drops the field on parse.

internal/parser/types.go:17 declares Assignee and no CreatedBy. Add:
  CreatedBy string \`json:\"created_by,omitempty\"\`

internal/formatting/details.go:100 already prints a Created: line in its Metadata block. Suffix it the way bd-lite's bd show does, omitting the suffix when the field is absent so existing issues render unchanged:
  Created: 2026-07-09 12:07 by Andy Nutter-Upham

Scope to the JSONL parser path only.

OPEN QUESTION, do not assume: internal/storage/sqlite.go:195 selects an explicit column list from beads.db. Adding created_by to that list would error against any older database lacking the column. Decide whether to guard the column, migrate, or leave the SQLite path unaware.

Cross-repo context: bd-lite's spec is at ~/projects/bd-lite/docs/superpowers/specs/2026-07-09-whodunnit-design.md. Filing this ticket was blocked on bd-lite-g0m, because bd-lite's store.Save() used to destroy design and notes fields on this very repo's tracker."
```

- [ ] **Step 4: Verify nothing was damaged by the filing**

```bash
cd ~/projects/beads-tui && git diff --stat .beads/issues.jsonl
python3 - <<'PY'
import json, subprocess
old = subprocess.run(['git','show','HEAD:.beads/issues.jsonl'],
                     capture_output=True, text=True, cwd='.').stdout
before = {json.loads(l)['id']: json.loads(l) for l in old.splitlines() if l.strip()}
after  = {json.loads(l)['id']: json.loads(l)
          for l in open('.beads/issues.jsonl') if l.strip()}
lost = sorted({k for i, b in before.items() for k in b if k not in after.get(i, {})})
print("issues:", len(before), "->", len(after), "(expect +1)")
print("LOST FIELDS:", lost or "none")
PY
```

Expected: `164 -> 165` and `LOST FIELDS: none`.

If a field was lost, `git checkout .beads/issues.jsonl` to restore, and reopen bd-lite-g0m.

- [ ] **Step 5: Commit the new ticket in beads-tui**

```bash
cd ~/projects/beads-tui && git add .beads/issues.jsonl
git commit -m "bd: file created_by surfacing ticket"
```

- [ ] **Step 6: Push both repos**

```bash
cd ~/projects/bd-lite && git push origin main
cd ~/projects/beads-tui && git push
```

---

## Self-Review

**Spec coverage.** Every spec section maps to a task: data model and absent values to Tasks 1 and 3; identity resolution to Task 2; which-records-carry-it to Task 3; the bd-lite-g0m prerequisite to Task 1; `bd show` display to Task 3; the age column to Task 4; testing to the test steps in each; the beads-tui sister ticket to Task 5. JSON mode needs no task, as the spec states: the struct tag carries it.

**Placeholders.** None. Every code step shows the code. Every command shows expected output.

**Type consistency.** `actor.Name()` is named identically in Tasks 2 and 3. `formatAge` is defined in Task 4 Step 4 and used in Task 4 Steps 2 and 5. `Issue.Extra`, `knownIssueKeys`, and `marshalNoEscape` are defined in Task 1 Steps 4 and 5, and Task 3 relies on `knownIssueKeys` reflecting the new `CreatedBy` tag without an edit, which it does because it iterates the struct's fields.

**Ordering.** Task 1 blocks Task 5 for the data-loss reason stated. Tasks 3 and 4 both edit `internal/output/output.go`, in `PrintIssue` and `PrintIssueList` respectively, so they must land sequentially rather than in parallel worktrees. Task 2 blocks Task 3.

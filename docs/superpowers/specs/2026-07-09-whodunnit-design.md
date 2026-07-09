# Recording who created a bead

Date: 2026-07-09
Status: approved, not yet implemented

## Problem

An issue in `.beads/issues.jsonl` records when it was created but not by whom. The
information is available at creation time and costs one field to keep.

Upstream beads already solved this. `beads/internal/types/types.go:40` carries

```go
CreatedBy string `json:"created_by,omitempty"` // Who created this issue (GH#748)
```

and resolves the value through `getActorWithGit()` (`beads/cmd/bd/main.go:137`).
bd-lite is wire-compatible with that format, so this work adopts the field rather
than inventing one.

## Decisions

### What the value means

`created_by` identifies **which human** created the issue, not which tool typed the
command. Claude Code sessions run as `$USER`, so an agent-vs-human distinction would
need `$CLAUDECODE` sniffing; that was considered and rejected. A creator is a person.

### How the value is resolved

New package `internal/actor`, one exported function:

```go
// Name returns the identity to stamp on records this process creates.
// $BD_ACTOR -> git config user.name -> $USER -> "" (caller omits the field).
func Name() string
```

`gitUserName` is a package-level `var func() string` so tests can substitute it
without constructing a temp git repo.

This is a trimmed version of upstream's chain. The `--actor` flag, the `$BEADS_ACTOR`
alias, and the literal `"unknown"` sentinel are all dropped. `$BD_ACTOR` survives
because it gives tests and scripts a seam. An unresolvable identity yields the empty
string, and the caller omits the field entirely rather than writing a placeholder.

**`Name()` is called lazily, at the two write sites.** It must not move into
`rootCmd.PersistentPreRunE`, which would fork a `git` subprocess on every `bd list`,
`bd show`, and `bd ready`.

### Which records carry it

| Record | Field | Before | After |
|---|---|---|---|
| Issue | `created_by` | absent | `actor.Name()` |
| Comment | `author` | `user.Current().Username` | `actor.Name()` |
| Dependency | `created_by` | never assigned | never assigned |

`cmd/comment.go` currently records the OS username while the new issue field would
record the git display name. Left alone, one person would appear in a single file
under two spellings with nothing to connect them. Both now come from `actor.Name()`.

This changes observable behavior: new comments record `Andy Nutter-Upham` where they
used to record `andy`. Existing comments keep their stored value and render unchanged.

`Dependency.CreatedBy` exists in the struct (`internal/types/types.go:93`) and is
assigned by nothing (`store.go:377` sets only `CreatedAt`). Populating it would
produce data no command displays. It stays dead.

`bd update` and `bd close` do not touch `created_by`. It is a fact about creation.

### Absent values

`created_by` is optional permanently, not transitionally. Nothing backfills it. An
issue written by upstream `bd`, by an older bd-lite, or by hand is valid without it,
and `bd show` renders such an issue byte-identically to today.

## Prerequisite: bd-lite-g0m

Discovered while scoping the beads-tui ticket, and filed as a P0 bug.

`store.Save()` and `SaveToFile()` re-encode every issue from `types.Issue`, and
`loadFromFile()` discards unrecognized keys at unmarshal time. Any JSON field absent
from the struct is destroyed on the next write of the file, not just on the next write
of that issue.

Measured against `~/projects/beads-tui/.beads/issues.jsonl` (164 issues, tracked by
real beads until February 2026, now on bd-lite by way of `$PATH`):

| Field | Issues affected |
|---|---|
| `design` | 6 (`tui-tjf` holds a theme-system architecture document) |
| `notes` | 5 (`tui-1oj` holds a root-cause writeup) |
| `created_by` | 1 (`tui-6f1`) |

A single `bd create` there rewrites all 164 lines and drops all of it. The loss is
recoverable, since `.beads/.gitignore` keeps `*.jsonl` and the file is tracked, but
nothing reports it.

Filing the beads-tui sister ticket with the `bd` on `$PATH` would itself have
triggered the loss. That is why the fix lands first.

**Fix:** preserve unknown keys generically.

```go
type Issue struct {
    // ...known fields...
    Extra map[string]json.RawMessage `json:"-"`
}

func (i *Issue) UnmarshalJSON(b []byte) error  // known fields via type alias; rest into Extra
func (i Issue)  MarshalJSON() ([]byte, error)  // merge Extra back over the known encoding
```

The set of known keys is derived by reflecting over the struct's json tags, so it
cannot drift from the struct. Adding `CreatedBy` later requires no change here.

The same treatment is required on the nested `Dependency` and `Comment` structs, not
just on `Issue`. Upstream writes `metadata` on dependencies. Scoping the fix to `Issue`
leaves those objects re-encoded from their Go definitions and their unknown keys
destroyed, which is the identical bug one level down. See bd-lite-ae3.

Known cost: an issue carrying `Extra` marshals through a `map`, so its keys emit in
alphabetical order rather than struct order. This churns those lines once in the jsonl
diff and is cosmetic thereafter. Measured against beads-tui's 164-issue tracker, one
`bd comment` rewrites 78 lines: 12 reorder top-level keys, 69 carried dependency
metadata, and 7 change only because a previously HTML-escaped byte is now written
literally.

## Display

### `bd show`

Suffix the existing Created line. No new line, no reordering.

```
  Created:  2026-07-09 12:07 by Andy Nutter-Upham    # created_by present
  Created:  2026-06-30 17:45                         # absent
```

### `bd list` and `bd ready`

Independent of whodunnit, sharing only the file. `internal/output/output.go:251`
defines an `Age()` helper that nothing calls. Wire it up as a right-aligned column
between type and title, four characters wide, which fits the longest value `12mo`.

Rendered by `"%s %s  P%d  %-12s %4s  %s\n"`:

```
[ ] bd-lite-x1y  P2  feature        2h  Record whodunnit on beads
[>] bd-lite-c3d  P1  bug           12d  Fix dep tree direction
[ ] bd-lite-q7r  P4  task          12mo  Rename internal/output
[ ] bd-lite-z8w  P4  chore          2y  Drop the vendored beads checkout
```

`12mo` is the widest value the formatter can emit: months roll over to years at 365
days, and no plausible year count exceeds two digits.

The column does not read monotonically. `Save()` (`store.go:93`) sorts by `CreatedAt`,
but `bd list` renders `Filter()` and `bd ready` renders `Ready()`, both of which sort
by priority first. Age therefore restarts at every priority boundary.

`Age()` currently bottoms out at days, rendering a year-old backlog item as `731d`.
Extend it through months and years, and split the pure formatting out of the clock
read so it can be tested against fixed durations:

```go
func Age(t time.Time) string { return formatAge(time.Since(t)) }

func formatAge(d time.Duration) string {
    if d < 0 { d = 0 }   // clock skew, or a hand-edited future timestamp
    switch {
    case d < time.Hour:        return fmt.Sprintf("%dm",  int(d.Minutes()))
    case d < 24*time.Hour:     return fmt.Sprintf("%dh",  int(d.Hours()))
    case d < 90*24*time.Hour:  return fmt.Sprintf("%dd",  int(d.Hours()/24))
    case d < 365*24*time.Hour: return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
    default:                   return fmt.Sprintf("%dy",  int(d.Hours()/24/365))
    }
}
```

This changes default `bd list` output for anything parsing it. Accepted.

### JSON mode

`created_by` appears automatically from the struct tag. No change to `printJSON`.

## Testing

`internal/actor/actor_test.go`, using `t.Setenv` and a stubbed `gitUserName`:
`$BD_ACTOR` beats git; git beats `$USER`; `$USER` is last; all empty yields `""`.

`internal/output/output_test.go`, using the existing `captureStdout` helper:
`formatAge` as a table test over fixed `time.Duration` values, including the negative
and boundary cases; `PrintIssue` with and without `CreatedBy`, asserting the absent
case is unchanged; `PrintIssueList` renders the age column.

`internal/types/types_test.go`, new: an issue carrying `design` and `notes` survives
an unmarshal/marshal round trip with both fields intact and their values unmodified.

`cmd/` has no test harness today and gains none here.

## Work breakdown

Four units. The first blocks the last.

1. **bd-lite-g0m** (bug, P0) preserve unknown JSONL fields across the load/save round
   trip.
2. **whodunnit** (feature) `internal/actor`, `created_by` on create, unified comment
   author, `bd show` suffix.
3. **age column** (feature) wire up `Age()`, extend past days, add the column.
4. **beads-tui sister ticket** (feature, filed in `~/projects/beads-tui`) surface
   `created_by` in the detail pane. Filed only after (1) lands, because filing it is
   itself a write to that repo's tracker.

Units 2 and 3 both edit `internal/output/output.go`, in different functions. They are
independent but want to land sequentially rather than in parallel worktrees.

## beads-tui sister ticket

`internal/parser/types.go:17` declares `Assignee` and no `CreatedBy`, so the field is
dropped when the TUI parses a bd-lite file. `internal/formatting/details.go:100`
already prints a `Created:` line in its Metadata block, the same shape bd-lite uses,
so the change mirrors this spec exactly.

The wrinkle is a second read path. `internal/storage/sqlite.go:195` selects an explicit
column list from `beads.db`. Adding `created_by` to that list would error against any
older database lacking the column. The ticket scopes to the JSONL parser path and
records the SQLite path as an open question rather than assuming both.

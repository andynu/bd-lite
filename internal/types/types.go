package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Issue represents a trackable work item.
// Wire-compatible with beads issues.jsonl format.
type Issue struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description,omitempty"`
	Status       Status        `json:"status"`
	Priority     int           `json:"priority"`
	IssueType    IssueType     `json:"issue_type"`
	Assignee     string        `json:"assignee,omitempty"`
	CreatedBy    string        `json:"created_by,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	ClosedAt     *time.Time    `json:"closed_at,omitempty"`
	CloseReason  string        `json:"close_reason,omitempty"`
	Labels       []string      `json:"labels,omitempty"`
	Dependencies []*Dependency `json:"dependencies,omitempty"`
	Comments     []*Comment    `json:"comments,omitempty"`

	// Extra carries JSONL keys this build of bd-lite does not model. Upstream
	// beads writes design, notes, acceptance_criteria and others; a bd-lite
	// write rewrites every line of the file, so anything not round-tripped here
	// is destroyed. Populated by UnmarshalJSON, re-emitted by MarshalJSON,
	// never read by application code.
	Extra map[string]json.RawMessage `json:"-"`
}

// Validate checks if the issue has valid field values.
func (i *Issue) Validate() error {
	if len(i.Title) == 0 {
		return fmt.Errorf("title is required")
	}
	if len(i.Title) > 500 {
		return fmt.Errorf("title must be 500 characters or less (got %d)", len(i.Title))
	}
	if i.Priority < 0 || i.Priority > 4 {
		return fmt.Errorf("priority must be between 0 and 4 (got %d)", i.Priority)
	}
	if !i.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", i.Status)
	}
	if !i.IssueType.IsValid() {
		return fmt.Errorf("invalid issue type: %s", i.IssueType)
	}
	if i.Status == StatusClosed && i.ClosedAt == nil {
		return fmt.Errorf("closed issues must have closed_at timestamp")
	}
	if i.Status != StatusClosed && i.ClosedAt != nil {
		return fmt.Errorf("non-closed issues cannot have closed_at timestamp")
	}
	return nil
}

// jsonKeys returns the set of JSON keys a struct type models directly,
// derived from its tags so it cannot drift as fields are added. A field
// tagged json:"-" (such as Extra itself) is correctly excluded.
func jsonKeys(t reflect.Type) map[string]struct{} {
	keys := make(map[string]struct{}, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			keys[name] = struct{}{}
		}
	}
	return keys
}

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
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// splitExtra decodes data into known (a pointer to a method-less struct
// alias), then returns whichever keys data carries that knownKeys does not
// model. known is decoded independently of the returned map, so callers
// still get every known field populated even when Extra ends up nil.
func splitExtra(data []byte, known any, knownKeys map[string]struct{}) (map[string]json.RawMessage, error) {
	if err := json.Unmarshal(data, known); err != nil {
		return nil, err
	}

	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	for k := range all {
		if _, known := knownKeys[k]; known {
			delete(all, k)
		}
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

// mergeExtra encodes known, then merges extra back over it. When extra is
// empty it returns the struct encoding untouched, preserving key order.
func mergeExtra(known any, extra map[string]json.RawMessage) ([]byte, error) {
	b, err := marshalNoEscape(known)
	if err != nil {
		return nil, err
	}
	if len(extra) == 0 {
		return b, nil // no detour through a map; struct key order survives
	}

	// extra is non-empty: encoding through this map emits keys in alphabetical
	// order (Go sorts map keys), not struct order. That is one-time diff churn
	// on lines that already carry unknown keys, not data loss — every value is
	// preserved as raw bytes.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for k, v := range extra {
		if _, exists := m[k]; !exists { // a known key always wins
			m[k] = v
		}
	}
	return marshalNoEscape(m)
}

// knownIssueKeys is the set of JSON keys Issue models directly.
var knownIssueKeys = jsonKeys(reflect.TypeOf(Issue{}))

// UnmarshalJSON decodes the known fields and stashes every other key in Extra.
func (i *Issue) UnmarshalJSON(data []byte) error {
	type plain Issue // a defined type inherits no methods, so this cannot recurse
	var p plain
	extra, err := splitExtra(data, &p, knownIssueKeys)
	if err != nil {
		return err
	}
	*i = Issue(p) // replaces the whole struct, resetting Extra
	i.Extra = extra
	return nil
}

// MarshalJSON emits the known fields, then merges Extra back in.
func (i Issue) MarshalJSON() ([]byte, error) {
	type plain Issue
	return mergeExtra(plain(i), i.Extra)
}

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusClosed     Status = "closed"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusOpen, StatusInProgress, StatusBlocked, StatusClosed:
		return true
	}
	return false
}

type IssueType string

const (
	TypeBug     IssueType = "bug"
	TypeFeature IssueType = "feature"
	TypeTask    IssueType = "task"
	TypeEpic    IssueType = "epic"
	TypeChore   IssueType = "chore"
)

func (t IssueType) IsValid() bool {
	switch t {
	case TypeBug, TypeFeature, TypeTask, TypeEpic, TypeChore:
		return true
	}
	return false
}

type Dependency struct {
	IssueID     string         `json:"issue_id"`
	DependsOnID string         `json:"depends_on_id"`
	Type        DependencyType `json:"type"`
	CreatedAt   time.Time      `json:"created_at"`
	CreatedBy   string         `json:"created_by,omitempty"`

	// Extra carries JSONL keys this build of bd-lite does not model, such as
	// upstream's "metadata". See Issue.Extra.
	Extra map[string]json.RawMessage `json:"-"`
}

// knownDependencyKeys is the set of JSON keys Dependency models directly.
var knownDependencyKeys = jsonKeys(reflect.TypeOf(Dependency{}))

// UnmarshalJSON decodes the known fields and stashes every other key in Extra.
func (d *Dependency) UnmarshalJSON(data []byte) error {
	type plain Dependency
	var p plain
	extra, err := splitExtra(data, &p, knownDependencyKeys)
	if err != nil {
		return err
	}
	*d = Dependency(p)
	d.Extra = extra
	return nil
}

// MarshalJSON emits the known fields, then merges Extra back in.
func (d Dependency) MarshalJSON() ([]byte, error) {
	type plain Dependency
	return mergeExtra(plain(d), d.Extra)
}

type DependencyType string

const (
	DepBlocks         DependencyType = "blocks"
	DepRelated        DependencyType = "related"
	DepParentChild    DependencyType = "parent-child"
	DepDiscoveredFrom DependencyType = "discovered-from"
)

func (d DependencyType) IsValid() bool {
	switch d {
	case DepBlocks, DepRelated, DepParentChild, DepDiscoveredFrom:
		return true
	}
	return false
}

type Comment struct {
	ID        int64     `json:"id"`
	IssueID   string    `json:"issue_id"`
	Author    string    `json:"author,omitempty"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`

	// Extra carries JSONL keys this build of bd-lite does not model. See
	// Issue.Extra.
	Extra map[string]json.RawMessage `json:"-"`
}

// knownCommentKeys is the set of JSON keys Comment models directly.
var knownCommentKeys = jsonKeys(reflect.TypeOf(Comment{}))

// UnmarshalJSON decodes the known fields and stashes every other key in Extra.
func (c *Comment) UnmarshalJSON(data []byte) error {
	type plain Comment
	var p plain
	extra, err := splitExtra(data, &p, knownCommentKeys)
	if err != nil {
		return err
	}
	*c = Comment(p)
	c.Extra = extra
	return nil
}

// MarshalJSON emits the known fields, then merges Extra back in.
func (c Comment) MarshalJSON() ([]byte, error) {
	type plain Comment
	return mergeExtra(plain(c), c.Extra)
}

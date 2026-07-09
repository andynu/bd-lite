package output

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"bd-lite/internal/store"
	"bd-lite/internal/types"
)

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

// bd dep add A B stores "A depends on B" (B blocks A). The rendered line must
// read in that direction; the dependency type ("blocks") is a label, not a
// verb between the two IDs.
func TestPrintIssueDependencyWording(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	issue := &types.Issue{
		ID:        "bd-lite-aaa",
		Title:     "Dependent issue",
		Status:    types.StatusOpen,
		IssueType: types.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
		Dependencies: []*types.Dependency{
			{IssueID: "bd-lite-aaa", DependsOnID: "bd-lite-bbb", Type: types.DepBlocks},
		},
	}

	out := captureStdout(t, func() { PrintIssue(issue) })

	if !strings.Contains(out, "depends on bd-lite-bbb (blocks)") {
		t.Errorf("expected dependency rendered as \"depends on bd-lite-bbb (blocks)\", got:\n%s", out)
	}
	if strings.Contains(out, "bd-lite-aaa blocks bd-lite-bbb") {
		t.Errorf("dependency rendered with reversed direction (\"A blocks B\" for A-depends-on-B):\n%s", out)
	}
}

func TestPrintTreeRendersRootAndChildren(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	mk := func(id, title string) *types.Issue {
		return &types.Issue{
			ID: id, Title: title,
			Status: types.StatusOpen, IssueType: types.TypeTask,
			CreatedAt: now, UpdatedAt: now,
		}
	}
	root := &store.TreeNode{
		Issue: mk("bd-lite-epc", "Epic"),
		Children: []*store.TreeNode{
			{Issue: mk("bd-lite-cc1", "Child one")},
			{Issue: mk("bd-lite-cc2", "Child two")},
		},
	}

	out := captureStdout(t, func() { PrintTree(root) })

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), out)
	}
	if !strings.HasPrefix(lines[0], "bd-lite-epc") {
		t.Errorf("expected root line first, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "├── bd-lite-cc1") {
		t.Errorf("expected first child connector line, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "└── bd-lite-cc2") {
		t.Errorf("expected last child connector line, got %q", lines[2])
	}
	// The tree draws structure only; it must not print a directional verb
	// that could contradict the stored semantics.
	if strings.Contains(out, "blocks") || strings.Contains(out, "depends") {
		t.Errorf("tree output should not contain directional wording:\n%s", out)
	}
}

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

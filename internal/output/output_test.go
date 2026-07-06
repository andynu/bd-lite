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

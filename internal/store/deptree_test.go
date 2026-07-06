package store

import (
	"testing"
)

func storeWithDeps(t *testing.T, ids []string, deps map[string][]string) *Store {
	t.Helper()
	s := storeWithIDs(ids...)
	for issueID, dependsOn := range deps {
		for _, depID := range dependsOn {
			if err := s.AddDependency(issueID, depID); err != nil {
				t.Fatalf("AddDependency(%s, %s): %v", issueID, depID, err)
			}
		}
	}
	return s
}

// bd dep tree <id> shows what <id> depends on, recursively. For an epic that
// depends on its children, the epic is the root and the children hang under it.
func TestDepTreeFollowsDependencies(t *testing.T) {
	s := storeWithDeps(t,
		[]string{"bd-lite-epc", "bd-lite-cc1", "bd-lite-cc2", "bd-lite-gc1"},
		map[string][]string{
			"bd-lite-epc": {"bd-lite-cc1", "bd-lite-cc2"},
			"bd-lite-cc1": {"bd-lite-gc1"},
		})

	tree, err := s.DepTree("bd-lite-epc")
	if err != nil {
		t.Fatal(err)
	}

	if tree.Issue.ID != "bd-lite-epc" {
		t.Fatalf("root = %s, want bd-lite-epc", tree.Issue.ID)
	}
	if len(tree.Children) != 2 {
		t.Fatalf("epic children = %d, want 2 (its dependencies)", len(tree.Children))
	}
	if tree.Children[0].Issue.ID != "bd-lite-cc1" || tree.Children[1].Issue.ID != "bd-lite-cc2" {
		t.Errorf("epic children = [%s, %s], want [bd-lite-cc1, bd-lite-cc2]",
			tree.Children[0].Issue.ID, tree.Children[1].Issue.ID)
	}
	if len(tree.Children[0].Children) != 1 || tree.Children[0].Children[0].Issue.ID != "bd-lite-gc1" {
		t.Errorf("bd-lite-cc1 should have single child bd-lite-gc1 (its own dependency)")
	}
}

func TestDepTreeLeafHasNoChildren(t *testing.T) {
	s := storeWithDeps(t,
		[]string{"bd-lite-epc", "bd-lite-cc1"},
		map[string][]string{"bd-lite-epc": {"bd-lite-cc1"}})

	tree, err := s.DepTree("bd-lite-cc1")
	if err != nil {
		t.Fatal(err)
	}
	// bd-lite-cc1 depends on nothing; issues that depend on it do not belong
	// in its dependency tree.
	if len(tree.Children) != 0 {
		t.Errorf("leaf children = %d, want 0 (tree must follow dependencies, not dependents)", len(tree.Children))
	}
}

func TestDepTreeHandlesCycles(t *testing.T) {
	s := storeWithDeps(t,
		[]string{"bd-lite-aaa", "bd-lite-bbb"},
		map[string][]string{
			"bd-lite-aaa": {"bd-lite-bbb"},
			"bd-lite-bbb": {"bd-lite-aaa"},
		})

	tree, err := s.DepTree("bd-lite-aaa")
	if err != nil {
		t.Fatal(err)
	}
	// Must terminate; the revisited node appears without re-expanding.
	if len(tree.Children) != 1 || tree.Children[0].Issue.ID != "bd-lite-bbb" {
		t.Fatalf("expected single child bd-lite-bbb")
	}
	b := tree.Children[0]
	if len(b.Children) != 1 || b.Children[0].Issue.ID != "bd-lite-aaa" {
		t.Fatalf("expected cycle to surface bd-lite-aaa under bd-lite-bbb once")
	}
	if len(b.Children[0].Children) != 0 {
		t.Errorf("revisited node must not be re-expanded")
	}
}

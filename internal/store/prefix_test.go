package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractIssuePrefix(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"bd-abc", "bd"},
		{"bd-123", "bd"},
		{"bd-abc.1", "bd"},
		// Hash suffixes that start with digits read as numbers to a partial
		// Sscanf; ones that start with letters do not. Both must split alike.
		{"hamilos-metadata-15e", "hamilos-metadata"},
		{"hamilos-metadata-9p7", "hamilos-metadata"},
		{"hamilos-metadata-e5q", "hamilos-metadata"},
		{"rails-tron-city-map-5w2", "rails-tron-city-map"},
		{"nohyphen", ""},
		{"-abc", ""},
	}
	for _, c := range cases {
		if got := extractIssuePrefix(c.id); got != c.want {
			t.Errorf("extractIssuePrefix(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

func TestLoadDetectsHyphenatedPrefixWithoutConfig(t *testing.T) {
	dir := t.TempDir()
	lines := []string{
		issueLine("hamilos-metadata-15e", "andy"),
		issueLine("hamilos-metadata-e5q", "andy"),
		issueLine("hamilos-metadata-9p7", "andy"),
	}
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "issues.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.prefix != "hamilos-metadata" {
		t.Errorf("prefix = %q, want %q", s.prefix, "hamilos-metadata")
	}
}

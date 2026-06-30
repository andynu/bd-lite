package store

import (
	"fmt"
	"strings"
)

// ResolveID finds an issue by full or partial ID.
// Returns the full ID and nil error, or an error if ambiguous/not found.
func (s *Store) ResolveID(partial string) (string, error) {
	// Exact match first
	if _, ok := s.issues[partial]; ok {
		return partial, nil
	}

	// Partial match: accept either a prefix of the full ID (e.g. "bd-lite-pr")
	// or a prefix of the bare suffix code (e.g. "prv" for "bd-lite-prv").
	seen := make(map[string]bool)
	var matches []string
	for id := range s.issues {
		if strings.HasPrefix(id, partial) || strings.HasPrefix(suffixOf(id), partial) {
			if !seen[id] {
				seen[id] = true
				matches = append(matches, id)
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no issue found matching '%s'", partial)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous ID '%s' matches %d issues: %s", partial, len(matches), strings.Join(matches, ", "))
	}
}

// suffixOf returns the bare suffix code of an issue ID (the segment after the
// last "-"). For "bd-lite-prv" it returns "prv"; for an ID with no "-" it
// returns the whole string.
func suffixOf(id string) string {
	if i := strings.LastIndex(id, "-"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// ResolveIDs resolves multiple partial IDs, returning full IDs.
func (s *Store) ResolveIDs(partials []string) ([]string, error) {
	result := make([]string, 0, len(partials))
	for _, p := range partials {
		id, err := s.ResolveID(p)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}

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

	// Partial match: check if partial is a prefix of any ID
	var matches []string
	for id := range s.issues {
		if strings.HasPrefix(id, partial) {
			matches = append(matches, id)
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

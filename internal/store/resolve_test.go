package store

import (
	"strings"
	"testing"

	"bd-lite/internal/types"
)

func storeWithIDs(ids ...string) *Store {
	issues := make(map[string]*types.Issue, len(ids))
	for _, id := range ids {
		issues[id] = &types.Issue{ID: id}
	}
	return &Store{issues: issues, prefix: "bd-lite"}
}

func TestResolveID(t *testing.T) {
	tests := []struct {
		name    string
		ids     []string
		partial string
		want    string
		wantErr string // substring expected in error, "" means no error
	}{
		{
			name:    "exact full id",
			ids:     []string{"bd-lite-prv", "bd-lite-gx2"},
			partial: "bd-lite-prv",
			want:    "bd-lite-prv",
		},
		{
			name:    "full prefix",
			ids:     []string{"bd-lite-prv", "bd-lite-gx2"},
			partial: "bd-lite-pr",
			want:    "bd-lite-prv",
		},
		{
			name:    "bare suffix unambiguous",
			ids:     []string{"bd-lite-prv", "bd-lite-gx2"},
			partial: "prv",
			want:    "bd-lite-prv",
		},
		{
			name:    "bare suffix prefix unambiguous",
			ids:     []string{"bd-lite-prv", "bd-lite-gx2"},
			partial: "pr",
			want:    "bd-lite-prv",
		},
		{
			name:    "bare suffix ambiguous",
			ids:     []string{"bd-lite-prv", "bd-lite-pre"},
			partial: "pr",
			wantErr: "ambiguous",
		},
		{
			name:    "not found",
			ids:     []string{"bd-lite-prv"},
			partial: "zzz",
			wantErr: "no issue found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := storeWithIDs(tt.ids...)
			got, err := s.ResolveID(tt.partial)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (resolved to %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveID(%q) = %q, want %q", tt.partial, got, tt.want)
			}
		})
	}
}

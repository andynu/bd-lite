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
		name    string
		bdActor string
		gitName string
		user    string
		want    string
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

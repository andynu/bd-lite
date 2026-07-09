package actor

import (
	"os/user"
	"testing"
)

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
		{"USER beats the OS-account fallback", "", "", "andy", "andy"},
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

// TestNameFallsBackToOSUser covers the all-empty case: with $BD_ACTOR and $USER
// cleared and git returning nothing, Name() must still resolve to the OS account
// name via os/user.Current() rather than yielding "" (which bd show renders as
// "anonymous"). This is the last-resort source that never depends on the
// environment, restored after the chain briefly ended at $USER.
func TestNameFallsBackToOSUser(t *testing.T) {
	t.Setenv("BD_ACTOR", "")
	t.Setenv("USER", "")
	stubGit(t, "")

	u, err := user.Current()
	if err != nil {
		t.Skipf("os/user.Current() unavailable on this platform: %v", err)
	}
	if got := Name(); got != u.Username {
		t.Errorf("Name() = %q, want OS username %q", got, u.Username)
	}
}

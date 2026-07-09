// Package actor resolves the identity recorded as the creator of new records.
package actor

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// gitUserName is a package variable so tests can substitute it without building
// a temporary git repository.
var gitUserName = func() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Name returns the identity to stamp on issues and comments this process
// creates, resolved in order: $BD_ACTOR, else git config user.name, else $USER,
// else the OS account name from os/user.Current(). That last source reads the
// password database (getpwuid) and needs no environment, so in practice it always
// resolves; it is the fallback cmd/comment.go relied on before it moved to this
// chain. An empty result (only when even os/user.Current() fails) means the caller
// should omit the field rather than store a placeholder.
//
// Call this from write paths only. It forks a git subprocess, so it must not be
// hoisted into rootCmd.PersistentPreRunE, where bd list and bd show would pay
// for it on every invocation.
func Name() string {
	if a := strings.TrimSpace(os.Getenv("BD_ACTOR")); a != "" {
		return a
	}
	if n := gitUserName(); n != "" {
		return n
	}
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	if u, err := user.Current(); err == nil {
		return strings.TrimSpace(u.Username)
	}
	return ""
}

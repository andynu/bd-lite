// Package actor resolves the identity recorded as the creator of new records.
package actor

import (
	"os"
	"os/exec"
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
// creates: $BD_ACTOR, else git config user.name, else $USER. An empty result
// means the caller should omit the field rather than store a placeholder.
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
	return strings.TrimSpace(os.Getenv("USER"))
}

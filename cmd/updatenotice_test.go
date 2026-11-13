package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecutePrintsUpdateNoticeToStderr(t *testing.T) {
	var out, errOut bytes.Buffer
	origNotice, origStderr := updateNotice, stderr
	t.Cleanup(func() {
		updateNotice, stderr = origNotice, origStderr
		rootCmd.SetArgs(nil)
		rootCmd.SetOut(nil)
	})
	updateNotice = func() string { return "NOTICE\n" }
	stderr = &errOut
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"--version"})

	if err := Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.Contains(errOut.String(), "NOTICE") {
		t.Errorf("stderr = %q, want the notice", errOut.String())
	}
	if strings.Contains(out.String(), "NOTICE") {
		t.Errorf("stdout = %q, notice must not reach stdout", out.String())
	}
	if !strings.Contains(out.String(), "bd version") {
		t.Errorf("stdout = %q, want the command's own output", out.String())
	}
}

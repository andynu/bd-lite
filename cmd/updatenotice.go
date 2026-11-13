package cmd

import (
	"fmt"
	"io"
	"os"

	"bd-lite/internal/updatecheck"
)

var (
	stderr       io.Writer = os.Stderr
	updateNotice           = func() string { return updatecheck.Default(rootCmd.Version).Notice() }
)

// printUpdateNotice goes to stderr so --json output on stdout stays parseable.
func printUpdateNotice() {
	fmt.Fprint(stderr, updateNotice())
}

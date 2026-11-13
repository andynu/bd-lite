package cmd

import "runtime/debug"

// The version comes from the git tag that go build stamps into the binary,
// so there is no version constant to keep in step with the tags.
func init() {
	rootCmd.Version = versionFromBuildInfo(debug.ReadBuildInfo())
}

func versionFromBuildInfo(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return "dev"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return v
}

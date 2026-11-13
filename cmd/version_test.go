package cmd

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{"tagged build", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}}, true, "v0.1.0"},
		{"untagged build", &debug.BuildInfo{Main: debug.Module{Version: "v0.0.0-20261113120000-ed9446b1c2d3"}}, true, "v0.0.0-20261113120000-ed9446b1c2d3"},
		{"dirty tree", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0+dirty"}}, true, "v0.1.0+dirty"},
		{"go test or go run", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true, "dev"},
		{"empty version", &debug.BuildInfo{}, true, "dev"},
		{"no build info", nil, false, "dev"},
	}
	for _, c := range cases {
		if got := versionFromBuildInfo(c.info, c.ok); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestRootCommandHasVersion(t *testing.T) {
	if rootCmd.Version == "" {
		t.Fatal("rootCmd.Version is empty, so bd --version is not registered")
	}
}

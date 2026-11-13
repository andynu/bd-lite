package updatecheck

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fakeServer struct {
	*httptest.Server
	hits atomic.Int32
}

func newServer(t *testing.T, status int, body string) *fakeServer {
	t.Helper()
	fs := &fakeServer{}
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.hits.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(fs.Close)
	return fs
}

func latest(t *testing.T, tag string) *fakeServer {
	return newServer(t, http.StatusOK, `{"tag_name":"`+tag+`"}`)
}

type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func newChecker(t *testing.T, srv *fakeServer, current string, clk *clock) *Checker {
	t.Helper()
	t.Setenv("BD_NO_UPDATE_CHECK", "")
	return &Checker{
		Current:  current,
		APIURL:   srv.URL,
		CacheDir: t.TempDir(),
		ExePath:  "/home/s/.local/bin/bd",
		GOOS:     "linux",
		GOARCH:   "amd64",
		Now:      clk.Now,
		Client:   srv.Client(),
	}
}

func start() *clock { return &clock{now: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)} }

func TestNoticeWhenNewer(t *testing.T) {
	srv := latest(t, "v0.2.0")
	c := newChecker(t, srv, "v0.1.0", start())
	want := "bd: v0.2.0 is available (you have v0.1.0). To update:\n" +
		"  curl -fsSL https://github.com/andynu/bd-lite/releases/download/v0.2.0/bd-linux-amd64 -o /home/s/.local/bin/bd.new && chmod +x /home/s/.local/bin/bd.new && mv /home/s/.local/bin/bd.new /home/s/.local/bin/bd\n"
	if got := c.Notice(); got != want {
		t.Errorf("Notice() =\n%q\nwant\n%q", got, want)
	}
}

func TestNoNoticeWhenCurrentOrNewer(t *testing.T) {
	for _, current := range []string{"v0.1.0", "v0.2.0"} {
		srv := latest(t, "v0.1.0")
		c := newChecker(t, srv, current, start())
		if got := c.Notice(); got != "" {
			t.Errorf("current %s: Notice() = %q, want empty", current, got)
		}
	}
}

func TestPseudoVersionGetsNotice(t *testing.T) {
	srv := latest(t, "v0.1.0")
	c := newChecker(t, srv, "v0.0.0-20261113175741-ded8091195bd", start())
	if got := c.Notice(); !strings.Contains(got, "v0.1.0 is available") {
		t.Errorf("Notice() = %q, want an update notice", got)
	}
}

func TestSkippedVersions(t *testing.T) {
	for _, current := range []string{"dev", "v0.1.0+dirty", "v0.0.0-20261113175741-ded8091195bd+dirty", "garbage", ""} {
		srv := latest(t, "v9.9.9")
		c := newChecker(t, srv, current, start())
		if got := c.Notice(); got != "" {
			t.Errorf("current %q: Notice() = %q, want empty", current, got)
		}
		if n := srv.hits.Load(); n != 0 {
			t.Errorf("current %q: %d fetches, want 0", current, n)
		}
	}
}

func TestOptOutEnv(t *testing.T) {
	srv := latest(t, "v0.2.0")
	c := newChecker(t, srv, "v0.1.0", start())
	t.Setenv("BD_NO_UPDATE_CHECK", "1")
	if got := c.Notice(); got != "" {
		t.Errorf("Notice() = %q, want empty", got)
	}
	if n := srv.hits.Load(); n != 0 {
		t.Errorf("%d fetches, want 0", n)
	}
}

func TestCacheFreshSkipsFetch(t *testing.T) {
	srv := latest(t, "v0.2.0")
	clk := start()
	c := newChecker(t, srv, "v0.1.0", clk)

	first := c.Notice()
	clk.now = clk.now.Add(23 * time.Hour)
	second := c.Notice()
	if n := srv.hits.Load(); n != 1 {
		t.Fatalf("%d fetches within 24h, want 1", n)
	}
	if first == "" || second != first {
		t.Errorf("cached call should repeat the notice: first %q, second %q", first, second)
	}

	clk.now = clk.now.Add(2 * time.Hour)
	c.Notice()
	if n := srv.hits.Load(); n != 2 {
		t.Errorf("%d fetches after 25h, want 2", n)
	}
}

func TestFetchFailureRecordsCheck(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusForbidden, http.StatusNotFound} {
		srv := newServer(t, status, `{"message":"nope"}`)
		c := newChecker(t, srv, "v0.1.0", start())
		if got := c.Notice(); got != "" {
			t.Errorf("status %d: Notice() = %q, want empty", status, got)
		}
		c.Notice()
		if n := srv.hits.Load(); n != 1 {
			t.Errorf("status %d: %d fetches, want 1 (failure must still record the check)", status, n)
		}
	}
}

func TestFetchFailureKeepsPreviouslyKnownLatest(t *testing.T) {
	good := latest(t, "v0.2.0")
	clk := start()
	c := newChecker(t, good, "v0.1.0", clk)
	c.Notice()

	bad := newServer(t, http.StatusInternalServerError, "")
	c.APIURL, c.Client = bad.URL, bad.Client()
	clk.now = clk.now.Add(25 * time.Hour)
	if got := c.Notice(); !strings.Contains(got, "v0.2.0 is available") {
		t.Errorf("Notice() = %q, want the last known release still announced", got)
	}
}

func TestUnreachableServer(t *testing.T) {
	srv := latest(t, "v0.2.0")
	c := newChecker(t, srv, "v0.1.0", start())
	srv.Close()
	if got := c.Notice(); got != "" {
		t.Errorf("Notice() = %q, want empty", got)
	}
}

func TestCorruptCache(t *testing.T) {
	srv := latest(t, "v0.2.0")
	c := newChecker(t, srv, "v0.1.0", start())
	path := filepath.Join(c.CacheDir, "bd-lite", "latest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := c.Notice(); got == "" {
		t.Error("Notice() empty, want a notice after refetching")
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"latest":"v0.2.0"`) {
		t.Errorf("cache not rewritten: %q, %v", data, err)
	}
}

func TestUnwritableCacheDir(t *testing.T) {
	srv := latest(t, "v0.2.0")
	c := newChecker(t, srv, "v0.1.0", start())
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	c.CacheDir = blocker
	if got := c.Notice(); got == "" {
		t.Error("Notice() empty, want a notice from the fetch even though the cache cannot be written")
	}
}

func TestOtherPlatformPrintsReleasePage(t *testing.T) {
	srv := latest(t, "v0.2.0")
	c := newChecker(t, srv, "v0.1.0", start())
	c.GOOS = "darwin"
	want := "bd: v0.2.0 is available (you have v0.1.0). To update:\n" +
		"  see https://github.com/andynu/bd-lite/releases/tag/v0.2.0\n"
	if got := c.Notice(); got != want {
		t.Errorf("Notice() =\n%q\nwant\n%q", got, want)
	}
}

func TestInvalidLatestTag(t *testing.T) {
	srv := latest(t, "nightly")
	c := newChecker(t, srv, "v0.1.0", start())
	if got := c.Notice(); got != "" {
		t.Errorf("Notice() = %q, want empty for a non-semver release tag", got)
	}
}

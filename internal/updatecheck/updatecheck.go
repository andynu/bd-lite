// Package updatecheck tells the user when GitHub has a newer bd-lite release
// than the running binary. It only prints the update command; it never
// replaces the binary itself.
package updatecheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	releasesURL  = "https://github.com/andynu/bd-lite/releases"
	latestAPIURL = "https://api.github.com/repos/andynu/bd-lite/releases/latest"
	assetName    = "bd-linux-amd64"
	cacheTTL     = 24 * time.Hour
	fetchTimeout = 1500 * time.Millisecond
)

// Checker holds everything a check depends on, so tests can substitute the
// server, clock and cache location.
type Checker struct {
	Current  string
	APIURL   string
	CacheDir string
	ExePath  string
	GOOS     string
	GOARCH   string
	Now      func() time.Time
	Client   *http.Client
}

// Default returns a Checker for the running binary.
func Default(current string) *Checker {
	cacheDir, _ := os.UserCacheDir()
	exe, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
	}
	return &Checker{
		Current:  current,
		APIURL:   latestAPIURL,
		CacheDir: cacheDir,
		ExePath:  exe,
		GOOS:     runtime.GOOS,
		GOARCH:   runtime.GOARCH,
		Now:      time.Now,
		Client:   &http.Client{Timeout: fetchTimeout},
	}
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// Notice returns the update notice, or "" when there is nothing to say.
// Every failure (network, cache, parsing) yields "" rather than an error,
// since a broken check must never get in the way of the command.
func (c *Checker) Notice() string {
	if os.Getenv("BD_NO_UPDATE_CHECK") == "1" || !checkable(c.Current) {
		return ""
	}
	latest := c.latest()
	if !semver.IsValid(latest) || semver.Compare(latest, c.Current) <= 0 {
		return ""
	}
	return c.format(latest)
}

// Dev builds, modified trees and anything that is not a version have nothing
// meaningful to compare against a release.
func checkable(v string) bool {
	return semver.IsValid(v) && !strings.Contains(v, "+dirty")
}

func (c *Checker) cachePath() string {
	return filepath.Join(c.CacheDir, "bd-lite", "latest.json")
}

func (c *Checker) latest() string {
	var entry cacheEntry
	if data, err := os.ReadFile(c.cachePath()); err == nil {
		if json.Unmarshal(data, &entry) != nil {
			entry = cacheEntry{}
		}
	}
	now := c.Now()
	if !entry.CheckedAt.IsZero() && now.Sub(entry.CheckedAt) < cacheTTL {
		return entry.Latest
	}

	// A failed fetch keeps the last known release and still stamps the time,
	// so an offline machine does not retry on every invocation.
	if tag, ok := c.fetch(); ok {
		entry.Latest = tag
	}
	entry.CheckedAt = now
	c.save(entry)
	return entry.Latest
}

func (c *Checker) fetch() (string, bool) {
	req, err := http.NewRequest(http.MethodGet, c.APIURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "bd-lite")
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&release) != nil || release.TagName == "" {
		return "", false
	}
	return release.TagName, true
}

func (c *Checker) save(entry cacheEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	path := c.cachePath()
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func (c *Checker) format(latest string) string {
	head := fmt.Sprintf("bd: %s is available (you have %s). To update:\n", latest, c.Current)
	if c.GOOS != "linux" || c.GOARCH != "amd64" || c.ExePath == "" {
		return head + fmt.Sprintf("  see %s/tag/%s\n", releasesURL, latest)
	}
	tmp := c.ExePath + ".new"
	return head + fmt.Sprintf("  curl -fsSL %s/download/%s/%s -o %s && chmod +x %s && mv %s %s\n",
		releasesURL, latest, assetName, tmp, tmp, tmp, c.ExePath)
}

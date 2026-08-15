// Package update checks GitHub Releases for newer CEREBRO versions and
// builds the one-line upgrade hint shown at startup.
//
// The check is strictly best-effort: it runs once, never blocks the UI, and
// stays silent on local ("dev") builds, offline machines and API errors.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
)

// CurrentVersion is the compiled-in version (e.g. "v1.5.0"), stamped at build
// time via -ldflags "-X main.version=...". Local builds keep the default
// "dev", which never triggers the update notice.
var CurrentVersion = "dev"

// apiBase is the GitHub API base for release lookups (overridable in tests).
var apiBase = "https://api.github.com/repos/NamanGaonkar/CEREBRO"

const (
	windowsInstaller = "https://raw.githubusercontent.com/NamanGaonkar/CEREBRO/main/install.ps1"
	unixInstaller    = "https://raw.githubusercontent.com/NamanGaonkar/CEREBRO/main/install.sh"
)

type release struct {
	TagName string `json:"tag_name"`
}

// CheckLatest returns the latest published version (e.g. "1.6.0") when it is
// strictly newer than CurrentVersion, or "" when up to date, on a local
// "dev" build, or when the query fails (offline / rate-limited). The caller
// supplies a bounded context and client so the check can never hang the app.
func CheckLatest(ctx context.Context, client *http.Client) (string, error) {
	// "dev" is the default for local `go build` runs (nothing stamped via
	// ldflags), so there is nothing meaningful to compare against.
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(CurrentVersion)), "dev") {
		return "", nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "cerebro-update-check")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github: unexpected status %d", resp.StatusCode)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if !NewerVersion(CurrentVersion, rel.TagName) {
		return "", nil
	}
	return strings.TrimPrefix(rel.TagName, "v"), nil
}

// NewerVersion reports whether tag is strictly newer than current. Both are
// semver-ish tags like "v1.5.0" (a leading "v" is optional). Unparseable
// versions — including "dev" — are never treated as outdated.
func NewerVersion(current, tag string) bool {
	cur, ok := parseVersion(current)
	if !ok {
		return false
	}
	latest, ok := parseVersion(tag)
	if !ok {
		return false
	}
	for i := 0; i < len(cur); i++ {
		if latest[i] > cur[i] {
			return true
		}
		if latest[i] < cur[i] {
			return false
		}
	}
	return false
}

// parseVersion parses "v1.5.0" (or "1.5.0") into [major, minor, patch].
func parseVersion(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// Notice builds the one-line upgrade hint shown in the TUI banner. latest is
// a version without the leading "v" (as returned by CheckLatest).
func Notice(latest string) string {
	return fmt.Sprintf("UPDATE %s available — you're on %s · %s",
		latest, strings.TrimPrefix(CurrentVersion, "v"), hint())
}

// hint returns the platform-appropriate update command. On Windows it leads
// with Scoop (and how to install Scoop first if it's missing), since that is
// the one-command upgrade path.
func hint() string {
	// Keep the Windows hint short so it survives truncation on 80-col
	// terminals — the full commands live in the README's Updates section.
	if runtime.GOOS == "windows" {
		return "scoop update cerebro · no Scoop? winget install ScoopInstaller.Scoop"
	}
	return "re-run: curl -fsSL " + unixInstaller + " | bash"
}

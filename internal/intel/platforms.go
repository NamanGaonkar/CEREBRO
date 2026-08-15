package intel

import "strings"

// platform describes one recon probe: how to build the profile URL for a
// handle and how to judge the response. A nil check uses defaultCheck.
type platform struct {
	name  string
	url   func(handle string) string
	check func(code int, body string) Status
}

// defaultCheck: a hard 404/410 means no account; 2xx/3xx means the profile
// exists; anything else (login walls, rate limits) is unverifiable.
func defaultCheck(code int, body string) Status {
	switch {
	case code == 404 || code == 410:
		return StatusNotFound
	case code >= 200 && code < 400:
		return StatusFound
	default:
		return StatusUnverified
	}
}

// notFoundText treats a marker string in the body as "no such user" even when
// the server answers 200 (soft-404 sites like YouTube and Steam).
func notFoundText(marker string) func(int, string) Status {
	return notFoundAny(marker)
}

// notFoundAny treats ANY of the marker strings in the body as "no such user"
// even when the server answers 200. Sites that soft-404 with several phrasings
// (Reddit, TikTok, X) pass the whole set.
func notFoundAny(markers ...string) func(int, string) Status {
	return func(code int, body string) Status {
		if code == 404 || code == 410 {
			return StatusNotFound
		}
		low := strings.ToLower(body)
		for _, m := range markers {
			if strings.Contains(low, strings.ToLower(m)) {
				return StatusNotFound
			}
		}
		if code >= 200 && code < 400 {
			return StatusFound
		}
		return StatusUnverified
	}
}

// loginWallCheck: the site answers 200 for EVERYTHING (login walls, JS-only
// shells) so a bare 200 can never confirm a profile exists. A hard 404 or a
// soft-404 marker means NOT FOUND; anything else is UNVERIFIED — never a
// false-positive FOUND for Instagram/Facebook/X-style walls.
func loginWallCheck(markers ...string) func(int, string) Status {
	return func(code int, body string) Status {
		if code == 404 || code == 410 {
			return StatusNotFound
		}
		low := strings.ToLower(body)
		for _, m := range markers {
			if strings.Contains(low, strings.ToLower(m)) {
				return StatusNotFound
			}
		}
		return StatusUnverified
	}
}

// unverifiedCheck: the site always answers 200 (login wall / soft-404), so we
// can never confirm a profile exists — mark UNVERIFIED either way.
func unverifiedCheck(code int, body string) Status {
	if code == 404 || code == 410 {
		return StatusNotFound
	}
	return StatusUnverified
}

// githubCheck: the GitHub API answers 404 for missing users (reliable).
func githubCheck(code int, body string) Status {
	if code == 404 {
		return StatusNotFound
	}
	if code >= 200 && code < 400 {
		return StatusFound
	}
	return StatusUnverified
}

// jsonListCheck: X's public follow-button endpoint returns [] for unknown
// handles and [{...}] for existing ones.
func jsonListCheck(code int, body string) Status {
	if code != 200 {
		return StatusUnverified
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "[]" {
		return StatusNotFound
	}
	if strings.HasPrefix(trimmed, "[{") {
		return StatusFound
	}
	return StatusUnverified
}

var platforms = []platform{
	// ---- Developer / code ----
	{name: "GitHub", url: func(h string) string { return "https://github.com/" + h }, check: notFoundAny("Page not found", "Not Found")},
	{name: "GitLab", url: func(h string) string { return "https://gitlab.com/" + h }, check: notFoundAny("Page not found")},
	{name: "Docker Hub", url: func(h string) string { return "https://hub.docker.com/u/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "PyPI", url: func(h string) string { return "https://pypi.org/user/" + h }, check: notFoundAny("Page not found", "does not exist")},
	{name: "npm", url: func(h string) string { return "https://www.npmjs.com/~" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Hugging Face", url: func(h string) string { return "https://huggingface.co/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "LeetCode", url: func(h string) string { return "https://leetcode.com/" + h }, check: notFoundAny("Page not found")},
	{name: "Codeforces", url: func(h string) string { return "https://codeforces.com/profile/" + h }},
	{name: "Codeberg", url: func(h string) string { return "https://codeberg.org/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Bitbucket", url: func(h string) string { return "https://bitbucket.org/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "SourceForge", url: func(h string) string { return "https://sourceforge.net/u/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Replit", url: func(h string) string { return "https://replit.com/@" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Stack Overflow", url: func(h string) string { return "https://stackoverflow.com/users?search=" + h }, check: notFoundAny("Page not found", "404")},
	{name: "DEV Community", url: func(h string) string { return "https://dev.to/" + h }, check: notFoundAny("Page not found", "404")},
	// ---- Social / media ----
	{name: "Reddit", url: func(h string) string { return "https://www.reddit.com/user/" + h }, check: notFoundAny("Sorry, nobody on Reddit goes by that name", "This page has been deleted")},
	// X answers 200 with a JS shell for everything; only explicit markers/404
	// prove absence, otherwise UNVERIFIED (never a false FOUND).
	{name: "X / Twitter", url: func(h string) string { return "https://x.com/" + h }, check: loginWallCheck("This account doesn't exist", "This account does not exist", "Hmm...this page doesn't exist")},
	{name: "YouTube", url: func(h string) string { return "https://www.youtube.com/@" + h }, check: notFoundAny("This channel doesn't exist", "This channel does not exist")},
	{name: "Medium", url: func(h string) string { return "https://medium.com/@" + h }, check: notFoundAny("Page not found")},
	{name: "Hashnode", url: func(h string) string { return "https://hashnode.com/@" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Twitch", url: func(h string) string { return "https://www.twitch.tv/" + h }, check: notFoundAny("Sorry. Unless you've got a time machine", "Page not found")},
	{name: "Instagram", url: func(h string) string { return "https://www.instagram.com/" + h }, check: loginWallCheck("Sorry, this page isn't available", "The link you followed may be broken")},
	{name: "TikTok", url: func(h string) string { return "https://www.tiktok.com/@" + h }, check: loginWallCheck("Couldn't find this account", "This account doesn't exist")},
	{name: "Facebook", url: func(h string) string { return "https://www.facebook.com/" + h }, check: loginWallCheck("The link you followed may be broken", "This content isn't available")},
	{name: "Telegram", url: func(h string) string { return "https://t.me/" + h }, check: notFoundAny("Sorry, this channel doesn't seem to exist", "doesn't seem to exist")},
	{name: "Keybase", url: func(h string) string { return "https://keybase.io/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Pinterest", url: func(h string) string { return "https://www.pinterest.com/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Tumblr", url: func(h string) string { return "https://" + h + ".tumblr.com" }, check: notFoundAny("There's nothing here", "404")},
	{name: "Flickr", url: func(h string) string { return "https://www.flickr.com/people/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Behance", url: func(h string) string { return "https://www.behance.net/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Dribbble", url: func(h string) string { return "https://dribbble.com/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "SoundCloud", url: func(h string) string { return "https://soundcloud.com/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Bandcamp", url: func(h string) string { return "https://bandcamp.com/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Spotify", url: func(h string) string { return "https://open.spotify.com/user/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Steam", url: func(h string) string { return "https://steamcommunity.com/id/" + h }, check: notFoundAny("The specified profile could not be found")},
	// ---- Professional / academic ----
	{name: "LinkedIn", url: func(h string) string { return "https://www.linkedin.com/in/" + h }, check: loginWallCheck("This page doesn't exist", "Page not found")},
	{name: "ORCID", url: func(h string) string { return "https://orcid.org/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "ResearchGate", url: func(h string) string { return "https://www.researchgate.net/profile/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Google Scholar", url: func(h string) string { return "https://scholar.google.com/citations?user=" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Hacker News", url: func(h string) string { return "https://news.ycombinator.com/user?id=" + h }, check: notFoundAny("No such user")},
	{name: "Wikipedia", url: func(h string) string { return "https://en.wikipedia.org/wiki/User:" + h }, check: notFoundAny("There is currently no text in this page")},
	{name: "Vimeo", url: func(h string) string { return "https://vimeo.com/" + h }, check: notFoundAny("Page not found", "404")},
	{name: "Gravatar", url: func(h string) string { return "https://www.gravatar.com/" + h }, check: loginWallCheck("Page not found", "404")},
}

// platformCount exposes the registry size for tests.
func platformCount() int { return len(platforms) }

package git

import "regexp"

// Remotes in this setup embed credentials in their URLs, and git's own
// error text routinely quotes the URL ("fatal: unable to access
// 'https://user:token@host/x.git/'"). Anything git-derived that is shown to
// the user goes through ScrubURLs first.
var (
	// scheme://anything-up-to-whitespace-or-quote, e.g. https://u:p@host/x.git
	schemeURLRe = regexp.MustCompile(`[A-Za-z][A-Za-z0-9+.-]*://[^\s'"<>]+`)
	// scp-like user@host:path, e.g. git@host:org/repo.git
	scpURLRe = regexp.MustCompile(`[A-Za-z0-9._~-]+@[A-Za-z0-9.-]+:[^\s'"<>]+`)
	// a leftover credential-looking user:secret@host with no scheme
	credHostRe = regexp.MustCompile(`[A-Za-z0-9._~%-]+:[^\s@'"<>/]+@[A-Za-z0-9.-]+`)
	// transport messages that name the host without a URL, e.g.
	// "Failed to connect to host port 443" / "Could not resolve host: host"
	connectHostRe = regexp.MustCompile(`(?i)(connect to |resolve host:? ?)[^\s]+( port \d+)?`)
)

// ScrubURLs replaces anything that looks like a remote URL (or a bare
// credential@host fragment) with "<remote>", so a failure message can be
// shown without leaking the token embedded in a remote's URL.
func ScrubURLs(s string) string {
	s = schemeURLRe.ReplaceAllString(s, "<remote>")
	s = scpURLRe.ReplaceAllString(s, "<remote>")
	s = credHostRe.ReplaceAllString(s, "<remote>")
	s = connectHostRe.ReplaceAllString(s, "${1}<remote>")
	return s
}

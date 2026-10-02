package git

import (
	"fmt"
	"strings"
)

// DefaultRemote is the remote pmt syncs with unless told otherwise.
const DefaultRemote = "origin"

// RemoteExists reports whether a remote called name is configured. The
// remote's URL is deliberately never read here.
func RemoteExists(dir, name string) (bool, error) {
	out, err := Run(dir, "remote")
	if err != nil {
		return false, err
	}
	for _, r := range Lines(out) {
		if r == name {
			return true, nil
		}
	}
	return false, nil
}

// RemoteRef returns the remote-tracking ref for branch on remote, e.g.
// refs/remotes/origin/bug/x.
func RemoteRef(remote, branch string) string {
	return "refs/remotes/" + remote + "/" + branch
}

// scrubRemoteErr removes the remote's configured URL (fetch and push
// variants) from err's text. ScrubURLs catches anything URL-shaped; this
// also catches a URL that is a plain filesystem path, by matching the exact
// configured value. The URL is read here only to be removed, never shown.
func scrubRemoteErr(dir, remote string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	for _, key := range []string{"url", "pushurl"} {
		out, code, runErr := RunRaw(dir, "config", "--get-all", "remote."+remote+"."+key)
		if runErr != nil || code != 0 {
			continue
		}
		for _, u := range Lines(out) {
			if u != "" {
				msg = strings.ReplaceAll(msg, u, "<remote>")
			}
		}
	}
	return fmt.Errorf("%s", ScrubURLs(msg))
}

// FetchRemote updates remote-tracking refs for remote and prunes ones whose
// branch is gone. Any error text has the remote's location scrubbed.
func FetchRemote(dir, remote string) error {
	_, err := Run(dir, "fetch", remote, "--prune")
	return scrubRemoteErr(dir, remote, err)
}

// Push pushes branch to remote under the same name and records it as the
// upstream. It never forces: a non-fast-forward push fails.
func Push(dir, remote, branch string) error {
	_, err := Run(dir, "push", "--set-upstream", remote, "refs/heads/"+branch+":refs/heads/"+branch)
	return scrubRemoteErr(dir, remote, err)
}

// RevParse resolves ref to a full commit SHA.
func RevParse(dir, ref string) (string, error) {
	return Run(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
}

// ShortSHA abbreviates a SHA to the 7 characters used in reported ranges.
func ShortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// RelState describes how a local branch relates to its remote counterpart.
type RelState int

const (
	RelUpToDate    RelState = iota // same commit
	RelAhead                       // local has commits the remote lacks (push)
	RelBehind                      // remote has commits local lacks (fast-forward)
	RelDiverged                    // both have commits the other lacks
	RelNoRemote                    // no remote-tracking ref: never published, or deleted upstream
	RelNoLocal                     // remote branch with no local branch
	RelRemoteMoved                 // remote tip is a commit not present locally (unfetched)
)

func (s RelState) String() string {
	switch s {
	case RelUpToDate:
		return "up-to-date"
	case RelAhead:
		return "ahead"
	case RelBehind:
		return "behind"
	case RelDiverged:
		return "diverged"
	case RelNoRemote:
		return "no-remote"
	case RelNoLocal:
		return "no-local"
	case RelRemoteMoved:
		return "remote-moved"
	}
	return fmt.Sprintf("RelState(%d)", int(s))
}

// Relation is the result of Classify, carrying the tips so callers can
// report commit ranges without a second lookup.
type Relation struct {
	State  RelState
	Local  string // full SHA; empty for RelNoLocal
	Remote string // full SHA; empty for RelNoRemote
}

// Range renders the commit range a push or fast-forward would move, as
// "abc1234..def5678", or just the tip for states with only one side.
func (r Relation) Range() string {
	switch r.State {
	case RelAhead:
		return ShortSHA(r.Remote) + ".." + ShortSHA(r.Local)
	case RelBehind:
		return ShortSHA(r.Local) + ".." + ShortSHA(r.Remote)
	case RelDiverged:
		return ShortSHA(r.Local) + "..." + ShortSHA(r.Remote)
	case RelNoRemote:
		return ShortSHA(r.Local)
	case RelNoLocal:
		return ShortSHA(r.Remote)
	}
	return ShortSHA(r.Local)
}

// Classify compares refs/heads/<branch> with refs/remotes/<remote>/<branch>
// without changing anything. It never fetches; call Fetch first for a
// current answer.
func Classify(dir, remote, branch string) (Relation, error) {
	localOK, err := RefExists(dir, "refs/heads/"+branch)
	if err != nil {
		return Relation{}, err
	}
	remoteOK, err := RefExists(dir, RemoteRef(remote, branch))
	if err != nil {
		return Relation{}, err
	}

	var localSHA, remoteSHA string
	if localOK {
		if localSHA, err = RevParse(dir, "refs/heads/"+branch); err != nil {
			return Relation{}, err
		}
	}
	if remoteOK {
		if remoteSHA, err = RevParse(dir, RemoteRef(remote, branch)); err != nil {
			return Relation{}, err
		}
	}
	if !localOK && !remoteOK {
		return Relation{}, fmt.Errorf("branch %q exists neither locally nor on %s", branch, remote)
	}
	return ClassifyTips(dir, localSHA, remoteSHA)
}

// ClassifyTips compares two tips directly. An empty localSHA means there is
// no local branch; an empty remoteSHA means there is no remote branch. When
// the remote tip is a commit this repo does not have, ancestry cannot be
// computed, so the result is RelRemoteMoved (the remote has unfetched
// commits) rather than a guess.
func ClassifyTips(dir, localSHA, remoteSHA string) (Relation, error) {
	rel := Relation{Local: localSHA, Remote: remoteSHA}
	switch {
	case localSHA == "" && remoteSHA == "":
		return Relation{}, fmt.Errorf("neither a local nor a remote tip was given")
	case localSHA == "":
		rel.State = RelNoLocal
	case remoteSHA == "":
		rel.State = RelNoRemote
	case localSHA == remoteSHA:
		rel.State = RelUpToDate
	default:
		_, code, err := RunRaw(dir, "cat-file", "-e", remoteSHA+"^{commit}")
		if err != nil {
			return Relation{}, err
		}
		if code != 0 {
			rel.State = RelRemoteMoved
			return rel, nil
		}
		localIsAncestor, err := IsAncestor(dir, localSHA, remoteSHA)
		if err != nil {
			return Relation{}, err
		}
		remoteIsAncestor, err := IsAncestor(dir, remoteSHA, localSHA)
		if err != nil {
			return Relation{}, err
		}
		switch {
		case localIsAncestor:
			rel.State = RelBehind
		case remoteIsAncestor:
			rel.State = RelAhead
		default:
			rel.State = RelDiverged
		}
	}
	return rel, nil
}

// LsRemoteHeads returns the remote's branch tips (branch name -> SHA) by
// asking the remote directly, without writing any local ref. It is what lets
// a dry run see the real remote state while changing nothing.
func LsRemoteHeads(dir, remote string) (map[string]string, error) {
	out, err := Run(dir, "ls-remote", "--heads", remote)
	if err != nil {
		return nil, scrubRemoteErr(dir, remote, err)
	}
	tips := map[string]string{}
	for _, line := range Lines(out) {
		sha, ref, ok := strings.Cut(line, "\t")
		if !ok || !strings.HasPrefix(ref, "refs/heads/") {
			continue
		}
		tips[strings.TrimPrefix(ref, "refs/heads/")] = sha
	}
	return tips, nil
}

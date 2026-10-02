package git

import "fmt"

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

// FetchRemote updates remote-tracking refs for remote and prunes ones whose
// branch is gone. Any error text is already URL-scrubbed (see ExitError).
func FetchRemote(dir, remote string) error {
	_, err := Run(dir, "fetch", remote, "--prune")
	return err
}

// Push pushes branch to remote under the same name and records it as the
// upstream. It never forces: a non-fast-forward push fails.
func Push(dir, remote, branch string) error {
	_, err := Run(dir, "push", "--set-upstream", remote, "refs/heads/"+branch+":refs/heads/"+branch)
	return err
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
	RelUpToDate RelState = iota // same commit
	RelAhead                    // local has commits the remote lacks (push)
	RelBehind                   // remote has commits local lacks (fast-forward)
	RelDiverged                 // both have commits the other lacks
	RelNoRemote                 // no remote-tracking ref: never published, or deleted upstream
	RelNoLocal                  // remote branch with no local branch
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

	var rel Relation
	if localOK {
		if rel.Local, err = RevParse(dir, "refs/heads/"+branch); err != nil {
			return Relation{}, err
		}
	}
	if remoteOK {
		if rel.Remote, err = RevParse(dir, RemoteRef(remote, branch)); err != nil {
			return Relation{}, err
		}
	}

	switch {
	case !localOK && !remoteOK:
		return Relation{}, fmt.Errorf("branch %q exists neither locally nor on %s", branch, remote)
	case !localOK:
		rel.State = RelNoLocal
	case !remoteOK:
		rel.State = RelNoRemote
	case rel.Local == rel.Remote:
		rel.State = RelUpToDate
	default:
		localIsAncestor, err := IsAncestor(dir, rel.Local, rel.Remote)
		if err != nil {
			return Relation{}, err
		}
		remoteIsAncestor, err := IsAncestor(dir, rel.Remote, rel.Local)
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

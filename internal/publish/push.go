// Package publish implements pmt's remote publishing commands. Push sends
// issue branches (plus the archive and templates) to a remote without ever
// forcing; it classifies each branch against the remote's real tips so that
// a dry run and a real run take exactly the same decisions. Remotes carry
// embedded credentials, so nothing here ever prints a URL — only branch
// names and short commit ranges. See ACCEPTANCE.md S1.
package publish

import (
	"fmt"
	"sort"

	"github.com/JamesTryand/pmtooling/internal/archive"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/template"
)

// Verb is the fixed one-word outcome shown per branch (doc/commands.md).
type Verb string

const (
	VerbPushed   Verb = "pushed"     // fast-forwarded the remote branch
	VerbNew      Verb = "new"        // branch did not exist on the remote
	VerbUpToDate Verb = "up-to-date" // nothing to push
	VerbBehind   Verb = "behind"     // remote is ahead; pull/sync instead
	VerbDiverged Verb = "diverged"   // both sides have new commits; left alone
	VerbMoved    Verb = "moved"      // remote has commits not fetched locally
	VerbFailed   Verb = "failed"     // the push itself was rejected or errored
)

// NeedsAttention reports whether a verb should make the command exit 2.
func (v Verb) NeedsAttention() bool {
	switch v {
	case VerbBehind, VerbDiverged, VerbMoved, VerbFailed:
		return true
	}
	return false
}

// Result is one branch's outcome. Detail is a commit range or a short,
// URL-free explanation.
type Result struct {
	Branch string
	Verb   Verb
	Detail string
}

// ManagedBranches lists every local branch pmt publishes: live issue
// branches, the archive, and the templates. The repo's default branch is
// deliberately not included — it is the user's own, not pmt's.
func ManagedBranches(root string) ([]string, error) {
	issues, err := issue.ListIssues(root, "", "")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, iss := range issues {
		set[iss.Branch] = true
	}

	templates, err := template.List(root)
	if err != nil {
		return nil, err
	}
	for _, name := range templates {
		set["pmt/template/"+name] = true
	}

	if ok, err := git.RefExists(root, archive.Ref); err != nil {
		return nil, err
	} else if ok {
		set["pmt/archive"] = true
	}

	out := make([]string, 0, len(set))
	for b := range set {
		out = append(out, b)
	}
	sort.Strings(out)
	return out, nil
}

// Push publishes branches to remote. With dryRun it decides and reports but
// changes nothing, not even remote-tracking refs: it reads the remote's
// tips with ls-remote rather than fetching, and uses the same
// classification for the real run.
//
// A branch is only pushed when the remote branch is missing (new) or is an
// ancestor of the local tip (fast-forward). Everything else is reported and
// left alone; a single branch's failure never stops the others.
func Push(root, remote string, branches []string, dryRun bool) ([]Result, error) {
	ok, err := git.RemoteExists(root, remote)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured in this repo", remote)
	}

	tips, err := git.LsRemoteHeads(root, remote)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, branch := range branches {
		localSHA, err := git.RevParse(root, "refs/heads/"+branch)
		if err != nil || localSHA == "" {
			return nil, fmt.Errorf("no local branch %q", branch)
		}
		rel, err := git.ClassifyTips(root, localSHA, tips[branch])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", branch, err)
		}

		res := Result{Branch: branch, Detail: rel.Range()}
		switch rel.State {
		case git.RelUpToDate:
			res.Verb, res.Detail = VerbUpToDate, ""
		case git.RelBehind:
			res.Verb, res.Detail = VerbBehind, rel.Range()+"  (run pmt sync)"
		case git.RelDiverged:
			res.Verb = VerbDiverged
		case git.RelRemoteMoved:
			res.Verb, res.Detail = VerbMoved, "remote has commits not fetched here  (run pmt sync)"
		case git.RelNoRemote, git.RelAhead:
			res.Verb = VerbPushed
			if rel.State == git.RelNoRemote {
				res.Verb = VerbNew
			}
			if !dryRun {
				if err := git.Push(root, remote, branch); err != nil {
					res.Verb, res.Detail = VerbFailed, err.Error()
				}
			}
		}
		results = append(results, res)
	}
	return results, nil
}

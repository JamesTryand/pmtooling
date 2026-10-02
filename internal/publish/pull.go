package publish

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

// Verbs specific to pull (and later sync); see doc/commands.md.
const (
	VerbCreated  Verb = "created"  // worktree (and local branch if needed) created
	VerbUpdated  Verb = "updated"  // clean worktree fast-forwarded
	VerbAhead    Verb = "ahead"    // local has unpushed commits; nothing to pull
	VerbDirty    Verb = "dirty"    // worktree has uncommitted changes; untouched
	VerbPrunable Verb = "prunable" // worktree registered but its directory is gone
	VerbOrphaned Verb = "orphaned" // a directory is in the way but git does not know it
)

// Pull brings one issue branch from the remote into this machine: it
// creates the local branch and worktree if they are missing, or
// fast-forwards a clean existing worktree. It never merges, resets or
// discards anything: a dirty or diverged worktree is reported and left as
// it is. With dryRun nothing is written (no fetch, no ref, no worktree);
// the remote is read with ls-remote, so a remote tip this repo has never
// fetched is reported as `moved` instead of guessed at.
func Pull(root, worktreesDir, remote, branch string, dryRun bool) (Result, error) {
	typeName, title := issue.Split(branch)
	if typeName == "" || title == "" || typeName == "pmt" {
		return Result{}, fmt.Errorf("%q is not an issue branch (<type>/<title>); pmt pull handles issues, use pmt sync for the archive and templates", branch)
	}
	if err := git.CheckRefFormat(branch); err != nil {
		return Result{}, err
	}
	ok, err := git.RemoteExists(root, remote)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{}, fmt.Errorf("remote %q is not configured in this repo", remote)
	}

	// Where is the remote branch?
	var remoteSHA string
	if dryRun {
		tips, err := git.LsRemoteHeads(root, remote)
		if err != nil {
			return Result{}, err
		}
		remoteSHA = tips[branch]
	} else {
		if err := git.FetchRemote(root, remote); err != nil {
			return Result{}, err
		}
		if exists, _ := git.RefExists(root, git.RemoteRef(remote, branch)); exists {
			if remoteSHA, err = git.RevParse(root, git.RemoteRef(remote, branch)); err != nil {
				return Result{}, err
			}
		}
	}

	localSHA := ""
	if exists, err := git.RefExists(root, "refs/heads/"+branch); err != nil {
		return Result{}, err
	} else if exists {
		if localSHA, err = git.RevParse(root, "refs/heads/"+branch); err != nil {
			return Result{}, err
		}
	}

	if remoteSHA == "" {
		return Result{}, missingOnRemote(root, remote, branch, localSHA != "", dryRun)
	}

	rel, err := git.ClassifyTips(root, localSHA, remoteSHA)
	if err != nil {
		return Result{}, err
	}
	res := Result{Branch: branch}
	wtPath := git.ComputeWorktreePath(root, worktreesDir, typeName, title)

	worktrees, err := git.ListWorktrees(root)
	if err != nil {
		return Result{}, err
	}
	var wt *git.Worktree
	for i := range worktrees {
		if worktrees[i].Branch == branch {
			wt = &worktrees[i]
		}
	}

	switch {
	case rel.State == git.RelDiverged:
		res.Verb, res.Detail = VerbDiverged, rel.Range()
		return res, nil
	case rel.State == git.RelRemoteMoved:
		res.Verb, res.Detail = VerbMoved, "remote has commits not fetched here; a real run fetches first"
		return res, nil
	case rel.State == git.RelAhead && wt != nil:
		res.Verb, res.Detail = VerbAhead, rel.Range()+"  (run pmt push)"
		return res, nil
	}

	// A registered worktree whose directory is gone cannot be updated or
	// recreated safely from here.
	if wt != nil && wt.Prunable {
		res.Verb, res.Detail = VerbPrunable, wt.Path+"  (run git worktree prune, then pull again)"
		return res, nil
	}

	if wt != nil {
		dirty, err := git.IsWorktreeDirty(wt.Path)
		if err != nil {
			return Result{}, err
		}
		if dirty {
			res.Verb, res.Detail = VerbDirty, wt.Path
			return res, nil
		}
		if rel.State == git.RelUpToDate {
			res.Verb = VerbUpToDate
			return res, nil
		}
		// RelBehind: fast-forward in place.
		res.Verb, res.Detail = VerbUpdated, rel.Range()
		if !dryRun {
			if _, err := git.Run(wt.Path, "merge", "--ff-only", "-q", git.RemoteRef(remote, branch)); err != nil {
				return Result{}, err
			}
		}
		return res, nil
	}

	// No worktree for this branch: create one, creating or fast-forwarding
	// the local branch first when needed.
	if err := issue.CheckWorktreePathFree(wtPath); err != nil {
		if errors.Is(err, issue.ErrOrphanedWorktreePath) {
			res.Verb, res.Detail = VerbOrphaned, wtPath
			return res, nil
		}
		return Result{}, err
	}
	res.Verb = VerbCreated
	switch rel.State {
	case git.RelNoLocal:
		res.Detail = "new local branch at " + git.ShortSHA(remoteSHA) + " -> " + wtPath
	case git.RelBehind:
		res.Detail = "fast-forwarded " + rel.Range() + " -> " + wtPath
	default: // up to date or ahead, branch exists locally
		res.Detail = wtPath
	}
	if dryRun {
		return res, nil
	}
	switch rel.State {
	case git.RelNoLocal:
		if _, err := git.Run(root, "branch", "--track", branch, git.RemoteRef(remote, branch)); err != nil {
			return Result{}, err
		}
	case git.RelBehind:
		// old-value guard: only moves the branch if it is still where we classified it
		if _, err := git.Run(root, "update-ref", "refs/heads/"+branch, remoteSHA, localSHA); err != nil {
			return Result{}, err
		}
	}
	if err := git.WorktreeAdd(root, wtPath, branch); err != nil {
		return Result{}, err
	}
	return res, nil
}

// missingOnRemote builds the error for a branch the remote does not have,
// with near matches so a typo is easy to spot.
func missingOnRemote(root, remote, branch string, existsLocally, dryRun bool) error {
	if existsLocally {
		return fmt.Errorf("%s exists locally but is not on %s; publish it with: pmt push %s", branch, remote, branch)
	}

	var names []string
	if dryRun {
		if tips, err := git.LsRemoteHeads(root, remote); err == nil {
			for b := range tips {
				names = append(names, b)
			}
		}
	} else {
		refs, _ := git.ForEachRef(root, "refs/remotes/"+remote, "%(refname:short)")
		for _, r := range refs {
			names = append(names, strings.TrimPrefix(r, remote+"/"))
		}
	}

	typeName, title := issue.Split(branch)
	var near []string
	for _, n := range names {
		nt, ntitle := issue.Split(n)
		if nt == "pmt" || n == "HEAD" || n == "" {
			continue
		}
		if strings.Contains(ntitle, title) || strings.Contains(title, ntitle) || (nt == typeName && ntitle != "" && strings.HasPrefix(ntitle, title[:min(len(title), 3)])) {
			near = append(near, n)
		}
	}
	sort.Strings(near)
	if len(near) > 5 {
		near = near[:5]
	}
	msg := fmt.Sprintf("issue %s is not on %s", branch, remote)
	if len(near) > 0 {
		msg += "; did you mean: " + strings.Join(near, ", ")
	}
	return errors.New(msg)
}

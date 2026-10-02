package publish

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/archive"
	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/template"
)

// Verbs specific to sync (see also pull.go and prune.go).
const (
	VerbRemoved     Verb = "removed"     // closed issue's clean worktree and branch removed
	VerbUnpublished Verb = "unpublished" // local issue with no remote copy and not archived; kept
	VerbClosed      Verb = "closed"      // archived here, but the remote still has the branch
	VerbDetached    Verb = "detached"    // a worktree with a detached HEAD; untouched
	VerbIgnored     Verb = "ignored"     // remote branch that is not a pmt issue
)

// SyncOptions controls Sync.
type SyncOptions struct {
	Remote     string
	DryRun     bool // changes no branch, worktree, file or remote; still fetches (like the script's -WhatIf)
	SkipUpdate bool // only create missing worktrees; leave existing ones alone
}

// Sync reconciles this machine's worktree farm with the remote, using pmt's
// own issue and archive semantics. It never pushes, never merges, never
// resets, and never removes anything that is not provably safe:
//
//   - fetches and prunes remote-tracking refs;
//   - fast-forwards the main checkout's default branch when clean;
//   - fast-forwards pmt/archive and pmt/template/*;
//   - for each remote issue branch: creates its worktree, or fast-forwards a
//     clean one (see Pull); dirty, diverged, ahead, prunable and orphaned
//     cases are reported and left alone;
//   - for each local issue branch with no remote copy: removes it (and its
//     clean worktree) only if the issue is archived and the tip holds
//     nothing the archive lacks (see archive.PruneWith); an unarchived one
//     is an open issue nobody has pushed yet and is reported `unpublished`;
//   - reports detached worktrees and directories under the worktrees root
//     that git does not know.
func Sync(root string, cfg config.RepoConfig, opts SyncOptions) ([]Result, error) {
	remote := opts.Remote
	if remote == "" {
		remote = git.DefaultRemote
	}
	ok, err := git.RemoteExists(root, remote)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("remote %q is not configured in this repo", remote)
	}
	// Without a fetch refspec git has nothing to write into refs/remotes, and
	// `git fetch` fails with an obscure message; say what is actually wrong.
	if out, _, _ := git.RunRaw(root, "config", "--get-all", "remote."+remote+".fetch"); out == "" {
		return nil, fmt.Errorf("remote %q has no fetch refspec, so nothing can be tracked; fix with: git config --add remote.%s.fetch '+refs/heads/*:refs/remotes/%s/*'", remote, remote, remote)
	}
	if err := git.FetchRemote(root, remote); err != nil {
		return nil, err
	}

	remoteTips, err := tipsUnder(root, "refs/remotes/"+remote+"/")
	if err != nil {
		return nil, err
	}
	delete(remoteTips, "HEAD")
	localTips, err := tipsUnder(root, "refs/heads/")
	if err != nil {
		return nil, err
	}

	worktrees, err := git.ListWorktrees(root)
	if err != nil {
		return nil, err
	}
	var main *git.Worktree
	if len(worktrees) > 0 && !worktrees[0].Bare {
		main = &worktrees[0]
	}

	defaultBranch := defaultBranchName(root, remote)
	rc := reconciler{root: root, worktreesDir: cfg.WorktreesDir, remote: remote, dryRun: opts.DryRun, skipUpdate: opts.SkipUpdate}

	var results []Result
	add := func(r Result) {
		if r.Verb != VerbSkipped {
			results = append(results, r)
		}
	}

	// The main checkout's default branch.
	if defaultBranch != "" && main != nil && main.Branch == defaultBranch && remoteTips[defaultBranch] != "" && !opts.SkipUpdate {
		r, err := rc.ref(defaultBranch, localTips[defaultBranch], remoteTips[defaultBranch])
		if err != nil {
			return nil, err
		}
		add(r)
	}

	remoteNames := make([]string, 0, len(remoteTips))
	for b := range remoteTips {
		if b != defaultBranch {
			remoteNames = append(remoteNames, b)
		}
	}
	sort.Strings(remoteNames)

	hasTemplate := func(typeName string) (bool, error) {
		if _, onRemote := remoteTips["pmt/template/"+typeName]; onRemote {
			return true, nil
		}
		return template.Exists(root, typeName)
	}

	// pmt's own refs (templates, archive) first, so issues can find their templates.
	archiveRef := archive.Ref
	for _, b := range remoteNames {
		if !strings.HasPrefix(b, "pmt/") {
			continue
		}
		r, err := rc.ref(b, localTips[b], remoteTips[b])
		if err != nil {
			return nil, err
		}
		add(r)
		// A dry run does not move the local archive, so judge closed issues
		// against the remote's copy when the real run would have moved it.
		if b == "pmt/archive" && opts.DryRun && (r.Verb == VerbCreated || r.Verb == VerbUpdated) {
			archiveRef = git.RemoteRef(remote, b)
		}
	}

	archived, err := archive.ListArchivedAt(root, archiveRef, "")
	if err != nil {
		return nil, err
	}
	archivedSet := map[string]bool{}
	for _, a := range archived {
		archivedSet[a.Branch] = true
	}

	for _, b := range remoteNames {
		if strings.HasPrefix(b, "pmt/") {
			continue
		}
		typeName, title := issue.Split(b)
		if title == "" {
			add(Result{Branch: b, Verb: VerbIgnored, Detail: "not a <type>/<title> issue branch"})
			continue
		}
		if has, err := hasTemplate(typeName); err != nil {
			return nil, err
		} else if !has {
			add(Result{Branch: b, Verb: VerbIgnored, Detail: "no pmt/template/" + typeName + " template: not a pmt issue"})
			continue
		}

		// Closed here but the remote branch lives on: never resurrect it.
		if archivedSet[b] && localTips[b] == "" {
			reopened, err := archive.ReopenedSinceArchiveAt(root, archiveRef, b, remoteTips[b])
			if err != nil {
				return nil, err
			}
			if !reopened {
				add(Result{Branch: b, Verb: VerbClosed, Detail: "archived here but still on " + remote + "; delete it there: git push " + remote + " --delete " + b})
				continue
			}
		}

		r, err := rc.issue(b, localTips[b], remoteTips[b])
		if err != nil {
			return nil, err
		}
		add(r)
	}

	// Local issue branches with no remote copy.
	gone := map[string]bool{}
	for b := range localTips {
		if strings.HasPrefix(b, "pmt/") || b == defaultBranch {
			continue
		}
		if _, onRemote := remoteTips[b]; onRemote {
			continue
		}
		typeName, title := issue.Split(b)
		if title == "" {
			continue
		}
		if has, err := hasTemplate(typeName); err != nil {
			return nil, err
		} else if has {
			gone[b] = true
		}
	}
	pruned, err := archive.PruneWith(root, archive.PruneOptions{
		DryRun:               opts.DryRun,
		RemoveCleanWorktrees: true,
		Filter:               func(b string) bool { return gone[b] },
		ArchiveRef:           archiveRef,
	})
	if err != nil {
		return nil, err
	}
	handled := map[string]bool{}
	for _, p := range pruned {
		handled[p.Branch] = true
		add(Result{Branch: p.Branch, Verb: Verb(p.Outcome), Detail: p.Detail})
	}
	for b := range gone {
		if !handled[b] {
			add(Result{Branch: b, Verb: VerbUnpublished, Detail: "no copy on " + remote + " and not archived: an open issue, kept (publish it: pmt push " + b + ")"})
		}
	}

	// Worktrees may have been created or removed above; look again.
	worktrees, err = git.ListWorktrees(root)
	if err != nil {
		return nil, err
	}

	// Worktrees with a detached HEAD.
	for i, w := range worktrees {
		if i == 0 || w.Bare || !w.Detached {
			continue
		}
		add(Result{Branch: w.Path, Verb: VerbDetached, Detail: "detached HEAD; left alone"})
	}

	// Directories under the worktrees root that git does not know.
	orphans, err := orphanedDirs(git.WorktreesRoot(root, cfg.WorktreesDir), worktrees)
	if err != nil {
		return nil, err
	}
	for _, o := range orphans {
		add(Result{Branch: o, Verb: VerbOrphaned, Detail: "has .git but git does not know it as a worktree; review by hand"})
	}

	sort.SliceStable(results, func(i, j int) bool { return results[i].Branch < results[j].Branch })
	return results, nil
}

// ref reconciles a branch that is not an issue (the default branch,
// pmt/archive, pmt/template/*): create it if missing, fast-forward it if
// behind (in its worktree if one has it checked out and clean), otherwise
// report. Never removes anything.
func (rc reconciler) ref(branch, localSHA, remoteSHA string) (Result, error) {
	res := Result{Branch: branch}
	rel, err := git.ClassifyTips(rc.root, localSHA, remoteSHA)
	if err != nil {
		return Result{}, err
	}

	switch rel.State {
	case git.RelUpToDate:
		res.Verb = VerbUpToDate
	case git.RelAhead:
		res.Verb, res.Detail = VerbAhead, rel.Range()+"  (run pmt push)"
	case git.RelDiverged:
		res.Verb, res.Detail = VerbDiverged, rel.Range()
	case git.RelRemoteMoved:
		res.Verb, res.Detail = VerbMoved, "remote has commits not fetched here"
	case git.RelNoLocal:
		res.Verb, res.Detail = VerbCreated, "new local branch at "+git.ShortSHA(remoteSHA)
		if !rc.dryRun {
			if _, err := git.Run(rc.root, "branch", "--track", branch, git.RemoteRef(rc.remote, branch)); err != nil {
				return Result{}, err
			}
		}
	case git.RelBehind:
		if rc.skipUpdate {
			res.Verb = VerbSkipped
			return res, nil
		}
		worktrees, err := git.ListWorktrees(rc.root)
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
		case wt != nil && wt.Prunable:
			res.Verb, res.Detail = VerbPrunable, wt.Path+"  (run git worktree prune)"
		case wt != nil:
			dirty, err := git.IsWorktreeDirty(wt.Path)
			if err != nil {
				return Result{}, err
			}
			if dirty {
				res.Verb, res.Detail = VerbDirty, wt.Path
				break
			}
			res.Verb, res.Detail = VerbUpdated, rel.Range()
			if !rc.dryRun {
				if _, err := git.Run(wt.Path, "merge", "--ff-only", "-q", git.RemoteRef(rc.remote, branch)); err != nil {
					return Result{}, err
				}
			}
		default:
			res.Verb, res.Detail = VerbUpdated, rel.Range()
			if !rc.dryRun {
				if _, err := git.Run(rc.root, "update-ref", "refs/heads/"+branch, remoteSHA, localSHA); err != nil {
					return Result{}, err
				}
			}
		}
	}
	return res, nil
}

// tipsUnder maps branch name (with prefix stripped) to commit SHA for every
// ref under prefix, e.g. "refs/heads/" or "refs/remotes/origin/".
func tipsUnder(root, prefix string) (map[string]string, error) {
	lines, err := git.ForEachRef(root, strings.TrimSuffix(prefix, "/"), "%(refname) %(objectname)")
	if err != nil {
		return nil, err
	}
	tips := map[string]string{}
	for _, l := range lines {
		name, sha, ok := strings.Cut(l, " ")
		if !ok || !strings.HasPrefix(name, prefix) {
			continue
		}
		tips[strings.TrimPrefix(name, prefix)] = sha
	}
	return tips, nil
}

// defaultBranchName is the remote's default branch (its HEAD), or "".
func defaultBranchName(root, remote string) string {
	out, code, err := git.RunRaw(root, "symbolic-ref", "-q", "refs/remotes/"+remote+"/HEAD")
	if err != nil || code != 0 {
		return ""
	}
	return strings.TrimPrefix(out, "refs/remotes/"+remote+"/")
}

// orphanedDirs returns directories under worktreesRoot that contain a .git
// entry but are not registered as worktrees.
func orphanedDirs(worktreesRoot string, registered []git.Worktree) ([]string, error) {
	if _, err := os.Stat(worktreesRoot); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	known := map[string]bool{}
	for _, w := range registered {
		known[normPath(w.Path)] = true
	}
	var orphans []string
	err := filepath.WalkDir(worktreesRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == worktreesRoot {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil {
			if !known[normPath(path)] {
				orphans = append(orphans, path)
			}
			return filepath.SkipDir // a worktree: do not look inside
		}
		return nil
	})
	sort.Strings(orphans)
	return orphans, err
}

func normPath(p string) string {
	p = filepath.Clean(p)
	if runtime.GOOS == "windows" {
		p = strings.ToLower(p)
	}
	return p
}

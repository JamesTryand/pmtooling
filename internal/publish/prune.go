package publish

import (
	"github.com/JamesTryand/pmtooling/internal/archive"
)

// Verbs specific to prune.
const (
	VerbPruned  Verb = "pruned"  // leftover local branch of a closed issue deleted
	VerbInUse   Verb = "in-use"  // closed issue's branch still checked out in a worktree
	VerbDiffers Verb = "differs" // branch holds something the archive lacks; kept
)

// Prune deletes local branches of closed issues, under the safety rules
// documented on archive.Prune, and reports each in the shared verb format.
func Prune(root string, dryRun bool) ([]Result, error) {
	rs, err := archive.Prune(root, dryRun)
	if err != nil {
		return nil, err
	}
	out := make([]Result, 0, len(rs))
	for _, r := range rs {
		out = append(out, Result{Branch: r.Branch, Verb: Verb(r.Outcome), Detail: r.Detail})
	}
	return out, nil
}

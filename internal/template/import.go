package template

import (
	"errors"
	"fmt"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/git"
)

const incomingPrefix = "refs/heads/pmt/template-incoming/"

// incomingRefFor is the scratch ref Update lands a fetched-but-not-yet-
// merged template update at.
func incomingRefFor(typeName string) string {
	return incomingPrefix + typeName
}

// ErrNotFound is returned by Update when typeName doesn't exist locally
// yet — Import must run first.
var ErrNotFound = errors.New("template does not exist locally")

// Import fetches typeName's template branch from another repo
// (sourceRoot, already resolved to a canonical main root) into dir,
// creating pmt/template/<typeName> there. This is for a first-time
// import: it errors if a template of that name already exists locally
// (same collision semantics as New) — use Update to pull in later
// changes to an already-imported template.
func Import(dir, sourceRoot, typeName string) error {
	exists, err := Exists(dir, typeName)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w: %q", ErrExists, typeName)
	}
	ref := RefFor(typeName)
	return git.Fetch(dir, sourceRoot, ref+":"+ref)
}

// UpdateResult describes the outcome of Update.
type UpdateResult struct {
	UpToDate      bool   // nothing new to pull
	FastForwarded bool   // local template was moved forward automatically
	IncomingRef   string // short ref name (no refs/heads/ prefix); set only when a manual merge is needed
}

// Update fetches typeName's template branch from sourceRoot into a
// scratch ref (refs/heads/pmt/template-incoming/<typeName>). If the
// local template hasn't diverged from the fetched commit, it's fast-
// forwarded automatically and the scratch ref is cleaned up. If it has
// diverged, no merge is attempted — the scratch ref is left in place for
// the user to merge manually with plain git (doc/templates.md's existing
// stance: pmt scaffolds templates but doesn't manage their ongoing
// content). Errors with ErrNotFound if the template doesn't exist
// locally yet.
func Update(dir, sourceRoot, typeName string) (UpdateResult, error) {
	exists, err := Exists(dir, typeName)
	if err != nil {
		return UpdateResult{}, err
	}
	if !exists {
		return UpdateResult{}, fmt.Errorf("%w: %q (use `pmt template new %s --from <source>` to import it first)", ErrNotFound, typeName, typeName)
	}

	localRef := RefFor(typeName)
	incomingRef := incomingRefFor(typeName)
	if err := git.Fetch(dir, sourceRoot, localRef+":"+incomingRef); err != nil {
		return UpdateResult{}, err
	}

	localTip, err := git.Run(dir, "rev-parse", localRef)
	if err != nil {
		return UpdateResult{}, err
	}
	incomingTip, err := git.Run(dir, "rev-parse", incomingRef)
	if err != nil {
		return UpdateResult{}, err
	}

	if localTip == incomingTip {
		if err := git.DeleteRef(dir, incomingRef); err != nil {
			return UpdateResult{}, err
		}
		return UpdateResult{UpToDate: true}, nil
	}

	// Local already ahead of (or equal to — handled above) what's incoming.
	localAhead, err := git.IsAncestor(dir, incomingTip, localTip)
	if err != nil {
		return UpdateResult{}, err
	}
	if localAhead {
		if err := git.DeleteRef(dir, incomingRef); err != nil {
			return UpdateResult{}, err
		}
		return UpdateResult{UpToDate: true}, nil
	}

	canFastForward, err := git.IsAncestor(dir, localTip, incomingTip)
	if err != nil {
		return UpdateResult{}, err
	}
	if canFastForward {
		if err := git.UpdateRef(dir, localRef, incomingTip); err != nil {
			return UpdateResult{}, err
		}
		if err := git.DeleteRef(dir, incomingRef); err != nil {
			return UpdateResult{}, err
		}
		return UpdateResult{FastForwarded: true}, nil
	}

	// Diverged: leave the incoming ref for the user to merge manually.
	return UpdateResult{IncomingRef: strings.TrimPrefix(incomingRef, "refs/heads/")}, nil
}

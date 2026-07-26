package issue

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/template"
)

// Result is what Create returns on success.
type Result struct {
	Branch       string // full "<type>/<title>"
	WorktreePath string
}

// ErrOrphanedWorktreePath is returned when the computed worktree path is
// already occupied by a directory that isn't the branch pmt is about to
// create — pmt never deletes a directory it didn't just create itself.
var ErrOrphanedWorktreePath = errors.New("worktree path already occupied by an unrelated directory")

// maxAutoTitleRetries bounds the collision-retry loop for auto-generated
// titles (a race with a concurrent `pmt new`, or another tool creating
// the same branch name in between).
const maxAutoTitleRetries = 5

// Create implements `pmt new <type>[/<title>]` end-to-end, per
// doc/commands.md#pmt-new: validates the type, resolves its template,
// generates or validates the title, resolves title collisions, creates
// the branch at the template's tip, checks out a sibling worktree, and
// stamps + commits the issue's README.md front matter.
func Create(mainRepoRoot string, repoCfg config.RepoConfig, typeName, title string) (Result, error) {
	if err := ValidateType(typeName); err != nil {
		return Result{}, err
	}

	exists, err := template.Exists(mainRepoRoot, typeName)
	if err != nil {
		return Result{}, err
	}
	if !exists {
		return Result{}, fmt.Errorf(
			"template type %q not found; run `pmt template list` to see available types, or `pmt template new %s` to create one",
			typeName, typeName)
	}
	templateRef := template.RefFor(typeName)
	templateCommit, err := git.Run(mainRepoRoot, "rev-parse", templateRef)
	if err != nil {
		return Result{}, err
	}

	branch, resolvedTitle, err := resolveTitle(mainRepoRoot, typeName, title, repoCfg.TitlePadWidth)
	if err != nil {
		return Result{}, err
	}

	worktreePath := git.ComputeWorktreePath(mainRepoRoot, repoCfg.WorktreesDir, typeName, resolvedTitle)
	if err := CheckWorktreePathFree(worktreePath); err != nil {
		return Result{}, err
	}

	if _, err := git.Run(mainRepoRoot, "branch", branch, templateRef); err != nil {
		return Result{}, err
	}
	// From here on, a failure leaves an orphaned branch with no worktree —
	// deliberately not rolled back. That's already a state `pmt list`
	// (Phase 5) is designed to surface, so no separate cleanup logic is
	// needed for v1.
	if err := git.WorktreeAdd(mainRepoRoot, worktreePath, branch); err != nil {
		return Result{}, err
	}
	_, err = StampReadmeInWorktree(worktreePath, fmt.Sprintf("pmt: initialize issue %s", branch), func(meta *Meta) {
		meta.Type = typeName
		meta.Title = resolvedTitle
		meta.Branch = branch
		meta.Status = "open"
		meta.Created = time.Now().UTC().Format(time.RFC3339)
		meta.TemplateRef = templateCommit
	})
	if err != nil {
		return Result{}, err
	}

	return Result{Branch: branch, WorktreePath: worktreePath}, nil
}

// resolveTitle generates (if title is empty) and validates a title, then
// resolves collisions on refs/heads/<type>/<title>: a user-supplied title
// that collides is a hard error; an auto-generated one is retried up to
// maxAutoTitleRetries times (NextAutoTitle re-scans current refs each
// call, so a retry naturally picks up whatever a racing process just
// created).
func resolveTitle(mainRepoRoot, typeName, title string, padWidth int) (branch, resolvedTitle string, err error) {
	autoTitle := title == ""

	for attempt := 0; ; attempt++ {
		candidate := title
		if autoTitle {
			candidate, err = NextAutoTitle(mainRepoRoot, typeName, padWidth)
			if err != nil {
				return "", "", err
			}
		}
		if err := ValidateTitle(typeName, candidate); err != nil {
			return "", "", err
		}

		candidateBranch := typeName + "/" + candidate
		branchExists, err := git.RefExists(mainRepoRoot, "refs/heads/"+candidateBranch)
		if err != nil {
			return "", "", err
		}
		if !branchExists {
			return candidateBranch, candidate, nil
		}
		if !autoTitle {
			return "", "", fmt.Errorf("issue branch %q already exists", candidateBranch)
		}
		if attempt >= maxAutoTitleRetries-1 {
			return "", "", fmt.Errorf("could not find a free auto-generated title for type %q after %d attempts", typeName, maxAutoTitleRetries)
		}
	}
}

// CheckWorktreePathFree errors if path is already occupied by a
// directory that isn't a worktree pmt is about to create itself — pmt
// never deletes a directory it didn't just create. Shared by `pmt new`
// and `pmt reopen`, which both need to check this before calling
// git.WorktreeAdd.
func CheckWorktreePathFree(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s (remove it manually — and run `git worktree prune` if it was a stale worktree — before retrying)", ErrOrphanedWorktreePath, path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

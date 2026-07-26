package issue

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	if err := checkWorktreePathFree(worktreePath); err != nil {
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
	if err := stampReadme(worktreePath, typeName, resolvedTitle, branch, templateCommit); err != nil {
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

func checkWorktreePathFree(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s (remove it manually — and run `git worktree prune` if it was a stale worktree — before retrying)", ErrOrphanedWorktreePath, path)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

// stampReadme fills in the issue-specific README.md front-matter fields
// (doc/templates.md#readme-front-matter-schema) and commits the result
// inside the issue's own worktree — this is what lets `pmt list` (Phase
// 5) read metadata via `git show <branch>:README.md` with no worktree
// present.
func stampReadme(worktreePath, typeName, title, branch, templateCommit string) error {
	path := filepath.Join(worktreePath, "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	meta, body, _ := Parse(content) // ok ignored: absent/corrupted front matter just starts from a fresh Meta{}
	meta.Type = typeName
	meta.Title = title
	meta.Branch = branch
	meta.Status = "open"
	meta.Created = time.Now().UTC().Format(time.RFC3339)
	meta.TemplateRef = templateCommit

	rendered, err := Render(meta, body)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	if _, err := git.Run(worktreePath, "add", "README.md"); err != nil {
		return err
	}
	if _, err := git.Run(worktreePath, "commit", "-q", "-m", fmt.Sprintf("pmt: initialize issue %s", branch)); err != nil {
		return err
	}
	return nil
}

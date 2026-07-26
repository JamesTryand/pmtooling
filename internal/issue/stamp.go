package issue

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/JamesTryand/pmtooling/internal/git"
)

// StampReadmeInWorktree reads README.md from worktreePath, applies mutate
// to its parsed front matter (inserting a fresh block if absent, per
// Parse's contract), writes the result back, and commits it (git add +
// git commit) inside that worktree. Returns the new commit SHA.
func StampReadmeInWorktree(worktreePath, commitMessage string, mutate func(*Meta)) (string, error) {
	path := filepath.Join(worktreePath, "README.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	meta, body, _ := Parse(content)
	mutate(&meta)

	rendered, err := Render(meta, body)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, rendered, 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}

	if _, err := git.Run(worktreePath, "add", "README.md"); err != nil {
		return "", err
	}
	if _, err := git.Run(worktreePath, "commit", "-q", "-m", commitMessage); err != nil {
		return "", err
	}
	return git.Run(worktreePath, "rev-parse", "HEAD")
}

// StampReadmeViaPlumbing mutates the pmt front matter of README.md on
// branchRef (which need not have any worktree checked out) and returns
// the new commit SHA — built purely via plumbing (hash-object/mktree/
// commit-tree), mirroring how templates are created without a working
// tree (doc/templates.md). Does not move branchRef itself; callers
// decide what to do with the returned SHA.
func StampReadmeViaPlumbing(dir, branchRef, commitMessage string, mutate func(*Meta)) (string, error) {
	content, err := git.ReadBlob(dir, branchRef+":README.md")
	if err != nil {
		return "", fmt.Errorf("reading README.md from %s: %w", branchRef, err)
	}
	meta, body, _ := Parse(content)
	mutate(&meta)

	rendered, err := Render(meta, body)
	if err != nil {
		return "", err
	}

	currentTip, err := git.Run(dir, "rev-parse", branchRef)
	if err != nil {
		return "", err
	}
	entries, err := git.LsTree(dir, currentTip)
	if err != nil {
		return "", err
	}
	blobSHA, err := git.HashObject(dir, rendered)
	if err != nil {
		return "", err
	}

	// README.md always lives at the branch's tree root — replace that one
	// entry, keep every sibling (CLAUDE.md, .gitignore, .claude/, ...)
	// unchanged by SHA reference.
	newEntries := make([]git.TreeEntry, 0, len(entries)+1)
	for _, e := range entries {
		if e.Name != "README.md" {
			newEntries = append(newEntries, e)
		}
	}
	newEntries = append(newEntries, git.TreeEntry{Mode: "100644", Type: "blob", SHA: blobSHA, Name: "README.md"})

	newTree, err := git.Mktree(dir, newEntries)
	if err != nil {
		return "", err
	}
	return git.CommitTree(dir, newTree, commitMessage, currentTip)
}

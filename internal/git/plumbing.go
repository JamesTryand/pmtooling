package git

import (
	"fmt"
	"strings"
)

// HashObject writes content as a git blob object — without touching the
// working tree or index — and returns its SHA.
func HashObject(dir string, content []byte) (string, error) {
	return RunWithStdin(dir, content, "hash-object", "-w", "--stdin")
}

// TreeEntry is one line of `git mktree` input.
type TreeEntry struct {
	Mode string // "100644" for a regular file, "040000" for a subtree
	Type string // "blob" or "tree"
	SHA  string
	Name string
}

// Mktree builds a (non-recursive) tree object from entries and returns
// its SHA. Entries need not be pre-sorted — git normalizes tree ordering
// itself. Subtrees must already exist and be passed as "tree" entries.
func Mktree(dir string, entries []TreeEntry) (string, error) {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s %s %s\t%s\n", e.Mode, e.Type, e.SHA, e.Name)
	}
	return RunWithStdin(dir, []byte(b.String()), "mktree")
}

// CommitTree creates a commit object pointing at tree with the given
// parents (pass none for an orphan commit) and returns its SHA. It never
// touches the working tree, index, or HEAD — this is what lets
// `pmt template new` build a template's initial commit without disturbing
// whatever the caller currently has checked out.
func CommitTree(dir, tree, message string, parents ...string) (string, error) {
	args := []string{"commit-tree", tree}
	for _, p := range parents {
		args = append(args, "-p", p)
	}
	args = append(args, "-m", message)
	return Run(dir, args...)
}

// UpdateRef creates or updates ref to point at sha.
func UpdateRef(dir, ref, sha string) error {
	_, err := Run(dir, "update-ref", ref, sha)
	return err
}

// DeleteRef deletes ref.
func DeleteRef(dir, ref string) error {
	_, err := Run(dir, "update-ref", "-d", ref)
	return err
}

// Fetch runs `git fetch source refspec` — fetching one specific ref from
// another repo (a local path or a remote URL) directly into dir, without
// needing a configured remote.
func Fetch(dir, source, refspec string) error {
	_, err := Run(dir, "fetch", source, refspec)
	return err
}

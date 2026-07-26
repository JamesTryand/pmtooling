// Package template implements pmt's template branches: the
// refs/heads/pmt/template/<type> namespace, listing, and scaffolding new
// templates. See doc/templates.md.
package template

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/template/scaffold"
)

const refPrefix = "refs/heads/pmt/template/"

// RefFor returns the full ref name for a template type, e.g.
// RefFor("bug") == "refs/heads/pmt/template/bug". This is the single place
// the pmt/template/<type> namespace convention is expressed — see
// doc/templates.md for why templates can't be bare <type> branches (a
// git ref D/F conflict with issue branches <type>/<title>).
func RefFor(typeName string) string {
	return refPrefix + typeName
}

// Exists reports whether a template branch for typeName exists in the
// repo at dir.
func Exists(dir, typeName string) (bool, error) {
	return git.RefExists(dir, RefFor(typeName))
}

// List returns the names of all template branches in the repo at dir,
// sorted.
func List(dir string) ([]string, error) {
	refs, err := git.ForEachRef(dir, refPrefix+"*", "%(refname:short)")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(refs))
	for _, r := range refs {
		names = append(names, strings.TrimPrefix(r, "pmt/template/"))
	}
	sort.Strings(names)
	return names, nil
}

// ErrExists is returned by New when a template branch for typeName
// already exists.
var ErrExists = errors.New("template already exists")

// New scaffolds a new template branch for typeName: validates the name,
// checks for a collision, renders the starter files (doc/templates.md),
// and builds the initial commit purely with git plumbing
// (hash-object/mktree/commit-tree/update-ref) — never `git switch
// --orphan`, which would mutate the caller's current checkout/index in
// the main repo. Returns the new commit SHA.
func New(dir, typeName string) (string, error) {
	if strings.Contains(typeName, "/") {
		return "", fmt.Errorf("template name must not contain '/': got %q", typeName)
	}
	ref := RefFor(typeName)
	if err := git.CheckRefFormat(strings.TrimPrefix(ref, "refs/heads/")); err != nil {
		return "", fmt.Errorf("invalid template name %q: %w", typeName, err)
	}

	exists, err := Exists(dir, typeName)
	if err != nil {
		return "", err
	}
	if exists {
		return "", fmt.Errorf("%w: %q", ErrExists, typeName)
	}

	files, err := scaffold.Render(scaffold.Data{Type: typeName})
	if err != nil {
		return "", err
	}

	rootTree, err := buildTree(dir, files)
	if err != nil {
		return "", err
	}

	commit, err := git.CommitTree(dir, rootTree, fmt.Sprintf("pmt: initialize template '%s'", typeName))
	if err != nil {
		return "", err
	}
	if err := git.UpdateRef(dir, ref, commit); err != nil {
		return "", err
	}
	return commit, nil
}

// treeNode stages a flat list of scaffold.File paths into the nested
// shape git.Mktree needs — a real path like ".claude/settings.json"
// requires a subtree, not a single flat mktree call.
type treeNode struct {
	blobs map[string]string
	dirs  map[string]*treeNode
}

func newTreeNode() *treeNode {
	return &treeNode{blobs: map[string]string{}, dirs: map[string]*treeNode{}}
}

func buildTree(dir string, files []scaffold.File) (string, error) {
	root := newTreeNode()
	for _, f := range files {
		sha, err := git.HashObject(dir, f.Content)
		if err != nil {
			return "", fmt.Errorf("hashing %s: %w", f.Path, err)
		}
		parts := strings.Split(path.Clean(f.Path), "/")
		cur := root
		for i, part := range parts {
			if i == len(parts)-1 {
				cur.blobs[part] = sha
				continue
			}
			child, ok := cur.dirs[part]
			if !ok {
				child = newTreeNode()
				cur.dirs[part] = child
			}
			cur = child
		}
	}
	return writeTree(dir, root)
}

func writeTree(dir string, n *treeNode) (string, error) {
	entries := make([]git.TreeEntry, 0, len(n.blobs)+len(n.dirs))
	for name, sha := range n.blobs {
		entries = append(entries, git.TreeEntry{Mode: "100644", Type: "blob", SHA: sha, Name: name})
	}
	for name, child := range n.dirs {
		sha, err := writeTree(dir, child)
		if err != nil {
			return "", err
		}
		entries = append(entries, git.TreeEntry{Mode: "040000", Type: "tree", SHA: sha, Name: name})
	}
	return git.Mktree(dir, entries)
}

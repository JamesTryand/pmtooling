package git

import "strings"

// RevParseQuiet resolves rev to a SHA via `git rev-parse -q --verify`.
// Non-existence is reported as ok=false, not an error — only a genuine
// execution failure (e.g. git not found) is returned as err.
func RevParseQuiet(dir, rev string) (sha string, ok bool, err error) {
	out, code, err := RunRaw(dir, "rev-parse", "-q", "--verify", rev)
	if err != nil {
		return "", false, err
	}
	if code != 0 {
		return "", false, nil
	}
	return out, true, nil
}

// TreeEntrySHA resolves "<treeish>:<path>" to the blob/tree SHA at that
// path, or ok=false if the path doesn't exist there. Non-existence is
// not an error.
func TreeEntrySHA(dir, treeish, path string) (sha string, ok bool, err error) {
	return RevParseQuiet(dir, treeish+":"+path)
}

// LsTreeFiles lists every file under treeish, recursively, as a map from
// slash-separated path to blob SHA. Submodule entries are ignored.
func LsTreeFiles(dir, treeish string) (map[string]string, error) {
	out, err := Run(dir, "ls-tree", "-r", "-z", treeish)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, rec := range strings.Split(out, "\x00") {
		meta, path, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 || fields[1] != "blob" {
			continue
		}
		files[path] = fields[2]
	}
	return files, nil
}

// LsTree lists the direct (non-recursive) entries of treeish.
func LsTree(dir, treeish string) ([]TreeEntry, error) {
	out, err := Run(dir, "ls-tree", treeish)
	if err != nil {
		return nil, err
	}
	var entries []TreeEntry
	for _, line := range Lines(out) {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		fields := strings.Fields(parts[0])
		if len(fields) != 3 {
			continue
		}
		entries = append(entries, TreeEntry{Mode: fields[0], Type: fields[1], SHA: fields[2], Name: parts[1]})
	}
	return entries, nil
}

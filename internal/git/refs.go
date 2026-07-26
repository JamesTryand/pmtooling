package git

import "fmt"

// CheckRefFormat validates branch as a well-formed git branch name via
// `git check-ref-format --branch`, which is the authoritative check for
// `..`, `~^:?*[`, `@{`, leading/trailing/doubled `/`, `.lock` suffixes,
// and control characters. It does not require running inside a repository.
func CheckRefFormat(branch string) error {
	_, code, err := RunRaw("", "check-ref-format", "--branch", branch)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("invalid branch name: %q", branch)
	}
	return nil
}

// RefExists reports whether refName (e.g. "refs/heads/bug/dboverflow")
// exists in the repository at dir.
func RefExists(dir, refName string) (bool, error) {
	_, code, err := RunRaw(dir, "show-ref", "--verify", "--quiet", refName)
	if err != nil {
		return false, err
	}
	return code == 0, nil
}

// ForEachRef runs `git for-each-ref --format=format pattern` and returns
// one result string per matching ref, in git's own sort order.
func ForEachRef(dir, pattern, format string) ([]string, error) {
	out, err := Run(dir, "for-each-ref", "--format="+format, pattern)
	if err != nil {
		return nil, err
	}
	return Lines(out), nil
}

// IsAncestor reports whether ancestor is reachable from descendant (i.e.
// fast-forwarding ancestor to descendant would be safe). A commit is its
// own ancestor, so this is also true when the two are equal. Exit code 1
// ("not an ancestor") is a real answer (false, nil error); any other
// non-zero code (e.g. 128 for an invalid revision) is a genuine error,
// not silently treated as "false" — verified empirically that git
// distinguishes these two cases with different exit codes.
func IsAncestor(dir, ancestor, descendant string) (bool, error) {
	_, code, err := RunRaw(dir, "merge-base", "--is-ancestor", ancestor, descendant)
	if err != nil {
		return false, err
	}
	switch code {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, fmt.Errorf("git merge-base --is-ancestor %s %s: exit status %d", ancestor, descendant, code)
	}
}

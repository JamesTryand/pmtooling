package issue

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/JamesTryand/pmtooling/internal/git"
)

var numericTitleRe = regexp.MustCompile(`^[0-9]+$`)

// NextAutoTitle scans existing <typeName>/* branches and returns the
// next sequential numeric title, zero-padded to padWidth digits. Only
// purely numeric title segments are considered — a hand-created sibling
// like bug/investigate-perf is silently skipped, never misread as a
// number. This is max(existing)+1, not count+1, so a deleted/renamed
// issue never causes a future title to be reused. See
// doc/commands.md#pmt-new's auto-title algorithm.
func NextAutoTitle(dir, typeName string, padWidth int) (string, error) {
	refs, err := git.ForEachRef(dir, "refs/heads/"+typeName+"/*", "%(refname:short)")
	if err != nil {
		return "", err
	}

	prefix := typeName + "/"
	max := 0
	for _, r := range refs {
		title := strings.TrimPrefix(r, prefix)
		if !numericTitleRe.MatchString(title) {
			continue
		}
		n, err := strconv.Atoi(title)
		if err != nil {
			continue
		}
		if n > max {
			max = n
		}
	}
	return fmt.Sprintf("%0*d", padWidth, max+1), nil
}

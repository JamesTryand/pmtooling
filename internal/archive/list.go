package archive

import (
	"sort"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
)

// ArchivedIssue is one row of `pmt list --archived`.
type ArchivedIssue struct {
	Branch      string `json:"branch"`
	Closed      string `json:"closed"`
	Unparseable bool   `json:"unparseable"`
}

// ListArchived lists every currently-archived issue — one entry per
// <type>/<title> directory in the CURRENT tree of refs/heads/pmt/archive
// (not its full history). Since the archive is append-only, this always
// reflects the most recent close for anything not yet reopened; a
// reopened issue's stale entry remains listed until it's closed again
// (see the package doc — this is intentional, not a bug). A missing or
// unparseable README.md is non-fatal, matching `pmt list`'s own
// tolerance for corrupted metadata.
func ListArchived(dir, typeFilter string) ([]ArchivedIssue, error) {
	tip, ok, err := git.RevParseQuiet(dir, Ref)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	typeEntries, err := git.LsTree(dir, tip)
	if err != nil {
		return nil, err
	}

	var result []ArchivedIssue
	for _, te := range typeEntries {
		if te.Type != "tree" {
			continue
		}
		if typeFilter != "" && te.Name != typeFilter {
			continue
		}
		titleEntries, err := git.LsTree(dir, te.SHA)
		if err != nil {
			return nil, err
		}
		for _, tte := range titleEntries {
			if tte.Type != "tree" {
				continue
			}
			branch := te.Name + "/" + tte.Name
			ai := ArchivedIssue{Branch: branch}
			content, err := git.ReadBlob(dir, tip+":"+branch+"/README.md")
			if err != nil {
				ai.Unparseable = true
			} else if meta, _, ok := issue.Parse(content); ok {
				ai.Closed = meta.Closed
			} else {
				ai.Unparseable = true
			}
			result = append(result, ai)
		}
	}

	sort.Slice(result, func(i, j int) bool { return result[i].Branch < result[j].Branch })
	return result, nil
}

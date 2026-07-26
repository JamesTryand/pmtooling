package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

func newListCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var typeFilter string
	var jsonOut bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues in the target repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			issues, err := issue.ListIssues(r.Root, r.Config.WorktreesDir, typeFilter)
			if err != nil {
				return err
			}
			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(issues)
			}
			return renderIssueTable(cmd.OutOrStdout(), issues)
		},
	}
	cmd.Flags().StringVar(&typeFilter, "type", "", "filter by issue type")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "output as JSON")
	return cmd
}

func renderIssueTable(w io.Writer, issues []issue.Issue) error {
	if len(issues) == 0 {
		fmt.Fprintln(w, "No issues found.")
		return nil
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "BRANCH\tSTATUS\tCREATED\tWORKTREE")
	for _, iss := range issues {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", iss.Branch, statusDisplay(iss), iss.Created, worktreeDisplay(iss))
	}
	return tw.Flush()
}

func statusDisplay(iss issue.Issue) string {
	if iss.Unparseable {
		return "<unparseable>"
	}
	return iss.Status
}

func worktreeDisplay(iss issue.Issue) string {
	switch iss.WorktreeState {
	case issue.WorktreeOK:
		return iss.WorktreePath
	case issue.WorktreePrunable:
		return iss.WorktreePath + " (prunable)"
	case issue.WorktreeOrphaned:
		return iss.WorktreePath + " (orphaned dir)"
	default:
		return "(none)"
	}
}

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/publish"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

// ExitCodeError makes main exit with Code. It is for commands that finish
// but want to signal something (exit 2: some items need attention). An empty
// Msg prints nothing further; the command already reported what happened.
type ExitCodeError struct {
	Code int
	Msg  string
}

func (e *ExitCodeError) Error() string { return e.Msg }

// ExitCodeAttention is returned when a command completed but left items that
// need a person to look (see ACCEPTANCE.md G7).
const ExitCodeAttention = 2

func newPushCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	var all, dryRun bool
	var remote string

	cmd := &cobra.Command{
		Use:   "push [<type>/<title>]",
		Short: "Publish issue branches to the remote (never forces)",
		Long: `Push one issue's branch, or with --all every issue branch plus the archive
and templates, to the remote. With no argument, pushes the branch checked out
in the current directory.

Branches whose remote counterpart has moved are reported and left alone:
"behind" and "moved" need 'pmt sync', "diverged" needs a person. Exit status
is 0 when everything is pushed or up to date, 2 when some branches need
attention, 1 on error.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}
			if all && len(args) == 1 {
				return errors.New("give either an issue or --all, not both")
			}

			managed, err := publish.ManagedBranches(r.Root)
			if err != nil {
				return err
			}

			var branches []string
			switch {
			case all:
				branches = managed
			case len(args) == 1:
				branches = []string{args[0]}
			default:
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				b, onBranch, err := git.CurrentBranch(cwd)
				if err != nil || !onBranch {
					return errors.New("no issue given and the current directory is not on a branch; pass <type>/<title> or --all")
				}
				branches = []string{b}
			}

			if !all {
				known := map[string]bool{}
				for _, m := range managed {
					known[m] = true
				}
				if !known[branches[0]] {
					return fmt.Errorf("%q is not a pmt-managed branch (issue, pmt/archive or pmt/template/*); nothing pushed", branches[0])
				}
			}

			results, err := publish.Push(r.Root, remote, branches, dryRun)
			if err != nil {
				return err
			}
			return reportResults(cmd.OutOrStdout(), results, dryRun, "nothing to push")
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "push every issue branch, the archive and the templates")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be pushed without pushing")
	cmd.Flags().StringVar(&remote, "remote", git.DefaultRemote, "remote name to push to")
	return cmd
}

// reportResults prints one line per result and a count summary, and returns
// an ExitCodeError (exit 2) when any result needs attention. Shared by push,
// pull and sync so every command speaks the same verbs.
func reportResults(w io.Writer, results []publish.Result, dryRun bool, empty string) error {
	counts := map[publish.Verb]int{}
	attention := false
	for _, r := range results {
		counts[r.Verb]++
		attention = attention || r.Verb.NeedsAttention()
		line := fmt.Sprintf("%-11s %s", r.Verb, r.Branch)
		if r.Detail != "" {
			line += "  " + r.Detail
		}
		fmt.Fprintln(w, line)
	}

	var parts []string
	verbs := make([]string, 0, len(counts))
	for v := range counts {
		verbs = append(verbs, string(v))
	}
	sort.Strings(verbs)
	for _, v := range verbs {
		parts = append(parts, fmt.Sprintf("%d %s", counts[publish.Verb(v)], v))
	}
	summary := empty
	if len(parts) > 0 {
		summary = strings.Join(parts, ", ")
	}
	if dryRun {
		summary += " (dry run: nothing was changed)"
	}
	fmt.Fprintln(w, summary)

	if attention {
		return &ExitCodeError{Code: ExitCodeAttention}
	}
	return nil
}

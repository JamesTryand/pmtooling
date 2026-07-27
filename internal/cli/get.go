package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/archive"
	"github.com/JamesTryand/pmtooling/internal/git"
	"github.com/JamesTryand/pmtooling/internal/issue"
	"github.com/JamesTryand/pmtooling/internal/repo"
)

// newGetCmd implements `pmt get [<type>/<title>]`, per doc/commands.md.
// On success it prints exactly the issue's worktree path to stdout and
// nothing else, so it composes as `cd "$(pmt get ...)"` (or, safer,
// `dir=$(pmt get ...) && cd "$dir"`, which never cds at all on failure).
// Named "get" rather than "goto" precisely because it never changes any
// directory itself — a subprocess can't change its parent shell's cwd.
// Every other outcome — not found, archived, or a live branch with no
// usable worktree — writes a human-readable explanation to stderr via the
// returned error (cobra's default error path) and a non-zero exit, with
// stdout left empty either way.
func newGetCmd(resolve func() (repo.Repo, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "get [<type>/<title>]",
		Short: `Print an issue's worktree path (cd "$(pmt get ...)")`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := resolve()
			if err != nil {
				return err
			}

			target := ""
			if len(args) == 1 {
				target = args[0]
			}
			if target == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				branch, onBranch, err := git.CurrentBranch(cwd)
				if err != nil {
					return err
				}
				_, title := issue.Split(branch)
				if !onBranch || title == "" {
					return fmt.Errorf("no issue given, and the current branch doesn't look like an issue\nusage: pmt get <type>/<title>\nlist open issues:     pmt list\nlist archived issues: pmt list --archived")
				}
				target = branch
			}

			typeName, title := issue.Split(target)
			if title == "" {
				return fmt.Errorf("%q doesn't look like an issue (expected <type>/<title>)", target)
			}

			issues, err := issue.ListIssues(r.Root, r.Config.WorktreesDir, typeName)
			if err != nil {
				return err
			}
			for _, iss := range issues {
				if iss.Branch != target {
					continue
				}
				if iss.WorktreeState != issue.WorktreeOK {
					return fmt.Errorf("%s exists but its worktree isn't available (state: %s)\nsee: pmt list", target, iss.WorktreeState)
				}
				fmt.Fprintln(cmd.OutOrStdout(), iss.WorktreePath)
				return nil
			}

			archived, err := archive.ListArchived(r.Root, typeName)
			if err != nil {
				return err
			}
			for _, ai := range archived {
				if ai.Branch != target {
					continue
				}
				if ai.Closed != "" {
					return fmt.Errorf("%s is archived (closed %s)\nreopen it with: pmt reopen %s", target, ai.Closed, target)
				}
				return fmt.Errorf("%s is archived\nreopen it with: pmt reopen %s", target, target)
			}

			return fmt.Errorf("%s not found (open or archived)\nlist open issues:     pmt list\nlist archived issues: pmt list --archived", target)
		},
	}
}

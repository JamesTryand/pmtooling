package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/JamesTryand/pmtooling/internal/config"
	"github.com/JamesTryand/pmtooling/internal/git"
)

// newRepoCmd is unrelated to the global --repo flag / resolveRepo: it
// manages the user-level repos: nickname map itself (previously only
// hand-editable YAML), so it deliberately doesn't take the resolve
// closure other subcommands use.
func newRepoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Manage configured target-repo nicknames",
	}
	cmd.AddCommand(newRepoAddCmd())
	cmd.AddCommand(newRepoListCmd())
	cmd.AddCommand(newRepoRemoveCmd())
	cmd.AddCommand(newRepoSetDefaultCmd())
	return cmd
}

func newRepoAddCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "add <nickname> <path>",
		Short: "Add (or update) a target-repo nickname",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			nickname, path := args[0], args[1]
			if _, err := git.Discover(path); err != nil {
				return fmt.Errorf("%q does not resolve to a git repository: %w", path, err)
			}
			cfg, err := config.LoadUserConfig()
			if err != nil {
				return err
			}
			if err := cfg.AddRepo(nickname, path, force); err != nil {
				return err
			}
			if err := config.SaveUserConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added repo %q -> %s\n", nickname, path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing nickname")
	return cmd
}

func newRepoListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured target-repo nicknames",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUserConfig()
			if err != nil {
				return err
			}
			if len(cfg.Repos) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No repos configured. Run `pmt repo add <nickname> <path>` to add one.")
				return nil
			}
			names := make([]string, 0, len(cfg.Repos))
			for name := range cfg.Repos {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				marker := ""
				if name == cfg.DefaultRepo {
					marker = " (default)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s -> %s%s\n", name, cfg.Repos[name], marker)
			}
			return nil
		},
	}
}

func newRepoRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <nickname>",
		Short: "Remove a target-repo nickname",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUserConfig()
			if err != nil {
				return err
			}
			wasDefault := cfg.DefaultRepo == args[0]
			if err := cfg.RemoveRepo(args[0]); err != nil {
				return err
			}
			if err := config.SaveUserConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed repo %q\n", args[0])
			if wasDefault {
				fmt.Fprintln(cmd.OutOrStdout(), "It was the default repo; default_repo has been cleared.")
			}
			return nil
		},
	}
}

func newRepoSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <nickname>",
		Short: "Set the default target-repo nickname",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadUserConfig()
			if err != nil {
				return err
			}
			if err := cfg.SetDefault(args[0]); err != nil {
				return err
			}
			if err := config.SaveUserConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Default repo set to %q\n", args[0])
			return nil
		},
	}
}

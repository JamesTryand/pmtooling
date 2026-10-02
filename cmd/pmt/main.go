// Command pmt manages issues as git branches and worktrees in a target
// repository. See doc/commands.md for the command reference.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/JamesTryand/pmtooling/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		var exit *cli.ExitCodeError
		if errors.As(err, &exit) {
			if exit.Msg != "" {
				fmt.Fprintln(os.Stderr, "pmt:", exit.Msg)
			}
			os.Exit(exit.Code)
		}
		fmt.Fprintln(os.Stderr, "pmt:", err)
		os.Exit(1)
	}
}

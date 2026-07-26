// Command pmt manages issues as git branches and worktrees in a target
// repository. See doc/commands.md for the command reference.
package main

import (
	"fmt"
	"os"

	"github.com/JamesTryand/pmtooling/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "pmt:", err)
		os.Exit(1)
	}
}

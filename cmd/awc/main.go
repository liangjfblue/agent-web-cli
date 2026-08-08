// awc is the command-line entry point for agent-web-cli.
package main

import (
	"fmt"
	"os"

	"github.com/agent/web-cli/internal/cmd"
)

func main() {
	if err := cmd.NewRootCmd(&cmd.Runtime{}).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

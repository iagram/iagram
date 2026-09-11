// Command iagram is the local infrastructure-as-diagram tool.
package main

import (
	"os"

	"github.com/iagram/iagram/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}

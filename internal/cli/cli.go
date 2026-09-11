// Package cli implements the iagram command. Subcommands are kept on the
// standard library on purpose: the binary should stay small and boring.
package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// Version is set by the linker (-X) in releases.
var Version = "dev"

// Run executes the command line and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	var err error
	switch args[0] {
	case "init":
		err = runInit(args[1:], stdout)
	case "up":
		err = runUp(args[1:], stdout)
	case "validate":
		err = runValidate(args[1:], stdout)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "iagram %s\n", Version)
	case "help", "-h", "--help":
		usage(stdout)
	default:
		fmt.Fprintf(stderr, "iagram: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		fmt.Fprintf(stderr, "iagram: %v\n", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `iagram: infrastructure as diagram.

Usage:
  iagram init [name]       create iagram.json in the current directory
  iagram up [flags]        open the canvas at http://localhost:7777
  iagram validate [flags]  check iagram.json against the catalog
  iagram version           print the version

Flags for up/validate:
  -f, --file    diagram file (default "iagram.json")
  -p, --port    listen port for up (default 7777)
      --no-open do not open the browser

iagram never stores or transmits cloud credentials; it uses the same local
credential chain as the cloud CLIs and Terraform.
`)
}

const defaultFile = "iagram.json"

func fileFlag(fs *flag.FlagSet) *string {
	f := fs.String("file", defaultFile, "diagram file")
	fs.StringVar(f, "f", defaultFile, "diagram file")
	return f
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

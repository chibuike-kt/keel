// Package cli implements the keel command-line interface.
package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/chibuike-kt/keel/internal/buildinfo"
)

const usage = `keel scaffolds production-grade fintech backends.

Usage:
  keel [flags]

Flags:
`

// Run executes the keel CLI with the given arguments, writing normal output to
// out and diagnostics to errOut. It returns the process exit code.
func Run(args []string, out, errOut io.Writer) int {
	if len(args) > 0 && args[0] == "init" {
		return cmdInit(args[1:], out, errOut)
	}

	fs := flag.NewFlagSet("keel", flag.ContinueOnError)
	fs.SetOutput(errOut)

	version := fs.Bool("version", false, "print the keel version and exit")

	fs.Usage = func() {
		fmt.Fprint(errOut, usage)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *version {
		fmt.Fprintln(out, buildinfo.Version)
		return 0
	}

	fs.Usage()
	return 0
}

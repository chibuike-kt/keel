// Package cli implements the keel command-line interface.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

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
	// Every real subcommand must be matched here, before the
	// unknown-command catch-all below — that catch-all has to stay the
	// last thing checked, since it treats any non-flag args[0] that
	// reaches it as an error. A subcommand added above this block and
	// left unlisted here would be silently swallowed by the catch-all
	// instead of ever running: this exact bug already happened once,
	// when a merge dropped the "add" branch and left "list"'s
	// unreachable beneath the catch-all.
	if len(args) > 0 && args[0] == "init" {
		return cmdInit(args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "add" {
		return cmdAdd(args[1:], out, errOut)
	}
	if len(args) > 0 && args[0] == "list" {
		return cmdList(out, errOut)
	}

	// A non-flag first argument that isn't one of the known subcommands
	// above is a typo or an unimplemented command, not the "no command
	// given" case below — flag.FlagSet.Parse treats it as a positional
	// arg and returns no error, so without this check it would silently
	// fall through to the usage printer and exit 0.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		fmt.Fprintf(errOut, "keel: unknown command %q\n", args[0])
		fmt.Fprintln(errOut, "Run 'keel -h' for usage.")
		return 1
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

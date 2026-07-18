package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/chibuike-kt/keel/internal/buildinfo"
)

func TestRunVersionFlag(t *testing.T) {
	var out, errOut bytes.Buffer

	code := Run([]string{"--version"}, &out, &errOut)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := strings.TrimSpace(out.String()); got != buildinfo.Version {
		t.Fatalf("version = %q, want %q", got, buildinfo.Version)
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var out, errOut bytes.Buffer

	code := Run([]string{"--unknown"}, &out, &errOut)

	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
}

// TestRun_UnknownCommandReturnsNonZeroExit proves an unrecognized
// subcommand (a typo, or a not-yet-implemented command like "add" or
// "list") is a real error, not the "no command given" case: it must
// exit non-zero and name what was typed on stderr, not silently fall
// through to the usage printer with exit 0.
func TestRun_UnknownCommandReturnsNonZeroExit(t *testing.T) {
	for _, cmd := range []string{"add", "list", "innit"} {
		t.Run(cmd, func(t *testing.T) {
			var out, errOut bytes.Buffer

			code := Run([]string{cmd}, &out, &errOut)

			if code == 0 {
				t.Fatalf("exit code = 0, want non-zero for unknown command %q", cmd)
			}
			if !strings.Contains(errOut.String(), cmd) {
				t.Fatalf("stderr = %q, want it to mention the unrecognized command %q", errOut.String(), cmd)
			}
		})
	}
}

// TestRunNoArgsPrintsUsageAndExitsZero proves "no command given" stays
// the non-error case it always was: it must not be caught by the
// unknown-command check above.
func TestRunNoArgsPrintsUsageAndExitsZero(t *testing.T) {
	var out, errOut bytes.Buffer

	code := Run(nil, &out, &errOut)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for no arguments", code)
	}
}

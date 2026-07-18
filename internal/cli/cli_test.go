package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chibuike-kt/keel/internal/buildinfo"
	"github.com/chibuike-kt/keel/internal/state"
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
// subcommand (a typo, or a command that was never implemented) is a
// real error, not the "no command given" case: it must exit non-zero
// and name what was typed on stderr, not silently fall through to the
// usage printer with exit 0. "add" and "list" are deliberately not
// tested here — they're real, dispatched commands (see
// TestRun_KnownCommandsDispatchToRealImplementations), and asserting
// them as "unknown" would silently stop meaning anything the moment
// either command's dispatch broke, exactly as happened once already
// when a merge dropped the "add" branch.
func TestRun_UnknownCommandReturnsNonZeroExit(t *testing.T) {
	for _, cmd := range []string{"innit", "frobnicate", "remove"} {
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

// TestRun_KnownCommandsDispatchToRealImplementations calls Run itself
// with realistic argv-shaped input for init, add, and list — not
// runInit/runAdd/runList directly — and asserts something that could
// only be true if each command's real logic actually executed, not
// just "exit code was zero". This is the regression test for the bug
// that let a merge silently drop Run()'s "add" dispatch and leave
// "list"'s dispatch unreachable beneath the unknown-command catch-all:
// runAdd and runList already had thorough direct tests, so neither gap
// showed up in go test ./... until Run() itself — the actual argv
// entry point — was exercised end to end. If either dispatch broke
// again the same way, every assertion below would fail: init's project
// files, add's rendered module and updated state.json, and list's
// selection markers can only exist if cmdInit/cmdAdd/cmdList genuinely
// ran.
func TestRun_KnownCommandsDispatchToRealImplementations(t *testing.T) {
	t.Run("init", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer

		code := Run([]string{"init", "myapp", "--modules", "idempotency"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("Run(init) = %d, want 0\nstderr:\n%s", code, errOut.String())
		}

		if _, err := os.Stat(filepath.Join("myapp", "go.mod")); err != nil {
			t.Fatalf("Run(init) did not generate a real project: %v", err)
		}
		if _, err := os.Stat(filepath.Join("myapp", "internal", "idempotency", "middleware.go")); err != nil {
			t.Fatalf("Run(init) did not render the requested module's real template: %v", err)
		}
	})

	t.Run("add", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer
		if code := Run([]string{"init", "myapp", "--modules", "idempotency"}, &out, &errOut); code != 0 {
			t.Fatalf("Run(init) setup failed: %d\n%s", code, errOut.String())
		}
		if err := os.Chdir("myapp"); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		out.Reset()
		errOut.Reset()
		code := Run([]string{"add", "security"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("Run(add) = %d, want 0\nstderr:\n%s", code, errOut.String())
		}

		if _, err := os.Stat(filepath.Join("internal", "security", "headers.go")); err != nil {
			t.Fatalf("Run(add) did not render security's real template: %v", err)
		}
		st, err := state.Load(".")
		if err != nil {
			t.Fatalf("state.Load: %v", err)
		}
		if !st.Has("security") {
			t.Fatalf("state.json modules = %v, want security recorded as added", st.Modules)
		}
	})

	t.Run("list", func(t *testing.T) {
		t.Chdir(t.TempDir())
		var out, errOut bytes.Buffer
		if code := Run([]string{"init", "myapp", "--modules", "idempotency"}, &out, &errOut); code != 0 {
			t.Fatalf("Run(init) setup failed: %d\n%s", code, errOut.String())
		}
		if err := os.Chdir("myapp"); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		out.Reset()
		errOut.Reset()
		code := Run([]string{"list"}, &out, &errOut)
		if code != 0 {
			t.Fatalf("Run(list) = %d, want 0\nstderr:\n%s", code, errOut.String())
		}

		got := out.String()
		if !strings.Contains(got, "[x] idempotency") {
			t.Fatalf("Run(list) output missing idempotency marked selected:\n%s", got)
		}
		if !strings.Contains(got, "[ ] ledger") {
			t.Fatalf("Run(list) output missing ledger marked unselected:\n%s", got)
		}
	})

	t.Run("unknown_command_still_rejected", func(t *testing.T) {
		var out, errOut bytes.Buffer
		code := Run([]string{"frobnicate"}, &out, &errOut)
		if code != 1 {
			t.Fatalf("Run(frobnicate) = %d, want 1", code)
		}
		if !strings.Contains(errOut.String(), `unknown command "frobnicate"`) {
			t.Fatalf("stderr = %q, want it to name the unknown command", errOut.String())
		}
	})
}

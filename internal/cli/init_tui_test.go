package cli

import (
	"os"
	"testing"
)

// TestIsInteractiveTerminalRejectsNonTTY confirms the TTY-detection
// function itself: an os.Pipe's read end is a real *os.File (unlike a
// test's bytes.Reader/strings.Reader, which isn't an *os.File at all and
// so never reaches this function — see runInit's stdin.(*os.File) type
// switch), but it is never a terminal, so this must report false.
func TestIsInteractiveTerminalRejectsNonTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if isInteractiveTerminal(r) {
		t.Fatal("isInteractiveTerminal(pipe) = true, want false: a pipe is never a terminal")
	}
}

// The huh-based picker (promptForModulesTUI, and the runInit branch that
// calls it) is deliberately NOT exercised by an automated test here.
//
// huh drives a real Bubble Tea program against a terminal — rendering
// frames, reading raw key events, repainting on WindowSizeMsg — none of
// which exists in a CI process whose stdin/stdout are pipes, not a PTY.
// Forcing this under test would mean either faking a PTY (heavyweight,
// fragile, and still wouldn't prove real terminal behavior) or reaching
// into huh's internals to drive its Bubble Tea model directly (coupling
// this codebase to huh's implementation details instead of its public
// API, and testing huh's own rendering, not keel's use of it).
//
// This is the same category of documented gap as Nomba's
// replay-protection note and health's goroutine-leak note elsewhere in
// this project: a real, known limitation, stated plainly rather than
// worked around with a test that doesn't actually prove what it claims.
// runInit's TTY branch itself is covered structurally — isInteractiveTerminal
// is tested above, and the type switch in runInit means a non-*os.File
// stdin (every existing test's strings.Reader) can never reach the huh
// path at all, which is exactly why every prior plain-text test keeps
// passing unchanged. The huh rendering path itself was verified manually
// instead; see the PR description / commit message for what was observed.

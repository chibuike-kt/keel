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

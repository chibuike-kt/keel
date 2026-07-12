package renderer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
)

func TestWriteEnvExampleContent(t *testing.T) {
	dir := t.TempDir()
	vars := []manifest.EnvVar{
		{Name: "PORT", Description: "HTTP server port", Required: false, Default: "8080"},
		{Name: "DATABASE_URL", Description: "PostgreSQL connection string", Required: true},
	}

	if err := writeEnvExample(dir, vars); err != nil {
		t.Fatalf("writeEnvExample: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	// Sorted alphabetically by name: DATABASE_URL before PORT, regardless
	// of input order.
	want := "# PostgreSQL connection string\n# Required\nDATABASE_URL=\n" +
		"\n# HTTP server port\n# Optional (default: 8080)\nPORT=8080\n"
	if string(got) != want {
		t.Fatalf(".env.example = %q, want %q", got, want)
	}
}

func TestWriteEnvExampleEmptyStillWritesHeaderOnlyFile(t *testing.T) {
	dir := t.TempDir()

	if err := writeEnvExample(dir, nil); err != nil {
		t.Fatalf("writeEnvExample: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "# No environment variables required\n"
	if string(got) != want {
		t.Fatalf(".env.example = %q, want %q", got, want)
	}
}

func TestWriteEnvExampleDeterministic(t *testing.T) {
	// Deliberately out of name order, to prove sorting is what makes the
	// output deterministic, not discovery order — same shape as the
	// go.mod determinism test.
	vars := []manifest.EnvVar{
		{Name: "ZOO_URL", Description: "z", Required: true},
		{Name: "APP_NAME", Description: "a", Required: false, Default: "myapp"},
	}

	dirA, dirB := t.TempDir(), t.TempDir()
	if err := writeEnvExample(dirA, vars); err != nil {
		t.Fatalf("writeEnvExample: %v", err)
	}
	if err := writeEnvExample(dirB, vars); err != nil {
		t.Fatalf("writeEnvExample: %v", err)
	}

	a, err := os.ReadFile(filepath.Join(dirA, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dirB, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf(".env.example not byte-identical:\na = %q\nb = %q", a, b)
	}
	if got := string(a); got[:len("# a\n")] != "# a\n" {
		t.Fatalf(".env.example not sorted by name (APP_NAME should come first): %q", got)
	}
}

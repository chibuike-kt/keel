package renderer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteGoModContent(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{ModulePath: "github.com/user/myapp", GoVersion: "1.26"}
	deps := []dependency{
		{module: "github.com/redis/go-redis/v9", version: "v9.5.1", via: "idempotency"},
		{module: "github.com/x/a", version: "v1.0.0", via: "other"},
	}

	if err := writeGoMod(dir, ctx, deps); err != nil {
		t.Fatalf("writeGoMod: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "module github.com/user/myapp\n\ngo 1.26\n\nrequire (\n\tgithub.com/redis/go-redis/v9 v9.5.1\n\tgithub.com/x/a v1.0.0\n)\n"
	if string(got) != want {
		t.Fatalf("go.mod = %q, want %q", got, want)
	}
}

func TestWriteGoModNoDependencies(t *testing.T) {
	dir := t.TempDir()
	ctx := Context{ModulePath: "github.com/user/myapp", GoVersion: "1.26"}

	if err := writeGoMod(dir, ctx, nil); err != nil {
		t.Fatalf("writeGoMod: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "module github.com/user/myapp\n\ngo 1.26\n"
	if string(got) != want {
		t.Fatalf("go.mod = %q, want %q", got, want)
	}
}

func TestWriteGoModDeterministic(t *testing.T) {
	ctx := Context{ModulePath: "github.com/user/myapp", GoVersion: "1.26"}
	// Deliberately out of module-path order, to prove sorting is what makes
	// the output deterministic, not discovery order.
	deps := []dependency{
		{module: "github.com/z/last", version: "v1.0.0", via: "m1"},
		{module: "github.com/a/first", version: "v1.0.0", via: "m2"},
	}

	dirA, dirB := t.TempDir(), t.TempDir()
	if err := writeGoMod(dirA, ctx, deps); err != nil {
		t.Fatalf("writeGoMod: %v", err)
	}
	if err := writeGoMod(dirB, ctx, deps); err != nil {
		t.Fatalf("writeGoMod: %v", err)
	}

	a, err := os.ReadFile(filepath.Join(dirA, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dirB, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("go.mod not byte-identical:\na = %q\nb = %q", a, b)
	}

	requireA := "require (\n\tgithub.com/a/first v1.0.0\n\tgithub.com/z/last v1.0.0\n)\n"
	if got := string(a); got[len(got)-len(requireA):] != requireA {
		t.Fatalf("go.mod requires not sorted by module path: %q", got)
	}
}

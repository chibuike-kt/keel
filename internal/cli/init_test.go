package cli

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"

	keel "github.com/chibuike-kt/keel"
	"github.com/chibuike-kt/keel/internal/catalog"
	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
)

func realCatalog(t *testing.T) resolver.MapCatalog {
	t.Helper()

	modulesFS, err := fs.Sub(keel.Modules, "modules")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}
	cat, err := catalog.LoadCatalog(modulesFS)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	return cat
}

// TestRunInitGeneratesBuildableProject is the end-to-end test: real
// embedded catalog and templates, a fixed --modules selection (so the
// interactive stdin prompt is never reached), targeting a relative name
// in a temp working directory (t.Chdir) — name must pass validateName,
// so it can no longer be an absolute path the way earlier revisions of
// this test used. It asserts the expected files exist AND that the
// generated project actually compiles — the same real-build-verification
// pattern internal/e2e already established for base's empty-selection
// case.
func TestRunInitGeneratesBuildableProject(t *testing.T) {
	t.Chdir(t.TempDir())

	var out, errOut bytes.Buffer
	code := runInit(
		[]string{"myapp", "--modules", "idempotency,security"},
		&out, &errOut, strings.NewReader(""),
		realCatalog(t), keel.Modules,
	)

	if code != 0 {
		t.Fatalf("runInit() exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on success", errOut.String())
	}

	for _, want := range []string{
		"myapp/README.md", "myapp/.gitignore", "myapp/cmd/api/main.go", "myapp/go.mod", "myapp/.env.example",
		"myapp/internal/idempotency/middleware.go",
		"myapp/internal/security/headers.go",
	} {
		if _, err := os.Stat(want); err != nil {
			t.Errorf("generated project missing %s: %v", want, err)
		}
	}

	// The printed summary must actually name what got generated and
	// where, not just say "done" — this is the one place a human sees
	// next steps, so it has to be real information, not a placeholder.
	if !strings.Contains(out.String(), "idempotency") || !strings.Contains(out.String(), "security") || !strings.Contains(out.String(), "base") {
		t.Errorf("success summary = %q, want it to name base, idempotency, and security", out.String())
	}
	if !strings.Contains(out.String(), "cat README.md") {
		t.Errorf("success summary = %q, want a pointer to README.md", out.String())
	}

	// The renderer doesn't emit go.sum, so any external dependency (here,
	// idempotency's go-redis) needs a tidy first — exactly what a real
	// user would need to run, and offline-safe since go-redis is already
	// in the local module cache.
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = "myapp"
	if tidyOut, err := tidyCmd.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy in generated project failed: %v\n%s", err, tidyOut)
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = "myapp"
	buildOut, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... in generated project failed: %v\n%s", err, buildOut)
	}
}

// TestRunInitPrintsEveryAggregatedRenderProblem proves RenderError's
// aggregated problems all reach stderr, not just the first — using a
// synthetic conflicting catalog, since no two real shipped modules
// currently conflict with each other. Constructs two independent
// conflicts (a duplicate output path AND a dependency version mismatch)
// in one plan, so a test that only checked "an error happened" or "the
// first problem is mentioned" would still pass even if the second
// conflict type were silently dropped — this test requires BOTH to be
// visible.
func TestRunInitPrintsEveryAggregatedRenderProblem(t *testing.T) {
	t.Chdir(t.TempDir())

	base := &manifest.Manifest{
		Name: "base",
		Languages: map[string]manifest.Language{
			"go": {Templates: []manifest.Template{{From: "x.tmpl", To: "base_marker.go"}}},
		},
	}
	a := &manifest.Manifest{
		Name: "a",
		Languages: map[string]manifest.Language{
			"go": {
				Templates:    []manifest.Template{{From: "x.tmpl", To: "internal/shared.go"}},
				Dependencies: []manifest.Dependency{{Module: "example.com/thing", Version: "v1.0.0"}},
			},
		},
	}
	b := &manifest.Manifest{
		Name: "b",
		Languages: map[string]manifest.Language{
			"go": {
				Templates:    []manifest.Template{{From: "x.tmpl", To: "internal/shared.go"}},
				Dependencies: []manifest.Dependency{{Module: "example.com/thing", Version: "v2.0.0"}},
			},
		},
	}
	cat := resolver.MapCatalog{"base": base, "a": a, "b": b}

	templates := fstest.MapFS{
		"modules/base/x.tmpl": &fstest.MapFile{Data: []byte("package base\n")},
		"modules/a/x.tmpl":    &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/x.tmpl":    &fstest.MapFile{Data: []byte("package b\n")},
	}

	var out, errOut bytes.Buffer
	code := runInit(
		[]string{"myapp", "--modules", "a,b"},
		&out, &errOut, strings.NewReader(""),
		cat, templates,
	)

	if code == 0 {
		t.Fatalf("runInit() exit code = 0, want non-zero (a and b conflict)")
	}

	stderr := errOut.String()
	if !strings.Contains(stderr, "internal/shared.go") {
		t.Errorf("stderr missing the duplicate-output-path problem:\n%s", stderr)
	}
	if !strings.Contains(stderr, "example.com/thing") {
		t.Errorf("stderr missing the dependency-conflict problem:\n%s", stderr)
	}
	if _, err := os.Stat("myapp"); err == nil {
		t.Errorf("target directory was created despite a failed render")
	}
}

// TestRunInitRejectsInvalidNames proves validateName runs before
// anything else uses name: for every case here, runInit must exit
// non-zero with a message naming the validation failure, and must never
// reach renderer.Render (which would instead produce a "generation
// failed" message, or succeed and create a directory).
func TestRunInitRejectsInvalidNames(t *testing.T) {
	tests := []struct {
		name    string
		argName string
	}{
		{"colon", "my:app"},
		{"backslash", `my\app`},
		{"double_quote", `weird"name`},
		{"newline", "weird\nname"},
		{"path_traversal", "../evil"},
		{"empty", ""},
	}

	cat, templates := minimalCatalog()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			var out, errOut bytes.Buffer
			code := runInit(
				[]string{tt.argName, "--modules", "base"},
				&out, &errOut, strings.NewReader(""),
				cat, templates,
			)

			if code == 0 {
				t.Fatalf("runInit() exit code = 0, want non-zero for invalid name %q", tt.argName)
			}
			if !strings.Contains(errOut.String(), "invalid project name") {
				t.Fatalf("stderr = %q, want it to mention \"invalid project name\"", errOut.String())
			}
			if strings.Contains(errOut.String(), "generation failed") {
				t.Fatalf("stderr = %q, should never reach renderer.Render for an invalid name", errOut.String())
			}
		})
	}
}

// TestRunInitAcceptsValidNames confirms validateName isn't overly
// strict: ordinary project names must still work end to end.
func TestRunInitAcceptsValidNames(t *testing.T) {
	cat, templates := minimalCatalog()

	for _, name := range []string{"myapp", "my-app_2"} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			var out, errOut bytes.Buffer
			code := runInit(
				[]string{name, "--modules", "base"},
				&out, &errOut, strings.NewReader(""),
				cat, templates,
			)

			if code != 0 {
				t.Fatalf("runInit() exit code = %d, want 0\nstderr:\n%s", code, errOut.String())
			}
			if _, err := os.Stat(name); err != nil {
				t.Errorf("generated project directory %q missing: %v", name, err)
			}
		})
	}
}

// TestRunInitInteractivePromptAll proves the "all" stdin response
// includes every catalog module, not just base.
func TestRunInitInteractivePromptAll(t *testing.T) {
	t.Chdir(t.TempDir())

	cat, templates := twoModuleCatalog()

	var out, errOut bytes.Buffer
	code := runInit([]string{"myapp"}, &out, &errOut, strings.NewReader("all\n"), cat, templates)

	if code != 0 {
		t.Fatalf("runInit() exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "modules: base, extra") {
		t.Fatalf("success summary = %q, want both base and extra included for an \"all\" response", out.String())
	}
}

// TestRunInitInteractivePromptEmptyMeansBaseOnly proves a blank stdin
// response resolves to base only, not zero modules and not every
// module — the one behavior the prompt text has to state explicitly so
// it isn't a surprise.
func TestRunInitInteractivePromptEmptyMeansBaseOnly(t *testing.T) {
	t.Chdir(t.TempDir())

	cat, templates := twoModuleCatalog()

	var out, errOut bytes.Buffer
	code := runInit([]string{"myapp"}, &out, &errOut, strings.NewReader("\n"), cat, templates)

	if code != 0 {
		t.Fatalf("runInit() exit code = %d, want 0\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
	// Check the actual final summary line specifically, not "extra"
	// anywhere in stdout — the module listing itself legitimately prints
	// "extra" as an available option regardless of what gets selected.
	if !strings.Contains(out.String(), "Generated myapp with modules: base\n") {
		t.Fatalf("success summary = %q, want exactly \"modules: base\" for a blank response (base only, extra excluded)", out.String())
	}

	// The prompt text itself must say what blank does, not just this
	// test's expectations — otherwise a user hitting Enter has no way
	// to know what they're about to get.
	if !strings.Contains(out.String(), "base only") {
		t.Fatalf("prompt output = %q, want it to state explicitly that a blank response means base only", out.String())
	}
}

// TestRunInitHelpFlagPrintsUsageAndCreatesNothing is the regression test
// for the bug found live during the 2026-07-16 audit: "keel init -h"
// treated "-h" as the project name (it passes validateName) and
// generated a real project directory named "-h". Both -h and --help
// must instead print init's usage to stdout, exit 0, and — the actual
// failure mode that happened — must never create a directory at all.
func TestRunInitHelpFlagPrintsUsageAndCreatesNothing(t *testing.T) {
	cat, templates := minimalCatalog()

	for _, flag := range []string{"-h", "--help", "-help"} {
		t.Run(flag, func(t *testing.T) {
			t.Chdir(t.TempDir())

			var out, errOut bytes.Buffer
			code := runInit([]string{flag}, &out, &errOut, strings.NewReader(""), cat, templates)

			if code != 0 {
				t.Fatalf("runInit() exit code = %d, want 0\nstderr:\n%s", code, errOut.String())
			}
			if !strings.Contains(out.String(), "keel init <name>") {
				t.Fatalf("stdout = %q, want init's usage text", out.String())
			}
			if errOut.Len() != 0 {
				t.Fatalf("stderr = %q, want empty on --help", errOut.String())
			}

			entries, err := os.ReadDir(".")
			if err != nil {
				t.Fatalf("ReadDir(.): %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("directory entries created = %v, want none", entries)
			}
		})
	}
}

// TestRunInitMissingNameErrorsClearly covers the two "no usable name"
// shapes the audit flagged as unconfirmed: no arguments at all, and
// flags given with no positional name (which, before the fix, would
// have been misread as the project name itself instead of producing a
// clear error).
func TestRunInitMissingNameErrorsClearly(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"no_args", nil},
		{"flags_only_no_name", []string{"--modules=idempotency,security"}},
	}

	cat, templates := minimalCatalog()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())

			var out, errOut bytes.Buffer
			code := runInit(tt.args, &out, &errOut, strings.NewReader(""), cat, templates)

			if code == 0 {
				t.Fatalf("runInit() exit code = 0, want non-zero for args %v", tt.args)
			}
			if !strings.Contains(errOut.String(), "missing project name") {
				t.Fatalf("stderr = %q, want it to mention a missing project name", errOut.String())
			}

			entries, err := os.ReadDir(".")
			if err != nil {
				t.Fatalf("ReadDir(.): %v", err)
			}
			if len(entries) != 0 {
				t.Fatalf("directory entries created = %v, want none", entries)
			}
		})
	}
}

func minimalCatalog() (resolver.MapCatalog, fs.FS) {
	base := &manifest.Manifest{
		Name: "base",
		Languages: map[string]manifest.Language{
			"go": {Templates: []manifest.Template{{From: "x.tmpl", To: "base_marker.go"}}},
		},
	}
	templates := fstest.MapFS{
		"modules/base/x.tmpl": &fstest.MapFile{Data: []byte("package base\n")},
	}
	return resolver.MapCatalog{"base": base}, templates
}

func twoModuleCatalog() (resolver.MapCatalog, fs.FS) {
	base := &manifest.Manifest{
		Name:    "base",
		Summary: "the base module",
		Languages: map[string]manifest.Language{
			"go": {Templates: []manifest.Template{{From: "x.tmpl", To: "base_marker.go"}}},
		},
	}
	extra := &manifest.Manifest{
		Name:    "extra",
		Summary: "an extra, optional module",
		Languages: map[string]manifest.Language{
			"go": {Templates: []manifest.Template{{From: "x.tmpl", To: "extra_marker.go"}}},
		},
	}
	templates := fstest.MapFS{
		"modules/base/x.tmpl":  &fstest.MapFile{Data: []byte("package base\n")},
		"modules/extra/x.tmpl": &fstest.MapFile{Data: []byte("package extra\n")},
	}
	return resolver.MapCatalog{"base": base, "extra": extra}, templates
}

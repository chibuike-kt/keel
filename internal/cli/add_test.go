package cli

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
	"github.com/chibuike-kt/keel/internal/state"
)

// addTestCatalog is base + three addable modules: a (no deps/env), b
// (a dependency and a required env var), and conflict (the same
// dependency as b but at a different version, for the requires/conflicts
// tests).
func addTestCatalog() (resolver.MapCatalog, fs.FS) {
	base := &manifest.Manifest{
		Name:    "base",
		Version: "0.1.0",
		Languages: map[string]manifest.Language{
			"go": {Templates: []manifest.Template{{From: "x.tmpl", To: "base_marker.go"}}},
		},
	}
	a := &manifest.Manifest{
		Name:    "a",
		Version: "0.1.0",
		Languages: map[string]manifest.Language{
			"go": {Templates: []manifest.Template{{From: "a.tmpl", To: "internal/a.go"}}},
		},
	}
	b := &manifest.Manifest{
		Name:    "b",
		Version: "0.2.0",
		Languages: map[string]manifest.Language{
			"go": {
				Templates:    []manifest.Template{{From: "b.tmpl", To: "internal/b.go"}},
				Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}},
			},
		},
		EnvVars: []manifest.EnvVar{{Name: "B_VAR", Description: "b var", Required: true}},
	}
	conflict := &manifest.Manifest{
		Name:    "conflict",
		Version: "0.1.0",
		Languages: map[string]manifest.Language{
			"go": {
				Templates:    []manifest.Template{{From: "c.tmpl", To: "internal/conflict.go"}},
				Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v2.0.0"}},
			},
		},
	}

	templates := fstest.MapFS{
		"modules/base/x.tmpl":     &fstest.MapFile{Data: []byte("package base\n")},
		"modules/a/a.tmpl":        &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/b.tmpl":        &fstest.MapFile{Data: []byte("package b\n")},
		"modules/conflict/c.tmpl": &fstest.MapFile{Data: []byte("package conflict\n")},
	}
	return resolver.MapCatalog{"base": base, "a": a, "b": b, "conflict": conflict}, templates
}

// bootstrapProject runs the real runInit path to produce a project at
// "myapp" under the test's current (already-chdir'd) temp directory,
// selecting only the modules named in selected, and returns its
// absolute path.
func bootstrapProject(t *testing.T, cat resolver.MapCatalog, templates fs.FS, selected ...string) string {
	t.Helper()

	var out, errOut bytes.Buffer
	args := []string{"myapp"}
	if len(selected) > 0 {
		args = append(args, "--modules", strings.Join(selected, ","))
	} else {
		args = append(args, "--modules", "") // base only, no interactive prompt
	}
	if code := runInit(args, &out, &errOut, strings.NewReader(""), cat, templates); code != 0 {
		t.Fatalf("bootstrapProject runInit() = %d, stderr:\n%s", code, errOut.String())
	}

	abs, err := filepath.Abs("myapp")
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return abs
}

func TestRunAddNotAKeelProject(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, templates := addTestCatalog()

	var out, errOut bytes.Buffer
	code := runAdd([]string{"a"}, &out, &errOut, cat, templates, t.TempDir())

	if code != 1 {
		t.Fatalf("runAdd() = %d, want 1", code)
	}
	if strings.TrimSpace(errOut.String()) != notKeelProjectMsg {
		t.Fatalf("stderr = %q, want %q", errOut.String(), notKeelProjectMsg)
	}
}

func TestRunAddMissingModuleName(t *testing.T) {
	cat, templates := addTestCatalog()

	var out, errOut bytes.Buffer
	code := runAdd(nil, &out, &errOut, cat, templates, t.TempDir())

	if code != 2 {
		t.Fatalf("runAdd() = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "missing module name") {
		t.Fatalf("stderr = %q, want it to mention a missing module name", errOut.String())
	}
}

func TestRunAddSuccess(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, templates := addTestCatalog()
	projectDir := bootstrapProject(t, cat, templates, "a")

	var out, errOut bytes.Buffer
	code := runAdd([]string{"b"}, &out, &errOut, cat, templates, projectDir)

	if code != 0 {
		t.Fatalf("runAdd() = %d, want 0\nstderr:\n%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Added modules: b") {
		t.Fatalf("stdout = %q, want it to name b as added", out.String())
	}

	if _, err := os.Stat(filepath.Join(projectDir, "internal", "b.go")); err != nil {
		t.Fatalf("internal/b.go not written: %v", err)
	}

	st, err := state.Load(projectDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if !st.Has("a") || !st.Has("b") || !st.Has("base") {
		t.Fatalf("state.Modules = %v, want base, a, and b", st.Modules)
	}

	goMod, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "github.com/x/y v1.0.0") {
		t.Fatalf("go.mod missing b's dependency:\n%s", goMod)
	}

	env, err := os.ReadFile(filepath.Join(projectDir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile .env.example: %v", err)
	}
	if !strings.Contains(string(env), "B_VAR") {
		t.Fatalf(".env.example missing B_VAR:\n%s", env)
	}
}

// TestRunAddIdempotentNoOp is the test the brief specifically asked
// for: running "keel add b" when b is already in .keel/state.json must
// be a clean no-op — exit 0, a message saying it's already present,
// and (asserted directly, not just inferred from the exit code) no
// change to any file in the project.
func TestRunAddIdempotentNoOp(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, templates := addTestCatalog()
	projectDir := bootstrapProject(t, cat, templates, "a", "b")

	stateBefore, err := os.ReadFile(filepath.Join(projectDir, ".keel", "state.json"))
	if err != nil {
		t.Fatalf("ReadFile state.json: %v", err)
	}
	goModBefore, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile go.mod: %v", err)
	}
	envBefore, err := os.ReadFile(filepath.Join(projectDir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile .env.example: %v", err)
	}

	var out, errOut bytes.Buffer
	code := runAdd([]string{"b"}, &out, &errOut, cat, templates, projectDir)

	if code != 0 {
		t.Fatalf("runAdd() = %d, want 0\nstderr:\n%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "b is already in this project") {
		t.Fatalf("stdout = %q, want it to say b is already present", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty on a no-op", errOut.String())
	}

	stateAfter, err := os.ReadFile(filepath.Join(projectDir, ".keel", "state.json"))
	if err != nil {
		t.Fatalf("ReadFile state.json: %v", err)
	}
	if string(stateAfter) != string(stateBefore) {
		t.Fatalf("state.json changed on an idempotent no-op:\nbefore: %s\nafter: %s", stateBefore, stateAfter)
	}
	goModAfter, _ := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if string(goModAfter) != string(goModBefore) {
		t.Fatal("go.mod changed on an idempotent no-op")
	}
	envAfter, _ := os.ReadFile(filepath.Join(projectDir, ".env.example"))
	if string(envAfter) != string(envBefore) {
		t.Fatal(".env.example changed on an idempotent no-op")
	}
}

func TestRunAddDryRun(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, templates := addTestCatalog()
	projectDir := bootstrapProject(t, cat, templates, "a")

	var out, errOut bytes.Buffer
	code := runAdd([]string{"--dry-run", "b"}, &out, &errOut, cat, templates, projectDir)

	if code != 0 {
		t.Fatalf("runAdd() = %d, want 0\nstderr:\n%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Would add modules: b") {
		t.Fatalf("stdout = %q, want \"Would add modules: b\"", out.String())
	}

	if _, err := os.Stat(filepath.Join(projectDir, "internal", "b.go")); !os.IsNotExist(err) {
		t.Fatalf("--dry-run wrote internal/b.go, stat err = %v", err)
	}
	st, err := state.Load(projectDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if st.Has("b") {
		t.Fatal("--dry-run recorded b in state.json")
	}
}

// TestRunAddForceStillErrorsOnRealConflict proves --force's narrow
// scope end to end through the CLI: adding a module that genuinely
// conflicts (here, a dependency version conflict) with an already
// selected module must still fail even with --force, and the failure
// must be the real conflict — not a file-collision message, which
// would mean --force reached something it should never touch.
func TestRunAddForceStillErrorsOnRealConflict(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, templates := addTestCatalog()
	projectDir := bootstrapProject(t, cat, templates, "b") // b requires github.com/x/y v1.0.0

	var out, errOut bytes.Buffer
	code := runAdd([]string{"--force", "conflict"}, &out, &errOut, cat, templates, projectDir)

	if code == 0 {
		t.Fatalf("runAdd() = 0, want non-zero: a real dependency conflict must survive --force")
	}
	if !strings.Contains(errOut.String(), "github.com/x/y") {
		t.Fatalf("stderr = %q, want it to name the conflicting dependency", errOut.String())
	}
	if strings.Contains(errOut.String(), "already exists") {
		t.Fatalf("stderr = %q, looks like a file-collision message, not a dependency conflict", errOut.String())
	}

	st, err := state.Load(projectDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if st.Has("conflict") {
		t.Fatal("state records conflict as present despite the conflict error")
	}
}

func TestRunAddUnknownModule(t *testing.T) {
	t.Chdir(t.TempDir())
	cat, templates := addTestCatalog()
	projectDir := bootstrapProject(t, cat, templates, "a")

	var out, errOut bytes.Buffer
	code := runAdd([]string{"does-not-exist"}, &out, &errOut, cat, templates, projectDir)

	if code != 1 {
		t.Fatalf("runAdd() = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "does-not-exist") {
		t.Fatalf("stderr = %q, want it to name the unknown module", errOut.String())
	}
}

func TestRunAddHelpFlag(t *testing.T) {
	cat, templates := addTestCatalog()

	var out, errOut bytes.Buffer
	code := runAdd([]string{"-h"}, &out, &errOut, cat, templates, t.TempDir())

	if code != 0 {
		t.Fatalf("runAdd() = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "keel add <module>") {
		t.Fatalf("stdout = %q, want add's usage text", out.String())
	}
}

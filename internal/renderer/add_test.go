package renderer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
	"github.com/chibuike-kt/keel/internal/state"
)

// bootstrap renders plan into a fresh project via the real Render path
// (init's own entry point), so RenderAdd tests start from a realistic
// go.mod/.env.example/state.json instead of hand-fabricated fixtures.
func bootstrap(t *testing.T, templates fstest.MapFS, plan *resolver.Plan, ctx Context) (targetDir string, existing []state.Module) {
	t.Helper()

	targetDir = filepath.Join(t.TempDir(), "myapp")
	r := New(templates)
	if err := r.Render(plan, ctx, targetDir); err != nil {
		t.Fatalf("bootstrap Render: %v", err)
	}

	st, err := state.Load(targetDir)
	if err != nil {
		t.Fatalf("bootstrap state.Load: %v", err)
	}
	return targetDir, st.Modules
}

func TestRenderAddNewModuleWritesFilesAndUpdatesState(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleA.Version = "0.1.0"
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})
	moduleB.Version = "0.2.0"

	ctx := Context{ProjectName: "myapp", ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	result, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{})
	if err != nil {
		t.Fatalf("RenderAdd: %v", err)
	}

	if len(result.NewModules) != 1 || result.NewModules[0].Name != "b" || result.NewModules[0].Version != "0.2.0" {
		t.Fatalf("NewModules = %+v, want just b@0.2.0", result.NewModules)
	}
	if len(result.Files) != 1 || result.Files[0] != "internal/b.go" {
		t.Fatalf("Files = %v, want [internal/b.go]", result.Files)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "internal", "b.go")); err != nil {
		t.Fatalf("internal/b.go not written: %v", err)
	}

	st, err := state.Load(targetDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if !st.Has("a") || !st.Has("b") {
		t.Fatalf("state.Modules = %v, want both a and b", st.Modules)
	}
}

func TestRenderAddSkipsAlreadyPresentModuleFiles(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	// If RenderAdd tried to re-render a's already-present file, this
	// would either overwrite it silently or trip the collision check —
	// neither of which should happen, since a is already in existing.
	marker := filepath.Join(targetDir, "internal", "a.go")
	if err := os.WriteFile(marker, []byte("hand-edited\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	result, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{})
	if err != nil {
		t.Fatalf("RenderAdd: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0] != "internal/b.go" {
		t.Fatalf("Files = %v, want only internal/b.go (a's file should be skipped entirely)", result.Files)
	}

	got, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hand-edited\n" {
		t.Fatalf("internal/a.go = %q, want untouched hand-edited content", got)
	}
}

func TestRenderAddFileCollisionBlocksWithoutForce(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	// Something (a hand-written file, unrelated to any module) already
	// occupies b's destination path before b is ever added.
	collidePath := filepath.Join(targetDir, "internal", "b.go")
	if err := os.MkdirAll(filepath.Dir(collidePath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(collidePath, []byte("pre-existing\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	_, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{})

	addErr, ok := errors.AsType[*AddError](err)
	if !ok {
		t.Fatalf("RenderAdd() error = %v, want *AddError", err)
	}
	if len(addErr.FileCollisions) != 1 || addErr.FileCollisions[0].Path != "internal/b.go" {
		t.Fatalf("FileCollisions = %+v, want exactly internal/b.go", addErr.FileCollisions)
	}

	got, readErr := os.ReadFile(collidePath)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(got) != "pre-existing\n" {
		t.Fatalf("colliding file = %q, want untouched (no write attempted)", got)
	}

	if _, err := state.Load(targetDir); err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	st, _ := state.Load(targetDir)
	if st.Has("b") {
		t.Fatal("state records b as present despite the blocked collision")
	}
}

func TestRenderAddForceOverwritesCollision(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	collidePath := filepath.Join(targetDir, "internal", "b.go")
	if err := os.MkdirAll(filepath.Dir(collidePath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(collidePath, []byte("pre-existing\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	result, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{Force: true})
	if err != nil {
		t.Fatalf("RenderAdd with Force: %v", err)
	}
	if len(result.Files) != 1 || result.Files[0] != "internal/b.go" {
		t.Fatalf("Files = %v, want internal/b.go", result.Files)
	}

	got, readErr := os.ReadFile(collidePath)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(got) != "package b\n" {
		t.Fatalf("collidePath = %q, want overwritten with b's rendered content", got)
	}

	st, err := state.Load(targetDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if !st.Has("b") {
		t.Fatal("state should record b as present after a forced overwrite")
	}
}

// TestRenderAddDependencyConflictSurfacesEvenWithForce is the test the
// brief specifically asked for: --force must never bypass a real
// conflict between modules. This constructs an existing module and a
// newly-requested module that require the same Go dependency at two
// different versions, passes Force: true, and asserts the specific
// error type (*RenderError with a DependencyConflictError) still
// surfaces — not just a non-nil error, and not *AddError, which would
// mean Force incorrectly reached a conflict it should never touch.
func TestRenderAddDependencyConflictSurfacesEvenWithForce(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{
		Templates:    []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}},
		Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}},
	})
	moduleB := moduleGo("b", manifest.Language{
		Templates:    []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}},
		Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v2.0.0"}},
	})

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	_, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{Force: true})

	renderErr, ok := errors.AsType[*RenderError](err)
	if !ok {
		t.Fatalf("RenderAdd() error = %v, want *RenderError even with Force", err)
	}
	if len(renderErr.DependencyConflicts) != 1 {
		t.Fatalf("DependencyConflicts = %+v, want exactly 1", renderErr.DependencyConflicts)
	}
	dc := renderErr.DependencyConflicts[0]
	if dc.Module != "github.com/x/y" || dc.VersionA != "v1.0.0" || dc.VersionB != "v2.0.0" {
		t.Fatalf("DependencyConflictError = %+v", dc)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "internal", "b.go")); !os.IsNotExist(err) {
		t.Fatalf("internal/b.go should not have been written, stat err = %v", err)
	}
	st, _ := state.Load(targetDir)
	if st.Has("b") {
		t.Fatal("state should not record b after a conflict, even with Force")
	}
}

// TestRenderAddEnvVarConflictSurfacesEvenWithForce is the env-var
// equivalent of the dependency-conflict test above: a real
// EnvVarConflictError must surface regardless of Force.
func TestRenderAddEnvVarConflictSurfacesEvenWithForce(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleA.EnvVars = []manifest.EnvVar{{Name: "REDIS_URL", Description: "a's redis", Required: true}}
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})
	moduleB.EnvVars = []manifest.EnvVar{{Name: "REDIS_URL", Description: "b's redis", Required: false, Default: "redis://localhost"}}

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	_, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{Force: true})

	renderErr, ok := errors.AsType[*RenderError](err)
	if !ok {
		t.Fatalf("RenderAdd() error = %v, want *RenderError even with Force", err)
	}
	if len(renderErr.EnvVarConflicts) != 1 || renderErr.EnvVarConflicts[0].Name != "REDIS_URL" {
		t.Fatalf("EnvVarConflicts = %+v, want exactly one REDIS_URL conflict", renderErr.EnvVarConflicts)
	}
}

func TestRenderAddGoModMergePreservesHandAddedDependency(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleB := moduleGo("b", manifest.Language{
		Templates:    []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}},
		Dependencies: []manifest.Dependency{{Module: "github.com/redis/go-redis/v9", Version: "v9.21.0"}},
	})

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	// Simulate the user hand-adding an unrelated dependency directly to
	// go.mod between init and add — something no keel module knows
	// about at all.
	goModPath := filepath.Join(targetDir, "go.mod")
	existingGoMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("ReadFile go.mod: %v", err)
	}
	handAdded := string(existingGoMod) + "\nrequire github.com/hand-added/pkg v1.2.3\n"
	if err := os.WriteFile(goModPath, []byte(handAdded), 0o644); err != nil {
		t.Fatalf("WriteFile go.mod: %v", err)
	}

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	result, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{})
	if err != nil {
		t.Fatalf("RenderAdd: %v", err)
	}
	if len(result.GoModDeps) != 1 || result.GoModDeps[0] != "github.com/redis/go-redis/v9" {
		t.Fatalf("GoModDeps = %v, want github.com/redis/go-redis/v9", result.GoModDeps)
	}

	got, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	final := string(got)
	if !strings.Contains(final, "github.com/hand-added/pkg v1.2.3") {
		t.Fatalf("go.mod lost the hand-added dependency:\n%s", final)
	}
	if !strings.Contains(final, "github.com/redis/go-redis/v9 v9.21.0") {
		t.Fatalf("go.mod missing the newly merged dependency:\n%s", final)
	}
}

func TestRenderAddEnvExampleMergePreservesHandEditedValue(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleA.EnvVars = []manifest.EnvVar{{Name: "APP_NAME", Description: "app name", Required: false, Default: "myapp"}}
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})
	moduleB.EnvVars = []manifest.EnvVar{{Name: "LEDGER_DATABASE_URL", Description: "ledger db", Required: true}}

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	// The user filled in a real-looking value for APP_NAME after init —
	// this line must survive byte-for-byte.
	envPath := filepath.Join(targetDir, ".env.example")
	handEdited := "# app name\n# Optional (default: myapp)\nAPP_NAME=SuperCoolApp\n"
	if err := os.WriteFile(envPath, []byte(handEdited), 0o644); err != nil {
		t.Fatalf("WriteFile .env.example: %v", err)
	}

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	result, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{})
	if err != nil {
		t.Fatalf("RenderAdd: %v", err)
	}
	if len(result.EnvVars) != 1 || result.EnvVars[0] != "LEDGER_DATABASE_URL" {
		t.Fatalf("EnvVars = %v, want LEDGER_DATABASE_URL", result.EnvVars)
	}

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	final := string(got)
	if !strings.HasPrefix(final, handEdited) {
		t.Fatalf(".env.example does not preserve the hand-edited APP_NAME block byte-for-byte:\n%s", final)
	}
	if !strings.Contains(final, "LEDGER_DATABASE_URL=") {
		t.Fatalf(".env.example missing newly appended LEDGER_DATABASE_URL:\n%s", final)
	}
	if !strings.Contains(final, "This file is appended to, not regenerated") {
		t.Fatalf(".env.example missing the explanatory append-only header:\n%s", final)
	}
}

func TestRenderAddEnvExampleHeaderOnlyOnFirstAppend(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
		"modules/c/templates/go/z.tmpl": &fstest.MapFile{Data: []byte("package c\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleB := moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}})
	moduleB.EnvVars = []manifest.EnvVar{{Name: "B_VAR", Description: "b var", Required: true}}
	moduleC := moduleGo("c", manifest.Language{Templates: []manifest.Template{{From: "templates/go/z.tmpl", To: "internal/c.go"}}})
	moduleC.EnvVars = []manifest.EnvVar{{Name: "C_VAR", Description: "c var", Required: true}}

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	r := New(templates)
	planAB := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	if _, err := r.RenderAdd(planAB, ctx, targetDir, existing, AddOptions{}); err != nil {
		t.Fatalf("RenderAdd (b): %v", err)
	}

	st, err := state.Load(targetDir)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	planABC := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB, moduleC}}
	if _, err := r.RenderAdd(planABC, ctx, targetDir, st.Modules, AddOptions{}); err != nil {
		t.Fatalf("RenderAdd (c): %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	final := string(got)

	explanationCount := strings.Count(final, "This file is appended to, not regenerated")
	if explanationCount != 1 {
		t.Fatalf("explanatory header appears %d times, want exactly 1:\n%s", explanationCount, final)
	}
	markerCount := strings.Count(final, "--- Added by 'keel add")
	if markerCount != 2 {
		t.Fatalf("'Added by' marker appears %d times, want exactly 2 (one per add):\n%s", markerCount, final)
	}
	if !strings.Contains(final, "B_VAR") || !strings.Contains(final, "C_VAR") {
		t.Fatalf(".env.example missing one of the appended vars:\n%s", final)
	}
}

func TestRenderAddTargetMissing(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})

	r := New(templates)
	plan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}
	_, err := r.RenderAdd(plan, Context{}, filepath.Join(t.TempDir(), "does-not-exist"), nil, AddOptions{})
	if !errors.Is(err, ErrTargetMissing) {
		t.Fatalf("RenderAdd() error = %v, want ErrTargetMissing", err)
	}
}

func TestRenderAddDryRunWritesNothing(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	moduleA := moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}})
	moduleB := moduleGo("b", manifest.Language{
		Templates:    []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}},
		Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}},
	})
	moduleB.EnvVars = []manifest.EnvVar{{Name: "B_VAR", Description: "b var", Required: true}}

	ctx := Context{ModulePath: "example.com/myapp", GoVersion: "1.26"}
	targetDir, existing := bootstrap(t, templates, &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA}}, ctx)

	goModBefore, err := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile go.mod: %v", err)
	}
	envBefore, err := os.ReadFile(filepath.Join(targetDir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile .env.example: %v", err)
	}
	stateBefore, err := os.ReadFile(filepath.Join(targetDir, ".keel", "state.json"))
	if err != nil {
		t.Fatalf("ReadFile state.json: %v", err)
	}

	r := New(templates)
	fullPlan := &resolver.Plan{Language: "go", Modules: []*manifest.Manifest{moduleA, moduleB}}
	result, err := r.RenderAdd(fullPlan, ctx, targetDir, existing, AddOptions{DryRun: true})
	if err != nil {
		t.Fatalf("RenderAdd dry-run: %v", err)
	}

	if len(result.Files) != 1 || result.Files[0] != "internal/b.go" {
		t.Fatalf("Files = %v, want internal/b.go reported even under dry-run", result.Files)
	}
	if len(result.GoModDeps) != 1 || len(result.EnvVars) != 1 {
		t.Fatalf("dry-run result incomplete: %+v", result)
	}

	if _, err := os.Stat(filepath.Join(targetDir, "internal", "b.go")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote internal/b.go, stat err = %v", err)
	}

	goModAfter, _ := os.ReadFile(filepath.Join(targetDir, "go.mod"))
	if string(goModAfter) != string(goModBefore) {
		t.Fatal("dry-run modified go.mod")
	}
	envAfter, _ := os.ReadFile(filepath.Join(targetDir, ".env.example"))
	if string(envAfter) != string(envBefore) {
		t.Fatal("dry-run modified .env.example")
	}
	stateAfter, _ := os.ReadFile(filepath.Join(targetDir, ".keel", "state.json"))
	if string(stateAfter) != string(stateBefore) {
		t.Fatal("dry-run modified state.json")
	}
}

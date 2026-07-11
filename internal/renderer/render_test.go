package renderer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
)

func moduleGo(name string, l manifest.Language) *manifest.Manifest {
	return &manifest.Manifest{
		Name:      name,
		Languages: map[string]manifest.Language{"go": l},
	}
}

func TestRenderSingleModule(t *testing.T) {
	templates := fstest.MapFS{
		"modules/idempotency/templates/go/middleware.go.tmpl": &fstest.MapFile{
			Data: []byte("package middleware // {{.ProjectName}}\n"),
		},
	}
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("idempotency", manifest.Language{
				Templates: []manifest.Template{
					{From: "templates/go/middleware.go.tmpl", To: "internal/middleware/idempotency.go"},
				},
			}),
		},
	}

	targetDir := filepath.Join(t.TempDir(), "myapp")
	r := New(templates)
	ctx := Context{ProjectName: "myapp", ModulePath: "github.com/user/myapp", GoVersion: "1.26"}

	if err := r.Render(plan, ctx, targetDir); err != nil {
		t.Fatalf("Render: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(targetDir, "internal", "middleware", "idempotency.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if want := "package middleware // myapp\n"; string(got) != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}

	if _, err := os.ReadFile(filepath.Join(targetDir, "go.mod")); err != nil {
		t.Fatalf("go.mod missing: %v", err)
	}

	entries, err := os.ReadDir(filepath.Dir(targetDir))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parent dir entries = %v, want only targetDir (no leftover staging dir)", entries)
	}
}

func TestRenderTwoIndependentModules(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/a.go.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/b.go.tmpl": &fstest.MapFile{Data: []byte("package b\n")},
	}
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/a.go.tmpl", To: "internal/a.go"}}}),
			moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/b.go.tmpl", To: "internal/b.go"}}}),
		},
	}

	targetDir := filepath.Join(t.TempDir(), "myapp")
	r := New(templates)
	if err := r.Render(plan, Context{ModulePath: "m", GoVersion: "1.26"}, targetDir); err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, rel := range []string{"internal/a.go", "internal/b.go"} {
		if _, err := os.ReadFile(filepath.Join(targetDir, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("ReadFile(%s): %v", rel, err)
		}
	}
}

func TestRenderDuplicateOutputPath(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("a\n")},
		"modules/b/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("b\n")},
	}
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/shared.go"}}}),
			moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/shared.go"}}}),
		},
	}

	targetDir := filepath.Join(t.TempDir(), "myapp")
	r := New(templates)
	err := r.Render(plan, Context{}, targetDir)

	renderErr, ok := errors.AsType[*RenderError](err)
	if !ok {
		t.Fatalf("Render() error = %v, want *RenderError", err)
	}
	if len(renderErr.DuplicateOutputPaths) != 1 {
		t.Fatalf("DuplicateOutputPaths = %+v", renderErr.DuplicateOutputPaths)
	}
	if _, statErr := os.Stat(targetDir); !os.IsNotExist(statErr) {
		t.Fatalf("targetDir should not exist, stat err = %v", statErr)
	}
}

func TestRenderDependencyConflict(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("b\n")},
	}
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("a", manifest.Language{
				Templates:    []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/x.go"}},
				Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}},
			}),
			moduleGo("b", manifest.Language{
				Templates:    []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/y.go"}},
				Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v2.0.0"}},
			}),
		},
	}

	targetDir := filepath.Join(t.TempDir(), "myapp")
	r := New(templates)
	err := r.Render(plan, Context{}, targetDir)

	renderErr, ok := errors.AsType[*RenderError](err)
	if !ok {
		t.Fatalf("Render() error = %v, want *RenderError", err)
	}
	if len(renderErr.DependencyConflicts) != 1 {
		t.Fatalf("DependencyConflicts = %+v", renderErr.DependencyConflicts)
	}
	dc := renderErr.DependencyConflicts[0]
	if dc.Module != "github.com/x/y" || dc.VersionA != "v1.0.0" || dc.ViaA != "a" || dc.VersionB != "v2.0.0" || dc.ViaB != "b" {
		t.Fatalf("DependencyConflictError = %+v", dc)
	}
	if _, statErr := os.Stat(targetDir); !os.IsNotExist(statErr) {
		t.Fatalf("targetDir should not exist, stat err = %v", statErr)
	}
}

func TestRenderPathEscapeDefenseInDepth(t *testing.T) {
	templates := fstest.MapFS{
		"modules/evil/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("evil\n")},
	}
	// Template{To: "../escape"} constructed directly: manifest.Validate was
	// never called, so this proves Render's own containment check is real.
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("evil", manifest.Language{
				Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "../escape"}},
			}),
		},
	}

	parent := t.TempDir()
	targetDir := filepath.Join(parent, "myapp")
	r := New(templates)
	err := r.Render(plan, Context{}, targetDir)

	renderErr, ok := errors.AsType[*RenderError](err)
	if !ok {
		t.Fatalf("Render() error = %v, want *RenderError", err)
	}
	if len(renderErr.PathEscapes) != 1 {
		t.Fatalf("PathEscapes = %+v", renderErr.PathEscapes)
	}
	if _, statErr := os.Stat(filepath.Join(parent, "escape")); !os.IsNotExist(statErr) {
		t.Fatalf("escape file should not have been written, stat err = %v", statErr)
	}
}

func TestRenderMidPlanFailureLeavesNoTrace(t *testing.T) {
	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
		"modules/b/templates/go/y.tmpl": &fstest.MapFile{Data: []byte("{{.Broken")},
		"modules/c/templates/go/z.tmpl": &fstest.MapFile{Data: []byte("package c\n")},
	}
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}}),
			moduleGo("b", manifest.Language{Templates: []manifest.Template{{From: "templates/go/y.tmpl", To: "internal/b.go"}}}),
			moduleGo("c", manifest.Language{Templates: []manifest.Template{{From: "templates/go/z.tmpl", To: "internal/c.go"}}}),
		},
	}

	parent := t.TempDir()
	targetDir := filepath.Join(parent, "myapp")
	r := New(templates)
	err := r.Render(plan, Context{}, targetDir)

	if _, ok := errors.AsType[*TemplateError](err); !ok {
		t.Fatalf("Render() error = %v, want *TemplateError", err)
	}
	if _, statErr := os.Stat(targetDir); !os.IsNotExist(statErr) {
		t.Fatalf("targetDir should not exist, stat err = %v", statErr)
	}

	entries, readErr := os.ReadDir(parent)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("parent dir = %v, want empty (no leftover staging dir)", entries)
	}
}

func TestRenderTargetExists(t *testing.T) {
	parent := t.TempDir()
	targetDir := filepath.Join(parent, "myapp")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	marker := filepath.Join(targetDir, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	templates := fstest.MapFS{
		"modules/a/templates/go/x.tmpl": &fstest.MapFile{Data: []byte("package a\n")},
	}
	plan := &resolver.Plan{
		Language: "go",
		Modules: []*manifest.Manifest{
			moduleGo("a", manifest.Language{Templates: []manifest.Template{{From: "templates/go/x.tmpl", To: "internal/a.go"}}}),
		},
	}

	r := New(templates)
	err := r.Render(plan, Context{}, targetDir)
	if !errors.Is(err, ErrTargetExists) {
		t.Fatalf("Render() error = %v, want ErrTargetExists", err)
	}

	got, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	if string(got) != "keep" {
		t.Fatalf("marker file was touched: %q", got)
	}
	entries, readErr := os.ReadDir(targetDir)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	if len(entries) != 1 {
		t.Fatalf("targetDir entries = %v, want only the pre-existing marker", entries)
	}
}

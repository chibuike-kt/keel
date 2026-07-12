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

func TestRenderTemplateSubstitutesContext(t *testing.T) {
	templates := fstest.MapFS{
		"modules/idempotency/templates/go/a.go.tmpl": &fstest.MapFile{
			Data: []byte("package main // {{.ProjectName}} {{.ModulePath}} go{{.GoVersion}}\n"),
		},
	}
	r := New(templates)
	mod := &manifest.Manifest{Name: "idempotency"}
	task := renderTask{module: mod, from: "modules/idempotency/templates/go/a.go.tmpl", to: "internal/a.go"}
	stagingDir := t.TempDir()

	ctx := Context{ProjectName: "myapp", ModulePath: "github.com/user/myapp", GoVersion: "1.26"}
	if err := r.renderTemplate(task, ctx, nil, stagingDir); err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(stagingDir, "internal", "a.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "package main // myapp github.com/user/myapp go1.26\n"
	if string(got) != want {
		t.Fatalf("rendered = %q, want %q", got, want)
	}
}

func TestRenderTemplateMissingKeyErrors(t *testing.T) {
	templates := fstest.MapFS{
		"modules/idempotency/templates/go/a.go.tmpl": &fstest.MapFile{
			Data: []byte("{{.Typo}}\n"),
		},
	}
	r := New(templates)
	mod := &manifest.Manifest{Name: "idempotency"}
	task := renderTask{module: mod, from: "modules/idempotency/templates/go/a.go.tmpl", to: "internal/a.go"}
	stagingDir := t.TempDir()

	err := r.renderTemplate(task, Context{ProjectName: "myapp"}, nil, stagingDir)
	tmplErr, ok := errors.AsType[*TemplateError](err)
	if !ok {
		t.Fatalf("renderTemplate() error = %v, want *TemplateError", err)
	}
	if tmplErr.Module != "idempotency" || tmplErr.Path != "internal/a.go" {
		t.Fatalf("TemplateError = %+v", tmplErr)
	}
}

func TestModuleInfosSortedAlphabeticallyNotTopologically(t *testing.T) {
	// plan.Modules is in topological (dependency) order — zebra before
	// alpha here, deliberately not alphabetical — moduleInfos must
	// re-sort by name regardless.
	plan := &resolver.Plan{
		Modules: []*manifest.Manifest{
			{Name: "zebra", Summary: "the last one"},
			{Name: "alpha", Summary: "the first one"},
		},
	}

	got := moduleInfos(plan)
	want := []ModuleInfo{
		{Name: "alpha", Summary: "the first one"},
		{Name: "zebra", Summary: "the last one"},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("moduleInfos() = %+v, want %+v", got, want)
	}
}

func TestRenderTemplateBadSyntaxErrors(t *testing.T) {
	templates := fstest.MapFS{
		"modules/idempotency/templates/go/a.go.tmpl": &fstest.MapFile{
			Data: []byte("{{.ProjectName"),
		},
	}
	r := New(templates)
	mod := &manifest.Manifest{Name: "idempotency"}
	task := renderTask{module: mod, from: "modules/idempotency/templates/go/a.go.tmpl", to: "internal/a.go"}

	err := r.renderTemplate(task, Context{}, nil, t.TempDir())
	if _, ok := errors.AsType[*TemplateError](err); !ok {
		t.Fatalf("renderTemplate() error = %v, want *TemplateError", err)
	}
}

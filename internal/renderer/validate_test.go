package renderer

import (
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
)

func planOf(mods ...*manifest.Manifest) *resolver.Plan {
	return &resolver.Plan{Language: "go", Modules: mods}
}

func TestValidatePlanOK(t *testing.T) {
	m := &manifest.Manifest{
		Name: "idempotency",
		Languages: map[string]manifest.Language{
			"go": {
				Templates: []manifest.Template{
					{From: "templates/go/a.go.tmpl", To: "internal/a.go"},
				},
				Dependencies: []manifest.Dependency{
					{Module: "github.com/redis/go-redis/v9", Version: "v9.5.1"},
				},
			},
		},
	}

	tasks, deps, err := validatePlan(planOf(m))
	if err != nil {
		t.Fatalf("validatePlan: %v", err)
	}
	if len(tasks) != 1 || tasks[0].from != "modules/idempotency/templates/go/a.go.tmpl" || tasks[0].to != "internal/a.go" {
		t.Fatalf("tasks = %+v", tasks)
	}
	if len(deps) != 1 || deps[0].module != "github.com/redis/go-redis/v9" || deps[0].version != "v9.5.1" {
		t.Fatalf("deps = %+v", deps)
	}
}

func TestValidatePlanPathEscapeDefenseInDepth(t *testing.T) {
	// Constructed directly, bypassing manifest.Validate entirely — proves
	// Render's own containment check is real, not just schema validation.
	m := &manifest.Manifest{
		Name: "evil",
		Languages: map[string]manifest.Language{
			"go": {
				Templates: []manifest.Template{
					{From: "templates/go/a.go.tmpl", To: "../escape"},
				},
			},
		},
	}

	_, _, err := validatePlan(planOf(m))
	renderErr, ok := err.(*RenderError)
	if !ok {
		t.Fatalf("validatePlan() error = %v (%T), want *RenderError", err, err)
	}
	if len(renderErr.PathEscapes) != 1 {
		t.Fatalf("PathEscapes = %+v, want 1", renderErr.PathEscapes)
	}
	pe := renderErr.PathEscapes[0]
	if pe.Module != "evil" || pe.Path != "../escape" {
		t.Fatalf("PathEscapeError = %+v", pe)
	}
}

func TestCleanDestPathRejections(t *testing.T) {
	tests := []string{
		"",
		"/etc/passwd",
		"../escape",
		"a/../../escape",
		"..",
		".",
		`..\escape`,
		"C:evil",
	}
	for _, to := range tests {
		if _, ok := cleanDestPath(to); ok {
			t.Errorf("cleanDestPath(%q) = ok, want rejected", to)
		}
	}
}

func TestCleanDestPathAccepts(t *testing.T) {
	got, ok := cleanDestPath("internal/middleware/a.go")
	if !ok || got != "internal/middleware/a.go" {
		t.Fatalf("cleanDestPath() = %q, %v", got, ok)
	}
}

func TestValidatePlanDuplicateOutputPath(t *testing.T) {
	a := &manifest.Manifest{Name: "a", Languages: map[string]manifest.Language{
		"go": {Templates: []manifest.Template{{From: "x.tmpl", To: "internal/shared.go"}}},
	}}
	b := &manifest.Manifest{Name: "b", Languages: map[string]manifest.Language{
		"go": {Templates: []manifest.Template{{From: "y.tmpl", To: "internal/shared.go"}}},
	}}

	_, _, err := validatePlan(planOf(a, b))
	renderErr, ok := err.(*RenderError)
	if !ok {
		t.Fatalf("validatePlan() error = %v (%T), want *RenderError", err, err)
	}
	if len(renderErr.DuplicateOutputPaths) != 1 {
		t.Fatalf("DuplicateOutputPaths = %+v, want 1", renderErr.DuplicateOutputPaths)
	}
	dup := renderErr.DuplicateOutputPaths[0]
	if dup.Path != "internal/shared.go" || dup.ModuleA != "a" || dup.ModuleB != "b" {
		t.Fatalf("DuplicateOutputPathError = %+v", dup)
	}
}

func TestValidatePlanDependencyConflict(t *testing.T) {
	a := &manifest.Manifest{Name: "a", Languages: map[string]manifest.Language{
		"go": {Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}}},
	}}
	b := &manifest.Manifest{Name: "b", Languages: map[string]manifest.Language{
		"go": {Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v2.0.0"}}},
	}}

	_, _, err := validatePlan(planOf(a, b))
	renderErr, ok := err.(*RenderError)
	if !ok {
		t.Fatalf("validatePlan() error = %v (%T), want *RenderError", err, err)
	}
	if len(renderErr.DependencyConflicts) != 1 {
		t.Fatalf("DependencyConflicts = %+v, want 1", renderErr.DependencyConflicts)
	}
	dc := renderErr.DependencyConflicts[0]
	if dc.Module != "github.com/x/y" || dc.VersionA != "v1.0.0" || dc.ViaA != "a" || dc.VersionB != "v2.0.0" || dc.ViaB != "b" {
		t.Fatalf("DependencyConflictError = %+v", dc)
	}
}

func TestValidatePlanSameVersionDedupesSilently(t *testing.T) {
	a := &manifest.Manifest{Name: "a", Languages: map[string]manifest.Language{
		"go": {Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}}},
	}}
	b := &manifest.Manifest{Name: "b", Languages: map[string]manifest.Language{
		"go": {Dependencies: []manifest.Dependency{{Module: "github.com/x/y", Version: "v1.0.0"}}},
	}}

	_, deps, err := validatePlan(planOf(a, b))
	if err != nil {
		t.Fatalf("validatePlan: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("deps = %+v, want exactly one merged entry", deps)
	}
}

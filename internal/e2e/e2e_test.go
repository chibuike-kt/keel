// Package e2e exercises the full generation pipeline — manifest loading,
// resolution, and rendering — against the real modules/ tree, then
// compiles the result. Every other package's tests use synthetic
// fixtures; this is the one place that proves the real modules and the
// real code that consumes them still agree.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/renderer"
	"github.com/chibuike-kt/keel/internal/resolver"
)

// repoRoot is relative to this package's directory.
const repoRoot = "../.."

// loadCatalog loads and validates every real module.yaml under modules/.
func loadCatalog(t *testing.T) resolver.MapCatalog {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(repoRoot, "modules/*/module.yaml"))
	if err != nil {
		t.Fatalf("glob module manifests: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no module manifests found")
	}

	catalog := resolver.MapCatalog{}
	for _, p := range paths {
		m, err := manifest.LoadFile(p)
		if err != nil {
			t.Fatalf("LoadFile(%s): %v", p, err)
		}
		if err := manifest.Validate(m); err != nil {
			t.Fatalf("Validate(%s): %v", p, err)
		}
		catalog[m.Name] = m
	}
	return catalog
}

// renderProject renders plan into a fresh temp directory and returns its path.
func renderProject(t *testing.T, plan *resolver.Plan) string {
	t.Helper()

	r := renderer.New(os.DirFS(repoRoot))
	targetDir := filepath.Join(t.TempDir(), "myapp")
	ctx := renderer.Context{
		ProjectName: "myapp",
		ModulePath:  "example.com/myapp",
		GoVersion:   "1.26",
	}
	if err := r.Render(plan, ctx, targetDir); err != nil {
		t.Fatalf("Render: %v", err)
	}
	return targetDir
}

func TestGenerateEmptySelectionBuilds(t *testing.T) {
	// Empty requested list: ResolveProject must still include base on its
	// own, per its contract.
	plan, err := resolver.ResolveProject(loadCatalog(t), nil, "go")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}

	targetDir := renderProject(t, plan)

	for _, want := range []string{"README.md", ".gitignore", "cmd/api/main.go", "go.mod", ".env.example"} {
		if _, err := os.Stat(filepath.Join(targetDir, want)); err != nil {
			t.Errorf("generated project missing %s: %v", want, err)
		}
	}

	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = targetDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./... in generated project failed: %v\n%s", err, out)
	}
}

// TestIdempotencyAndRatelimitDedupeRedisURL proves the retrofit didn't just
// pass validation in isolation: idempotency and ratelimit both declare
// REDIS_URL as required with no default, and selecting both together must
// merge into a single .env.example entry, not a conflict — the identical
// dedupe path in collectEnvVars, exercised with real manifests instead of
// synthetic fixtures.
func TestIdempotencyAndRatelimitDedupeRedisURL(t *testing.T) {
	plan, err := resolver.ResolveProject(loadCatalog(t), []string{"idempotency", "ratelimit"}, "go")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}

	targetDir := renderProject(t, plan)

	got, err := os.ReadFile(filepath.Join(targetDir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile(.env.example): %v", err)
	}

	if n := strings.Count(string(got), "REDIS_URL="); n != 1 {
		t.Fatalf(".env.example has %d REDIS_URL= entries, want exactly 1 (deduped):\n%s", n, got)
	}
}

// TestWebhookEnvVarsInEnvExample proves webhook's 4 provider secrets all
// appear as optional (no default) entries in .env.example, since only a
// subset of providers may be wired up in any given project.
func TestWebhookEnvVarsInEnvExample(t *testing.T) {
	plan, err := resolver.ResolveProject(loadCatalog(t), []string{"webhook"}, "go")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}

	targetDir := renderProject(t, plan)

	got, err := os.ReadFile(filepath.Join(targetDir, ".env.example"))
	if err != nil {
		t.Fatalf("ReadFile(.env.example): %v", err)
	}
	content := string(got)

	for _, name := range []string{
		"STRIPE_WEBHOOK_SECRET",
		"PAYSTACK_SECRET_KEY",
		"FLUTTERWAVE_SECRET_HASH",
		"NOMBA_SECRET_KEY",
	} {
		if !strings.Contains(content, name+"=") {
			t.Errorf(".env.example missing %s:\n%s", name, content)
		}
		if !strings.Contains(content, "# Optional") {
			t.Errorf(".env.example entries should be optional, got:\n%s", content)
		}
	}
}

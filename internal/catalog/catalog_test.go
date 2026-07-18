package catalog_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	keel "github.com/chibuike-kt/keel"
	"github.com/chibuike-kt/keel/internal/catalog"
)

// TestLoadCatalogRealEmbeddedModules is a real integration test against
// keel's own embedded module tree — not a synthetic fixture. This is the
// test that would catch a module accidentally missing from the embed, or
// failing validation only in the compiled-binary context, as opposed to
// passing when tested against the raw filesystem (which is all that's
// been verified until now, in internal/manifest's and internal/renderer's
// own glob-based tests).
func TestLoadCatalogRealEmbeddedModules(t *testing.T) {
	modulesFS, err := fs.Sub(keel.Modules, "modules")
	if err != nil {
		t.Fatalf("fs.Sub: %v", err)
	}

	cat, err := catalog.LoadCatalog(modulesFS)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	want := []string{
		"audit", "auth", "base", "config", "health", "idempotency", "ledger",
		"logging", "observability", "ratelimit", "reconcile", "security", "webhook",
	}
	for _, name := range want {
		if _, ok := cat.Module(name); !ok {
			t.Errorf("catalog missing module %q", name)
		}
	}
	if len(cat) != len(want) {
		names := make([]string, 0, len(cat))
		for name := range cat {
			names = append(names, name)
		}
		t.Errorf("catalog has %d modules, want exactly %d (got %v)", len(cat), len(want), names)
	}
}

// TestLoadCatalogAggregatesAllValidationFailures proves multiple bad
// manifests are all reported together, not just the first one hit —
// deleting the aggregation (i.e. returning on the first error) would
// still make this test fail, since it specifically checks for two
// distinct, differently-named problems both being present.
func TestLoadCatalogAggregatesAllValidationFailures(t *testing.T) {
	fsys := fstest.MapFS{
		"bad-one/module.yaml": &fstest.MapFile{Data: []byte(`
apiVersion: keel/v1
name: bad-one
summary: missing required fields entirely
`)},
		"bad-two/module.yaml": &fstest.MapFile{Data: []byte(`not: valid: yaml: at: all: {{{`)},
		"good/module.yaml": &fstest.MapFile{Data: []byte(`
apiVersion: keel/v1
name: good
version: 0.1.0
summary: a perfectly valid module
tags: [reliability]
requires: []
conflicts: []
languages:
  go:
    templates:
      - from: templates/go/x.tmpl
        to: internal/x.go
`)},
	}

	cat, err := catalog.LoadCatalog(fsys)
	if err == nil {
		t.Fatal("LoadCatalog() error = nil, want aggregated errors for bad-one and bad-two")
	}
	if cat != nil {
		t.Fatalf("LoadCatalog() catalog = %v, want nil when there are validation failures", cat)
	}

	var loadErr *catalog.LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("LoadCatalog() error type = %T, want *catalog.LoadError", err)
	}
	if len(loadErr.Errors) != 2 {
		t.Fatalf("LoadError.Errors has %d entries, want exactly 2 (bad-one and bad-two, not stopping at the first)", len(loadErr.Errors))
	}

	msg := loadErr.Error()
	if !strings.Contains(msg, "bad-one") {
		t.Errorf("aggregated error message missing bad-one's problem:\n%s", msg)
	}
	if !strings.Contains(msg, "bad-two") {
		t.Errorf("aggregated error message missing bad-two's problem:\n%s", msg)
	}
}

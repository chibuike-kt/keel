package manifest_test

import (
	"path/filepath"
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// TestRealModuleManifestsValidate loads every module.yaml under modules/
// through manifest.LoadFile and checks it validates. This is the first
// place a real manifest — not a testdata fixture — meets the loader and
// validator together, so it proves the schema and the code agree in
// practice, not just in theory.
func TestRealModuleManifestsValidate(t *testing.T) {
	paths, err := filepath.Glob("../../modules/*/module.yaml")
	if err != nil {
		t.Fatalf("glob module manifests: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no module manifests found under modules/*/module.yaml")
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			m, err := manifest.LoadFile(path)
			if err != nil {
				t.Fatalf("LoadFile(%s): %v", path, err)
			}
			if err := manifest.Validate(m); err != nil {
				t.Fatalf("Validate(%s): %v", path, err)
			}
		})
	}
}

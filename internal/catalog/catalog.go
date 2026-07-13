// Package catalog loads a resolver.Catalog from a filesystem of
// module.yaml manifests — the embedded modules/ tree in the compiled
// keel binary, or a test fixture in an fstest.MapFS.
package catalog

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
)

// LoadError aggregates every module.yaml that failed to load or
// validate, rather than stopping at the first bad manifest — the same
// posture as every other aggregation point in this codebase
// (manifest.ValidationError, renderer.RenderError).
type LoadError struct {
	Errors []error
}

func (e *LoadError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return strings.Join(msgs, "\n")
}

func (e *LoadError) Unwrap() []error {
	return e.Errors
}

// LoadCatalog walks fsys for every module.yaml one level down and loads
// and validates each one, returning a resolver.MapCatalog keyed by
// module name.
//
// fsys must be rooted at the directory that directly contains each
// module's subdirectory — e.g. an fs.Sub of keel's embedded catalog
// re-rooted at "modules", not the embedded root itself (which has
// "modules" as a single top-level entry, not the module directories
// themselves — go:embed keeps the embedded directory's own name as a
// path component, it does not flatten it away).
func LoadCatalog(fsys fs.FS) (resolver.MapCatalog, error) {
	paths, err := fs.Glob(fsys, "*/module.yaml")
	if err != nil {
		return nil, fmt.Errorf("catalog: glob module manifests: %w", err)
	}

	cat := make(resolver.MapCatalog, len(paths))
	var loadErr LoadError

	for _, path := range paths {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			loadErr.Errors = append(loadErr.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}

		m, err := manifest.Load(bytes.NewReader(data), path)
		if err != nil {
			loadErr.Errors = append(loadErr.Errors, err)
			continue
		}

		if err := manifest.Validate(m); err != nil {
			loadErr.Errors = append(loadErr.Errors, fmt.Errorf("%s: %w", path, err))
			continue
		}

		cat[m.Name] = m
	}

	if len(loadErr.Errors) > 0 {
		return nil, &loadErr
	}
	return cat, nil
}

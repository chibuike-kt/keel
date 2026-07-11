// Package resolver expands a requested module set into a deterministic,
// dependency-ordered build plan. See docs/architecture.md for the
// resolver's role and guarantees.
package resolver

import "github.com/chibuike-kt/keel/internal/manifest"

// Catalog looks up a module manifest by name.
type Catalog interface {
	Module(name string) (*manifest.Manifest, bool)
}

// MapCatalog is a Catalog backed by an in-memory map, keyed by module name.
type MapCatalog map[string]*manifest.Manifest

// Module implements Catalog.
func (c MapCatalog) Module(name string) (*manifest.Manifest, bool) {
	m, ok := c[name]
	return m, ok
}

// Plan is a dependency-ordered build set for one target language: every
// module's requires precede it in Modules.
type Plan struct {
	Language string
	Modules  []*manifest.Manifest
}

// Resolve expands requested into its transitive requires closure and
// produces a deterministic, dependency-ordered Plan for language, or an
// error.
//
// requested is deduped and sorted before anything else, so the result never
// depends on the caller's order or on repeats. An unknown module — whether
// requested directly or pulled in via requires — is a hard stop, reported
// as *UnknownModuleError before the closure is checked further. Once the
// closure is known, cycles, conflicts, and missing language support are all
// checked and aggregated into a single *ResolutionError; only when none of
// them fire is the closure topologically sorted into a Plan.
func Resolve(c Catalog, requested []string, language string) (*Plan, error) {
	canonical := canonicalRequested(requested)

	closure, err := buildClosure(c, canonical)
	if err != nil {
		return nil, err
	}

	var resErr ResolutionError
	failed := false

	if cycle := detectCycle(closure); cycle != nil {
		resErr.Cycle = cycle
		failed = true
	}

	if conflicts := findConflicts(closure, canonical); len(conflicts) > 0 {
		resErr.Conflicts = &ConflictError{Conflicts: conflicts}
		failed = true
	}

	if unsupported := findUnsupportedLanguages(closure, language); len(unsupported) > 0 {
		resErr.UnsupportedLanguage = &UnsupportedLanguageError{Language: language, Modules: unsupported}
		failed = true
	}

	if failed {
		return nil, &resErr
	}

	return &Plan{Language: language, Modules: topoSort(closure)}, nil
}

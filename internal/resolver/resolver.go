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

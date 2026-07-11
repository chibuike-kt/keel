package resolver

import "github.com/chibuike-kt/keel/internal/manifest"

// mod builds a minimal manifest for resolver tests. Resolver operates on
// already-validated manifests, so fixtures only set the fields resolution
// cares about.
func mod(name string, requires ...string) *manifest.Manifest {
	return &manifest.Manifest{
		Name:     name,
		Requires: requires,
		Languages: map[string]manifest.Language{
			"go": {},
		},
	}
}

// conflicting builds a minimal manifest like mod, plus a conflicts list.
func conflicting(name string, conflicts []string) *manifest.Manifest {
	m := mod(name)
	m.Conflicts = conflicts
	return m
}

func catalog(mods ...*manifest.Manifest) MapCatalog {
	c := make(MapCatalog, len(mods))
	for _, m := range mods {
		c[m.Name] = m
	}
	return c
}

package resolver

import "errors"

// ErrMissingBaseModule is returned by ResolveProject when the catalog has
// no "base" module at all — that indicates keel's own embedded module set
// is broken (a build problem), not a caller mistake.
var ErrMissingBaseModule = errors.New(`resolver: catalog missing required "base" module`)

// ResolveProject wraps Resolve, ensuring the "base" module — providing
// README, .gitignore, a starter main.go, and other foundational project
// files — is always included regardless of what the caller requested.
// This is what keel init calls; Resolve itself remains general-purpose
// and has no special knowledge of any particular module.
//
// Returns ErrMissingBaseModule if the catalog has no "base" module at
// all, rather than silently producing a project with no README.
func ResolveProject(c Catalog, requested []string, language string) (*Plan, error) {
	if _, ok := c.Module("base"); !ok {
		return nil, ErrMissingBaseModule
	}

	merged := make([]string, 0, len(requested)+1)
	merged = append(merged, requested...)
	merged = append(merged, "base")
	return Resolve(c, merged, language)
}

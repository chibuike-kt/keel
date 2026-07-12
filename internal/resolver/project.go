package resolver

import "fmt"

// ResolveProject wraps Resolve, ensuring the "base" module — providing
// README, .gitignore, a starter main.go, and other foundational project
// files — is always included regardless of what the caller requested.
// This is what keel init calls; Resolve itself remains general-purpose
// and has no special knowledge of any particular module.
//
// Returns an error if the catalog has no "base" module at all — that
// indicates keel's own embedded module set is broken (a build problem),
// and should fail loudly rather than silently produce a project with no
// README.
func ResolveProject(c Catalog, requested []string, language string) (*Plan, error) {
	if _, ok := c.Module("base"); !ok {
		return nil, fmt.Errorf("resolver: catalog missing required %q module", "base")
	}

	merged := make([]string, 0, len(requested)+1)
	merged = append(merged, requested...)
	merged = append(merged, "base")
	return Resolve(c, merged, language)
}

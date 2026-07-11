package resolver

import (
	"sort"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// canonicalRequested dedupes and sorts a requested module list so the rest
// of resolution is a deterministic function of the set, not the caller's
// order.
func canonicalRequested(requested []string) []string {
	set := make(map[string]struct{}, len(requested))
	for _, name := range requested {
		set[name] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// buildClosure walks the requires graph from a canonical requested set,
// returning every reachable module keyed by name. It hard-stops with an
// *UnknownModuleError on the first module missing from the catalog,
// traversing requested roots and each module's requires in sorted order so
// which missing module is reported does not depend on catalog or caller
// iteration order.
func buildClosure(c Catalog, requested []string) (map[string]*manifest.Manifest, error) {
	closure := make(map[string]*manifest.Manifest, len(requested))
	queue := make([]string, 0, len(requested))

	for _, name := range requested {
		mod, ok := c.Module(name)
		if !ok {
			return nil, &UnknownModuleError{Name: name}
		}
		closure[name] = mod
		queue = append(queue, name)
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		reqs := append([]string(nil), closure[name].Requires...)
		sort.Strings(reqs)

		for _, req := range reqs {
			if _, ok := closure[req]; ok {
				continue
			}
			mod, ok := c.Module(req)
			if !ok {
				return nil, &UnknownModuleError{Name: req, RequiredBy: name}
			}
			closure[req] = mod
			queue = append(queue, req)
		}
	}

	return closure, nil
}

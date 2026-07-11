package resolver

import (
	"sort"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// topoSort orders an acyclic closure so every module's requires precede it,
// breaking ties lexicographically by name (Kahn's algorithm with a sorted
// ready set). Callers must ensure closure has no cycle.
func topoSort(closure map[string]*manifest.Manifest) []*manifest.Manifest {
	inDegree := make(map[string]int, len(closure))
	dependents := make(map[string][]string, len(closure))

	for name := range closure {
		inDegree[name] = 0
	}
	for name, mod := range closure {
		for _, req := range mod.Requires {
			inDegree[name]++
			dependents[req] = append(dependents[req], name)
		}
	}

	var ready []string
	for name, deg := range inDegree {
		if deg == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)

	ordered := make([]*manifest.Manifest, 0, len(closure))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, closure[name])

		next := append([]string(nil), dependents[name]...)
		sort.Strings(next)
		for _, dep := range next {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				ready = insertSorted(ready, dep)
			}
		}
	}

	return ordered
}

// insertSorted inserts name into a sorted slice, keeping it sorted.
func insertSorted(sorted []string, name string) []string {
	i := sort.SearchStrings(sorted, name)
	sorted = append(sorted, "")
	copy(sorted[i+1:], sorted[i:])
	sorted[i] = name
	return sorted
}

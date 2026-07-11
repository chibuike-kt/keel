package resolver

import (
	"slices"
	"sort"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// shortestPaths computes, for every module in the closure, the shortest
// requires-path from any requested root via multi-source BFS. Roots and
// each module's requires are visited in sorted order, so when more than one
// shortest path exists the one reported is deterministic.
func shortestPaths(closure map[string]*manifest.Manifest, requested []string) map[string][]string {
	paths := make(map[string][]string, len(closure))
	queue := make([]string, 0, len(requested))

	for _, name := range requested {
		paths[name] = []string{name}
		queue = append(queue, name)
	}

	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]

		reqs := append([]string(nil), closure[name].Requires...)
		sort.Strings(reqs)

		for _, req := range reqs {
			if _, ok := paths[req]; ok {
				continue
			}
			path := append(append([]string(nil), paths[name]...), req)
			paths[req] = path
			queue = append(queue, req)
		}
	}

	return paths
}

// findConflicts checks every pair in the closure for a symmetric
// conflicts-with declaration, returning the violations sorted by (A, B).
func findConflicts(closure map[string]*manifest.Manifest, requested []string) []Conflict {
	names := make([]string, 0, len(closure))
	for name := range closure {
		names = append(names, name)
	}
	sort.Strings(names)

	declares := func(from, to string) bool {
		return slices.Contains(closure[from].Conflicts, to)
	}

	paths := shortestPaths(closure, requested)

	var conflicts []Conflict
	for i, a := range names {
		for _, b := range names[i+1:] {
			if declares(a, b) || declares(b, a) {
				conflicts = append(conflicts, Conflict{
					A: a, B: b,
					ViaA: paths[a], ViaB: paths[b],
				})
			}
		}
	}
	return conflicts
}

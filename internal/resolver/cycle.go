package resolver

import (
	"sort"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// detectCycle runs DFS with a recursion stack over the closure's requires
// edges, returning the first cycle found. Modules and their requires are
// visited in sorted order so the reported cycle is deterministic even when
// the closure contains more than one.
func detectCycle(closure map[string]*manifest.Manifest) *CycleError {
	const (
		unvisited = iota
		visiting
		done
	)

	state := make(map[string]int, len(closure))
	var stack []string

	var visit func(name string) *CycleError
	visit = func(name string) *CycleError {
		state[name] = visiting
		stack = append(stack, name)

		reqs := append([]string(nil), closure[name].Requires...)
		sort.Strings(reqs)

		for _, req := range reqs {
			switch state[req] {
			case visiting:
				start := 0
				for i, n := range stack {
					if n == req {
						start = i
						break
					}
				}
				cycle := append([]string(nil), stack[start:]...)
				cycle = append(cycle, req)
				return &CycleError{Cycle: cycle}
			case unvisited:
				if err := visit(req); err != nil {
					return err
				}
			}
		}

		stack = stack[:len(stack)-1]
		state[name] = done
		return nil
	}

	names := make([]string, 0, len(closure))
	for name := range closure {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if state[name] == unvisited {
			if err := visit(name); err != nil {
				return err
			}
		}
	}
	return nil
}

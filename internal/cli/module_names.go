package cli

import (
	"sort"

	"github.com/chibuike-kt/keel/internal/resolver"
)

// sortedModuleNames returns cat's module names sorted alphabetically.
// resolver.MapCatalog is a plain map, so its iteration order carries no
// meaning — every place that lists a catalog (the non-interactive
// picker, the TUI picker, keel list) needs a stable, human-readable
// order and shares this instead of re-deriving it independently.
func sortedModuleNames(cat resolver.MapCatalog) []string {
	names := make([]string, 0, len(cat))
	for name := range cat {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

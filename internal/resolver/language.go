package resolver

import (
	"sort"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// findUnsupportedLanguages returns, sorted, the name of every closure module
// that does not declare an implementation for language.
func findUnsupportedLanguages(closure map[string]*manifest.Manifest, language string) []string {
	var unsupported []string
	for name, mod := range closure {
		if _, ok := mod.Languages[language]; !ok {
			unsupported = append(unsupported, name)
		}
	}
	sort.Strings(unsupported)
	return unsupported
}

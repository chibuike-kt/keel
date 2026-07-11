package resolver

import (
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
)

func names(mods []*manifest.Manifest) []string {
	out := make([]string, len(mods))
	for i, m := range mods {
		out[i] = m.Name
	}
	return out
}

func TestTopoSortLinearChain(t *testing.T) {
	closure := mustClosure(t, catalog(mod("a", "b"), mod("b", "c"), mod("c")), "a")

	got := names(topoSort(closure))
	want := []string{"c", "b", "a"}
	if !equalStrings(got, want) {
		t.Fatalf("topoSort() = %v, want %v", got, want)
	}
}

func TestTopoSortDiamondTieBreak(t *testing.T) {
	closure := mustClosure(t, catalog(mod("d", "a", "b"), mod("a", "c"), mod("b", "c"), mod("c")), "d")

	got := names(topoSort(closure))
	want := []string{"c", "a", "b", "d"}
	if !equalStrings(got, want) {
		t.Fatalf("topoSort() = %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

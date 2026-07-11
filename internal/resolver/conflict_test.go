package resolver

import (
	"reflect"
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
)

func mustClosure(t *testing.T, c MapCatalog, requested ...string) map[string]*manifest.Manifest {
	t.Helper()
	closure, err := buildClosure(c, canonicalRequested(requested))
	if err != nil {
		t.Fatalf("buildClosure: %v", err)
	}
	return closure
}

func TestFindConflictsNone(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b"))
	closure := mustClosure(t, c, "a")

	if got := findConflicts(closure, canonicalRequested([]string{"a"})); len(got) != 0 {
		t.Fatalf("findConflicts() = %v, want none", got)
	}
}

func TestFindConflictsDirect(t *testing.T) {
	c := catalog(conflicting("a", []string{"b"}), mod("b"))
	requested := canonicalRequested([]string{"a", "b"})
	closure := mustClosure(t, c, requested...)

	got := findConflicts(closure, requested)
	want := []Conflict{{A: "a", B: "b", ViaA: []string{"a"}, ViaB: []string{"b"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findConflicts() = %+v, want %+v", got, want)
	}
}

func TestFindConflictsOneSided(t *testing.T) {
	// Only "a" declares the conflict; the check must still catch it.
	c := catalog(conflicting("a", []string{"b"}), mod("b"))
	requested := canonicalRequested([]string{"a", "b"})
	closure := mustClosure(t, c, requested...)

	got := findConflicts(closure, requested)
	if len(got) != 1 || got[0].A != "a" || got[0].B != "b" {
		t.Fatalf("findConflicts() = %+v, want one conflict a/b", got)
	}
}

func TestFindConflictsSymmetricNotDuplicated(t *testing.T) {
	// Both sides declare the conflict; it must be reported once.
	c := catalog(conflicting("a", []string{"b"}), conflicting("b", []string{"a"}))
	requested := canonicalRequested([]string{"a", "b"})
	closure := mustClosure(t, c, requested...)

	got := findConflicts(closure, requested)
	if len(got) != 1 {
		t.Fatalf("findConflicts() = %+v, want exactly one conflict", got)
	}
}

func TestFindConflictsTransitiveProvenance(t *testing.T) {
	// root1 -> a -> x, root2 -> b -> y, x conflicts with y.
	c := catalog(
		mod("root1", "a"), mod("a", "x"),
		mod("root2", "b"), mod("b", "y"),
		conflicting("x", []string{"y"}),
		mod("y"),
	)
	requested := canonicalRequested([]string{"root1", "root2"})
	closure := mustClosure(t, c, requested...)

	got := findConflicts(closure, requested)
	want := []Conflict{{
		A: "x", B: "y",
		ViaA: []string{"root1", "a", "x"},
		ViaB: []string{"root2", "b", "y"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findConflicts() = %+v, want %+v", got, want)
	}
}

func TestFindConflictsSortedByPair(t *testing.T) {
	c := catalog(
		conflicting("z", []string{"y"}),
		conflicting("a", []string{"b"}),
		mod("y"), mod("b"),
	)
	requested := canonicalRequested([]string{"z", "y", "a", "b"})
	closure := mustClosure(t, c, requested...)

	got := findConflicts(closure, requested)
	if len(got) != 2 {
		t.Fatalf("findConflicts() = %+v, want 2 conflicts", got)
	}
	if got[0].A != "a" || got[0].B != "b" || got[1].A != "y" || got[1].B != "z" {
		t.Fatalf("findConflicts() not sorted: %+v", got)
	}
}

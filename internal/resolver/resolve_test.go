package resolver

import (
	"errors"
	"math/rand"
	"reflect"
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
)

func TestResolveLinearChain(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b", "c"), mod("c"))

	plan, err := Resolve(c, []string{"a"}, "go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got, want := names(plan.Modules), []string{"c", "b", "a"}; !equalStrings(got, want) {
		t.Fatalf("Modules = %v, want %v", got, want)
	}
	if plan.Language != "go" {
		t.Fatalf("Language = %q, want go", plan.Language)
	}
}

func TestResolveDiamondSharedDepOnce(t *testing.T) {
	c := catalog(mod("d", "a", "b"), mod("a", "c"), mod("b", "c"), mod("c"))

	plan, err := Resolve(c, []string{"d"}, "go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got, want := names(plan.Modules), []string{"c", "a", "b", "d"}; !equalStrings(got, want) {
		t.Fatalf("Modules = %v, want %v (c once, a before b)", got, want)
	}
}

func TestResolveUnknownRequested(t *testing.T) {
	c := catalog(mod("a"))

	_, err := Resolve(c, []string{"ghost"}, "go")
	var unknown *UnknownModuleError
	if !errors.As(err, &unknown) {
		t.Fatalf("Resolve() error = %v, want *UnknownModuleError", err)
	}
	if unknown.Name != "ghost" || unknown.RequiredBy != "" {
		t.Fatalf("unknown = %+v", unknown)
	}
}

func TestResolveUnknownRequired(t *testing.T) {
	c := catalog(mod("a", "ghost"))

	_, err := Resolve(c, []string{"a"}, "go")
	var unknown *UnknownModuleError
	if !errors.As(err, &unknown) {
		t.Fatalf("Resolve() error = %v, want *UnknownModuleError", err)
	}
	if unknown.Name != "ghost" || unknown.RequiredBy != "a" {
		t.Fatalf("unknown = %+v", unknown)
	}
}

func TestResolveDirectCycle(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b", "a"))

	_, err := Resolve(c, []string{"a"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("Resolve() error = %v, want *ResolutionError", err)
	}
	if resErr.Cycle == nil {
		t.Fatalf("resErr.Cycle = nil, want a cycle")
	}
}

func TestResolveTransitiveCycle(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b", "c"), mod("c", "a"))

	_, err := Resolve(c, []string{"a"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("Resolve() error = %v, want *ResolutionError", err)
	}
	want := []string{"a", "b", "c", "a"}
	if resErr.Cycle == nil || !reflect.DeepEqual(resErr.Cycle.Cycle, want) {
		t.Fatalf("resErr.Cycle = %v, want %v", resErr.Cycle, want)
	}
}

func TestResolveDirectConflict(t *testing.T) {
	c := catalog(conflicting("a", []string{"b"}), mod("b"))

	_, err := Resolve(c, []string{"a", "b"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("Resolve() error = %v, want *ResolutionError", err)
	}
	if resErr.Conflicts == nil || len(resErr.Conflicts.Conflicts) != 1 {
		t.Fatalf("resErr.Conflicts = %v, want one conflict", resErr.Conflicts)
	}
}

func TestResolveTransitiveConflictProvenance(t *testing.T) {
	c := catalog(
		mod("root1", "a"), mod("a", "x"),
		mod("root2", "b"), mod("b", "y"),
		conflicting("x", []string{"y"}),
		mod("y"),
	)

	_, err := Resolve(c, []string{"root1", "root2"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("Resolve() error = %v, want *ResolutionError", err)
	}
	if resErr.Conflicts == nil || len(resErr.Conflicts.Conflicts) != 1 {
		t.Fatalf("resErr.Conflicts = %v, want one conflict", resErr.Conflicts)
	}
	got := resErr.Conflicts.Conflicts[0]
	wantViaA := []string{"root1", "a", "x"}
	wantViaB := []string{"root2", "b", "y"}
	if !reflect.DeepEqual(got.ViaA, wantViaA) || !reflect.DeepEqual(got.ViaB, wantViaB) {
		t.Fatalf("conflict provenance = %+v, want ViaA=%v ViaB=%v", got, wantViaA, wantViaB)
	}
}

func TestResolveOneSidedConflictStillDetected(t *testing.T) {
	c := catalog(conflicting("a", []string{"b"}), mod("b"))

	_, err := Resolve(c, []string{"a", "b"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) || resErr.Conflicts == nil {
		t.Fatalf("Resolve() error = %v, want conflict even one-sided", err)
	}
}

func TestResolveUnsupportedLanguageAggregate(t *testing.T) {
	c := catalog(mod("a", "b", "c"))
	c["b"] = manifestNoGo("b")
	c["c"] = manifestNoGo("c")

	_, err := Resolve(c, []string{"a"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("Resolve() error = %v, want *ResolutionError", err)
	}
	if resErr.UnsupportedLanguage == nil {
		t.Fatal("resErr.UnsupportedLanguage = nil, want a failure")
	}
	want := []string{"b", "c"}
	if !equalStrings(resErr.UnsupportedLanguage.Modules, want) {
		t.Fatalf("Modules = %v, want %v", resErr.UnsupportedLanguage.Modules, want)
	}
}

func TestResolveAggregatesConflictAndLanguage(t *testing.T) {
	c := catalog(conflicting("a", []string{"b"}))
	c["b"] = manifestNoGo("b")

	_, err := Resolve(c, []string{"a", "b"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("Resolve() error = %v, want *ResolutionError", err)
	}
	if resErr.Cycle != nil {
		t.Fatalf("resErr.Cycle = %v, want nil", resErr.Cycle)
	}
	if resErr.Conflicts == nil {
		t.Fatal("resErr.Conflicts = nil, want a conflict")
	}
	if resErr.UnsupportedLanguage == nil {
		t.Fatal("resErr.UnsupportedLanguage = nil, want a failure")
	}
}

func TestResolveDeterministic(t *testing.T) {
	c := catalog(
		mod("d", "a", "b"), mod("a", "c"), mod("b", "c"), mod("c"),
		mod("e"),
	)
	requested := []string{"d", "e", "d", "a"}

	first, err := Resolve(c, requested, "go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	for i := range 20 {
		shuffled := append([]string(nil), requested...)
		rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

		plan, err := Resolve(c, shuffled, "go")
		if err != nil {
			t.Fatalf("run %d: Resolve: %v", i, err)
		}
		if !reflect.DeepEqual(names(plan.Modules), names(first.Modules)) {
			t.Fatalf("run %d: Modules = %v, want %v", i, names(plan.Modules), names(first.Modules))
		}
	}
}

// manifestNoGo builds a module with no language support at all, for
// unsupported-language tests.
func manifestNoGo(name string) *manifest.Manifest {
	return &manifest.Manifest{Name: name, Languages: map[string]manifest.Language{}}
}

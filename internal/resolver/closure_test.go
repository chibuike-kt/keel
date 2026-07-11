package resolver

import (
	"errors"
	"reflect"
	"testing"
)

func TestCanonicalRequested(t *testing.T) {
	got := canonicalRequested([]string{"b", "a", "b", "c", "a"})
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonicalRequested() = %v, want %v", got, want)
	}
}

func TestBuildClosureLinearChain(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b", "c"), mod("c"))

	closure, err := buildClosure(c, canonicalRequested([]string{"a"}))
	if err != nil {
		t.Fatalf("buildClosure: %v", err)
	}
	if len(closure) != 3 {
		t.Fatalf("closure = %v, want 3 modules", closure)
	}
}

func TestBuildClosureDiamond(t *testing.T) {
	c := catalog(mod("d", "a", "b"), mod("a", "c"), mod("b", "c"), mod("c"))

	closure, err := buildClosure(c, canonicalRequested([]string{"d"}))
	if err != nil {
		t.Fatalf("buildClosure: %v", err)
	}
	if len(closure) != 4 {
		t.Fatalf("closure = %v, want 4 modules (c shared, not duplicated)", closure)
	}
}

func TestBuildClosureUnknownRequested(t *testing.T) {
	c := catalog(mod("a"))

	_, err := buildClosure(c, canonicalRequested([]string{"ghost"}))
	var unknown *UnknownModuleError
	if !errors.As(err, &unknown) {
		t.Fatalf("buildClosure() error = %v, want *UnknownModuleError", err)
	}
	if unknown.Name != "ghost" || unknown.RequiredBy != "" {
		t.Fatalf("unknown = %+v, want Name=ghost RequiredBy=\"\"", unknown)
	}
}

func TestBuildClosureUnknownRequired(t *testing.T) {
	c := catalog(mod("a", "ghost"))

	_, err := buildClosure(c, canonicalRequested([]string{"a"}))
	var unknown *UnknownModuleError
	if !errors.As(err, &unknown) {
		t.Fatalf("buildClosure() error = %v, want *UnknownModuleError", err)
	}
	if unknown.Name != "ghost" || unknown.RequiredBy != "a" {
		t.Fatalf("unknown = %+v, want Name=ghost RequiredBy=a", unknown)
	}
}

func TestBuildClosureDeterministicUnknownRequired(t *testing.T) {
	// Two requestees are each missing a different dependency; the reported
	// one must not depend on map iteration order.
	c := catalog(mod("a", "ghost-a"), mod("b", "ghost-b"))

	for i := range 20 {
		_, err := buildClosure(c, canonicalRequested([]string{"a", "b"}))
		var unknown *UnknownModuleError
		if !errors.As(err, &unknown) {
			t.Fatalf("buildClosure() error = %v, want *UnknownModuleError", err)
		}
		if unknown.Name != "ghost-a" || unknown.RequiredBy != "a" {
			t.Fatalf("run %d: unknown = %+v, want Name=ghost-a RequiredBy=a", i, unknown)
		}
	}
}

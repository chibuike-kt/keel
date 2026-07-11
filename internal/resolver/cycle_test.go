package resolver

import (
	"reflect"
	"testing"
)

func TestDetectCycleNone(t *testing.T) {
	closure, err := buildClosure(catalog(mod("a", "b"), mod("b")), canonicalRequested([]string{"a"}))
	if err != nil {
		t.Fatalf("buildClosure: %v", err)
	}
	if cycle := detectCycle(closure); cycle != nil {
		t.Fatalf("detectCycle() = %v, want nil", cycle)
	}
}

func TestDetectCycleDirect(t *testing.T) {
	closure, err := buildClosure(catalog(mod("a", "b"), mod("b", "a")), canonicalRequested([]string{"a"}))
	if err != nil {
		t.Fatalf("buildClosure: %v", err)
	}

	cycle := detectCycle(closure)
	if cycle == nil {
		t.Fatal("detectCycle() = nil, want a cycle")
	}
	want := []string{"a", "b", "a"}
	if !reflect.DeepEqual(cycle.Cycle, want) {
		t.Fatalf("cycle = %v, want %v", cycle.Cycle, want)
	}
}

func TestDetectCycleTransitive(t *testing.T) {
	closure, err := buildClosure(catalog(mod("a", "b"), mod("b", "c"), mod("c", "a")), canonicalRequested([]string{"a"}))
	if err != nil {
		t.Fatalf("buildClosure: %v", err)
	}

	cycle := detectCycle(closure)
	if cycle == nil {
		t.Fatal("detectCycle() = nil, want a cycle")
	}
	want := []string{"a", "b", "c", "a"}
	if !reflect.DeepEqual(cycle.Cycle, want) {
		t.Fatalf("cycle = %v, want %v", cycle.Cycle, want)
	}
}

func TestDetectCycleDeterministic(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b", "c"), mod("c", "a"))

	var first *CycleError
	for i := range 20 {
		closure, err := buildClosure(c, canonicalRequested([]string{"a"}))
		if err != nil {
			t.Fatalf("buildClosure: %v", err)
		}
		cycle := detectCycle(closure)
		if cycle == nil {
			t.Fatal("detectCycle() = nil, want a cycle")
		}
		if first == nil {
			first = cycle
			continue
		}
		if !reflect.DeepEqual(first.Cycle, cycle.Cycle) {
			t.Fatalf("run %d: cycle = %v, want %v", i, cycle.Cycle, first.Cycle)
		}
	}
}

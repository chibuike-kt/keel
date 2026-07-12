package resolver

import (
	"errors"
	"testing"
)

func TestResolveProjectIncludesBaseAlongsideRequested(t *testing.T) {
	c := catalog(mod("base"), mod("idempotency"), mod("ledger"))

	plan, err := ResolveProject(c, []string{"idempotency", "ledger"}, "go")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}

	got := names(plan.Modules)
	want := map[string]bool{"base": true, "idempotency": true, "ledger": true}
	if len(got) != len(want) {
		t.Fatalf("Modules = %v, want exactly %v", got, want)
	}
	for _, name := range got {
		if !want[name] {
			t.Fatalf("Modules = %v, unexpected module %q", got, name)
		}
	}
}

func TestResolveProjectRequestingBaseExplicitlyNoDuplicate(t *testing.T) {
	// Resolve's existing dedup handles the actual merging; this only
	// confirms ResolveProject's own "always append base" step doesn't
	// break that guarantee.
	c := catalog(mod("base"), mod("idempotency"))

	plan, err := ResolveProject(c, []string{"base", "idempotency"}, "go")
	if err != nil {
		t.Fatalf("ResolveProject: %v", err)
	}

	count := 0
	for _, m := range plan.Modules {
		if m.Name == "base" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf(`"base" appears %d times in the plan, want exactly 1`, count)
	}
}

func TestResolveProjectMissingBaseErrors(t *testing.T) {
	c := catalog(mod("idempotency"))

	_, err := ResolveProject(c, []string{"idempotency"}, "go")
	if !errors.Is(err, ErrMissingBaseModule) {
		t.Fatalf("ResolveProject() error = %v, want errors.Is(err, ErrMissingBaseModule)", err)
	}
}

func TestResolveProjectSurfacesConflictWithBase(t *testing.T) {
	// Proves the wrapper doesn't swallow or alter Resolve's normal error
	// behavior — it only adds one name to the input before delegating.
	c := catalog(mod("base"), conflicting("legacy-thing", []string{"base"}))

	_, err := ResolveProject(c, []string{"legacy-thing"}, "go")
	var resErr *ResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("ResolveProject() error = %v (%T), want *ResolutionError, same as Resolve would return", err, err)
	}
	if resErr.Conflicts == nil {
		t.Fatal("resErr.Conflicts = nil, want a conflict between legacy-thing and base")
	}
}

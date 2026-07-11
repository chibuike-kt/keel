package resolver

import (
	"fmt"
	"strings"
)

// UnknownModuleError reports a module name absent from the catalog. It is a
// hard stop: resolution does not continue once one is found.
type UnknownModuleError struct {
	// Name is the module name missing from the catalog.
	Name string
	// RequiredBy is the module whose requires list named Name. Empty when
	// Name was requested directly rather than pulled in transitively.
	RequiredBy string
}

func (e *UnknownModuleError) Error() string {
	if e.RequiredBy == "" {
		return fmt.Sprintf("unknown module %q", e.Name)
	}
	return fmt.Sprintf("module %q requires unknown module %q", e.RequiredBy, e.Name)
}

// CycleError reports a dependency cycle in the requires graph, e.g.
// ["a", "b", "c", "a"].
type CycleError struct {
	Cycle []string
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle: %s", strings.Join(e.Cycle, " -> "))
}

// Conflict describes a symmetric conflicts-with violation between two
// modules in the closure, with the shortest requires-path from a requested
// root to each side.
type Conflict struct {
	A, B       string
	ViaA, ViaB []string
}

// ConflictError reports every conflicts-with violation found in the
// closure, sorted by (A, B).
type ConflictError struct {
	Conflicts []Conflict
}

func (e *ConflictError) Error() string {
	lines := make([]string, len(e.Conflicts))
	for i, c := range e.Conflicts {
		lines[i] = fmt.Sprintf("%q conflicts with %q (via %s / via %s)",
			c.A, c.B, strings.Join(c.ViaA, " -> "), strings.Join(c.ViaB, " -> "))
	}
	return strings.Join(lines, "\n")
}

// UnsupportedLanguageError reports every closure module that does not
// declare an implementation for the requested language.
type UnsupportedLanguageError struct {
	Language string
	Modules  []string
}

func (e *UnsupportedLanguageError) Error() string {
	return fmt.Sprintf("modules do not support language %q: %s", e.Language, strings.Join(e.Modules, ", "))
}

// ResolutionError aggregates the structural failures found while checking a
// closure: at most one of each kind, since each check runs exactly once over
// the whole closure.
type ResolutionError struct {
	Cycle               *CycleError
	Conflicts           *ConflictError
	UnsupportedLanguage *UnsupportedLanguageError
}

func (e *ResolutionError) Error() string {
	errs := e.Unwrap()
	parts := make([]string, len(errs))
	for i, err := range errs {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "\n")
}

// Unwrap exposes the individual failures for errors.As and errors.Is.
func (e *ResolutionError) Unwrap() []error {
	var errs []error
	if e.Cycle != nil {
		errs = append(errs, e.Cycle)
	}
	if e.Conflicts != nil {
		errs = append(errs, e.Conflicts)
	}
	if e.UnsupportedLanguage != nil {
		errs = append(errs, e.UnsupportedLanguage)
	}
	return errs
}

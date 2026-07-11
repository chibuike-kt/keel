package renderer

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTargetExists is returned when targetDir already exists. Render refuses
// to touch it — no parents created, nothing written inside it.
var ErrTargetExists = errors.New("target directory already exists")

// PathEscapeError reports a template destination that would write outside
// the target directory. Computed independently of manifest validation —
// Render never trusts that a Template.To was already checked.
type PathEscapeError struct {
	Module string
	Path   string
}

func (e *PathEscapeError) Error() string {
	return fmt.Sprintf("module %q: template destination %q escapes the target directory", e.Module, e.Path)
}

// DuplicateOutputPathError reports two modules writing to the same
// destination path.
type DuplicateOutputPathError struct {
	Path             string
	ModuleA, ModuleB string
}

func (e *DuplicateOutputPathError) Error() string {
	return fmt.Sprintf("%q is written by both %q and %q", e.Path, e.ModuleA, e.ModuleB)
}

// DependencyConflictError reports the same Go module path required at two
// different versions. Via names the requiring module, same provenance style
// as the resolver's Conflict.
type DependencyConflictError struct {
	Module   string
	VersionA string
	ViaA     string
	VersionB string
	ViaB     string
}

func (e *DependencyConflictError) Error() string {
	return fmt.Sprintf("module %q required at %q (via %q) and %q (via %q)",
		e.Module, e.VersionA, e.ViaA, e.VersionB, e.ViaB)
}

// TemplateError wraps a parse or execution failure for one module's
// template.
type TemplateError struct {
	Module string
	Path   string
	Err    error
}

func (e *TemplateError) Error() string {
	return fmt.Sprintf("module %q: template %q: %v", e.Module, e.Path, e.Err)
}

func (e *TemplateError) Unwrap() error { return e.Err }

// RenderError aggregates every failure found while validating a plan (pass
// 1), rather than stopping at the first.
type RenderError struct {
	PathEscapes          []*PathEscapeError
	DuplicateOutputPaths []*DuplicateOutputPathError
	DependencyConflicts  []*DependencyConflictError
}

func (e *RenderError) Error() string {
	errs := e.Unwrap()
	parts := make([]string, len(errs))
	for i, err := range errs {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "\n")
}

// Unwrap exposes the individual failures for errors.As and errors.Is.
func (e *RenderError) Unwrap() []error {
	var errs []error
	for _, pe := range e.PathEscapes {
		errs = append(errs, pe)
	}
	for _, de := range e.DuplicateOutputPaths {
		errs = append(errs, de)
	}
	for _, ce := range e.DependencyConflicts {
		errs = append(errs, ce)
	}
	return errs
}

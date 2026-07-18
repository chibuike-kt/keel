package renderer

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTargetExists is returned when targetDir already exists. Render refuses
// to touch it — no parents created, nothing written inside it.
var ErrTargetExists = errors.New("target directory already exists")

// ErrTargetMissing is returned by RenderAdd when targetDir does not
// exist — the mirror image of ErrTargetExists: RenderAdd only ever
// writes into an already-generated project, never creates one.
var ErrTargetMissing = errors.New("target directory does not exist")

// FileCollisionError reports a new module's template destination that
// already exists on disk in the target project. This is the only
// failure --force is allowed to bypass — a genuine conflict between
// modules (DependencyConflictError, EnvVarConflictError) or in the
// resolver's own requires/conflicts graph is never something a file
// flag should override.
type FileCollisionError struct {
	Module string
	Path   string
}

func (e *FileCollisionError) Error() string {
	return fmt.Sprintf("module %q: %q already exists (use --force to overwrite)", e.Module, e.Path)
}

// AddError aggregates every file collision RenderAdd finds before
// writing anything, the same aggregate-everything-first posture as
// RenderError.
type AddError struct {
	FileCollisions []*FileCollisionError
}

func (e *AddError) Error() string {
	errs := e.Unwrap()
	parts := make([]string, len(errs))
	for i, err := range errs {
		parts[i] = err.Error()
	}
	return strings.Join(parts, "\n")
}

// Unwrap exposes the individual collisions for errors.As and errors.Is.
func (e *AddError) Unwrap() []error {
	errs := make([]error, len(e.FileCollisions))
	for i, c := range e.FileCollisions {
		errs[i] = c
	}
	return errs
}

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

// EnvVarConflictError reports the same environment variable declared by
// two modules with different Required or Default settings. Identical
// declarations (same Required and Default; Description may differ, that's
// just prose) dedupe silently instead of producing this error.
type EnvVarConflictError struct {
	Name      string
	ModuleA   string
	RequiredA bool
	DefaultA  string
	ModuleB   string
	RequiredB bool
	DefaultB  string
}

func (e *EnvVarConflictError) Error() string {
	return fmt.Sprintf("env var %q declared differently by %q (required=%t, default=%q) and %q (required=%t, default=%q)",
		e.Name, e.ModuleA, e.RequiredA, e.DefaultA, e.ModuleB, e.RequiredB, e.DefaultB)
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
	EnvVarConflicts      []*EnvVarConflictError
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
	for _, ee := range e.EnvVarConflicts {
		errs = append(errs, ee)
	}
	return errs
}

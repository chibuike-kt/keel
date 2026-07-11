// Package renderer executes a resolver.Plan's templates into a target
// directory. Rendering is transactional: targetDir either ends up fully
// populated or is never created. See docs/architecture.md for the
// renderer's role and guarantees. Phase 1 scope: Go output only.
package renderer

import (
	"io/fs"
	"os"
	"path/filepath"
	"fmt"

	"github.com/chibuike-kt/keel/internal/resolver"
)

// Context is the substitution data available to every template.
type Context struct {
	ProjectName string // e.g. "myapp"
	ModulePath  string // Go import path, e.g. "github.com/user/myapp"
	GoVersion   string // e.g. "1.26" — target project's go.mod go line
}

// Renderer executes module templates sourced from an fs.FS. A manifest
// Template.From (e.g. "templates/go/x.tmpl") resolves against templates as
// path.Join("modules", moduleName, from).
type Renderer struct {
	templates fs.FS
}

// New constructs a Renderer over templates, e.g. an embed.FS in the real CLI
// or a fstest.MapFS in tests.
func New(templates fs.FS) *Renderer {
	return &Renderer{templates: templates}
}

// Render executes plan against ctx and writes the result to targetDir.
// targetDir must not already exist. On any failure, targetDir is never
// created and no partial output is left behind.
//
// Pass 1 validates the whole plan — destination paths, containment, and
// merged Go dependencies — without touching disk, aggregating every
// violation it finds. Only if that passes does pass 2 render templates into
// a staging directory beside targetDir and atomically rename it into place,
// so targetDir either appears fully populated or not at all.
func (r *Renderer) Render(plan *resolver.Plan, ctx Context, targetDir string) error {
	if _, err := os.Stat(targetDir); err == nil {
		return ErrTargetExists
	} else if !os.IsNotExist(err) {
		return err
	}

	tasks, deps, err := validatePlan(plan)
	if err != nil {
		return err
	}

	stagingDir, err := os.MkdirTemp(filepath.Dir(targetDir), ".keel-staging-*")
	if err != nil {
		return err
	}

renameFailed := false
	defer func() {
		if renameFailed {
			return
		}
		if err := os.RemoveAll(stagingDir); err != nil {
			// Best-effort cleanup after an already-failed render. There's no
			// error return path from a defer, so this can't be folded into
			// Render's own error — log it so a leaked staging directory is
			// at least visible instead of silently discarded.
			fmt.Fprintf(os.Stderr, "keel: warning: failed to remove staging directory %s: %v\n", stagingDir, err)
		}
	}()

	for _, task := range tasks {
		if err := r.renderTemplate(task, ctx, stagingDir); err != nil {
			return err
		}
	}

	if err := writeGoMod(stagingDir, ctx, deps); err != nil {
		return err
	}

	if err := os.Rename(stagingDir, targetDir); err != nil {
		// The staging tree is fully rendered at this point; a failed rename
		// is unexpected (e.g. targetDir's parent vanished, or a
		// cross-device move on a platform where MkdirTemp's sibling
		// placement didn't guarantee same-filesystem). Leave stagingDir on
		// disk as evidence for debugging instead of deleting it — this is
		// intentional, not a leaked temp directory.
		renameFailed = true
		return err
	}
	return nil
}

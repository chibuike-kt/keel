package renderer

import (
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

// renderTemplate parses and executes one task's template, writing the
// result under stagingDir. ctx is executed as a map rather than the raw
// Context struct: with Option("missingkey=error"), a field typo'd in a
// template then fails the render instead of silently printing nothing into
// generated source.
func (r *Renderer) renderTemplate(task renderTask, ctx Context, stagingDir string) error {
	src, err := fs.ReadFile(r.templates, task.from)
	if err != nil {
		return &TemplateError{Module: task.module.Name, Path: task.to, Err: err}
	}

	tmpl, err := template.New(task.to).Option("missingkey=error").Parse(string(src))
	if err != nil {
		return &TemplateError{Module: task.module.Name, Path: task.to, Err: err}
	}

	dest := filepath.Join(stagingDir, filepath.FromSlash(task.to))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil { //nolint:gosec // generated project directories are ordinary, readable source trees, not secrets
		return err
	}

	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644) //nolint:gosec // generated source files must be group/world-readable like any normal codebase
	if err != nil {
		return err
	}
	defer f.Close()

	if err := tmpl.Execute(f, ctxMap(ctx)); err != nil {
		return &TemplateError{Module: task.module.Name, Path: task.to, Err: err}
	}
	return nil
}

// ctxMap converts Context to the map executed against templates.
func ctxMap(ctx Context) map[string]any {
	return map[string]any{
		"ProjectName": ctx.ProjectName,
		"ModulePath":  ctx.ModulePath,
		"GoVersion":   ctx.GoVersion,
	}
}

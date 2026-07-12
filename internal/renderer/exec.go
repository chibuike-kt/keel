package renderer

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/chibuike-kt/keel/internal/resolver"
)

// ModuleInfo is the per-module data exposed to templates under the
// "Modules" key. It is derived by Render from plan.Modules, sorted
// alphabetically by name — deliberately NOT plan.Modules' own dependency
// order (topological, e.g. base last). Templates like base's README want a
// stable, human-readable listing, not build order.
type ModuleInfo struct {
	Name    string
	Summary string
}

// moduleInfos builds the sorted []ModuleInfo exposed to templates from a
// plan's resolved modules.
func moduleInfos(plan *resolver.Plan) []ModuleInfo {
	infos := make([]ModuleInfo, len(plan.Modules))
	for i, mod := range plan.Modules {
		infos[i] = ModuleInfo{Name: mod.Name, Summary: mod.Summary}
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// renderTemplate parses and executes one task's template, writing the
// result under stagingDir. ctx and modules are executed as a map rather
// than raw structs: with Option("missingkey=error"), a field typo'd in a
// template then fails the render instead of silently printing nothing into
// generated source.
func (r *Renderer) renderTemplate(task renderTask, ctx Context, modules []ModuleInfo, stagingDir string) error {
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

	if err := tmpl.Execute(f, ctxMap(ctx, modules)); err != nil {
		return &TemplateError{Module: task.module.Name, Path: task.to, Err: err}
	}
	return nil
}

// ctxMap converts Context and the plan's module list to the map executed
// against templates. Modules is Render-derived and never caller-set — it
// is not part of the Context struct itself.
func ctxMap(ctx Context, modules []ModuleInfo) map[string]any {
	return map[string]any{
		"ProjectName": ctx.ProjectName,
		"ModulePath":  ctx.ModulePath,
		"GoVersion":   ctx.GoVersion,
		"Modules":     modules,
	}
}

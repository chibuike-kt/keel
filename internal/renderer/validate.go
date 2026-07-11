package renderer

import (
	"path"
	"strings"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
)

// renderTask is one template ready to execute: its source in the templates
// fs.FS and its cleaned, safe destination relative to targetDir.
type renderTask struct {
	module *manifest.Manifest
	from   string
	to     string
}

// dependency is one deduped Go module requirement, with provenance for
// error messages.
type dependency struct {
	module  string
	version string
	via     string
}

// validatePlan runs pass 1: computing every template's destination and the
// merged dependency set, aggregating every violation into a *RenderError
// rather than stopping at the first. It never touches disk.
func validatePlan(plan *resolver.Plan) ([]renderTask, []dependency, error) {
	var renderErr RenderError

	tasks := collectTasks(plan, &renderErr)
	deps := collectDependencies(plan, &renderErr)

	if len(renderErr.PathEscapes) > 0 || len(renderErr.DuplicateOutputPaths) > 0 || len(renderErr.DependencyConflicts) > 0 {
		return nil, nil, &renderErr
	}
	return tasks, deps, nil
}

func collectTasks(plan *resolver.Plan, renderErr *RenderError) []renderTask {
	var tasks []renderTask
	writtenBy := make(map[string]string, len(plan.Modules))

	for _, mod := range plan.Modules {
		for _, tmpl := range mod.Languages[plan.Language].Templates {
			to, ok := cleanDestPath(tmpl.To)
			if !ok {
				renderErr.PathEscapes = append(renderErr.PathEscapes, &PathEscapeError{
					Module: mod.Name,
					Path:   tmpl.To,
				})
				continue
			}

			if first, ok := writtenBy[to]; ok {
				renderErr.DuplicateOutputPaths = append(renderErr.DuplicateOutputPaths, &DuplicateOutputPathError{
					Path:    to,
					ModuleA: first,
					ModuleB: mod.Name,
				})
				continue
			}
			writtenBy[to] = mod.Name

			tasks = append(tasks, renderTask{
				module: mod,
				from:   path.Join("modules", mod.Name, tmpl.From),
				to:     to,
			})
		}
	}
	return tasks
}

func collectDependencies(plan *resolver.Plan, renderErr *RenderError) []dependency {
	var deps []dependency
	anchors := make(map[string]dependency, len(plan.Modules))

	for _, mod := range plan.Modules {
		for _, dep := range mod.Languages[plan.Language].Dependencies {
			if dep.Module == "" {
				continue
			}

			anchor, ok := anchors[dep.Module]
			if !ok {
				anchor = dependency{module: dep.Module, version: dep.Version, via: mod.Name}
				anchors[dep.Module] = anchor
				deps = append(deps, anchor)
				continue
			}

			if anchor.version != dep.Version {
				renderErr.DependencyConflicts = append(renderErr.DependencyConflicts, &DependencyConflictError{
					Module:   dep.Module,
					VersionA: anchor.version,
					ViaA:     anchor.via,
					VersionB: dep.Version,
					ViaB:     mod.Name,
				})
			}
			// Same module at the same version from another module dedupes
			// silently: nothing more to record.
		}
	}
	return deps
}

// cleanDestPath validates and cleans a manifest Template.To, independent of
// whatever manifest.Validate already checked. Rejects anything absolute,
// anything that escapes the target directory once cleaned, and — since the
// cleaned path is later joined onto a real directory with filepath.FromSlash
// — any backslash or colon, which path.Clean would not treat as separators
// but a Windows filesystem would.
func cleanDestPath(to string) (string, bool) {
	if to == "" {
		return "", false
	}
	if strings.ContainsAny(to, `\:`) {
		return "", false
	}
	if strings.HasPrefix(to, "/") {
		return "", false
	}

	cleaned := path.Clean(to)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

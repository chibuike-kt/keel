package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/chibuike-kt/keel/internal/manifest"
	"github.com/chibuike-kt/keel/internal/resolver"
	"github.com/chibuike-kt/keel/internal/state"
)

// AddOptions configures RenderAdd.
type AddOptions struct {
	// Force skips the per-file collision check — a destination a new
	// module's template would write to that already exists on disk.
	// It never bypasses anything validatePlan finds (DependencyConflictError,
	// EnvVarConflictError) or anything resolver.Resolve itself rejects
	// (cycles, requires/conflicts, unsupported language): those represent
	// a genuine incompatibility between modules, not an incidental file
	// clash, and no flag overrides them in v1.
	Force bool
	// DryRun computes and reports what RenderAdd would do without
	// writing anything — no files, no go.mod/.env.example edits, no
	// state.json update.
	DryRun bool
}

// AddedModule is one module RenderAdd newly rendered.
type AddedModule struct {
	Name    string
	Version string
}

// AddResult summarizes what RenderAdd did, or — under AddOptions.DryRun —
// would do. Every slice is sorted for deterministic output.
type AddResult struct {
	NewModules []AddedModule // modules not previously present, now rendered
	Files      []string      // template destinations written, relative to targetDir
	GoModDeps  []string      // dependency module paths newly added to go.mod
	EnvVars    []string      // env var names newly appended to .env.example
}

// RenderAdd renders newly-requested modules' templates into an
// already-existing project, merging go.mod and .env.example instead of
// overwriting them, and updates .keel/state.json. This is keel add's
// entry point — the mirror image of Render, which refuses to run
// against a directory that already exists; RenderAdd refuses to run
// against one that doesn't (ErrTargetMissing).
//
// plan is the full resolved closure — existing modules (named in
// existing) union whatever was newly requested — exactly as
// resolver.Resolve/ResolveProject already produces for keel init.
// Nothing about conflict, cycle, or requires detection changes for
// keel add: it is the same Resolve call and the same validatePlan
// pass, over a larger requested set. A module already listed in
// existing is never re-rendered; its templates are assumed already on
// disk from a prior init or add.
//
// Unlike Render, RenderAdd writes directly into targetDir rather than
// staging into a sibling directory and renaming — there is no atomic
// swap available for a directory that already has other content in it.
// If writing fails partway through, already-written new files are left
// in place; validation (the conflict and collision checks) happens
// entirely before the first write, so a failure discovered there always
// leaves targetDir untouched, but an I/O failure mid-write does not roll
// back. This asymmetry with Render's all-or-nothing guarantee is a
// property of adding to an existing tree, not an oversight.
func (r *Renderer) RenderAdd(plan *resolver.Plan, ctx Context, targetDir string, existing []state.Module, opts AddOptions) (*AddResult, error) {
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTargetMissing
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("renderer: %s is not a directory", targetDir)
	}

	// Full-plan validation first, unaffected by Force: a DependencyConflictError
	// or EnvVarConflictError here means two modules are genuinely
	// incompatible, not that a file happens to be in the way.
	tasks, deps, envVars, err := validatePlan(plan)
	if err != nil {
		return nil, err
	}

	existingNames := make(map[string]bool, len(existing))
	for _, m := range existing {
		existingNames[m.Name] = true
	}

	var newModules []*manifest.Manifest
	newModuleNames := make(map[string]bool)
	for _, mod := range plan.Modules {
		if !existingNames[mod.Name] {
			newModules = append(newModules, mod)
			newModuleNames[mod.Name] = true
		}
	}

	var newTasks []renderTask
	for _, t := range tasks {
		if newModuleNames[t.module.Name] {
			newTasks = append(newTasks, t)
		}
	}

	// Per-file collision check against what's already on disk — separate
	// from validatePlan's within-plan duplicate-output-path check above,
	// and the only failure Force is allowed to bypass.
	var collisions []*FileCollisionError
	for _, t := range newTasks {
		dest := filepath.Join(targetDir, filepath.FromSlash(t.to))
		if _, err := os.Stat(dest); err == nil {
			collisions = append(collisions, &FileCollisionError{Module: t.module.Name, Path: t.to})
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if len(collisions) > 0 && !opts.Force {
		return nil, &AddError{FileCollisions: collisions}
	}

	mf, err := readGoMod(targetDir)
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}
	missingDeps := missingGoModDeps(mf, deps)

	// A var already declared by a module that was already present is
	// already in .env.example — never re-appended. Anything conflicting
	// (same name, different Required/Default) already failed above via
	// validatePlan's EnvVarConflictError; anything that reaches here with
	// the same name and the same Required/Default was already deduped by
	// collectEnvVars into a single entry in envVars, so it's simply
	// filtered out here rather than double-checked.
	existingVarNames := make(map[string]bool)
	for _, mod := range plan.Modules {
		if existingNames[mod.Name] {
			for _, ev := range mod.EnvVars {
				existingVarNames[ev.Name] = true
			}
		}
	}
	var newVars []manifest.EnvVar
	for _, ev := range envVars {
		if !existingVarNames[ev.Name] {
			newVars = append(newVars, ev)
		}
	}

	result := buildAddResult(newModules, newTasks, missingDeps, newVars)

	if opts.DryRun {
		return result, nil
	}

	modInfos := moduleInfos(plan)
	for _, t := range newTasks {
		if err := r.renderTemplate(t, ctx, modInfos, targetDir); err != nil {
			return nil, err
		}
	}

	if err := mergeGoMod(targetDir, missingDeps); err != nil {
		return nil, err
	}

	if len(newVars) > 0 {
		addedNames := make([]string, len(newModules))
		for i, m := range newModules {
			addedNames[i] = m.Name
		}
		sort.Strings(addedNames)
		if err := mergeEnvExample(targetDir, newVars, addedNames); err != nil {
			return nil, err
		}
	}

	merged := make([]state.Module, 0, len(existing)+len(newModules))
	merged = append(merged, existing...)
	for _, m := range result.NewModules {
		merged = append(merged, state.Module{Name: m.Name, Version: m.Version})
	}
	if err := state.Save(targetDir, &state.State{
		SchemaVersion: state.SchemaVersion,
		Language:      plan.Language,
		Modules:       merged,
	}); err != nil {
		return nil, fmt.Errorf("updating state: %w", err)
	}

	return result, nil
}

func buildAddResult(newModules []*manifest.Manifest, newTasks []renderTask, missingDeps []dependency, newVars []manifest.EnvVar) *AddResult {
	result := &AddResult{}

	for _, m := range newModules {
		result.NewModules = append(result.NewModules, AddedModule{Name: m.Name, Version: m.Version})
	}
	sort.Slice(result.NewModules, func(i, j int) bool { return result.NewModules[i].Name < result.NewModules[j].Name })

	for _, t := range newTasks {
		result.Files = append(result.Files, t.to)
	}
	sort.Strings(result.Files)

	for _, d := range missingDeps {
		result.GoModDeps = append(result.GoModDeps, d.module)
	}
	sort.Strings(result.GoModDeps)

	for _, v := range newVars {
		result.EnvVars = append(result.EnvVars, v.Name)
	}
	sort.Strings(result.EnvVars)

	return result
}

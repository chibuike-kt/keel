package renderer

import (
	"github.com/chibuike-kt/keel/internal/resolver"
	"github.com/chibuike-kt/keel/internal/state"
)

// writeState records plan's resolved module set to
// stagingDir/.keel/state.json — the one thing keel init has never
// persisted before, and the only way a later keel add invocation can
// find out what a project already has. Runs inside Render's staging
// pass, so it participates in the same atomic rename as every other
// generated file: a project either has state.json or doesn't exist at
// all, never a half-rendered gap between the two.
func writeState(stagingDir string, plan *resolver.Plan) error {
	modules := make([]state.Module, len(plan.Modules))
	for i, m := range plan.Modules {
		modules[i] = state.Module{Name: m.Name, Version: m.Version}
	}
	return state.Save(stagingDir, &state.State{
		SchemaVersion: state.SchemaVersion,
		Language:      plan.Language,
		Modules:       modules,
	})
}

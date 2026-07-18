package renderer

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
)

// writeGoMod generates go.mod in stagingDir from ctx and deps. deps is
// sorted by module path before writing — the plan's dependency merge order
// is otherwise just discovery order, and this file must be byte-identical
// across repeated Render calls on the same plan.
func writeGoMod(stagingDir string, ctx Context, deps []dependency) error {
	sorted := slices.Clone(deps)
	slices.SortFunc(sorted, func(a, b dependency) int {
		return cmp.Compare(a.module, b.module)
	})

	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n\ngo %s\n", ctx.ModulePath, ctx.GoVersion)

	if len(sorted) > 0 {
		b.WriteString("\nrequire (\n")
		for _, d := range sorted {
			fmt.Fprintf(&b, "\t%s %s\n", d.module, d.version)
		}
		b.WriteString(")\n")
	}

	return os.WriteFile(filepath.Join(stagingDir, "go.mod"), []byte(b.String()), 0o644) //nolint:gosec // go.mod is ordinary project source, not sensitive
}

// readGoMod parses targetDir/go.mod via golang.org/x/mod/modfile — the
// standard tooling-adjacent package for programmatically reading and
// editing go.mod, not string manipulation. RenderAdd needs this twice:
// to compute which dependencies are already satisfied, and (via
// ContextFromExisting) to recover the project's module path and Go
// version without asking the user to re-supply them.
func readGoMod(targetDir string) (*modfile.File, error) {
	path := filepath.Join(targetDir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return modfile.Parse(path, data, nil)
}

// missingGoModDeps returns the subset of deps whose module path isn't
// already present in mf's require block, in deps' given order. A
// module path already present at any version — whether keel put it
// there or the user hand-added it — is left alone entirely: mergeGoMod
// only ever adds a missing require, never rewrites an existing one, so
// a user's own version pin always wins over what a module manifest
// asks for.
func missingGoModDeps(mf *modfile.File, deps []dependency) []dependency {
	present := make(map[string]bool, len(mf.Require))
	for _, r := range mf.Require {
		present[r.Mod.Path] = true
	}

	var missing []dependency
	for _, d := range deps {
		if !present[d.module] {
			missing = append(missing, d)
		}
	}
	return missing
}

// mergeGoMod adds each of missing's dependencies to targetDir/go.mod as
// a new require, re-parsing and re-serializing the whole file via
// modfile rather than editing its text — the same principle
// mergeEnvExample follows for .env.example, applied consistently to
// both merge paths instead of just one. Everything already in the file
// (including requires keel's own modules know nothing about) survives
// untouched.
func mergeGoMod(targetDir string, missing []dependency) error {
	if len(missing) == 0 {
		return nil
	}

	path := filepath.Join(targetDir, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	mf, err := modfile.Parse(path, data, nil)
	if err != nil {
		return err
	}

	for _, d := range missing {
		if err := mf.AddRequire(d.module, d.version); err != nil {
			return fmt.Errorf("go.mod: adding require %s %s: %w", d.module, d.version, err)
		}
	}
	mf.Cleanup()

	out, err := mf.Format()
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644) //nolint:gosec // go.mod is ordinary project source, not sensitive
}

// ContextFromExisting derives a Context for RenderAdd from a project
// that already exists on disk: ModulePath and GoVersion are read back
// from the real go.mod (keel add never asks the user to re-supply
// them — they were fixed at keel init time), and ProjectName from
// targetDir's own directory name.
func ContextFromExisting(targetDir string) (Context, error) {
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return Context{}, err
	}

	mf, err := readGoMod(targetDir)
	if err != nil {
		return Context{}, fmt.Errorf("reading go.mod: %w", err)
	}

	ctx := Context{ProjectName: filepath.Base(abs)}
	if mf.Module != nil {
		ctx.ModulePath = mf.Module.Mod.Path
	}
	if mf.Go != nil {
		ctx.GoVersion = mf.Go.Version
	}
	return ctx, nil
}

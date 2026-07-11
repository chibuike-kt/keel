package renderer

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

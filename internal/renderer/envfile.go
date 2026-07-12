package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chibuike-kt/keel/internal/manifest"
)

// writeEnvExample generates .env.example in stagingDir from vars, sorted
// alphabetically by name before writing — the same determinism
// requirement as go.mod's dependency sort: repeated Render calls on the
// same plan must produce byte-identical output.
//
// If vars is empty (a plan with no env-var-declaring modules), the file
// is still written, with a single header comment, rather than omitted —
// an unexpectedly missing file is more confusing than an
// empty-but-present one.
func writeEnvExample(stagingDir string, vars []manifest.EnvVar) error {
	sorted := make([]manifest.EnvVar, len(vars))
	copy(sorted, vars)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var b strings.Builder
	if len(sorted) == 0 {
		b.WriteString("# No environment variables required\n")
	}
	for i, ev := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "# %s\n", ev.Description)
		if ev.Required {
			b.WriteString("# Required\n")
			fmt.Fprintf(&b, "%s=\n", ev.Name)
		} else {
			fmt.Fprintf(&b, "# Optional (default: %s)\n", ev.Default)
			fmt.Fprintf(&b, "%s=%s\n", ev.Name, ev.Default)
		}
	}

	return os.WriteFile(filepath.Join(stagingDir, ".env.example"), []byte(b.String()), 0o644) //nolint:gosec // .env.example is a template with no real secrets, ordinary project source
}

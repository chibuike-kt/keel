package renderer

import (
	"bytes"
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
		writeEnvVarBlock(&b, ev)
	}

	return os.WriteFile(filepath.Join(stagingDir, ".env.example"), []byte(b.String()), 0o644) //nolint:gosec // .env.example is a template with no real secrets, ordinary project source
}

// writeEnvVarBlock writes one var's three-or-four-line block — shared
// by writeEnvExample's from-scratch generation and mergeEnvExample's
// appended entries, so a var looks identical however it got into the
// file.
func writeEnvVarBlock(b *strings.Builder, ev manifest.EnvVar) {
	fmt.Fprintf(b, "# %s\n", ev.Description)
	if ev.Required {
		b.WriteString("# Required\n")
		fmt.Fprintf(b, "%s=\n", ev.Name)
	} else {
		fmt.Fprintf(b, "# Optional (default: %s)\n", ev.Default)
		fmt.Fprintf(b, "%s=%s\n", ev.Name, ev.Default)
	}
}

// envAddedMarker is the substring that identifies a section
// mergeEnvExample has already appended — its presence anywhere in the
// file is what distinguishes a project's first keel add (which also
// gets the explanatory header) from a later one (which doesn't repeat
// it).
const envAddedMarker = "--- Added by 'keel add"

// mergeEnvExample appends newVars to targetDir/.env.example under a
// marker naming addedModules, instead of regenerating the file from
// scratch. Every line already in the file — including anything the
// user hand-edited since keel init — is left untouched; this is the
// same "never re-parse or rewrite existing content" posture mergeGoMod
// takes with go.mod, applied to the file where it matters even more
// since .env.example is meant to be filled in by hand.
//
// The first time the file is ever touched this way, the appended
// section also carries a short explanation that the file is now
// append-only — that property has to be visible in the file itself,
// not just in docs, the same way base's generated README documents
// that modules aren't auto-wired into routes.
func mergeEnvExample(targetDir string, newVars []manifest.EnvVar, addedModules []string) error {
	if len(newVars) == 0 {
		return nil
	}

	path := filepath.Join(targetDir, ".env.example")
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	sorted := make([]manifest.EnvVar, len(newVars))
	copy(sorted, newVars)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	firstAppend := !bytes.Contains(existing, []byte(envAddedMarker))

	var b strings.Builder
	b.Write(existing)
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n\n")) {
		if bytes.HasSuffix(existing, []byte("\n")) {
			b.WriteString("\n")
		} else {
			b.WriteString("\n\n")
		}
	}

	fmt.Fprintf(&b, "# %s '%s' ---\n", envAddedMarker, strings.Join(addedModules, " "))
	if firstAppend {
		b.WriteString("# This file is appended to, not regenerated, so your edits above are\n")
		b.WriteString("# preserved by future 'keel add' runs.\n")
	}
	b.WriteString("\n")

	for i, ev := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		writeEnvVarBlock(&b, ev)
	}

	return os.WriteFile(path, []byte(b.String()), 0o644) //nolint:gosec // .env.example is a template with no real secrets, ordinary project source
}

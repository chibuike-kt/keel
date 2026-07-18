package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	keel "github.com/chibuike-kt/keel"
	"github.com/chibuike-kt/keel/internal/catalog"
	"github.com/chibuike-kt/keel/internal/resolver"
	"github.com/chibuike-kt/keel/internal/state"
)

// cmdList is "keel list"'s real entry point: it loads keel's embedded
// module catalog and looks for .keel/state.json in the current working
// directory. See runList for the testable core this wraps.
func cmdList(out, errOut io.Writer) int {
	modulesFS, err := fs.Sub(keel.Modules, "modules")
	if err != nil {
		fmt.Fprintf(errOut, "keel list: %v\n", err)
		return 1
	}

	cat, err := catalog.LoadCatalog(modulesFS)
	if err != nil {
		fmt.Fprintf(errOut, "keel list: loading module catalog: %v\n", err)
		return 1
	}

	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(errOut, "keel list: %v\n", err)
		return 1
	}

	return runList(out, errOut, cat, dir)
}

// runList implements "keel list", parameterized over its catalog and
// target directory so it's testable against a synthetic catalog and a
// controlled directory, without touching the real embedded module set
// or the process's real working directory.
//
// list is read-only: it never writes, creates, or modifies anything on
// disk, including .keel/state.json — it only ever reads the catalog
// and (if present) the state file, the same read-only posture as
// state.Load itself.
//
// Unlike keel add, a missing .keel/state.json is not an error here:
// list works as a plain "browse what's available" command from any
// directory, project or not. Selection markers appear only when a
// state file is actually found; a state file that exists but fails to
// parse is reported as a warning, not a hard failure, since list's job
// (show the catalog) is still fully answerable without it.
func runList(out, errOut io.Writer, cat resolver.MapCatalog, targetDir string) int {
	var selected map[string]bool

	st, err := state.Load(targetDir)
	switch {
	case err == nil:
		selected = make(map[string]bool, len(st.Modules))
		for _, m := range st.Modules {
			selected[m.Name] = true
		}
	case errors.Is(err, state.ErrNotFound):
		// Not a keel project — list still works, just without markers.
	default:
		fmt.Fprintf(errOut, "keel list: warning: reading %s: %v\n", state.RelPath, err)
	}

	printCatalog(out, cat, selected)
	return 0
}

// printCatalog prints every module in cat, sorted alphabetically
// (resolver.MapCatalog is a plain map — its iteration order carries no
// meaning), one per line with its summary, name-column aligned so
// summaries line up regardless of name length.
//
// selected == nil means no project context was found: names print
// bare, with no marker column at all, matching list's "works anywhere"
// contract — a bracket column of all blanks would misleadingly imply
// "nothing selected" when what's actually true is "unknown". selected
// != nil (even if empty) marks each catalog module already present in
// the project with "[x]", everything else with "[ ]".
func printCatalog(out io.Writer, cat resolver.MapCatalog, selected map[string]bool) {
	names := sortedModuleNames(cat)

	nameWidth := 0
	for _, name := range names {
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
	}

	for _, name := range names {
		m, _ := cat.Module(name)
		if selected == nil {
			fmt.Fprintf(out, "%-*s  %s\n", nameWidth, name, m.Summary)
			continue
		}
		marker := "[ ]"
		if selected[name] {
			marker = "[x]"
		}
		fmt.Fprintf(out, "%s %-*s  %s\n", marker, nameWidth, name, m.Summary)
	}
}

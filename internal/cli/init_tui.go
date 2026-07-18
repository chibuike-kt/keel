package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"charm.land/huh/v2"
	"golang.org/x/term"

	"github.com/chibuike-kt/keel/internal/resolver"
)

// isInteractiveTerminal reports whether f is connected to a real
// terminal, as opposed to piped input, a regular file, or (in
// non-interactive contexts generally) anything else with no line
// discipline behind it. Only *os.File values can be probed this way —
// os.Stdin in real use, or an os.Pipe/regular file in tests — a plain
// io.Reader has no file descriptor to check at all.
func isInteractiveTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// promptForModulesTUI shows a huh.MultiSelect populated from cat, sorted
// alphabetically to match the plain-text fallback's listing order, then
// a huh.Confirm summarizing exactly what will be generated — a cheap
// safety check before Render ever writes anything to disk, the same
// "confirm before an irreversible action" instinct behind Render's own
// transactional all-or-nothing guarantee. Returns the selected module
// names (not including "base", which resolver.ResolveProject always
// adds regardless), whether the user confirmed, and any error from
// running the form itself (e.g. the user pressed Ctrl+C).
func promptForModulesTUI(name string, cat resolver.MapCatalog) (selected []string, confirmed bool, err error) {
	names := sortedModuleNames(cat)

	options := make([]huh.Option[string], len(names))
	for i, n := range names {
		m, _ := cat.Module(n)
		options[i] = huh.NewOption(fmt.Sprintf("%s - %s", n, m.Summary), n)
	}

	multiSelect := huh.NewMultiSelect[string]().
		Title("Select modules to include").
		Description(`"base" is always included regardless of what you pick here.`).
		Options(options...).
		Value(&selected)

	if err := huh.NewForm(huh.NewGroup(multiSelect)).Run(); err != nil {
		return nil, false, err
	}

	confirm := huh.NewConfirm().
		Title(fmt.Sprintf("Generate %s with: %s?", name, summaryList(selected))).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed)

	if err := huh.NewForm(huh.NewGroup(confirm)).Run(); err != nil {
		return nil, false, err
	}

	return selected, confirmed, nil
}

// summaryList renders "base" plus the given selection, deduped and
// sorted, as a human-readable comma-separated list for the confirm
// prompt — showing exactly what resolver.ResolveProject will actually
// include, since base is always in the final plan regardless of
// selected.
func summaryList(selected []string) string {
	seen := make(map[string]bool, len(selected)+1)
	names := make([]string, 0, len(selected)+1)
	for _, n := range append([]string{"base"}, selected...) {
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

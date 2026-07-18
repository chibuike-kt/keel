package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	keel "github.com/chibuike-kt/keel"
	"github.com/chibuike-kt/keel/internal/catalog"
	"github.com/chibuike-kt/keel/internal/renderer"
	"github.com/chibuike-kt/keel/internal/resolver"
)

const initUsage = `Usage:
  keel init <name> [flags]

Flags:
`

var validNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateName rejects anything that isn't a safe, bare identifier
// before name is used anywhere — as the target directory, substituted
// into a Go string literal in main.go.tmpl (ProjectName), and as the
// default Go module path. A name containing a path separator, quote,
// backslash, or whitespace could reach any of those contexts and break
// it (an absolute path breaking Go string-literal escaping, or a quote
// breaking out of the literal entirely) — validating once here, before
// any use, is the fix; sanitizing at each point of use separately (e.g.
// filepath.Base) only papers over the specific symptom found, not the
// underlying gap.
func validateName(name string) error {
	if !validNameRE.MatchString(name) {
		return fmt.Errorf("invalid project name %q: must match %s", name, validNameRE.String())
	}
	return nil
}

// cmdInit is "keel init"'s real entry point: it loads keel's embedded
// module catalog and template tree, and reads the interactive module
// picker from the process's real stdin. See runInit for the testable
// core this wraps.
func cmdInit(args []string, out, errOut io.Writer) int {
	modulesFS, err := fs.Sub(keel.Modules, "modules")
	if err != nil {
		fmt.Fprintf(errOut, "keel init: %v\n", err)
		return 1
	}

	cat, err := catalog.LoadCatalog(modulesFS)
	if err != nil {
		fmt.Fprintf(errOut, "keel init: loading module catalog: %v\n", err)
		return 1
	}

	return runInit(args, out, errOut, os.Stdin, cat, keel.Modules)
}

// runInit implements "keel init", parameterized over its catalog, stdin
// source, and template filesystem so it's testable against a synthetic
// catalog and a controlled target directory, without touching the real
// embedded module set or the process's real stdin.
func runInit(args []string, out, errOut io.Writer, stdin io.Reader, cat resolver.MapCatalog, templates fs.FS) int {
	flagSet := flag.NewFlagSet("init", flag.ContinueOnError)
	flagSet.SetOutput(errOut)
	lang := flagSet.String("lang", "go", "target language")
	modulesFlag := flagSet.String("modules", "", `comma-separated module names to include (omit to be prompted interactively)`)
	modulePathFlag := flagSet.String("module-path", "", "Go import path for the generated project's go.mod (default: the project name)")

	flagSet.Usage = func() {
		fmt.Fprint(errOut, initUsage)
		flagSet.PrintDefaults()
	}

	if len(args) == 0 {
		fmt.Fprintln(errOut, "keel init: missing project name")
		flagSet.Usage()
		return 2
	}
	name := args[0]
	if err := validateName(name); err != nil {
		fmt.Fprintf(errOut, "keel init: %v\n", err)
		return 2
	}

	if err := flagSet.Parse(args[1:]); err != nil {
		return 2
	}

	modulesSet := false
	flagSet.Visit(func(f *flag.Flag) {
		if f.Name == "modules" {
			modulesSet = true
		}
	})

	var selected []string
	switch f, isFile := stdin.(*os.File); {
	case modulesSet:
		selected = splitModules(*modulesFlag)
	case isFile && isInteractiveTerminal(f):
		var confirmed bool
		var err error
		selected, confirmed, err = promptForModulesTUI(name, cat)
		if err != nil {
			fmt.Fprintf(errOut, "keel init: %v\n", err)
			return 1
		}
		if !confirmed {
			fmt.Fprintln(out, "keel init: cancelled")
			return 0
		}
	default:
		// stdin isn't a real terminal — piped input, CI, or a test's
		// io.Reader that isn't even an *os.File. Falls back to the
		// plain-text picker unchanged.
		selected = promptForModules(out, stdin, cat)
	}

	// name is already validated against validNameRE above, so it's safe
	// to use directly here: as the target directory, as ProjectName
	// (substituted into a Go string literal in main.go.tmpl), and as
	// the default module path.
	modulePath := *modulePathFlag
	if modulePath == "" {
		modulePath = name
		fmt.Fprintf(out, "note: no --module-path given, defaulting go.mod's module line to %q — edit it before publishing if that's not the real import path.\n", modulePath)
	}

	plan, err := resolver.ResolveProject(cat, selected, *lang)
	if err != nil {
		printResolutionError(errOut, "keel init", err)
		return 1
	}

	r := renderer.New(templates)
	ctx := renderer.Context{
		ProjectName: name,
		ModulePath:  modulePath,
		GoVersion:   strings.TrimPrefix(runtime.Version(), "go"),
	}
	if err := r.Render(plan, ctx, name); err != nil {
		printRenderError(errOut, err)
		return 1
	}

	printSuccess(out, name, plan)
	return 0
}

// splitModules parses a comma-separated --modules value into trimmed,
// non-empty names.
func splitModules(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			names = append(names, p)
		}
	}
	return names
}

// promptForModules prints every catalog module, sorted by name and
// numbered, with its summary, then reads one line from stdin: "all" for
// everything, blank for none, or a comma-separated list of numbers
// and/or literal names. base is always included regardless, per
// resolver.ResolveProject's own contract — the prompt says so explicitly
// so that isn't a surprise later.
func promptForModules(out io.Writer, stdin io.Reader, cat resolver.MapCatalog) []string {
	names := sortedModuleNames(cat)

	fmt.Fprintln(out, "Available modules:")
	for i, name := range names {
		m, _ := cat.Module(name)
		fmt.Fprintf(out, "  %d. %s - %s\n", i+1, name, m.Summary)
	}
	fmt.Fprintln(out, `Select modules to include: comma-separated numbers or names, "all" for everything, or press Enter to include base only.`)
	fmt.Fprintln(out, `"base" is always included regardless of what you pick here — pressing Enter alone gives you base and nothing else.`)
	fmt.Fprint(out, "> ")

	line, _ := bufio.NewReader(stdin).ReadString('\n')
	line = strings.TrimSpace(line)

	switch line {
	case "":
		return nil
	case "all":
		return names
	}

	tokens := strings.Split(line, ",")
	selected := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if n, err := strconv.Atoi(tok); err == nil {
			if n >= 1 && n <= len(names) {
				selected = append(selected, names[n-1])
			}
			continue
		}
		selected = append(selected, tok)
	}
	return selected
}

// printResolutionError prints every problem resolver.ResolveProject
// aggregated, not just the first, when it returns a *resolver.
// ResolutionError. cmd names the caller ("keel init", "keel add") so
// the message accurately reflects which command actually failed.
func printResolutionError(errOut io.Writer, cmd string, err error) {
	if resErr, ok := errors.AsType[*resolver.ResolutionError](err); ok {
		fmt.Fprintf(errOut, "%s: could not resolve the requested modules:\n", cmd)
		for _, problem := range resErr.Unwrap() {
			fmt.Fprintf(errOut, "  - %s\n", problem)
		}
		return
	}
	fmt.Fprintf(errOut, "%s: %v\n", cmd, err)
}

// printRenderError prints every problem renderer.Render aggregated, not
// just the first, when it returns a *renderer.RenderError — the whole
// point of that aggregation is surfacing every issue at once, and the
// CLI is where that finally reaches an actual human.
func printRenderError(errOut io.Writer, err error) {
	if renderErr, ok := errors.AsType[*renderer.RenderError](err); ok {
		fmt.Fprintln(errOut, "keel init: generation failed:")
		for _, problem := range renderErr.Unwrap() {
			fmt.Fprintf(errOut, "  - %s\n", problem)
		}
		return
	}
	fmt.Fprintf(errOut, "keel init: %v\n", err)
}

func printSuccess(out io.Writer, name string, plan *resolver.Plan) {
	names := make([]string, len(plan.Modules))
	for i, m := range plan.Modules {
		names[i] = m.Name
	}
	sort.Strings(names)

	fmt.Fprintf(out, "Generated %s with modules: %s\n", name, strings.Join(names, ", "))
	fmt.Fprintf(out, "cd %s && cat README.md to get started\n", name)
}

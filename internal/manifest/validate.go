package manifest

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
)

var (
	nameRE       = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	versionRE    = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	envVarNameRE = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

var knownLanguages = map[string]bool{
	"go":         true,
	"typescript": true,
}

// FieldError describes a single validation failure at a field path, e.g.
// "languages.go.templates[0].to".
type FieldError struct {
	Field   string
	Message string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationError aggregates every FieldError found while validating a
// Manifest, rather than stopping at the first one.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		msgs[i] = fe.Error()
	}
	return strings.Join(msgs, "\n")
}

func (e *ValidationError) add(field, format string, args ...any) {
	e.Errors = append(e.Errors, FieldError{Field: field, Message: fmt.Sprintf(format, args...)})
}

// Validate checks a decoded Manifest against the module schema, collecting
// every violation before returning. A nil error means m is well-formed; a
// non-nil error is always a *ValidationError.
func Validate(m *Manifest) error {
	errs := &ValidationError{}

	validateIdentity(errs, m)
	validateLists(errs, m)
	validateEnvVars(errs, m)
	validateLanguages(errs, m)

	if len(errs.Errors) == 0 {
		return nil
	}
	return errs
}

func validateIdentity(errs *ValidationError, m *Manifest) {
	const wantAPIVersion = "keel/v1"
	if m.APIVersion != wantAPIVersion {
		errs.add("apiVersion", "must equal %q, got %q", wantAPIVersion, m.APIVersion)
	}

	switch {
	case m.Name == "":
		errs.add("name", "is required")
	case !nameRE.MatchString(m.Name):
		errs.add("name", "must match %s, got %q", nameRE.String(), m.Name)
	}

	switch {
	case m.Version == "":
		errs.add("version", "is required")
	case !versionRE.MatchString(m.Version):
		errs.add("version", "must be a semver core version with optional -prerelease or +build, got %q", m.Version)
	}

	switch {
	case m.Summary == "":
		errs.add("summary", "is required")
	case strings.ContainsAny(m.Summary, "\n\r"):
		errs.add("summary", "must be a single line")
	case len(m.Summary) > 120:
		errs.add("summary", "must be at most 120 characters, got %d", len(m.Summary))
	}
}

// validateNameSet checks a tags/requires/conflicts style list: every entry
// lowercase, non-empty, and unique. It returns the set of valid entries seen,
// for callers that need to cross-check against other lists.
func validateNameSet(errs *ValidationError, field string, values []string) map[string]bool {
	seen := make(map[string]bool, len(values))
	for i, v := range values {
		entryField := fmt.Sprintf("%s[%d]", field, i)
		if v == "" {
			errs.add(entryField, "must not be empty")
			continue
		}
		if v != strings.ToLower(v) {
			errs.add(entryField, "must be lowercase, got %q", v)
		}
		if seen[v] {
			errs.add(entryField, "duplicates another entry in %s", field)
			continue
		}
		seen[v] = true
	}
	return seen
}

func validateLists(errs *ValidationError, m *Manifest) {
	validateNameSet(errs, "tags", m.Tags)
	requires := validateNameSet(errs, "requires", m.Requires)
	conflicts := validateNameSet(errs, "conflicts", m.Conflicts)

	if requires[m.Name] {
		errs.add("requires", "must not include the module's own name %q", m.Name)
	}
	if conflicts[m.Name] {
		errs.add("conflicts", "must not include the module's own name %q", m.Name)
	}

	for name := range requires {
		if conflicts[name] {
			errs.add("requires", "%q must not appear in both requires and conflicts", name)
		}
	}
}

func validateEnvVars(errs *ValidationError, m *Manifest) {
	seen := make(map[string]bool, len(m.EnvVars))
	for i, ev := range m.EnvVars {
		validateEnvVar(errs, fmt.Sprintf("env[%d]", i), ev, seen)
	}
}

func validateEnvVar(errs *ValidationError, field string, ev EnvVar, seen map[string]bool) {
	if ev.Name == "" {
		errs.add(field+".name", "is required")
	} else {
		if !envVarNameRE.MatchString(ev.Name) {
			errs.add(field+".name", "must match %s, got %q", envVarNameRE.String(), ev.Name)
		}
		if seen[ev.Name] {
			errs.add(field+".name", "duplicates another entry in env")
		}
		seen[ev.Name] = true
	}

	if ev.Description == "" {
		errs.add(field+".description", "is required")
	}

	if ev.Required && ev.Default != "" {
		errs.add(field, "must not set default when required is true (a var with a default isn't really required)")
	}
}

func validateLanguages(errs *ValidationError, m *Manifest) {
	if len(m.Languages) == 0 {
		errs.add("languages", "must declare at least one language")
		return
	}

	names := make([]string, 0, len(m.Languages))
	for name := range m.Languages {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		field := fmt.Sprintf("languages.%s", name)
		if !knownLanguages[name] {
			errs.add(field, "unknown language %q", name)
			continue
		}
		validateLanguage(errs, field, name, m.Languages[name])
	}
}

func validateLanguage(errs *ValidationError, field, name string, l Language) {
	if len(l.Templates) == 0 {
		errs.add(field+".templates", "must declare at least one template")
	}
	for i, t := range l.Templates {
		validateTemplate(errs, fmt.Sprintf("%s.templates[%d]", field, i), t)
	}

	seen := make(map[string]bool, len(l.Dependencies))
	for i, d := range l.Dependencies {
		validateDependency(errs, fmt.Sprintf("%s.dependencies[%d]", field, i), name, d, seen)
	}
}

func validateTemplate(errs *ValidationError, field string, t Template) {
	if t.From == "" {
		errs.add(field+".from", "is required")
	}

	switch {
	case t.To == "":
		errs.add(field+".to", "is required")
	case strings.HasPrefix(t.To, "/"):
		errs.add(field+".to", "must be relative, got %q", t.To)
	case escapesTargetDir(t.To):
		errs.add(field+".to", "must not escape the target directory, got %q", t.To)
	}
}

func escapesTargetDir(to string) bool {
	cleaned := path.Clean(to)
	return cleaned == ".." || strings.HasPrefix(cleaned, "../")
}

func validateDependency(errs *ValidationError, field, lang string, d Dependency, seen map[string]bool) {
	if d.Version == "" {
		errs.add(field+".version", "is required")
	}

	var key string
	switch lang {
	case "go":
		if d.Module == "" {
			errs.add(field+".module", "is required for go dependencies")
		}
		if d.Package != "" {
			errs.add(field+".package", "must not be set for go dependencies")
		}
		key = d.Module
	case "typescript":
		if d.Package == "" {
			errs.add(field+".package", "is required for typescript dependencies")
		}
		if d.Module != "" {
			errs.add(field+".module", "must not be set for typescript dependencies")
		}
		key = d.Package
	}

	if key == "" {
		return
	}
	if seen[key] {
		errs.add(field, "duplicate dependency %q for %s", key, lang)
		return
	}
	seen[key] = true
}

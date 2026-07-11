// Package manifest loads and validates module.yaml manifests into typed
// structs. See docs/architecture.md for the manifest schema.
package manifest

// Manifest is the decoded, unvalidated form of a module.yaml file.
type Manifest struct {
	APIVersion string `yaml:"apiVersion"`
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Summary    string `yaml:"summary"`

	Tags      []string `yaml:"tags"`
	Requires  []string `yaml:"requires"`
	Conflicts []string `yaml:"conflicts"`

	Languages map[string]Language `yaml:"languages"`
}

// Language is a per-language implementation of a module: the dependencies it
// introduces and the templates it renders.
type Language struct {
	Dependencies []Dependency `yaml:"dependencies"`
	Templates    []Template   `yaml:"templates"`
}

// Dependency is a single dependency contributed to the generated project's
// dependency manifest. Exactly one of Module (Go) or Package (npm) is set,
// matching the language it appears under.
type Dependency struct {
	Module  string `yaml:"module"`
	Package string `yaml:"package"`
	Version string `yaml:"version"`
}

// Template maps a source template file to a destination path relative to the
// generated project root.
type Template struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

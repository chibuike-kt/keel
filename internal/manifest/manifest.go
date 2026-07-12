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

	// EnvVars describes the environment variables a project needs to
	// configure when this module is selected. It does not mean this
	// module's own Go code calls os.Getenv directly — none of the shipped
	// modules do; secrets and settings are passed as constructor
	// parameters instead (e.g. ProviderStripe(secret)). This field exists
	// so tooling — the renderer's env var aggregation — can generate an
	// accurate .env.example for the generated project, not to describe
	// this module's internal implementation.
	EnvVars []EnvVar `yaml:"env"`

	Languages map[string]Language `yaml:"languages"`
}

// EnvVar describes one environment variable a project needs to configure
// when the declaring module is selected.
type EnvVar struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"` // must be empty when Required is true
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

package manifest

import (
	"errors"
	"strings"
	"testing"
)

// validManifest returns a Manifest that passes every validation rule. Tests
// mutate a copy of it to isolate exactly one violation per case.
func validManifest() *Manifest {
	return &Manifest{
		APIVersion: "keel/v1",
		Name:       "idempotency",
		Version:    "1.0.0",
		Summary:    "Idempotency-key middleware that makes unsafe writes replay-safe.",
		Tags:       []string{"reliability", "payments"},
		Requires:   []string{"logging"},
		Conflicts:  []string{"legacy-idempotency"},
		Languages: map[string]Language{
			"go": {
				Dependencies: []Dependency{
					{Module: "github.com/redis/go-redis/v9", Version: "v9.5.1"},
				},
				Templates: []Template{
					{From: "templates/go/middleware/idempotency.go.tmpl", To: "internal/middleware/idempotency.go"},
				},
			},
			"typescript": {
				Dependencies: []Dependency{
					{Package: "ioredis", Version: "^5.4.1"},
				},
				Templates: []Template{
					{From: "templates/typescript/src/middleware/idempotency.ts.tmpl", To: "src/middleware/idempotency.ts"},
				},
			},
		},
	}
}

func TestValidateValid(t *testing.T) {
	if err := Validate(validManifest()); err != nil {
		t.Fatalf("Validate(valid manifest) = %v, want nil", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(m *Manifest)
		wantField string
	}{
		{
			name:      "apiVersion wrong value",
			mutate:    func(m *Manifest) { m.APIVersion = "keel/v2" },
			wantField: "apiVersion",
		},
		{
			name:      "name missing",
			mutate:    func(m *Manifest) { m.Name = "" },
			wantField: "name",
		},
		{
			name:      "name invalid characters",
			mutate:    func(m *Manifest) { m.Name = "Idempotency_v2" },
			wantField: "name",
		},
		{
			name:      "version missing",
			mutate:    func(m *Manifest) { m.Version = "" },
			wantField: "version",
		},
		{
			name:      "version not semver core",
			mutate:    func(m *Manifest) { m.Version = "1.0" },
			wantField: "version",
		},
		{
			name:      "summary missing",
			mutate:    func(m *Manifest) { m.Summary = "" },
			wantField: "summary",
		},
		{
			name:      "summary multiline",
			mutate:    func(m *Manifest) { m.Summary = "line one\nline two" },
			wantField: "summary",
		},
		{
			name:      "summary too long",
			mutate:    func(m *Manifest) { m.Summary = strings.Repeat("a", 121) },
			wantField: "summary",
		},
		{
			name:      "tag not lowercase",
			mutate:    func(m *Manifest) { m.Tags = []string{"Payments"} },
			wantField: "tags[0]",
		},
		{
			name:      "tag empty",
			mutate:    func(m *Manifest) { m.Tags = []string{""} },
			wantField: "tags[0]",
		},
		{
			name:      "tag duplicate",
			mutate:    func(m *Manifest) { m.Tags = []string{"payments", "payments"} },
			wantField: "tags[1]",
		},
		{
			name:      "requires contains own name",
			mutate:    func(m *Manifest) { m.Requires = []string{"idempotency"} },
			wantField: "requires",
		},
		{
			name:      "conflicts contains own name",
			mutate:    func(m *Manifest) { m.Conflicts = []string{"idempotency"} },
			wantField: "conflicts",
		},
		{
			name: "requires and conflicts overlap",
			mutate: func(m *Manifest) {
				m.Requires = []string{"rate-limiting"}
				m.Conflicts = []string{"rate-limiting"}
			},
			wantField: "requires",
		},
		{
			name:      "languages empty",
			mutate:    func(m *Manifest) { m.Languages = nil },
			wantField: "languages",
		},
		{
			name:      "language unknown",
			mutate:    func(m *Manifest) { m.Languages["python"] = Language{} },
			wantField: "languages.python",
		},
		{
			name: "language without templates",
			mutate: func(m *Manifest) {
				m.Languages["go"] = Language{Dependencies: m.Languages["go"].Dependencies}
			},
			wantField: "languages.go.templates",
		},
		{
			name: "template from empty",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Templates[0].From = ""
				m.Languages["go"] = l
			},
			wantField: "languages.go.templates[0].from",
		},
		{
			name: "template to empty",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Templates[0].To = ""
				m.Languages["go"] = l
			},
			wantField: "languages.go.templates[0].to",
		},
		{
			name: "template to absolute",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Templates[0].To = "/etc/passwd"
				m.Languages["go"] = l
			},
			wantField: "languages.go.templates[0].to",
		},
		{
			name: "template to escapes target dir",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Templates[0].To = "../../etc/passwd"
				m.Languages["go"] = l
			},
			wantField: "languages.go.templates[0].to",
		},
		{
			name: "go dependency missing version",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Dependencies[0].Version = ""
				m.Languages["go"] = l
			},
			wantField: "languages.go.dependencies[0].version",
		},
		{
			name: "go dependency missing module",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Dependencies[0].Module = ""
				m.Languages["go"] = l
			},
			wantField: "languages.go.dependencies[0].module",
		},
		{
			name: "go dependency sets package",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Dependencies[0].Package = "should-not-be-here"
				m.Languages["go"] = l
			},
			wantField: "languages.go.dependencies[0].package",
		},
		{
			name: "typescript dependency missing package",
			mutate: func(m *Manifest) {
				l := m.Languages["typescript"]
				l.Dependencies[0].Package = ""
				m.Languages["typescript"] = l
			},
			wantField: "languages.typescript.dependencies[0].package",
		},
		{
			name: "typescript dependency sets module",
			mutate: func(m *Manifest) {
				l := m.Languages["typescript"]
				l.Dependencies[0].Module = "should-not-be-here"
				m.Languages["typescript"] = l
			},
			wantField: "languages.typescript.dependencies[0].module",
		},
		{
			name: "duplicate go dependency",
			mutate: func(m *Manifest) {
				l := m.Languages["go"]
				l.Dependencies = append(l.Dependencies, l.Dependencies[0])
				m.Languages["go"] = l
			},
			wantField: "languages.go.dependencies[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.mutate(m)

			err := Validate(m)
			if err == nil {
				t.Fatalf("Validate() = nil, want an error naming field %q", tt.wantField)
			}

			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() error type = %T, want *ValidationError", err)
			}

			for _, fe := range verr.Errors {
				if fe.Field == tt.wantField {
					return
				}
			}
			t.Fatalf("Validate() errors = %v, want one naming field %q", verr.Errors, tt.wantField)
		})
	}
}

func TestValidateAggregatesAllErrors(t *testing.T) {
	m := validManifest()
	m.APIVersion = "keel/v2"
	m.Name = ""
	m.Version = ""

	err := Validate(m)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("Validate() error type = %T, want *ValidationError", err)
	}
	if len(verr.Errors) < 3 {
		t.Fatalf("Validate() found %d errors, want at least 3: %v", len(verr.Errors), verr.Errors)
	}
}

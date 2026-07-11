package manifest

import (
	"reflect"
	"strings"
	"testing"
)

func TestLoadGolden(t *testing.T) {
	got, err := LoadFile("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	want := &Manifest{
		APIVersion: "keel/v1",
		Name:       "idempotency",
		Version:    "1.0.0",
		Summary:    "Idempotency-key middleware that makes unsafe writes replay-safe.",
		Tags:       []string{"reliability", "payments"},
		Requires:   []string{},
		Conflicts:  []string{},
		Languages: map[string]Language{
			"go": {
				Dependencies: []Dependency{
					{Module: "github.com/redis/go-redis/v9", Version: "v9.5.1"},
				},
				Templates: []Template{
					{From: "templates/go/middleware/idempotency.go.tmpl", To: "internal/middleware/idempotency.go"},
					{From: "templates/go/middleware/idempotency_test.go.tmpl", To: "internal/middleware/idempotency_test.go"},
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

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadFile(testdata/valid.yaml) =\n%#v\nwant\n%#v", got, want)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string // substring expected in the error, empty means no error
	}{
		{
			name: "unknown field",
			yaml: `apiVersion: keel/v1
name: foo
oops: true
`,
			wantErr: `unknown field "oops"`,
		},
		{
			name: "duplicate key",
			yaml: `apiVersion: keel/v1
name: foo
name: bar
`,
			wantErr: `mapping key "name" already defined`,
		},
		{
			name:    "empty document",
			yaml:    ``,
			wantErr: "empty manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(tt.yaml), "inline")
			if err == nil {
				t.Fatalf("Load() error = nil, want containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadSyntaxErrorAnnotatesSource(t *testing.T) {
	_, err := LoadFile("testdata/syntax-error.yaml")
	if err == nil {
		t.Fatal("LoadFile() error = nil, want a syntax error")
	}

	got := err.Error()
	if !strings.Contains(got, "testdata/syntax-error.yaml") {
		t.Errorf("error %q does not name the source", got)
	}
	if !strings.Contains(got, "[7:1]") {
		t.Errorf("error %q does not carry a line:column position", got)
	}
	if !strings.Contains(got, "languages:") {
		t.Errorf("error %q does not include the annotated source snippet", got)
	}
}

func TestLoadFileMissing(t *testing.T) {
	_, err := LoadFile("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("LoadFile() error = nil, want a not-exist error")
	}
}

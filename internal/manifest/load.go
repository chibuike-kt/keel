package manifest

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml"
)

// Load decodes a module.yaml manifest from r. name identifies the source
// (typically a file path) and is used only to label errors.
//
// Decoding is strict: unknown fields and duplicate keys are rejected. It does
// not validate the manifest's contents — call Validate for that.
func Load(r io.Reader, name string) (*Manifest, error) {
	var m Manifest

	dec := yaml.NewDecoder(r, yaml.DisallowUnknownField())
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s: empty manifest", name)
		}
		// goccy's error already carries a line:column prefix and an
		// annotated source snippet; wrapping preserves both.
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return &m, nil
}

// LoadFile reads and decodes the manifest at path. See Load for decoding
// semantics.
func LoadFile(path string) (*Manifest, error) {
	f, err := os.Open(path) //nolint:gosec // path is a manifest location supplied by the caller, not untrusted input
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer f.Close()

	return Load(f, path)
}

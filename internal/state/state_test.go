package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chibuike-kt/keel/internal/state"
)

func TestLoadMissingReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := state.Load(dir)
	if !errors.Is(err, state.ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	dir := t.TempDir()

	want := &state.State{
		SchemaVersion: state.SchemaVersion,
		Language:      "go",
		Modules: []state.Module{
			{Name: "idempotency", Version: "0.1.0"},
			{Name: "base", Version: "0.1.0"},
		},
	}
	if err := state.Save(dir, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := state.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.SchemaVersion != want.SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", got.SchemaVersion, want.SchemaVersion)
	}
	if got.Language != want.Language {
		t.Errorf("Language = %q, want %q", got.Language, want.Language)
	}
	if len(got.Modules) != 2 {
		t.Fatalf("Modules = %v, want 2 entries", got.Modules)
	}
	// Save sorts by name — proves the sort actually ran, not just that
	// round-tripping preserved whatever order was passed in.
	if got.Modules[0].Name != "base" || got.Modules[1].Name != "idempotency" {
		t.Errorf("Modules = %v, want sorted [base, idempotency]", got.Modules)
	}
}

func TestSaveIsDeterministic(t *testing.T) {
	dir1, dir2 := t.TempDir(), t.TempDir()

	s := &state.State{
		SchemaVersion: state.SchemaVersion,
		Language:      "go",
		Modules: []state.Module{
			{Name: "webhook", Version: "0.1.0"},
			{Name: "base", Version: "0.1.0"},
			{Name: "ledger", Version: "0.1.0"},
		},
	}
	if err := state.Save(dir1, s); err != nil {
		t.Fatalf("Save(dir1): %v", err)
	}
	if err := state.Save(dir2, s); err != nil {
		t.Fatalf("Save(dir2): %v", err)
	}

	data1, err := os.ReadFile(filepath.Join(dir1, ".keel", "state.json"))
	if err != nil {
		t.Fatalf("ReadFile(dir1): %v", err)
	}
	data2, err := os.ReadFile(filepath.Join(dir2, ".keel", "state.json"))
	if err != nil {
		t.Fatalf("ReadFile(dir2): %v", err)
	}
	if string(data1) != string(data2) {
		t.Fatalf("Save output not deterministic:\n%s\n---\n%s", data1, data2)
	}
}

func TestHas(t *testing.T) {
	s := &state.State{Modules: []state.Module{{Name: "ledger", Version: "0.1.0"}}}

	if !s.Has("ledger") {
		t.Error("Has(\"ledger\") = false, want true")
	}
	if s.Has("webhook") {
		t.Error("Has(\"webhook\") = true, want false")
	}
}

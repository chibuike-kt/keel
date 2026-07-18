package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chibuike-kt/keel/internal/resolver"
	"github.com/chibuike-kt/keel/internal/state"
)

func listTestCatalog() resolver.MapCatalog {
	return resolver.MapCatalog{
		"zebra": {Name: "zebra", Version: "0.1.0", Summary: "the last one alphabetically"},
		"alpha": {Name: "alpha", Version: "0.1.0", Summary: "the first one alphabetically"},
		"mid":   {Name: "mid", Version: "0.1.0", Summary: "somewhere in the middle"},
	}
}

// TestRunListNoStateFilePrintsFullCatalogNoMarkers proves list works
// as a plain "browse what's available" command outside any project —
// per the brief, this must never error just because there's no
// .keel/state.json, and must print every module with no selection
// marker column at all (not a column of all-blank markers, which
// would misleadingly imply "nothing selected" rather than "unknown").
func TestRunListNoStateFilePrintsFullCatalogNoMarkers(t *testing.T) {
	cat := listTestCatalog()

	var out, errOut bytes.Buffer
	code := runList(&out, &errOut, cat, t.TempDir())

	if code != 0 {
		t.Fatalf("runList() = %d, want 0\nstderr:\n%s", code, errOut.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}

	got := out.String()
	for _, want := range []string{"alpha", "mid", "zebra", "the first one alphabetically", "somewhere in the middle", "the last one alphabetically"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
	if strings.ContainsAny(got, "[]") {
		t.Errorf("output contains a marker column outside any project, want none:\n%s", got)
	}
}

// TestRunListWithStateMarksSelectedModules proves the marker column
// reflects exactly what .keel/state.json says — some modules present,
// not all — not "everything marked" or "nothing marked".
func TestRunListWithStateMarksSelectedModules(t *testing.T) {
	cat := listTestCatalog()
	dir := t.TempDir()

	if err := state.Save(dir, &state.State{
		SchemaVersion: state.SchemaVersion,
		Language:      "go",
		Modules:       []state.Module{{Name: "alpha", Version: "0.1.0"}, {Name: "zebra", Version: "0.1.0"}},
	}); err != nil {
		t.Fatalf("state.Save: %v", err)
	}

	var out, errOut bytes.Buffer
	code := runList(&out, &errOut, cat, dir)

	if code != 0 {
		t.Fatalf("runList() = %d, want 0\nstderr:\n%s", code, errOut.String())
	}

	// Markers are "[x]"/"[ ]" — a fixed 3-character prefix. Splitting on
	// whitespace instead would wrongly break "[ ]" into two fields at
	// its interior space.
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	marks := make(map[string]string)
	for _, line := range lines {
		if len(line) < 5 || line[3] != ' ' {
			t.Fatalf("unexpected line format: %q", line)
		}
		marker := line[:3]
		name := strings.Fields(line[4:])[0]
		marks[name] = marker
	}

	if marks["alpha"] != "[x]" {
		t.Errorf("alpha marker = %q, want [x] (present in state)", marks["alpha"])
	}
	if marks["zebra"] != "[x]" {
		t.Errorf("zebra marker = %q, want [x] (present in state)", marks["zebra"])
	}
	if marks["mid"] != "[ ]" {
		t.Errorf("mid marker = %q, want [ ] (absent from state)", marks["mid"])
	}
}

func TestRunListSortsAlphabetically(t *testing.T) {
	cat := listTestCatalog()

	var out, errOut bytes.Buffer
	if code := runList(&out, &errOut, cat, t.TempDir()); code != 0 {
		t.Fatalf("runList() = %d, want 0", code)
	}

	got := out.String()
	iAlpha := strings.Index(got, "alpha")
	iMid := strings.Index(got, "mid")
	iZebra := strings.Index(got, "zebra")
	if !(iAlpha < iMid && iMid < iZebra) {
		t.Fatalf("modules not in alphabetical order (alpha=%d, mid=%d, zebra=%d):\n%s", iAlpha, iMid, iZebra, got)
	}
}

// TestRunListNeverWritesAnything is the regression test the brief
// called out specifically: list is read-only and must never create
// .keel/state.json (when absent) or modify it (when present), or
// create any other file — a read-only command that accidentally
// writes something would be a real, surprising bug.
func TestRunListNeverWritesAnything(t *testing.T) {
	cat := listTestCatalog()

	t.Run("no_state_file_present", func(t *testing.T) {
		dir := t.TempDir()

		var out, errOut bytes.Buffer
		if code := runList(&out, &errOut, cat, dir); code != 0 {
			t.Fatalf("runList() = %d, want 0\nstderr:\n%s", code, errOut.String())
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) != 0 {
			t.Fatalf("directory entries after runList = %v, want none created", entries)
		}
	})

	t.Run("state_file_present", func(t *testing.T) {
		dir := t.TempDir()
		if err := state.Save(dir, &state.State{
			SchemaVersion: state.SchemaVersion,
			Language:      "go",
			Modules:       []state.Module{{Name: "alpha", Version: "0.1.0"}},
		}); err != nil {
			t.Fatalf("state.Save: %v", err)
		}

		before, err := os.ReadFile(filepath.Join(dir, ".keel", "state.json"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		var out, errOut bytes.Buffer
		if code := runList(&out, &errOut, cat, dir); code != 0 {
			t.Fatalf("runList() = %d, want 0\nstderr:\n%s", code, errOut.String())
		}

		after, err := os.ReadFile(filepath.Join(dir, ".keel", "state.json"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if string(before) != string(after) {
			t.Fatalf("state.json changed:\nbefore: %s\nafter: %s", before, after)
		}
	})
}

// TestRunListCorruptStateWarnsButStillListsCatalog proves a state.json
// that fails to parse doesn't take down list entirely — the catalog is
// still fully answerable without it, so list degrades to "no markers"
// with a warning rather than failing outright.
func TestRunListCorruptStateWarnsButStillListsCatalog(t *testing.T) {
	cat := listTestCatalog()
	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, ".keel"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".keel", "state.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var out, errOut bytes.Buffer
	code := runList(&out, &errOut, cat, dir)

	if code != 0 {
		t.Fatalf("runList() = %d, want 0 even with a corrupt state file\nstderr:\n%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Fatalf("output missing the catalog despite corrupt state:\n%s", out.String())
	}
	if errOut.Len() == 0 {
		t.Fatal("stderr empty, want a warning about the unreadable state file")
	}
}

package renderer

import (
	"os"
	"path/filepath"
	"testing"
	"text/template"
)

// TestModuleTemplatesParse walks every module's templates (all languages,
// not just Go) and parses each one. This is pure static analysis — no
// database, no network — so it runs in every CI run for free, the same way
// manifest's own modules_test.go glob does.
//
// It exists because of a real bug: a Go elided composite literal like
// []Entry{{AccountID: 1}, {AccountID: 2}} contains a literal "{{", which
// text/template reads as the start of an action rather than Go syntax. A
// template with that pattern parses as valid Go but fails at render time
// with an unrelated-looking error ("function ... not defined"). Parsing
// every template here catches that class of bug at the source, before it
// ever reaches a real Render call.
func TestModuleTemplatesParse(t *testing.T) {
	root := "../../modules"

	var checked int
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".tmpl" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: reading file: %v", path, err)
			return nil
		}

		if _, err := template.New(filepath.Base(path)).Parse(string(data)); err != nil {
			t.Errorf("%s: template.Parse: %v", path, err)
			return nil
		}
		checked++
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if checked == 0 {
		t.Fatal("no .tmpl files found under modules/ — glob is broken, or modules/ is empty")
	}
}

package resolver

import (
	"reflect"
	"testing"

	"github.com/chibuike-kt/keel/internal/manifest"
)

func TestFindUnsupportedLanguagesNone(t *testing.T) {
	c := catalog(mod("a", "b"), mod("b"))
	closure := mustClosure(t, c, "a")

	if got := findUnsupportedLanguages(closure, "go"); len(got) != 0 {
		t.Fatalf("findUnsupportedLanguages() = %v, want none", got)
	}
}

func TestFindUnsupportedLanguagesAggregate(t *testing.T) {
	tsOnly := &manifest.Manifest{
		Name:      "b",
		Languages: map[string]manifest.Language{"typescript": {}},
	}
	noLang := &manifest.Manifest{Name: "c", Languages: map[string]manifest.Language{}}

	c := catalog(mod("a", "b", "c"))
	c["b"] = tsOnly
	c["c"] = noLang
	closure := mustClosure(t, c, "a")

	got := findUnsupportedLanguages(closure, "go")
	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findUnsupportedLanguages() = %v, want %v", got, want)
	}
}

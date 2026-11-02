package index

import (
	"path/filepath"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestSQLiteStoreFileSymbolsCRUDAndIdempotentWrite(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	fs := &model.FileSymbols{
		File:     "a.go",
		Language: model.LanguageGo,
		Package:  "p",
		Symbols:  []*model.Symbol{{Name: "A", Package: "p", File: "a.go", Line: 1}},
		Imports:  []string{"fmt"},
	}
	if err := store.StoreFileSymbols(fs); err != nil {
		t.Fatal(err)
	}
	if err := store.StoreFileSymbols(fs); err != nil {
		t.Fatalf("idempotent StoreFileSymbols failed: %v", err)
	}
	got, err := store.GetFileSymbols("a.go")
	if err != nil {
		t.Fatal(err)
	}
	if got.File != fs.File || got.Language != fs.Language || len(got.Symbols) != 1 || got.Symbols[0].Name != "A" {
		t.Fatalf("got %+v, want original file symbols", got)
	}
}

func TestSQLiteStoreCallSites(t *testing.T) {
	store, err := NewSQLiteStore(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	caller := &model.Symbol{Name: "Caller", Package: "p"}
	callee := &model.Symbol{Name: "Callee", Package: "p"}
	if err := store.StoreCallSite(&model.CallSite{Caller: caller, Callee: callee, File: "a.go", Line: 2, Resolved: true}); err != nil {
		t.Fatal(err)
	}
	sites, err := store.GetCallSitesInFile("a.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Line != 2 {
		t.Fatalf("sites=%v, want one callsite at line 2", sites)
	}
	callers, err := store.GetCallersOf("p.Callee")
	if err != nil {
		t.Fatal(err)
	}
	if len(callers) != 1 || callers[0].Name != "p.Caller" {
		t.Fatalf("callers=%v, want p.Caller", callers)
	}
}

package graph

import (
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

func TestGoBareCallDoesNotResolveToMethod(t *testing.T) {
	for _, tc := range []struct {
		name     string
		language model.Language
		mixed    bool
		want     int
	}{
		{"go_bare", model.LanguageGo, false, 0},
		{"go_bare_and_receiver", model.LanguageGo, true, 1},
		{"javascript_unchanged", model.LanguageJavaScript, false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cg := NewCallGraph()
			caller := &model.Symbol{Name: "Call", Owner: "Service", Package: "p", File: "/p/a", Language: tc.language}
			target := &model.Symbol{Name: "Convert", Owner: "Service", Package: "p", File: "/p/a", Kind: model.SymbolKindMethod, Language: tc.language}
			cg.AddEdge(caller, &model.Symbol{Name: "Convert", Language: tc.language})
			if tc.mixed {
				cg.AddEdge(caller, &model.Symbol{Name: "Convert", Owner: "s", Kind: model.SymbolKindMethod, Language: tc.language})
			}
			cg.Finalize()
			if got := len(cg.GetCallers(target)); got != tc.want {
				t.Fatalf("GetCallers=%d want %d", got, tc.want)
			}
			if got := len(cg.GetCallersByKind(target, model.EdgeKindCall)); got != tc.want {
				t.Fatalf("GetCallersByKind=%d want %d", got, tc.want)
			}
		})
	}
}

func TestGoFreeFunctionRejectsMethodFallback(t *testing.T) {
	for _, tc := range []struct {
		name        string
		knownMethod bool
		want        int
	}{
		{"convert", false, 0}, // Unexported: importing cannot make it callable.
		{"Convert", true, 0},  // Exported, but source-backed callee is a method.
		{"Convert", false, 1}, // Unknown selector may be an imported package alias.
	} {
		t.Run(tc.name, func(t *testing.T) {
			cg := NewCallGraph()
			cg.SetFileImportsFunc(func(string) []string { return []string{"example.com/godeep"} })
			caller := &model.Symbol{Name: "Call", Package: "golang", Owner: "GoAdapter", File: "/golang/adapter.go", Language: model.LanguageGo}
			callee := &model.Symbol{Name: tc.name, Owner: "aliasOrReceiver", Kind: model.SymbolKindMethod, Language: model.LanguageGo}
			if tc.knownMethod {
				callee.File = "/golang/adapter.go"
				callee.Package = "golang"
				callee.Owner = "GoAdapter"
			}
			cg.AddEdge(caller, callee)
			cg.Finalize()
			target := &model.Symbol{Name: tc.name, Package: "godeep", Kind: model.SymbolKindFunction, Language: model.LanguageGo, File: "/godeep/helper.go"}
			if got := len(cg.GetCallers(target)); got != tc.want {
				t.Fatalf("GetCallers=%d want %d", got, tc.want)
			}
			if got := len(cg.GetCallersByKind(target, model.EdgeKindCall)); got != tc.want {
				t.Fatalf("GetCallersByKind=%d want %d", got, tc.want)
			}
		})
	}
}

func TestGoBareForwardCallPreservesDeclaration(t *testing.T) {
	for _, sourceFirst := range []bool{false, true} {
		cg := NewCallGraph()
		caller := &model.Symbol{Name: "Caller", Package: "p", File: "p.go", Language: model.LanguageGo}
		free := &model.Symbol{Name: "Convert", Package: "p", File: "p.go", Kind: model.SymbolKindFunction, Language: model.LanguageGo}
		method := &model.Symbol{Name: "Convert", Package: "p", File: "p.go", Owner: "Service", Kind: model.SymbolKindMethod, Language: model.LanguageGo}
		cg.AddSymbol(method)
		if sourceFirst {
			cg.AddSymbol(free)
		}
		cg.AddEdge(caller, &model.Symbol{Name: "Convert", Package: "p", Language: model.LanguageGo})
		if !sourceFirst {
			cg.AddSymbol(free)
		}
		cg.Finalize()
		if got, _ := cg.GetSymbol("p.Convert"); got != free {
			t.Errorf("sourceFirst=%v source declaration replaced: %+v", sourceFirst, got)
		}
		for _, got := range [][]*model.Symbol{cg.GetCallees(caller), cg.GetCalleesByKind(caller, model.EdgeKindCall)} {
			if len(got) != 1 || got[0] != free {
				t.Errorf("sourceFirst=%v forward resolution=%v want free declaration", sourceFirst, got)
			}
		}
	}
	// No exact qualified lookup: fallback must still reject same-name methods.
	cg := NewCallGraph()
	caller := &model.Symbol{Name: "Caller", Package: "p", Language: model.LanguageGo}
	free := &model.Symbol{Name: "Convert", Package: "p", File: "p.go", Language: model.LanguageGo, Kind: model.SymbolKindFunction}
	cg.AddSymbol(free)
	cg.AddSymbol(&model.Symbol{Name: "Convert", Package: "p", Owner: "Service", File: "p.go", Language: model.LanguageGo, Kind: model.SymbolKindMethod})
	cg.AddEdge(caller, &model.Symbol{Name: "Convert", Language: model.LanguageGo})
	cg.Finalize()
	for _, got := range [][]*model.Symbol{cg.GetCallees(caller), cg.GetCalleesByKind(caller, model.EdgeKindCall)} {
		if len(got) != 1 || got[0] != free {
			t.Errorf("bare fallback=%v want only free declaration", got)
		}
	}
}

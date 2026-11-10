package orchestrator

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/config"
	"github.com/nano-harness/impact-analyzer/internal/graph"
	_ "github.com/nano-harness/impact-analyzer/internal/lang/all"
	"github.com/nano-harness/impact-analyzer/internal/model"
)

// A same-named method, imported function, or repeated index entry must not
// interfere with a Go unqualified call to a local free function.
func TestGoBareCalleePackage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		copies  int
		foreign bool
		want    string
	}{
		{"local_only", 1, false, "godeep"}, {"foreign_method_and_function", 1, true, "godeep"}, {"duplicate_candidates", 4, true, "godeep"}, {"foreign_only_not_bare", 0, true, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := &Orchestrator{callGraph: graph.NewCallGraph()}
			for i := 0; i < tc.copies; i++ {
				o.callGraph.AddSymbol(&model.Symbol{Name: "convert", Package: "godeep", Kind: model.SymbolKindFunction})
			}
			if tc.foreign {
				o.callGraph.AddSymbol(&model.Symbol{Name: "convert", Package: "golang", Owner: "GoAdapter", Kind: model.SymbolKindMethod})
				o.callGraph.AddSymbol(&model.Symbol{Name: "convert", Package: "foreign", Kind: model.SymbolKindFunction})
			}
			fs := &model.FileSymbols{Package: "godeep", Imports: []string{"example.com/foreign"}}
			if got := o.inferGoBareCalleePackage("convert", fs); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if got := o.inferGoBareCalleePackage("unknown", fs); got != "" {
				t.Fatalf("unknown call inferred as %q", got)
			}
		})
	}
}

// Same source files, fresh orchestrators, no SSA; only indexing order varies.
func TestGoSameNameCallsOrderIndependent(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"godeep/session.go": `package godeep
 type Session struct{}
 func (s *Session) callSites() { ssaFunctionToSymbol() }
 func Exported() {}
 `,
		"godeep/helper.go": `package godeep
func ssaFunctionToSymbol() {}
`,
		"golang/adapter.go": `package golang
 import deep "example.com/order/godeep"
 type GoAdapter struct{}
 func (a *GoAdapter) ssaFunctionToSymbol() {}
 func (a *GoAdapter) BuildPackageCallGraph() { a.ssaFunctionToSymbol(); deep.Exported() }
 `,
	}
	for file, content := range files {
		p := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name    string
		files   []string
		callers []string
	}{
		{"free_function_first", []string{"godeep/helper.go", "godeep/session.go", "golang/adapter.go"}, []string{"godeep.Session.callSites"}},
		{"method_first", []string{"golang/adapter.go", "godeep/session.go", "godeep/helper.go"}, []string{"godeep.Session.callSites"}},
		{"caller_first_helper_last", []string{"godeep/session.go", "golang/adapter.go", "godeep/helper.go"}, []string{"godeep.Session.callSites"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{} // UseDeepAnalysis=false: no packages.Load/SSA/RTA.
			o, err := NewOrchestratorWithConfig(root, t.TempDir(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer o.Close()
			if err = o.buildCallGraph(tc.files); err != nil {
				t.Fatal(err)
			}
			o.callGraph.Finalize()
			target := &model.Symbol{Name: "ssaFunctionToSymbol", Package: "godeep", Kind: model.SymbolKindFunction, Language: model.LanguageGo, File: filepath.Join(root, "godeep/helper.go")}
			var got []string
			for _, s := range o.callGraph.GetCallers(target) {
				got = append(got, s.QualifiedName())
			}
			t.Logf("direct callers=%v", got)
			if !reflect.DeepEqual(got, tc.callers) {
				t.Fatalf("got %v want %v", got, tc.callers)
			}

			for _, query := range []struct {
				target *model.Symbol
				want   []string
			}{
				{&model.Symbol{Name: "ssaFunctionToSymbol", Package: "golang", Owner: "GoAdapter", Kind: model.SymbolKindMethod, Language: model.LanguageGo, File: filepath.Join(root, "golang/adapter.go")}, []string{"golang.GoAdapter.BuildPackageCallGraph"}},
				{&model.Symbol{Name: "Exported", Package: "godeep", Kind: model.SymbolKindFunction, Language: model.LanguageGo, File: filepath.Join(root, "godeep/session.go")}, []string{"golang.GoAdapter.BuildPackageCallGraph"}},
			} {
				var callers []string
				for _, sym := range o.callGraph.GetCallers(query.target) {
					callers = append(callers, sym.QualifiedName())
				}
				if !reflect.DeepEqual(callers, query.want) {
					t.Errorf("%s callers=%v want %v", query.target.QualifiedName(), callers, query.want)
				}
			}
		})
	}
}

func TestGoDotImportCallerSurvivesExactBucket(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"p/p.go": "package p\nfunc F() {}\nfunc Local() { F() }\n",
		"q/q.go": "package q\nimport . \"example.com/p\"\nfunc External() { F() }\n",
	}
	for file, content := range files {
		p := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, order := range [][]string{{"p/p.go", "q/q.go"}, {"q/q.go", "p/p.go"}} {
		o, err := NewOrchestratorWithConfig(root, t.TempDir(), &config.Config{})
		if err != nil {
			t.Fatal(err)
		}
		if err := o.buildCallGraph(order); err != nil {
			t.Fatal(err)
		}
		o.callGraph.Finalize()
		target := &model.Symbol{Name: "F", Package: "p", Language: model.LanguageGo, Kind: model.SymbolKindFunction}
		for _, callers := range [][]*model.Symbol{
			o.callGraph.GetCallers(target),
			o.callGraph.GetCallersByKind(target, model.EdgeKindCall),
		} {
			var got []string
			for _, caller := range callers {
				got = append(got, caller.QualifiedName())
			}
			if want := []string{"p.Local", "q.External"}; !reflect.DeepEqual(got, want) {
				t.Errorf("order %v callers=%v want %v", order, got, want)
			}
		}
		o.Close()
	}
}

package godeep

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"sort"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// No packages.Load: isolate conversion from loading, SSA construction and RTA.
func callerConversionFixture(t testing.TB) (*Session, *callgraph.Graph) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", `package main
 type Reader[T any] interface { Read() T }
 type Box[T any] struct { Value T }
 func identity[T any](v T) T { return func() T { return v }() }
 func (b Box[T]) Read() T { return identity(b.Value) }
 func consume(v int) {}
 func main() {
  var r Reader[int] = Box[int]{Value: 42}
  consume(r.Read())
  f := Box[int]{Value: 7}.Read
  consume(f())
  func() { consume(identity(8)) }()
 }
 `, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := ssautil.BuildPackage(&types.Config{}, fset, types.NewPackage("example.com/callerperf", "main"), []*ast.File{file}, ssa.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	a := &Analyzer{Prog: pkg.Prog, mains: []*ssa.Package{pkg}}
	cg, err := a.BuildCallGraph()
	if err != nil {
		t.Fatal(err)
	}
	return &Session{analyzer: a}, cg
}

// Preserve the pre-optimization loop as an independent equivalence oracle and
// benchmark baseline. Caller metadata is deliberately discarded, as before.
func callSitesWithFullCaller(s *Session, cg *callgraph.Graph) []*model.CallSite {
	var calls []*model.CallSite
	for _, node := range cg.Nodes {
		if node.Func == nil {
			continue
		}
		caller := ssaFunctionToSymbol(s.analyzer.Prog, node.Func)
		if caller == nil {
			continue
		}
		for _, edge := range node.Out {
			if edge.Callee == nil || edge.Callee.Func == nil {
				continue
			}
			callee := ssaFunctionToSymbol(s.analyzer.Prog, edge.Callee.Func)
			if callee == nil {
				continue
			}
			pos := s.analyzer.Prog.Fset.Position(edge.Pos())
			calls = append(calls, &model.CallSite{File: pos.Filename, Line: pos.Line, Column: pos.Column, EdgeKind: model.EdgeKindCall, Resolved: true, Callee: callee})
		}
	}
	return calls
}

func TestCallerEligibilityPreservesCallSites(t *testing.T) {
	s, cg := callerConversionFixture(t)
	generic, closure, wrapper, iface := false, false, false, false
	for fn, node := range cg.Nodes {
		source := SourceFunction(fn)
		eligible := source != nil && source.Pkg != nil
		if got := ssaFunctionToSymbol(s.analyzer.Prog, fn) != nil; got != eligible {
			t.Fatalf("caller eligibility changed for %v: full=%v fast=%v", fn, got, eligible)
		}
		if fn == nil {
			continue
		}
		generic = generic || fn.Origin() != nil
		closure = closure || fn.Parent() != nil
		wrapper = wrapper || (fn.Pkg == nil && fn.Origin() == nil && len(node.Out) > 0)
		for _, edge := range node.Out {
			iface = iface || (edge.Site != nil && edge.Site.Common().IsInvoke())
		}
	}
	if !generic || !closure || !wrapper || !iface {
		t.Fatalf("fixture missing coverage: generic=%v closure=%v wrapper=%v interface=%v", generic, closure, wrapper, iface)
	}
	normalize := func(calls []*model.CallSite) []string {
		out := make([]string, 0, len(calls))
		for _, call := range calls {
			data, err := json.Marshal(call)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, string(data))
		}
		sort.Strings(out)
		return out
	}
	want := normalize(callSitesWithFullCaller(s, cg))
	got := normalize(s.callSites(cg))
	if len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("calls changed:\ngot %v\nwant %v", got, want)
	}
}

var callerConversionSink []*model.CallSite

func BenchmarkCallerConversion(b *testing.B) {
	s, cg := callerConversionFixture(b)
	for _, tc := range []struct {
		name    string
		convert func(*Session, *callgraph.Graph) []*model.CallSite
	}{
		{"full_caller", callSitesWithFullCaller},
		{"eligibility_only", func(s *Session, g *callgraph.Graph) []*model.CallSite { return s.callSites(g) }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				callerConversionSink = tc.convert(s, cg)
			}
		})
	}
}

var rtaGuardSink *rta.Result

func BenchmarkRTAGuard(b *testing.B) {
	s, _ := callerConversionFixture(b)
	pkg := s.analyzer.mains[0]
	roots := []*ssa.Function{pkg.Func("init"), pkg.Func("main")}
	for _, tc := range []struct {
		name string
		run  func([]*ssa.Function) (*rta.Result, error)
	}{
		{"raw", func(roots []*ssa.Function) (*rta.Result, error) { return rta.Analyze(roots, true), nil }},
		{"guarded", analyzeRTA},
	} {
		b.Run(tc.name, func(b *testing.B) {
			if _, err := tc.run(roots); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := tc.run(roots)
				if err != nil {
					b.Fatal(err)
				}
				rtaGuardSink = result
			}
		})
	}
}

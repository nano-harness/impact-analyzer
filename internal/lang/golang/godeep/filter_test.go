package godeep

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func normalizeFilteredCalls(t testing.TB, calls []*model.CallSite) []string {
	t.Helper()
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

func postFilterCalls(calls []*model.CallSite, keep func(string) bool) []*model.CallSite {
	var out []*model.CallSite
	for _, call := range calls {
		if keep == nil || keep(call.File) {
			out = append(out, call)
		}
	}
	return out
}

func scopedConversionFixture(t testing.TB) (*Session, *callgraph.Graph) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", `package main
//line /repo/inside.go:1
func inside() {}
//line /dependency/outside.go:1
func outside() { inside() }
//line relative.go:1
func relative() { outside() }
//line /repo-other/collision.go:1
func collision() { inside() }
//line /repo/main.go:1
func main() { outside(); relative(); collision(); inside(); outside() }
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := ssautil.BuildPackage(&types.Config{}, fset, types.NewPackage("example.com/scoped", "main"), []*ast.File{file}, ssa.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	a := &Analyzer{Prog: pkg.Prog, mains: []*ssa.Package{pkg}}
	cg, err := a.BuildCallGraph()
	if err != nil {
		t.Fatal(err)
	}
	// A synthetic edge has token.NoPos and therefore an empty filename.
	callgraph.AddEdge(cg.Nodes[pkg.Func("main")], nil, cg.Nodes[pkg.Func("outside")])
	s := &Session{analyzer: a, initialByDir: map[string][]*packages.Package{"/repo": {{Types: pkg.Pkg}}}, closures: make(map[string][]*ssa.Package)}
	return s, cg
}

func TestCallSitesFilteredMatchesLateFilter(t *testing.T) {
	fixtures := []struct {
		name  string
		build func(testing.TB) (*Session, *callgraph.Graph)
	}{
		{"generic_interface_closure_wrapper", callerConversionFixture},
		{"scope_crossings", scopedConversionFixture},
	}
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			s, cg := fixture.build(t)
			full := s.callSites(cg)
			for _, root := range []string{"/repo", "/repo/", "/repo/../repo", "/repo-other", "relative-root", ""} {
				absRoot := root
				if !filepath.IsAbs(absRoot) {
					if abs, err := filepath.Abs(absRoot); err == nil {
						absRoot = abs
					}
				}
				prefix := absRoot + string(filepath.Separator)
				keep := func(file string) bool {
					if !filepath.IsAbs(file) {
						file = filepath.Join(root, file)
					}
					return strings.HasPrefix(file, prefix)
				}
				got := normalizeFilteredCalls(t, s.callSitesFiltered(cg, keep))
				want := normalizeFilteredCalls(t, postFilterCalls(full, keep))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("root %q: got %v want %v", root, got, want)
				}
			}
			if got, want := normalizeFilteredCalls(t, s.callSitesFiltered(cg, nil)), normalizeFilteredCalls(t, full); !reflect.DeepEqual(got, want) {
				t.Fatalf("nil filter changed calls")
			}
		})
	}
}

func TestCallSitesFilteredUsesCallSiteNotDeclaration(t *testing.T) {
	s, cg := scopedConversionFixture(t)
	keep := func(file string) bool {
		if !filepath.IsAbs(file) {
			file = filepath.Join("/repo", file)
		}
		return strings.HasPrefix(file, "/repo/")
	}
	full := s.callSites(cg)
	paths := map[string]bool{}
	for _, call := range full {
		paths[call.File] = true
	}
	for _, path := range []string{"/repo/main.go", "/dependency/outside.go", "relative.go", "/repo-other/collision.go", ""} {
		if !paths[path] {
			t.Fatalf("fixture missing %q: %v", path, paths)
		}
	}
	calls := s.callSitesFiltered(cg, keep)
	externalCallees, relativeCalls := 0, 0
	for _, call := range calls {
		if call.File == "/repo/main.go" && call.Callee.File == "/dependency/outside.go" {
			externalCallees++
		}
		if call.File == "relative.go" {
			relativeCalls++
		}
		if !keep(call.File) {
			t.Fatalf("excluded caller retained: %+v", call)
		}
	}
	if externalCallees != 2 || relativeCalls != 1 {
		t.Fatalf("lost edge multiplicity or external/relative calls: external=%d relative=%d", externalCallees, relativeCalls)
	}
	// Also exercise the public entrypoint: no graph or RTA scope is pruned.
	all, err := s.CallSitesForDir("/repo")
	if err != nil {
		t.Fatal(err)
	}
	filtered, err := s.CallSitesForDirFiltered("/repo", keep)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalizeFilteredCalls(t, filtered), normalizeFilteredCalls(t, postFilterCalls(all, keep))) {
		t.Fatal("public filtered result differs from post-filter")
	}
}

var filteredCallCountSink int

func BenchmarkCallSiteFiltering(b *testing.B) {
	s, cg := scopedConversionFixture(b)
	keep := func(file string) bool {
		if !filepath.IsAbs(file) {
			file = filepath.Join("/repo", file)
		}
		return strings.HasPrefix(file, "/repo/")
	}
	for _, tc := range []struct {
		name string
		run  func() []*model.CallSite
	}{
		{"late", func() []*model.CallSite { return s.callSites(cg) }},
		{"early", func() []*model.CallSite { return s.callSitesFiltered(cg, keep) }},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				calls := tc.run()
				// The consumer retains its late guard in both paths.
				count := 0
				for _, call := range calls {
					if keep(call.File) {
						count++
					}
				}
				filteredCallCountSink = count
				callerConversionSink = calls
			}
		})
	}
}

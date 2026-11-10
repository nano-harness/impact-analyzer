package godeep

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Build real, instantiated SSA without packages.Load or external dependencies.
func rtaRecoveryFixture(t *testing.T) (*Analyzer, *Session, string) {
	t.Helper()
	dir := t.TempDir()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(dir, "main.go"), `package main

type Reader[T any] interface { Read() T }
type Box[T any] struct { Value T }
func identity[T any](v T) T { return v }
func (b Box[T]) Read() T { return identity(b.Value) }
func consume(v int) {}
func main() {
	var reader Reader[int] = Box[int]{Value: 42}
	consume(reader.Read())
}
`, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := ssautil.BuildPackage(&types.Config{}, fset,
		types.NewPackage("example.com/rtarecovery", "main"), []*ast.File{file}, ssa.InstantiateGenerics)
	if err != nil {
		t.Fatal(err)
	}
	a := &Analyzer{Prog: pkg.Prog, mains: []*ssa.Package{pkg}}
	s := &Session{
		analyzer:     a,
		initialByDir: map[string][]*packages.Package{dir: {{Types: pkg.Pkg}}},
		closures:     make(map[string][]*ssa.Package),
	}
	return a, s, dir
}

func TestRTARecovery_InjectedFailure(t *testing.T) {
	a, _, _ := rtaRecoveryFixture(t)
	root := a.mains[0].Func("init")
	// Deliberately corrupt root metadata to provoke an RTA panic. This tests
	// the recovery boundary, not reproduction of the historical TypeParam bug.
	root.Prog = nil
	roots := []*ssa.Function{root}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("invalid fixture must panic in unwrapped rta.Analyze")
			}
		}()
		rta.Analyze(roots, true)
	}()
	result, err := analyzeRTA(roots)
	if result != nil || err == nil || !strings.Contains(err.Error(), "RTA analysis panicked:") {
		t.Fatalf("analyzeRTA = (%v, %v), want nil result and recovered RTA error", result, err)
	}
}

func TestRTARecovery_EntryPaths(t *testing.T) {
	for _, path := range []string{"standalone", "shared-session"} {
		t.Run(path, func(t *testing.T) {
			a, s, dir := rtaRecoveryFixture(t)
			root := a.mains[0].Func("init")
			prog := root.Prog
			root.Prog = nil // Inject a fault in the first default root.
			// No custom entrypoints: exercise both default discovery paths.
			if path == "standalone" {
				graph, err := a.BuildCallGraph()
				if graph != nil || err == nil || !strings.Contains(err.Error(), "RTA analysis panicked:") {
					t.Fatalf("BuildCallGraph = (%v, %v), want nil graph and recovered RTA error", graph, err)
				}
			} else {
				calls, err := s.CallSitesForDir(dir)
				if calls != nil || err == nil || !strings.Contains(err.Error(), "RTA analysis panicked:") {
					t.Fatalf("CallSitesForDir = (%v, %v), want nil calls and recovered RTA error", calls, err)
				}
			}

			// Restoring the injected fault must leave valid generic interface
			// dispatch working through the same public entrypoint.
			root.Prog = prog
			reachable := make(map[string]bool)
			if path == "standalone" {
				graph, err := a.BuildCallGraph()
				if err != nil || graph == nil {
					t.Fatalf("valid BuildCallGraph = (%v, %v)", graph, err)
				}
				for fn := range graph.Nodes {
					if fn != nil {
						reachable[strings.SplitN(fn.Name(), "[", 2)[0]] = true
					}
				}
			} else {
				calls, err := s.CallSitesForDir(dir)
				if err != nil {
					t.Fatalf("valid CallSitesForDir: %v", err)
				}
				for _, call := range calls {
					if call.Callee != nil {
						reachable[call.Callee.Name] = true
					}
				}
			}
			for _, name := range []string{"Read", "identity", "consume"} {
				if !reachable[name] {
					t.Errorf("valid generic interface graph lost %s", name)
				}
			}
		})
	}
}

func TestRTARecovery_EmptyRoots(t *testing.T) {
	for _, roots := range [][]*ssa.Function{nil, {}} {
		result, err := analyzeRTA(roots)
		if result != nil || err != nil {
			t.Fatalf("analyzeRTA(%v) = (%v, %v), want (nil, nil)", roots, result, err)
		}
	}
	a := &Analyzer{Prog: ssa.NewProgram(token.NewFileSet(), ssa.InstantiateGenerics)}
	graph, err := a.BuildCallGraph()
	if graph == nil || err != nil {
		t.Fatalf("empty BuildCallGraph = (%v, %v), want a non-nil empty graph", graph, err)
	}
	if symbols := a.ConvertToModel(graph); len(symbols) != 0 {
		t.Fatalf("empty graph produced symbols: %v", symbols)
	}
	if positions := a.ReachableFunctionPositions(graph); len(positions) != 0 {
		t.Fatalf("empty graph produced reachable positions: %v", positions)
	}
	a.MarkReflectionReachable(graph)
	if graph.Nodes == nil || len(graph.Nodes) != 0 || graph.Root != nil {
		t.Fatalf("empty graph should have no nodes or root: %v", graph.Nodes)
	}
	dir := t.TempDir()
	s := &Session{analyzer: a, initialByDir: map[string][]*packages.Package{dir: nil}, closures: make(map[string][]*ssa.Package)}
	if calls, err := s.CallSitesForDir(dir); calls != nil || err != nil {
		t.Fatalf("empty CallSitesForDir = (%v, %v), want (nil, nil)", calls, err)
	}
}

package godeep

import (
	"fmt"
	"go/types"
	"path/filepath"

	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Session holds a single SSA program covering every Go package under an
// analysis root, so per-directory RTA call graphs can be computed without
// paying a full LoadPackages + SSA build per directory. One load replaces the
// N loads that N analyzed directories would otherwise trigger.
//
// Root equivalence with the standalone path: CallSitesForDir(dir) computes
// exactly the RTA roots that Analyzer.BuildCallGraph computes after a
// standalone LoadPackages(dir):
//   - the init/main functions of the package(s) located in dir — note that
//     ssautil.AllPackages returns the SSA packages for the *initial* loaded
//     packages (Analyzer.mains is a misnomer inherited from ssautil), so the
//     standalone "main" roots are the dir package's own init/main; or
//   - when the dir's packages have neither init nor main, every non-method
//     function in the dir package's transitive import closure — the same set
//     a standalone Prog.Build would contain for that dir.
//
// RTA itself is a root-driven fixpoint that only visits functions reachable
// from the roots, so running it on the larger shared program yields the same
// edges as running it on a program built from dir alone.
//
// A Session is not safe for concurrent use: rta.Analyze lazily populates the
// program's method-set cache.
type Session struct {
	analyzer *Analyzer
	// initialByDir maps an absolute package directory to the initial
	// packages.Load packages located there.
	initialByDir map[string][]*packages.Package
	// closures memoizes each directory's transitive SSA package set.
	closures map[string][]*ssa.Package
}

// LoadSession loads every Go package under root (pattern root+"/...") in a
// single packages.Load call and builds one shared SSA program from it.
func LoadSession(root string, buildTags []string) (*Session, error) {
	analyzer := NewAnalyzer()
	if len(buildTags) > 0 {
		analyzer = analyzer.WithBuildTags(buildTags)
	}
	if err := analyzer.LoadPackages(root + "/..."); err != nil {
		return nil, err
	}
	if err := analyzer.BuildSSA(); err != nil {
		return nil, err
	}

	s := &Session{
		analyzer:     analyzer,
		initialByDir: make(map[string][]*packages.Package),
		closures:     make(map[string][]*ssa.Package),
	}
	for _, pkg := range analyzer.pkgs {
		dir := packageDir(pkg)
		if dir == "" {
			continue
		}
		s.initialByDir[dir] = append(s.initialByDir[dir], pkg)
	}
	return s, nil
}

// PackageCount returns the number of initially loaded packages (those under
// the session root; dependencies are not counted).
func (s *Session) PackageCount() int {
	return len(s.analyzer.pkgs)
}

// CoversDir reports whether dir contains a package that was loaded into the
// shared session. Dirs outside the root, excluded by build constraints, or
// inside nested modules are not covered; callers should fall back to a
// standalone per-directory analysis for them.
func (s *Session) CoversDir(dir string) bool {
	_, ok := s.initialByDir[filepath.Clean(dir)]
	return ok
}

// CallSitesForDir runs RTA over the shared program with roots equivalent to a
// standalone analysis of dir and converts the resulting edges to call sites,
// mirroring the standalone BuildPackageCallGraph conversion. It returns an
// error for directories not covered by the session.
func (s *Session) CallSitesForDir(dir string, customEntryPoints ...string) ([]*model.CallSite, error) {
	roots, err := s.rootsForDir(dir, customEntryPoints)
	if err != nil {
		return nil, err
	}
	if len(roots) == 0 {
		// rta.Analyze returns nil for empty roots; the standalone path would
		// hand back an empty (nil) graph as well.
		return nil, nil
	}
	result, err := analyzeRTA(roots)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return s.callSites(result.CallGraph), nil
}

// FindInterfaceImplementations reuses the session's loaded packages to find
// named types implementing the named interface. The session loads the same
// root+"/..." pattern as the standalone repo analyzer, so results match.
func (s *Session) FindInterfaceImplementations(ifaceName string) []*model.Symbol {
	return s.analyzer.FindInterfaceImplementations(ifaceName)
}

// rootsForDir computes the RTA roots for dir exactly as Analyzer.BuildCallGraph
// would after a standalone LoadPackages(dir).
func (s *Session) rootsForDir(dir string, customEntryPoints []string) ([]*ssa.Function, error) {
	initial, ok := s.initialByDir[filepath.Clean(dir)]
	if !ok {
		return nil, fmt.Errorf("directory %s is not covered by the shared session", dir)
	}

	var roots []*ssa.Function

	// Custom entry points (#21), resolved over the shared program with the
	// same lookup the standalone analyzer uses.
	for _, qname := range customEntryPoints {
		if fn := s.analyzer.findFunctionByQualifiedName(qname); fn != nil {
			roots = append(roots, fn)
		}
	}

	// init/main of the dir's own packages (the standalone "mains" roots).
	if len(roots) == 0 {
		for _, pkg := range initial {
			spkg := s.analyzer.Prog.Package(pkg.Types)
			if spkg == nil {
				continue
			}
			if fn := spkg.Func("init"); fn != nil {
				roots = append(roots, fn)
			}
			if fn := spkg.Func("main"); fn != nil {
				roots = append(roots, fn)
			}
		}
	}

	// No init/main: every non-method function in the transitive import
	// closure — what Prog.AllPackages contains in a standalone load of dir.
	if len(roots) == 0 {
		for _, spkg := range s.closureForDir(dir, initial) {
			for _, mem := range spkg.Members {
				if fn, ok := mem.(*ssa.Function); ok {
					if fn.Signature.Recv() == nil { // Skip methods
						roots = append(roots, fn)
					}
				}
			}
		}
	}

	return roots, nil
}

// closureForDir returns the SSA packages in the transitive import closure of
// dir's initial packages — the set ssautil.AllPackages would create for a
// standalone load of dir.
func (s *Session) closureForDir(dir string, initial []*packages.Package) []*ssa.Package {
	if cached, ok := s.closures[dir]; ok {
		return cached
	}

	seen := make(map[*packages.Package]bool)
	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if p == nil || seen[p] {
			return
		}
		seen[p] = true
		for _, imp := range p.Imports {
			visit(imp)
		}
	}
	for _, p := range initial {
		visit(p)
	}

	closure := make([]*ssa.Package, 0, len(seen))
	for p := range seen {
		if p.Types == nil || p.IllTyped {
			continue // mirrors ssautil.AllPackages
		}
		if spkg := s.analyzer.Prog.Package(p.Types); spkg != nil {
			closure = append(closure, spkg)
		}
	}
	s.closures[dir] = closure
	return closure
}

// callSites converts RTA call graph edges to call sites, mirroring the
// conversion in the standalone BuildPackageCallGraph path: only the callee is
// attached; the caller symbol is computed solely to drop edges whose caller
// has no package.
func (s *Session) callSites(cg *callgraph.Graph) []*model.CallSite {
	var callSites []*model.CallSite
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

			// Get position info from the call site
			pos := s.analyzer.Prog.Fset.Position(edge.Pos())

			callSites = append(callSites, &model.CallSite{
				File:     pos.Filename,
				Line:     pos.Line,
				Column:   pos.Column,
				EdgeKind: model.EdgeKindCall,
				Resolved: true,
				Callee:   callee,
			})
		}
	}
	return callSites
}

// packageDir returns the directory containing the package's compiled files.
func packageDir(pkg *packages.Package) string {
	files := pkg.CompiledGoFiles
	if len(files) == 0 {
		files = pkg.GoFiles
	}
	if len(files) == 0 {
		return ""
	}
	return filepath.Clean(filepath.Dir(files[0]))
}

// ssaFunctionToSymbol converts an SSA function to our Symbol model (#21).
// Must set Owner field for consistency with AST-based symbols. Kept in sync
// with the conversion in the standalone BuildPackageCallGraph path.
func ssaFunctionToSymbol(prog *ssa.Program, fn *ssa.Function) *model.Symbol {
	if fn == nil {
		return nil
	}

	pkg := fn.Pkg
	if pkg == nil {
		return nil
	}

	pos := prog.Fset.Position(fn.Pos())

	kind := model.SymbolKindFunction
	var owner string
	if recv := fn.Signature.Recv(); recv != nil {
		kind = model.SymbolKindMethod
		// Extract receiver type for Owner field (#15 consistency)
		recvType := recv.Type()
		if ptr, ok := recvType.(*types.Pointer); ok {
			recvType = ptr.Elem()
		}
		if named, ok := recvType.(*types.Named); ok {
			owner = named.Obj().Name()
		}
	}

	isExported := fn.Object() != nil && fn.Object().Exported()

	return &model.Symbol{
		Name:       fn.Name(),
		Kind:       kind,
		File:       pos.Filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Language:   model.LanguageGo,
		Package:    pkg.Pkg.Path(),
		Owner:      owner, // #15: Set Owner for method symbols
		IsExported: isExported,
	}
}

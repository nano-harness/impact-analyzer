package godeep

import (
	"fmt"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Analyzer provides deep Go analysis using go/types and RTA callgraph
type Analyzer struct {
	pkgs      []*packages.Package
	Prog      *ssa.Program // Exported for use by extractors
	mains     []*ssa.Package
	buildTags []string // Build tags for package loading
}

// NewAnalyzer creates a new Go deep analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// LoadPackages loads Go packages for analysis
func (a *Analyzer) LoadPackages(patterns ...string) error {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo,
		// Set Dir to the first absolute pattern's directory to ensure
		// packages.Load resolves go.mod correctly. Without this, relative
		// patterns like "./xxx" or "xxx" may be interpreted as import paths
		// depending on the process cwd, causing partial package loading.
		Dir: deriveModuleDir(patterns),
	}

	// Apply build tags if configured
	if len(a.buildTags) > 0 {
		cfg.BuildFlags = []string{"-tags=" + joinTags(a.buildTags)}
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return fmt.Errorf("failed to load packages: %w", err)
	}

	// Tolerate errors in parts of the import graph we can still analyze —
	// a dependency that fails to type-check (e.g. it requires a newer Go
	// toolchain than the one running this build) must not abort SSA for
	// packages that loaded fine. Only fail when no root package produced
	// usable syntax, and surface the first real error message instead of an
	// opaque "errors loading packages".
	if !anyRootUsable(pkgs) {
		if msg := firstPackageError(pkgs); msg != "" {
			return fmt.Errorf("errors loading packages: %s", msg)
		}
		return fmt.Errorf("errors loading packages")
	}

	a.pkgs = pkgs
	return nil
}

// anyRootUsable reports whether at least one root package is well-typed —
// the same criterion ssautil uses to decide which packages get SSA code —
// meaning SSA construction has something to build.
func anyRootUsable(pkgs []*packages.Package) bool {
	for _, pkg := range pkgs {
		if pkg.Types != nil && !pkg.IllTyped {
			return true
		}
	}
	return false
}

// firstPackageError returns the first package or module error message across
// the import graph rooted at pkgs, for diagnostics when loading fails
// outright. Unlike packages.PrintErrors it never touches os.Stderr.
func firstPackageError(pkgs []*packages.Package) string {
	for pkg := range packages.Postorder(pkgs) {
		if len(pkg.Errors) > 0 {
			return pkg.Errors[0].Msg
		}
		if mod := pkg.Module; mod != nil && mod.Error != nil {
			return mod.Error.Err
		}
	}
	return ""
}

// BuildSSA builds SSA representation of loaded packages
func (a *Analyzer) BuildSSA() (err error) {
	if len(a.pkgs) == 0 {
		return fmt.Errorf("no packages loaded")
	}

	// Recover creation failures on this goroutine so callers can fall back to
	// shallower analysis. Package builds remain parallel; panics in x/tools
	// package-builder workers cannot be caught by this defer.
	defer func() {
		if r := recover(); r != nil {
			a.Prog, a.mains = nil, nil
			err = fmt.Errorf("SSA construction panicked: %v", r)
		}
	}()
	a.Prog, a.mains = ssautil.AllPackages(a.pkgs, ssa.InstantiateGenerics)
	a.Prog.Build()

	return nil
}

// analyzeRTA converts RTA failures into an error so callers can fall back to
// shallower analysis. It does not change the parallel SSA construction mode.
func analyzeRTA(roots []*ssa.Function) (result *rta.Result, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("RTA analysis panicked: %v", r)
		}
	}()
	return rta.Analyze(roots, true), nil
}

// BuildCallGraph builds a type-precise call graph using RTA algorithm
// customEntryPoints: optional list of qualified function names to use as entry points (#21)
func (a *Analyzer) BuildCallGraph(customEntryPoints ...string) (*callgraph.Graph, error) {
	if a.Prog == nil {
		return nil, fmt.Errorf("SSA program not built")
	}

	// Find entry points (main packages and init functions)
	var roots []*ssa.Function

	// If custom entry points are specified, use them (#21)
	if len(customEntryPoints) > 0 {
		for _, qname := range customEntryPoints {
			fn := a.findFunctionByQualifiedName(qname)
			if fn != nil {
				roots = append(roots, fn)
			}
		}
	}

	// Default entry point discovery if no custom ones or custom ones not found
	if len(roots) == 0 {
		for _, pkg := range a.mains {
			if pkg != nil {
				if pkg.Func("init") != nil {
					roots = append(roots, pkg.Func("init"))
				}
				if pkg.Func("main") != nil {
					roots = append(roots, pkg.Func("main"))
				}
			}
		}
	}

	// If no main packages, use all packages
	if len(roots) == 0 {
		for _, pkg := range a.Prog.AllPackages() {
			if pkg == nil {
				continue
			}
			// Add all exported functions as potential roots
			for _, mem := range pkg.Members {
				if fn, ok := mem.(*ssa.Function); ok {
					if fn.Signature.Recv() == nil { // Skip methods
						roots = append(roots, fn)
					}
				}
			}
		}
	}

	// Build RTA call graph
	result, err := analyzeRTA(roots)
	if err != nil {
		return nil, err
	}
	if result == nil {
		// Successful graph consumers expect a non-nil graph, even without roots.
		return &callgraph.Graph{Nodes: make(map[*ssa.Function]*callgraph.Node)}, nil
	}
	return result.CallGraph, nil
}

// ConvertToModel converts RTA callgraph to our model format
func (a *Analyzer) ConvertToModel(cg *callgraph.Graph) []*model.Symbol {
	symbols := make([]*model.Symbol, 0, len(cg.Nodes))
	seen := make(map[string]bool, len(cg.Nodes))

	for fn := range cg.Nodes {
		if fn == nil {
			continue
		}

		sym := a.functionToSymbol(fn)
		if sym != nil {
			key := sym.QualifiedName()
			if !seen[key] {
				seen[key] = true
				symbols = append(symbols, sym)
			}
		}
	}

	return symbols
}

// functionToSymbol converts an SSA function to our Symbol model
func (a *Analyzer) functionToSymbol(fn *ssa.Function) *model.Symbol {
	fn = SourceFunction(fn)
	if fn == nil {
		return nil
	}

	pkg := fn.Pkg
	if pkg == nil {
		return nil
	}

	pos := fn.Prog.Fset.Position(fn.Pos())

	kind := model.SymbolKindFunction
	var owner string
	if fn.Signature.Recv() != nil {
		kind = model.SymbolKindMethod
		// Derive Owner from receiver type to align with tree-sitter
		owner = deriveReceiverTypeName(fn.Signature.Recv().Type())
	}

	isExported := fn.Object() != nil && fn.Object().Exported()

	return &model.Symbol{
		Name:       fn.Name(),
		Owner:      owner,
		Kind:       kind,
		File:       pos.Filename,
		Line:       pos.Line,
		Column:     pos.Column,
		Language:   model.LanguageGo,
		Package:    pkg.Pkg.Path(),
		IsExported: isExported,
	}
}

// ExtractCallEdges extracts call edges from RTA callgraph
func (a *Analyzer) ExtractCallEdges(cg *callgraph.Graph) map[string][]string {
	edges := make(map[string][]string)

	for fn, node := range cg.Nodes {
		if fn == nil {
			continue
		}

		caller := a.functionToSymbol(fn)
		if caller == nil {
			continue
		}

		callerKey := caller.QualifiedName()

		for _, edge := range node.Out {
			if edge.Callee == nil || edge.Callee.Func == nil {
				continue
			}

			callee := a.functionToSymbol(edge.Callee.Func)
			if callee == nil {
				continue
			}

			calleeKey := callee.QualifiedName()
			edges[callerKey] = append(edges[callerKey], calleeKey)
		}
	}

	return edges
}

// FindImplementations finds all types that implement an interface
func (a *Analyzer) FindImplementations(iface *types.Interface) []types.Type {
	var impls []types.Type

	for _, pkg := range a.pkgs {
		if pkg.Types == nil {
			continue
		}

		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			if named, ok := obj.Type().(*types.Named); ok {
				if types.Implements(named, iface) {
					impls = append(impls, named)
				}
				// Also check pointer type
				ptr := types.NewPointer(named)
				if types.Implements(ptr, iface) {
					impls = append(impls, ptr)
				}
			}
		}
	}

	return impls
}

// FindInterfaceImplementations finds all named types in the loaded packages
// that implement the interface named ifaceName and returns them as symbols.
// Requires LoadPackages; SSA does not need to be built.
func (a *Analyzer) FindInterfaceImplementations(ifaceName string) []*model.Symbol {
	iface, ifaceNamed := a.lookupInterface(ifaceName)
	if iface == nil {
		return nil
	}

	var fset *token.FileSet
	if len(a.pkgs) > 0 {
		fset = a.pkgs[0].Fset
	}

	var result []*model.Symbol
	seen := make(map[string]bool)
	for _, t := range a.FindImplementations(iface) {
		named := t
		if ptr, ok := t.(*types.Pointer); ok {
			named = ptr.Elem()
		}
		n, ok := named.(*types.Named)
		if !ok || n.Obj() == nil {
			continue
		}
		// Skip the interface itself (an interface implements itself)
		if ifaceNamed != nil && types.Identical(n, ifaceNamed) {
			continue
		}
		obj := n.Obj()
		sym := &model.Symbol{
			Name:       obj.Name(),
			Kind:       model.SymbolKindStruct,
			Language:   model.LanguageGo,
			IsExported: obj.Exported(),
		}
		if obj.Pkg() != nil {
			sym.Package = obj.Pkg().Name()
		}
		if fset != nil {
			pos := fset.Position(obj.Pos())
			sym.File = pos.Filename
			sym.Line = pos.Line
			sym.Column = pos.Column
		}
		// Dedupe: value and pointer forms of the same type collapse here
		key := sym.QualifiedName()
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, sym)
	}
	return result
}

// lookupInterface finds a named interface type by name in the loaded packages.
func (a *Analyzer) lookupInterface(ifaceName string) (*types.Interface, *types.Named) {
	for _, pkg := range a.pkgs {
		if pkg.Types == nil {
			continue
		}
		obj := pkg.Types.Scope().Lookup(ifaceName)
		if obj == nil {
			continue
		}
		named, ok := obj.Type().(*types.Named)
		if !ok {
			continue
		}
		if iface, ok := named.Underlying().(*types.Interface); ok {
			return iface, named
		}
	}
	return nil, nil
}

// findFunctionByQualifiedName finds an SSA function by qualified name (#21)
// Format: "package/path.FunctionName" or "package/path.(*ReceiverType).MethodName"
func (a *Analyzer) findFunctionByQualifiedName(qname string) *ssa.Function {
	if a.Prog == nil {
		return nil
	}

	// Try to find in all packages
	for _, pkg := range a.Prog.AllPackages() {
		if pkg == nil {
			continue
		}

		// Check package members for functions
		for _, mem := range pkg.Members {
			fn, ok := mem.(*ssa.Function)
			if !ok {
				continue
			}

			// Build qualified name for comparison
			fnQName := pkg.Pkg.Path() + "." + fn.Name()
			if fn.Signature.Recv() != nil {
				// For methods, include receiver type
				recvType := fn.Signature.Recv().Type().String()
				fnQName = pkg.Pkg.Path() + "." + recvType + "." + fn.Name()
			}

			if fnQName == qname || fn.Name() == qname || fn.String() == qname {
				return fn
			}
		}

		// Also check for methods on types
		for _, mem := range pkg.Members {
			if tp, ok := mem.(*ssa.Type); ok {
				mset := a.Prog.MethodSets.MethodSet(tp.Type())
				for i := 0; i < mset.Len(); i++ {
					method := a.Prog.MethodValue(mset.At(i))
					if method != nil {
						methodQName := pkg.Pkg.Path() + "." + method.Name()
						if methodQName == qname || method.Name() == qname {
							return method
						}
					}
				}
			}
		}
	}

	return nil
}

// deriveModuleDir derives a stable module directory from patterns.
// If any pattern is an absolute path that refers to an existing directory,
// returns it. Otherwise returns "" (let packages.Load use process cwd).
func deriveModuleDir(patterns []string) string {
	for _, p := range patterns {
		// Trim a trailing "/..." wildcard so Stat can resolve the base
		// directory; absolute "dir/..." patterns should resolve to "dir".
		p = strings.TrimSuffix(p, "/...")
		if filepath.IsAbs(p) {
			// If pattern is an existing directory, use it directly.
			// Otherwise use its parent directory (might be a file pattern).
			if info, err := os.Stat(p); err == nil {
				if info.IsDir() {
					return p
				}
				return filepath.Dir(p)
			}
		}
	}
	return ""
}

// deriveReceiverTypeName extracts the receiver type name from a types.Type.
// Aligns with tree-sitter's stripGoReceiverType logic: removes pointer and generic parameters.
// Examples: *Server -> Server, Server[T] -> Server
func deriveReceiverTypeName(t types.Type) string {
	// Strip pointer
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	// Get the type name
	var typeName string
	switch typ := t.(type) {
	case *types.Named:
		typeName = typ.Obj().Name()
	default:
		typeName = t.String()
	}

	// Remove generic type parameters if present
	if idx := strings.Index(typeName, "["); idx != -1 {
		typeName = typeName[:idx]
	}

	return typeName
}

// joinTags joins build tags with commas for -tags flag
func joinTags(tags []string) string {
	return strings.Join(tags, ",")
}

// Position represents a file position for reachability tracking
type Position struct {
	File string // Absolute path
	Line int    // 1-based line number
	Name string // Function/method name to disambiguate same-line declarations
}

// ReachableFunctionPositions returns file:line positions of all reachable functions
func (a *Analyzer) ReachableFunctionPositions(cg *callgraph.Graph) map[Position]bool {
	positions := make(map[Position]bool)

	for fn := range cg.Nodes {
		fn = SourceFunction(fn)
		if fn == nil {
			continue
		}

		pos := a.Prog.Fset.Position(fn.Pos())
		if pos.Filename != "" && pos.Line > 0 {
			positions[Position{File: pos.Filename, Line: pos.Line, Name: fn.Name()}] = true
		}
	}

	return positions
}

// WithBuildTags sets build tags for package loading and returns the analyzer for chaining
func (a *Analyzer) WithBuildTags(tags []string) *Analyzer {
	a.buildTags = tags
	return a
}

// MarkReflectionReachable applies a conservative reflection model: if reflect.Value.Call
// or reflect.Value.CallSlice is reachable in the call graph, all address-taken functions
// are marked as reachable. This prevents false positives for functions called via reflection.
func (a *Analyzer) MarkReflectionReachable(cg *callgraph.Graph) {
	if cg == nil || a.Prog == nil {
		return
	}

	// Check if reflect.Value.Call or CallSlice is reachable
	reflectCallReachable := false
	for fn := range cg.Nodes {
		if fn != nil && fn.Package() != nil &&
			fn.Package().Pkg.Path() == "reflect" &&
			(fn.Name() == "Call" || fn.Name() == "CallSlice") {
			reflectCallReachable = true
			break
		}
	}
	if !reflectCallReachable {
		return
	}

	// Mark all address-taken functions as reachable
	for fn := range ssautil.AllFunctions(a.Prog) {
		if fn == nil {
			continue
		}
		// A function is address-taken if it has referrers (is used as a value)
		if fn.Referrers() != nil {
			if _, exists := cg.Nodes[fn]; !exists {
				cg.Nodes[fn] = &callgraph.Node{Func: fn}
			}
		}
	}
}

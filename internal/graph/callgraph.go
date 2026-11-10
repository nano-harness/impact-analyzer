package graph

import (
	"go/token"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/nano-harness/impact-analyzer/internal/model"
)

// CallGraph represents a directed graph of function/method calls
type CallGraph struct {
	// edges maps callee qualified name -> []callers (reverse graph for impact analysis)
	edges map[string][]*model.Symbol
	// edgesByKind maps edge kind -> callee qualified name -> []callers
	edgesByKind map[model.EdgeKind]map[string][]*model.Symbol
	// edgesByName maps callee bare name -> []nameEdge (fallback for unqualified callees)
	edgesByName map[string][]nameEdge
	// edgesByNameByKind maps edge kind -> callee bare name -> []nameEdge
	edgesByNameByKind map[model.EdgeKind]map[string][]nameEdge
	// forwardEdges maps caller qualified name -> []callees
	forwardEdges map[string][]*model.Symbol
	// forwardEdgesByKind maps edge kind -> caller qualified name -> []callees
	forwardEdgesByKind map[model.EdgeKind]map[string][]*model.Symbol
	// symbols maps qualified name -> symbol
	symbols map[string]*model.Symbol
	// symbolsByName maps bare names to symbols for unqualified call resolution
	symbolsByName map[string][]*model.Symbol
	// symbolsByFile maps file -> bare name -> symbols declared in that file.
	// Unlike symbolsByName it is never deduplicated by qualified name, so
	// same-named declarations in different files (whose qualified names
	// collide when they carry no package) all remain visible; the
	// local-shadow rule in isLocallyShadowed relies on this.
	symbolsByFile map[string]map[string][]*model.Symbol
	// permissiveFallback controls whether bare-name fallback allows cross-owner matches (#15)
	permissiveFallback bool
	// fileImportsFunc optionally returns the import paths declared by a file.
	// When set, cross-package bare-name callers are kept if the caller's file
	// imports the target package. Nil preserves the previous drop behavior.
	fileImportsFunc func(file string) []string
	// finalized is true after Finalize; edge slices are then already sorted
	// and deduplicated, so query paths can skip re-sorting. Any mutation
	// (AddEdge/AddSymbol) clears it.
	finalized bool
}

// nameEdge is a bare-name fallback index entry: the caller symbol plus the
// best-effort receiver owner hint captured at the call site (callee.Owner,
// e.g. "userService" for userService.find()). The hint is empty when the
// receiver was not a simple identifier/field-access chain or was this/self.
type nameEdge struct {
	caller      *model.Symbol
	calleeOwner string
	// Go has no implicit receiver: an unqualified call cannot name a method.
	goBareCall bool
	// A source-backed Go method is known, unlike an AST selector that may
	// actually address an imported package (including an alias).
	goMethodCall bool
}

// NewCallGraph creates a new call graph
func NewCallGraph() *CallGraph {
	return &CallGraph{
		edges:              make(map[string][]*model.Symbol),
		edgesByKind:        make(map[model.EdgeKind]map[string][]*model.Symbol),
		edgesByName:        make(map[string][]nameEdge),
		edgesByNameByKind:  make(map[model.EdgeKind]map[string][]nameEdge),
		forwardEdges:       make(map[string][]*model.Symbol),
		forwardEdgesByKind: make(map[model.EdgeKind]map[string][]*model.Symbol),
		symbols:            make(map[string]*model.Symbol),
		symbolsByName:      make(map[string][]*model.Symbol),
		symbolsByFile:      make(map[string]map[string][]*model.Symbol),
		permissiveFallback: true, // Default to true for backward compatibility
	}
}

// SetPermissiveFallback sets the permissive fallback flag (#15)
func (cg *CallGraph) SetPermissiveFallback(permissive bool) {
	cg.permissiveFallback = permissive
}

// SetFileImportsFunc sets the optional callback used to query a file's imports.
// It enables cross-package bare-name caller recovery in filterCallersByContext;
// when nil (the default), that filter behaves exactly as before.
func (cg *CallGraph) SetFileImportsFunc(fn func(file string) []string) {
	cg.fileImportsFunc = fn
}

// AddSymbol registers a symbol in the graph without creating edges
func (cg *CallGraph) AddSymbol(symbol *model.Symbol) {
	if symbol == nil {
		return
	}
	// A package-enriched call reference is not a declaration. Do not let
	// its missing source metadata replace an already registered definition.
	if existing := cg.symbols[symbol.QualifiedName()]; existing != nil && existing.File != "" && symbol.File == "" {
		return
	}
	cg.finalized = false
	cg.symbols[symbol.QualifiedName()] = symbol
	cg.symbolsByName[symbol.Name] = append(cg.symbolsByName[symbol.Name], symbol)
	if symbol.File != "" {
		byName := cg.symbolsByFile[symbol.File]
		if byName == nil {
			byName = make(map[string][]*model.Symbol)
			cg.symbolsByFile[symbol.File] = byName
		}
		byName[symbol.Name] = append(byName[symbol.Name], symbol)
	}
}

// AddEdge adds a call edge (caller calls callee)
func (cg *CallGraph) AddEdge(caller, callee *model.Symbol, kinds ...model.EdgeKind) {
	kind := model.EdgeKindCall
	if len(kinds) > 0 && kinds[0] != "" {
		kind = kinds[0]
	}
	cg.finalized = false
	// Index by qualified name (primary index)
	calleeQName := callee.QualifiedName()
	cg.edges[calleeQName] = append(cg.edges[calleeQName], caller)
	if cg.edgesByKind[kind] == nil {
		cg.edgesByKind[kind] = make(map[string][]*model.Symbol)
	}
	cg.edgesByKind[kind][calleeQName] = append(cg.edgesByKind[kind][calleeQName], caller)

	// Also index by bare name (fallback for callees extracted without package info).
	// callee.Owner, when present, is the call-site receiver hint (best-effort,
	// variable/field name) and travels with the entry so bare-name lookups can
	// narrow candidates by owner; see filterCallersByContext.
	entry := nameEdge{
		caller:       caller,
		calleeOwner:  callee.Owner,
		goBareCall:   callee.Language == model.LanguageGo && callee.Owner == "" && callee.Kind != model.SymbolKindMethod,
		goMethodCall: callee.Language == model.LanguageGo && callee.Kind == model.SymbolKindMethod && callee.Owner != "" && callee.File != "",
	}
	cg.edgesByName[callee.Name] = append(cg.edgesByName[callee.Name], entry)
	if cg.edgesByNameByKind[kind] == nil {
		cg.edgesByNameByKind[kind] = make(map[string][]nameEdge)
	}
	cg.edgesByNameByKind[kind][callee.Name] = append(cg.edgesByNameByKind[kind][callee.Name], entry)
	cg.forwardEdges[caller.QualifiedName()] = append(cg.forwardEdges[caller.QualifiedName()], callee)
	if cg.forwardEdgesByKind[kind] == nil {
		cg.forwardEdgesByKind[kind] = make(map[string][]*model.Symbol)
	}
	cg.forwardEdgesByKind[kind][caller.QualifiedName()] = append(cg.forwardEdgesByKind[kind][caller.QualifiedName()], callee)

	// Store symbols
	cg.AddSymbol(caller)
	if callee.File != "" || callee.Package != "" {
		cg.AddSymbol(callee)
	}
}

// GetSymbol returns a symbol by its qualified name if present
func (cg *CallGraph) GetSymbol(name string) (*model.Symbol, bool) {
	sym, ok := cg.symbols[name]
	return sym, ok
}

// Symbols returns all symbols registered in the graph
func (cg *CallGraph) Symbols() []*model.Symbol {
	result := make([]*model.Symbol, 0, len(cg.symbols))
	for _, sym := range cg.symbols {
		result = append(result, sym)
	}
	return result
}

// GetSymbolsByName returns all symbols with the given bare name
func (cg *CallGraph) GetSymbolsByName(name string) []*model.Symbol {
	return cg.symbolsByName[name]
}

// GetCallers returns all direct callers of a symbol
func (cg *CallGraph) GetCallers(symbol *model.Symbol) []*model.Symbol {
	qname := symbol.QualifiedName()

	var callers []*model.Symbol
	// Try exact match first (qualified name)
	if c, ok := cg.edges[qname]; ok && len(c) > 0 {
		callers = c
		// A package-less, owner-less target's "qualified" key is just its
		// bare name, so bare call sites from every file land in this bucket;
		// apply the local-shadow rule to drop callers whose bare call binds
		// to a same-named declaration in their own file.
		if symbol.Package == "" && symbol.Owner == "" {
			callers = cg.dropLocallyShadowed(callers, symbol)
		}
	} else if c, ok := cg.edgesByName[symbol.Name]; ok && len(c) > 0 {
		// Fallback: try bare name match (for callees extracted without package info)
		// Filter callers by context to avoid cross-package false matches
		callers = cg.filterCallersByContext(c, symbol)
	}

	// Sort to eliminate non-determinism from map iteration order
	// This ensures BFS traversal order is deterministic.
	// Skipped once finalized: Finalize already sorted these slices.
	if !cg.finalized {
		sort.Slice(callers, func(i, j int) bool {
			return callers[i].QualifiedName() < callers[j].QualifiedName()
		})
	}

	return callers
}

// GetCallersByKind returns all direct callers of a symbol connected by the given edge kind.
func (cg *CallGraph) GetCallersByKind(symbol *model.Symbol, kind model.EdgeKind) []*model.Symbol {
	if symbol == nil || kind == "" {
		return nil
	}
	qname := symbol.QualifiedName()

	var callers []*model.Symbol
	if byName := cg.edgesByKind[kind]; byName != nil {
		if c, ok := byName[qname]; ok && len(c) > 0 {
			callers = c
			// Same local-shadow rule as GetCallers: for a package-less,
			// owner-less target this bucket collects bare call sites from
			// every file.
			if symbol.Package == "" && symbol.Owner == "" {
				callers = cg.dropLocallyShadowed(callers, symbol)
			}
		}
	}
	if len(callers) == 0 {
		if byBareName := cg.edgesByNameByKind[kind]; byBareName != nil {
			if c, ok := byBareName[symbol.Name]; ok && len(c) > 0 {
				callers = cg.filterCallersByContext(c, symbol)
			}
		}
	}
	// Skipped once finalized: Finalize already sorted these slices.
	if !cg.finalized {
		sort.Slice(callers, func(i, j int) bool {
			return callers[i].QualifiedName() < callers[j].QualifiedName()
		})
	}
	return callers
}

// GetCallees returns all direct callees of a symbol.
func (cg *CallGraph) GetCallees(symbol *model.Symbol) []*model.Symbol {
	if symbol == nil {
		return nil
	}
	callees := cg.forwardEdges[symbol.QualifiedName()]
	result := make([]*model.Symbol, 0, len(callees))
	seen := make(map[string]bool, len(callees))
	for _, callee := range callees {
		for _, resolved := range cg.resolveCallee(callee, symbol) {
			qname := resolved.QualifiedName()
			if !seen[qname] {
				seen[qname] = true
				result = append(result, resolved)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].QualifiedName() < result[j].QualifiedName()
	})
	return result
}

// GetCalleesByKind returns all direct callees of a symbol connected by the given edge kind.
func (cg *CallGraph) GetCalleesByKind(symbol *model.Symbol, kind model.EdgeKind) []*model.Symbol {
	if symbol == nil || kind == "" {
		return nil
	}
	byKind := cg.forwardEdgesByKind[kind]
	if byKind == nil {
		return nil
	}
	callees := byKind[symbol.QualifiedName()]
	result := make([]*model.Symbol, 0, len(callees))
	seen := make(map[string]bool, len(callees))
	for _, callee := range callees {
		for _, resolved := range cg.resolveCallee(callee, symbol) {
			qname := resolved.QualifiedName()
			if !seen[qname] {
				seen[qname] = true
				result = append(result, resolved)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].QualifiedName() < result[j].QualifiedName()
	})
	return result
}

func (cg *CallGraph) resolveCallee(callee, caller *model.Symbol) []*model.Symbol {
	if callee == nil {
		return nil
	}
	if sym, ok := cg.symbols[callee.QualifiedName()]; ok && sym.File != "" {
		return []*model.Symbol{sym}
	}
	candidates := cg.symbolsByName[callee.Name]
	if callee.Language == model.LanguageGo && callee.Owner == "" && callee.Kind != model.SymbolKindMethod {
		// Go bare calls cannot resolve to receiver methods. Apply this to
		// both proximity resolution and the final permissive fallback.
		compatible := make([]*model.Symbol, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate != nil && candidate.Owner == "" && candidate.Kind != model.SymbolKindMethod {
				compatible = append(compatible, candidate)
			}
		}
		candidates = compatible
	}
	if len(candidates) == 0 {
		return []*model.Symbol{callee}
	}
	var filtered []*model.Symbol
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		// Receiver-hint veto: the call site addressed a receiver whose name
		// positively contradicts this candidate's owner (e.g. orderService.find
		// cannot resolve to UserService.find). Symmetric to the caller-side
		// hint handling in filterCallersByContext.
		if callee.Owner != "" && candidate.Owner != "" && !ownerHintMatches(callee.Owner, candidate.Owner) {
			continue
		}
		if callee.Package != "" && candidate.Package == callee.Package {
			filtered = append(filtered, candidate)
			continue
		}
		if caller != nil && (candidate.Package == caller.Package || candidate.File == caller.File) {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) > 0 {
		return filtered
	}
	if callee.Owner != "" {
		// Proximity rules matched nothing; do not fall back to candidates
		// the receiver hint positively contradicts.
		var hinted []*model.Symbol
		for _, candidate := range candidates {
			if candidate == nil {
				continue
			}
			if candidate.Owner != "" && !ownerHintMatches(callee.Owner, candidate.Owner) {
				continue
			}
			hinted = append(hinted, candidate)
		}
		return hinted
	}
	return candidates
}

// filterCallersByContext filters bare-name index entries to those likely to
// be calling the target symbol. This helps reduce false positives when
// matching by bare name alone.
func (cg *CallGraph) filterCallersByContext(entries []nameEdge, target *model.Symbol) []*model.Symbol {
	// If target has no package, keep all callers (can't filter by package),
	// except those vetoed by the local-shadow rule: a genuinely bare call
	// (no receiver hint) in a file that declares its own same-named symbol
	// binds locally, so it cannot reach a target declared in another file.
	if target.Package == "" {
		callers := make([]*model.Symbol, 0, len(entries))
		for _, e := range entries {
			if e.calleeOwner == "" && cg.isLocallyShadowed(e.caller, target) {
				continue
			}
			callers = append(callers, e.caller)
		}
		return callers
	}

	// #15: When target has Owner information, require Owner match for precision
	if target.Owner != "" {
		filtered := make([]*model.Symbol, 0, len(entries))
		for _, e := range entries {
			if target.Language == model.LanguageGo && e.goBareCall {
				continue
			}
			caller := e.caller
			// Same package AND same owner - most precise match
			if caller.Package == target.Package && caller.Owner == target.Owner {
				filtered = append(filtered, caller)
				continue
			}
			// Receiver-hint match: the call site addressed a receiver whose
			// name agrees with the target's owner (e.g. userService.find()
			// vs Owner "UserService"). Best-effort, so it only ever vouches
			// FOR a caller inside the same package/file; it cannot pull
			// callers across package boundaries.
			if e.calleeOwner != "" && ownerHintMatches(e.calleeOwner, target.Owner) &&
				(caller.Package == target.Package || caller.File == target.File) {
				filtered = append(filtered, caller)
				continue
			}
			// Same file - local call fallback
			if caller.File == target.File {
				filtered = append(filtered, caller)
				continue
			}
			// Cross-package: keep only when the caller's file imports the
			// target's package AND the call site carries no receiver hint
			// that positively contradicts the target owner. Mirrors the
			// import-evidence rule of the no-Owner branch below; without it
			// an owned target can never match a cross-package caller.
			if caller.Package != target.Package && cg.fileImportsFunc != nil && caller.File != "" &&
				importsMatchPackage(cg.fileImportsFunc(caller.File), target.Package) &&
				(e.calleeOwner == "" || ownerHintMatches(e.calleeOwner, target.Owner)) {
				filtered = append(filtered, caller)
			}
			// Same package but different owner - only allow in permissive
			// mode, and only when the call site carries no contradicting
			// receiver hint. A hint that fails ownerHintMatches is positive
			// evidence the call addressed a different owner, so it vetoes
			// the permissive fallback.
			if cg.permissiveFallback && caller.Package == target.Package && e.calleeOwner == "" {
				filtered = append(filtered, caller)
			}
		}
		return filtered
	}

	// Target has no Owner - use old logic (package + file matching)
	filtered := make([]*model.Symbol, 0, len(entries))
	for _, e := range entries {
		if target.Language == model.LanguageGo && e.goMethodCall {
			continue
		}
		caller := e.caller
		// Keep if same package (very likely to be correct)
		if caller.Package == target.Package {
			filtered = append(filtered, caller)
			continue
		}

		// Keep if same file (could be same package with different names)
		if caller.File == target.File {
			filtered = append(filtered, caller)
			continue
		}

		// Importing a Go package cannot make its unexported functions
		// callable from another package. Name is authoritative here because
		// synthetic symbols may not populate IsExported.
		if target.Language == model.LanguageGo && !token.IsExported(target.Name) {
			continue
		}

		// Cross-package bare-name match: keep the caller only when its file
		// imports the target package (evidence the bare call reaches it).
		// Without an import query callback, keep the previous drop behavior:
		// such matches are ambiguous and noisy. Exact cross-package calls are
		// still represented in the qualified-name index.
		if cg.fileImportsFunc != nil && caller.File != "" &&
			importsMatchPackage(cg.fileImportsFunc(caller.File), target.Package) {
			filtered = append(filtered, caller)
		}
	}

	return filtered
}

// dropLocallyShadowed removes callers whose bare-name call binds to a
// same-named declaration in their own file rather than to the target.
// The input slice is not mutated.
func (cg *CallGraph) dropLocallyShadowed(callers []*model.Symbol, target *model.Symbol) []*model.Symbol {
	filtered := make([]*model.Symbol, 0, len(callers))
	for _, caller := range callers {
		if cg.isLocallyShadowed(caller, target) {
			continue
		}
		filtered = append(filtered, caller)
	}
	return filtered
}

// isLocallyShadowed reports whether a bare-name call from caller's file must
// bind to a same-named declaration in that file instead of to target: the
// caller lives in a different file than the target, and the caller's file
// declares its own symbol with the target's bare name and a call-compatible
// kind. A target without file info cannot be distinguished from the local
// declaration itself, so the rule stays off in that case.
func (cg *CallGraph) isLocallyShadowed(caller, target *model.Symbol) bool {
	if caller == nil || target == nil {
		return false
	}
	if target.File == "" || caller.File == "" || caller.File == target.File {
		return false
	}
	for _, sym := range cg.symbolsByFile[caller.File][target.Name] {
		if sym != nil && bareCallKindCompatible(sym.Kind, target.Kind) {
			return true
		}
	}
	return false
}

// bareCallKindCompatible reports whether a local declaration of kind local
// can bind a bare-name call whose intended target has kind target. Same-kind
// declarations always shadow; when the target is a function/method (or its
// kind is unknown), function values held in variables or constants shadow too
// (e.g. `const greet = (n) => ...` in TS/JS).
func bareCallKindCompatible(local, target model.SymbolKind) bool {
	if local == target {
		return true
	}
	switch target {
	case "", model.SymbolKindFunction, model.SymbolKindMethod:
		switch local {
		case model.SymbolKindFunction, model.SymbolKindMethod, model.SymbolKindVariable, model.SymbolKindConstant:
			return true
		}
	}
	return false
}

// ownerHintMatches reports whether a call-site receiver hint (a variable or
// field name, e.g. "userService") is consistent with a definition-side owner
// (a class name, e.g. "UserService"). Best-effort: case-insensitive equality
// covers the common camelCase(variable) vs PascalCase(class) convention;
// anything else counts as no evidence. Resolving the variable's actual type
// is beyond tree-sitter extraction.
func ownerHintMatches(hint, owner string) bool {
	return hint != "" && owner != "" && strings.EqualFold(hint, owner)
}

// importsMatchPackage reports whether any import path references the given
// package: exact match, Go-style full path ending in the package name, or
// Java-style class import under the package (pkg.Class).
func importsMatchPackage(imports []string, pkg string) bool {
	if pkg == "" {
		return false
	}
	for _, imp := range imports {
		if imp == pkg ||
			strings.HasSuffix(imp, "/"+pkg) ||
			strings.HasPrefix(imp, pkg+".") {
			return true
		}
	}
	return false
}

// TraversalOptions controls caller BFS traversal behavior
type TraversalOptions struct {
	MaxDepth     int                      // Maximum depth, <0 means unlimited
	IsStop       func(*model.Symbol) bool // Optional; when true, stop traversing upward from this node
	CollectPaths bool                     // Whether to record shortest path from start to each hit node
}

// CallPath represents a calling path from one symbol to another
type CallPath struct {
	From  *model.Symbol   // Starting symbol
	To    *model.Symbol   // Ending symbol
	Chain []*model.Symbol // From -> ... -> To (including both endpoints)
	Depth int             // len(Chain) - 1
}

// TraversalResult contains the result of a caller traversal
type TraversalResult struct {
	Callers []*model.Symbol // All reached callers, including stop nodes themselves
	Paths   []*CallPath     // Only populated when CollectPaths=true, one per caller
}

// TraverseCallers is the unified caller BFS traversal entry point
// Semantics: BFS along caller direction, when IsStop returns true for a node,
// that node is added to Callers, but its upstream callers are not enqueued.
func (cg *CallGraph) TraverseCallers(start *model.Symbol, opts TraversalOptions) *TraversalResult {
	visited := make(map[string]bool)
	discovered := make(map[string]bool)
	var callers []*model.Symbol
	var paths []*CallPath

	verbose := os.Getenv("IMPACT_VERBOSE") != ""

	// Queue element: (symbol, chain if path collection enabled)
	type queueItem struct {
		symbol *model.Symbol
		chain  []*model.Symbol // nil if not collecting paths
	}

	startChain := []*model.Symbol{start}
	if !opts.CollectPaths {
		startChain = nil
	}
	discovered[start.QualifiedName()] = true
	queue := []queueItem{{symbol: start, chain: startChain}}
	depth := 0

	for len(queue) > 0 && (opts.MaxDepth < 0 || depth < opts.MaxDepth) {
		levelSize := len(queue)
		for i := 0; i < levelSize; i++ {
			current := queue[0]
			queue = queue[1:]

			qname := current.symbol.QualifiedName()
			if visited[qname] {
				continue
			}
			visited[qname] = true

			// Check if this node is a stop node
			isStopNode := opts.IsStop != nil && opts.IsStop(current.symbol)

			// If stop node, collect it but don't traverse upward
			if isStopNode {
				if verbose {
					log.Printf("[CallGraph] Hit stop node (collecting): %s\n", current.symbol.QualifiedName())
				}
				callers = append(callers, current.symbol)
				if opts.CollectPaths {
					paths = append(paths, &CallPath{
						From:  start,
						To:    current.symbol,
						Chain: current.chain,
						Depth: len(current.chain) - 1,
					})
				}
				continue
			}

			// Not a stop node - get its callers and enqueue
			directCallers := cg.GetCallers(current.symbol)
			for _, caller := range directCallers {
				callerQName := caller.QualifiedName()
				if discovered[callerQName] {
					continue
				}
				discovered[callerQName] = true

				// Build chain for this caller
				var newChain []*model.Symbol
				if opts.CollectPaths {
					newChain = make([]*model.Symbol, len(current.chain)+1)
					copy(newChain, current.chain)
					newChain[len(newChain)-1] = caller
				}

				// Check if caller itself is a stop node
				if opts.IsStop != nil && opts.IsStop(caller) {
					// This is a stop node - collect it but don't enqueue
					visited[callerQName] = true
					if verbose {
						log.Printf("[CallGraph] Hit stop node (collecting): %s\n", caller.QualifiedName())
					}
					callers = append(callers, caller)
					if opts.CollectPaths {
						paths = append(paths, &CallPath{
							From:  start,
							To:    caller,
							Chain: newChain,
							Depth: len(newChain) - 1,
						})
					}
				} else {
					// Not a stop node - add to result and queue
					callers = append(callers, caller)
					if opts.CollectPaths {
						paths = append(paths, &CallPath{
							From:  start,
							To:    caller,
							Chain: newChain,
							Depth: len(newChain) - 1,
						})
					}
					queue = append(queue, queueItem{symbol: caller, chain: newChain})
				}
			}
		}
		depth++
	}

	return &TraversalResult{
		Callers: callers,
		Paths:   paths,
	}
}

// GetAllCallers returns all transitive callers using BFS
func (cg *CallGraph) GetAllCallers(symbol *model.Symbol, maxDepth int) []*model.Symbol {
	return cg.GetAllCallersWithStop(symbol, maxDepth, nil)
}

// GetAllCallersWithStop returns all transitive callers using BFS with an optional stop function
// BEHAVIOR CHANGE: When stopFn returns true, the node IS NOW COLLECTED (previously it was not).
// This is a correctness fix - entry points are affected symbols and should be reported.
func (cg *CallGraph) GetAllCallersWithStop(symbol *model.Symbol, maxDepth int, stopFn func(*model.Symbol) bool) []*model.Symbol {
	result := cg.TraverseCallers(symbol, TraversalOptions{
		MaxDepth:     maxDepth,
		IsStop:       stopFn,
		CollectPaths: false,
	})
	return result.Callers
}

// Reach returns the total number of reachable callers
func (cg *CallGraph) Reach(symbol *model.Symbol, maxDepth int) int {
	return len(cg.GetAllCallers(symbol, maxDepth))
}

// Finalize sorts and deduplicates all edges to ensure deterministic query results
// Should be called once after all AddEdge calls are done
func (cg *CallGraph) Finalize() {
	// Sort and dedupe edges by qualified name
	for k, callers := range cg.edges {
		cg.edges[k] = dedupeAndSort(callers)
	}
	for kind, edges := range cg.edgesByKind {
		for k, callers := range edges {
			cg.edgesByKind[kind][k] = dedupeAndSort(callers)
		}
	}
	// Sort and dedupe edgesByName
	for k, entries := range cg.edgesByName {
		cg.edgesByName[k] = dedupeAndSortNameEdges(entries)
	}
	for kind, edges := range cg.edgesByNameByKind {
		for k, entries := range edges {
			cg.edgesByNameByKind[kind][k] = dedupeAndSortNameEdges(entries)
		}
	}
	for k, callees := range cg.forwardEdges {
		cg.forwardEdges[k] = dedupeAndSort(callees)
	}
	for kind, edges := range cg.forwardEdgesByKind {
		for k, callees := range edges {
			cg.forwardEdgesByKind[kind][k] = dedupeAndSort(callees)
		}
	}
	for k, symbols := range cg.symbolsByName {
		cg.symbolsByName[k] = dedupeAndSort(symbols)
	}
	cg.finalized = true
}

// dedupeAndSort deduplicates and sorts callers by qualified name
func dedupeAndSort(callers []*model.Symbol) []*model.Symbol {
	if len(callers) == 0 {
		return callers
	}

	seen := make(map[string]bool, len(callers))
	result := make([]*model.Symbol, 0, len(callers))
	for _, c := range callers {
		qname := c.QualifiedName()
		if !seen[qname] {
			seen[qname] = true
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].QualifiedName() < result[j].QualifiedName()
	})
	return result
}

// dedupeAndSortNameEdges deduplicates and sorts bare-name index entries by
// caller qualified name. When the same caller reaches the same bare callee
// name through receivers with differing hints, the hints conflict and the
// merged entry's hint is cleared (ambiguous receiver = no owner evidence).
func dedupeAndSortNameEdges(entries []nameEdge) []nameEdge {
	if len(entries) == 0 {
		return entries
	}

	seen := make(map[string]int, len(entries)) // caller qname -> index in result
	result := make([]nameEdge, 0, len(entries))
	for _, e := range entries {
		qname := e.caller.QualifiedName()
		if idx, ok := seen[qname]; ok {
			// A caller may use both bare and receiver-qualified calls of this
			// name. Only exclude it from method fallback when all are bare.
			result[idx].goBareCall = result[idx].goBareCall && e.goBareCall
			result[idx].goMethodCall = result[idx].goMethodCall && e.goMethodCall
			if result[idx].calleeOwner != e.calleeOwner {
				result[idx].calleeOwner = ""
			}
			continue
		}
		seen[qname] = len(result)
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].caller.QualifiedName() < result[j].caller.QualifiedName()
	})
	return result
}
